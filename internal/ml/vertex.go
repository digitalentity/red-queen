package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"redqueen/internal/config"
	"redqueen/internal/models"

	"cloud.google.com/go/vertexai/genai"
	"go.uber.org/zap"
)

type VertexAnalyzer struct {
	logger *zap.Logger
	cfg    config.MLConfig
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewVertexAnalyzer(ctx context.Context, logger *zap.Logger, cfg config.MLConfig) (*VertexAnalyzer, error) {
	client, err := genai.NewClient(ctx, cfg.ProjectID, cfg.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to create vertex ai client: %w", err)
	}

	model := client.GenerativeModel(cfg.ModelName)
	
	// Set system instruction to ensure JSON output
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(fmt.Sprintf(`You are a security video analyst. 
Analyze the provided video clip for security threats in the zone: %s.
Search specifically for: %s.
You MUST return a valid JSON object with the following structure:
{
  "is_threat": boolean,
  "confidence": float (0.0 to 1.0),
  "labels": [string],
  "description": string
}`, "unknown", strings.Join(cfg.TargetObjects, ", "))),
		},
	}
	model.ResponseMIMEType = "application/json"

	return &VertexAnalyzer{
		logger: logger,
		cfg:    cfg,
		client: client,
		model:  model,
	}, nil
}

type vertexResponse struct {
	IsThreat    bool     `json:"is_threat"`
	Confidence  float64  `json:"confidence"`
	Labels      []string `json:"labels"`
	Description string   `json:"description"`
}

func (a *VertexAnalyzer) Analyze(ctx context.Context, event *models.Event) (*Result, error) {
	data, err := os.ReadFile(event.FilePath)
	if err != nil {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("failed to read video file: %w", err))
	}

	// Dynamic prompt adjustment based on zone
	prompt := fmt.Sprintf("Analyze this video from zone '%s'. Detect any of these objects: %s.", 
		event.Zone, strings.Join(a.cfg.TargetObjects, ", "))

	res, err := a.model.GenerateContent(ctx,
		genai.Blob{
			MIMEType: "video/mp4", // Assuming mp4 for now, could be dynamic
			Data:     data,
		},
		genai.Text(prompt),
	)
	if err != nil {
		// Differentiate between soft and hard errors based on status codes if possible, 
		// but for now, treat API errors as soft (retryable).
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("vertex ai generation failed: %w", err))
	}

	if len(res.Candidates) == 0 || res.Candidates[0].Content == nil || len(res.Candidates[0].Content.Parts) == 0 {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("empty response from vertex ai"))
	}

	// Extract JSON from response
	var vResp vertexResponse
	part := res.Candidates[0].Content.Parts[0]
	if text, ok := part.(genai.Text); ok {
		if err := json.Unmarshal([]byte(text), &vResp); err != nil {
			a.logger.Error("Failed to parse vertex ai JSON response", zap.Error(err), zap.String("raw", string(text)))
			return nil, NewAnalysisError(ErrorHard, fmt.Errorf("invalid JSON from model: %w", err))
		}
	} else {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("unexpected part type in response"))
	}

	// Adjust threat status based on threshold if model says it's a threat but confidence is low
	isThreat := vResp.IsThreat && vResp.Confidence >= a.cfg.Threshold

	return &Result{
		IsThreat:   isThreat,
		Confidence: vResp.Confidence,
		Labels:     vResp.Labels,
		DetectedAt: event.Timestamp.Unix(),
	}, nil
}

func (a *VertexAnalyzer) Close() {
	if a.client != nil {
		a.client.Close()
	}
}
