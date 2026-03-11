package coordinator

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"

	"redqueen/internal/ml"
	"redqueen/internal/models"
	"redqueen/internal/notify"
	"redqueen/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCoordinator_Process_Threat(t *testing.T) {
	logger := zap.NewNop()
	
	// Prepare a temp file
	tmpFile, err := os.CreateTemp("", "threat*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	analyzer := &ml.MockAnalyzer{
		AnalyzeFunc: func(ctx context.Context, event *models.Event) (*ml.Result, error) {
			return &ml.Result{IsThreat: true, Confidence: 0.9}, nil
		},
	}
	store := &storage.MockProvider{}
	notifier := &notify.MockNotifier{}

	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{notifier}, CoordinatorConfig{
		RetainFiles: false,
		Concurrency: 1,
	})

	ctx := context.Background()
	c.Process(ctx, tmpFile.Name(), "1.2.3.4", "Main Gate")

	// File should be deleted
	_, err = os.Stat(tmpFile.Name())
	assert.True(t, os.IsNotExist(err), "File should be deleted after processing")

	// Storage should have one entry
	assert.Len(t, store.SavedEvents, 1)
	assert.Equal(t, "1.2.3.4", store.SavedEvents[0].CameraIP)

	// Notifier should have one entry
	assert.Len(t, notifier.SentAlerts, 1)
	assert.True(t, notifier.SentAlerts[0].Result.IsThreat)
	assert.Equal(t, "http://mock-storage.local/"+tmpFile.Name(), notifier.SentAlerts[0].ArtifactURL)
}

func TestCoordinator_Process_NoThreat(t *testing.T) {
	logger := zap.NewNop()
	tmpFile, err := os.CreateTemp("", "safe*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	analyzer := &ml.MockAnalyzer{} // Default is no threat
	store := &storage.MockProvider{}
	notifier := &notify.MockNotifier{}

	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{notifier}, CoordinatorConfig{
		RetainFiles: false,
		Concurrency: 1,
	})
	c.Process(context.Background(), tmpFile.Name(), "1.1.1.1", "Safe Zone")

	assert.Len(t, store.SavedEvents, 0)
	assert.Len(t, notifier.SentAlerts, 0)
}

func TestCoordinator_Process_SoftFailureRetry(t *testing.T) {
	logger := zap.NewNop()
	tmpFile, err := os.CreateTemp("", "retry*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	var calls atomic.Int32
	analyzer := &ml.MockAnalyzer{
		AnalyzeFunc: func(ctx context.Context, event *models.Event) (*ml.Result, error) {
			count := calls.Add(1)
			if count < 3 {
				return nil, ml.NewAnalysisError(ml.ErrorSoft, errors.New("temporary issue"))
			}
			return &ml.Result{IsThreat: true, Confidence: 0.8}, nil
		},
	}
	
	store := &storage.MockProvider{}
	notifier := &notify.MockNotifier{}

	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{notifier}, CoordinatorConfig{
		RetainFiles: false,
		Concurrency: 1,
	})
	c.Process(context.Background(), tmpFile.Name(), "1.1.1.1", "Retry Zone")

	assert.Equal(t, int32(3), calls.Load(), "Should have retried 3 times")
	assert.Len(t, store.SavedEvents, 1)
	assert.Len(t, notifier.SentAlerts, 1)
}

func TestCoordinator_Process_RetainFiles(t *testing.T) {
	logger := zap.NewNop()
	tmpFile, err := os.CreateTemp("", "retain*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	analyzer := &ml.MockAnalyzer{}
	store := &storage.MockProvider{}
	notifier := &notify.MockNotifier{}

	// Create coordinator with retainFiles = true
	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{notifier}, CoordinatorConfig{
		RetainFiles: true,
		Concurrency: 1,
	})
	c.Process(context.Background(), tmpFile.Name(), "1.1.1.1", "Retain Zone")

	// File should NOT be deleted
	_, err = os.Stat(tmpFile.Name())
	assert.NoError(t, err, "File should still exist after processing when retain_files is true")
}
