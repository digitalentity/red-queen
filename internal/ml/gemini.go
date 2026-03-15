package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/models"

	"go.uber.org/zap"
	"google.golang.org/genai"
)

type GeminiAnalyzer struct {
	logger *zap.Logger
	cfg    config.AnalyzerConfig
	client *genai.Client
}

func NewGeminiAnalyzer(ctx context.Context, cfg config.AnalyzerConfig, logger *zap.Logger) (*GeminiAnalyzer, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini-ai provider requires 'api_key' in configuration")
	}

	clientCfg := &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	}

	if cfg.Endpoint != "" {
		clientCfg.HTTPOptions.BaseURL = cfg.Endpoint
	}

	logger.Info("Using Gemini AI with API Key authentication")

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini ai client: %w", err)
	}

	return &GeminiAnalyzer{
		logger: logger,
		cfg:    cfg,
		client: client,
	}, nil
}

type geminiResponse struct {
	IsThreat    bool     `json:"is_threat"`
	Confidence  float64  `json:"confidence"`
	Labels      []string `json:"labels"`
	Description string   `json:"description"`
}

func (a *GeminiAnalyzer) Analyze(ctx context.Context, event *models.Event) (*Result, error) {
	// 1. Check file size before reading to avoid OOM
	fileInfo, err := os.Stat(event.FilePath)
	if err != nil {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("failed to stat file: %w", err))
	}

	maxSize := int64(20 * 1024 * 1024) // 20 MB default safety limit
	if a.cfg.MaxArtifactSize > 0 {
		maxSize = a.cfg.MaxArtifactSize
	}

	if fileInfo.Size() > maxSize {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("file size %d exceeds maximum allowed size %d", fileInfo.Size(), maxSize))
	}

	// 2. Detect MIME Type
	mimeType := a.detectMIMEType(event.FilePath)
	a.logger.Debug("Detected artifact type", zap.String("mime_type", mimeType), zap.String("path", event.FilePath))

	// 3. Read file into memory (Inline Data)
	// NOTE: We intentionally read the entire file into memory here to use the 'InlineData'
	// capability of the Gemini API. This is the most efficient approach for the artifact
	// sizes typical for this system (under 20MB). Memory usage is strictly bounded by
	// MaxArtifactSize and system concurrency limits.
	data, err := os.ReadFile(event.FilePath)
	if err != nil {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("failed to read file: %w", err))
	}

	// 4. Generate Content
	systemInstruction := fmt.Sprintf(`You are a physical security analyst. 
Analyze the provided media for security threats and indicate how confident you are in your findings.
Search specifically for following objects or bejaviors: %s.
You MUST return a valid JSON object with the following structure:
{
  "is_threat": boolean,
  "confidence": float (0.0 to 1.0),
  "labels": [string],
  "description": string
}`, event.Zone, strings.Join(a.cfg.TargetObjects, ", "))

	genCfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{
					Text: systemInstruction,
				},
			},
		},
		ResponseMIMEType: "application/json",
	}

	prompt := fmt.Sprintf("Analyze the artifact recorded in the location '%s' at %s", event.Zone, event.Timestamp.Local().Format(time.RFC850))
	a.logger.Info("Prepared gemini prompt", zap.String("prompt", prompt))

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     data,
					},
				},
				{
					Text: prompt,
				},
			},
		},
	}

	res, err := a.client.Models.GenerateContent(ctx, a.cfg.ModelName, contents, genCfg)
	if err != nil {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("gemini ai generation failed: %w", err))
	}

	if res == nil || len(res.Candidates) == 0 {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("no response candidates from gemini ai"))
	}

	// Extract text from response safely
	text := res.Text()
	if text == "" {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("empty response text from gemini ai"))
	}

	// Extract JSON from response
	var vResp geminiResponse
	if err := json.Unmarshal([]byte(text), &vResp); err != nil {
		a.logger.Error("Failed to parse gemini ai JSON response", zap.Error(err), zap.String("raw", text))
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("invalid JSON from model: %w", err))
	}

	// Adjust threat status based on threshold if model says it's a threat but confidence is low
	isThreat := vResp.IsThreat && vResp.Confidence >= a.cfg.Threshold

	a.logger.Info("Gemini AI analysis complete",
		zap.String("json", text),
		zap.Bool("is_threat", isThreat),
		zap.Float64("confidence", vResp.Confidence),
		zap.Strings("labels", vResp.Labels),
	)

	return &Result{
		IsThreat:   isThreat,
		Confidence: vResp.Confidence,
		Labels:     vResp.Labels,
		DetectedAt: event.Timestamp.Unix(),
	}, nil
}

func (a *GeminiAnalyzer) detectMIMEType(filePath string) string {
	// First, try to detect by sniffing the first 512 bytes
	f, err := os.Open(filePath)
	if err == nil {
		defer f.Close()
		buffer := make([]byte, 512)
		n, err := f.Read(buffer)
		if err == nil && n > 0 {
			contentType := http.DetectContentType(buffer[:n])
			if contentType != "application/octet-stream" {
				return contentType
			}
		}
	}

	// Fallback to extension
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "video/mp4" // Default assumption
	}
}

func (a *GeminiAnalyzer) Name() string {
	return "gemini-ai"
}
