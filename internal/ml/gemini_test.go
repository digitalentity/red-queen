package ml

import (
	"context"
	"os"
	"testing"

	"redqueen/internal/config"
	"redqueen/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGeminiAnalyzer_Analyze_Limit(t *testing.T) {
	logger := zap.NewNop()
	
	// Create a dummy file
	tmpFile, err := os.CreateTemp("", "artifact*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	_, err = tmpFile.Write([]byte("too large content"))
	require.NoError(t, err)
	tmpFile.Close()

	cfg := config.AnalyzerConfig{
		MaxArtifactSize: 5, // 5 bytes limit
	}

	// Note: we pass a nil client because the check happens before it's used
	analyzer := &GeminiAnalyzer{
		logger: logger,
		cfg:    cfg,
		client: nil,
	}

	event := &models.Event{
		FilePath: tmpFile.Name(),
	}

	_, err = analyzer.Analyze(context.Background(), event)
	
	require.Error(t, err)
	aErr, ok := err.(*AnalysisError)
	assert.True(t, ok)
	assert.Equal(t, ErrorHard, aErr.Type)
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")
}
