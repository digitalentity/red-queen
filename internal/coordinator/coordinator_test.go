package coordinator

import (
	"context"
	"errors"
	"os"
	"sync"
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
	savedEvents := store.GetSavedEvents()
	assert.Len(t, savedEvents, 1)
	assert.Equal(t, "1.2.3.4", savedEvents[0].CameraIP)

	// Notifier should have one entry
	sentAlerts := notifier.GetSentAlerts()
	assert.Len(t, sentAlerts, 1)
	assert.True(t, sentAlerts[0].Result.IsThreat)
	assert.Equal(t, "http://mock-storage.local/"+tmpFile.Name(), sentAlerts[0].ArtifactURL)
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

	assert.Len(t, store.GetSavedEvents(), 0)
	assert.Len(t, notifier.GetSentAlerts(), 0)
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
	assert.Len(t, store.GetSavedEvents(), 1)
	assert.Len(t, notifier.GetSentAlerts(), 1)
}

func TestCoordinator_Process_HardFailure(t *testing.T) {
	logger := zap.NewNop()
	tmpFile, err := os.CreateTemp("", "hard*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	var calls atomic.Int32
	analyzer := &ml.MockAnalyzer{
		AnalyzeFunc: func(ctx context.Context, event *models.Event) (*ml.Result, error) {
			calls.Add(1)
			return nil, ml.NewAnalysisError(ml.ErrorHard, errors.New("unsupported format"))
		},
	}
	store := &storage.MockProvider{}
	notifier := &notify.MockNotifier{}

	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{notifier}, CoordinatorConfig{
		RetainFiles: false,
		Concurrency: 1,
	})
	c.Process(context.Background(), tmpFile.Name(), "1.2.3.4", "Gate")

	// Hard failure must not retry.
	assert.Equal(t, int32(1), calls.Load(), "Hard failure should not be retried")
	assert.Len(t, store.GetSavedEvents(), 0)
	assert.Len(t, notifier.GetSentAlerts(), 0)
}

func TestCoordinator_Process_StorageFailure(t *testing.T) {
	logger := zap.NewNop()
	tmpFile, err := os.CreateTemp("", "storagefail*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	analyzer := &ml.MockAnalyzer{
		AnalyzeFunc: func(ctx context.Context, event *models.Event) (*ml.Result, error) {
			return &ml.Result{IsThreat: true, Confidence: 0.9}, nil
		},
	}
	store := &storage.MockProvider{
		SaveFunc: func(ctx context.Context, event *models.Event) (string, error) {
			return "", errors.New("disk full")
		},
	}
	notifier := &notify.MockNotifier{}

	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{notifier}, CoordinatorConfig{
		RetainFiles: false,
		Concurrency: 1,
	})
	c.Process(context.Background(), tmpFile.Name(), "1.2.3.4", "Gate")

	// Notification must still be sent even when storage fails.
	sentAlerts := notifier.GetSentAlerts()
	assert.Len(t, sentAlerts, 1)
	assert.Empty(t, sentAlerts[0].ArtifactURL, "Artifact URL must be empty when storage fails")
}

func TestCoordinator_Process_ConcurrencySemaphore(t *testing.T) {
	const concurrency = 3
	const total = 9
	logger := zap.NewNop()

	// Gate that holds each analysis until released, letting us verify the semaphore.
	gate := make(chan struct{})
	var active atomic.Int32
	var peak atomic.Int32

	analyzer := &ml.MockAnalyzer{
		AnalyzeFunc: func(ctx context.Context, event *models.Event) (*ml.Result, error) {
			cur := active.Add(1)
			// Track peak concurrent active calls.
			for {
				p := peak.Load()
				if cur <= p || peak.CompareAndSwap(p, cur) {
					break
				}
			}
			<-gate
			active.Add(-1)
			return &ml.Result{IsThreat: false}, nil
		},
	}

	c := NewCoordinator(logger, analyzer, &storage.MockProvider{}, []notify.Notifier{&notify.MockNotifier{}}, CoordinatorConfig{
		Concurrency: concurrency,
	})

	// Launch all events concurrently.
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		tmpFile, err := os.CreateTemp("", "concurrent*.txt")
		require.NoError(t, err)
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			c.Process(context.Background(), path, "1.2.3.4", "Zone")
		}(tmpFile.Name())
	}

	// Release all gates and wait for completion.
	for i := 0; i < total; i++ {
		gate <- struct{}{}
	}
	wg.Wait()

	assert.LessOrEqual(t, peak.Load(), int32(concurrency), "Concurrent active analyses must not exceed semaphore limit")
}

func TestCoordinator_Process_AlwaysStore(t *testing.T) {
	logger := zap.NewNop()
	tmpFile, err := os.CreateTemp("", "always_store*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	analyzer := &ml.MockAnalyzer{
		AnalyzeFunc: func(ctx context.Context, event *models.Event) (*ml.Result, error) {
			return &ml.Result{IsThreat: false, Confidence: 0.1}, nil
		},
	}
	store := &storage.MockProvider{}
	notifier := &notify.MockNotifier{}

	// AlwaysStore = true
	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{notifier}, CoordinatorConfig{
		AlwaysStore: true,
		Concurrency: 1,
	})
	c.Process(context.Background(), tmpFile.Name(), "1.1.1.1", "Always Store Zone")

	// Even though it's NOT a threat, it should be saved
	assert.Len(t, store.GetSavedEvents(), 1)
	// Default notification condition is "on_threat", so it should NOT be notified
	assert.Len(t, notifier.GetSentAlerts(), 0)
}

func TestCoordinator_Process_NotifyAlways(t *testing.T) {
	logger := zap.NewNop()
	tmpFile, err := os.CreateTemp("", "notify_always*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	analyzer := &ml.MockAnalyzer{
		AnalyzeFunc: func(ctx context.Context, event *models.Event) (*ml.Result, error) {
			return &ml.Result{IsThreat: false, Confidence: 0.1}, nil
		},
	}
	store := &storage.MockProvider{}
	
	n1 := &notify.MockNotifier{MockCondition: "always"}
	n2 := &notify.MockNotifier{MockCondition: "on_threat"}

	c := NewCoordinator(logger, analyzer, store, []notify.Notifier{n1, n2}, CoordinatorConfig{
		Concurrency: 1,
	})
	c.Process(context.Background(), tmpFile.Name(), "1.1.1.1", "Notify Zone")

	// Not saved by default
	assert.Len(t, store.GetSavedEvents(), 0)
	
	// n1 should have notified (always)
	assert.Len(t, n1.GetSentAlerts(), 1)
	// n2 should NOT have notified (on_threat)
	assert.Len(t, n2.GetSentAlerts(), 0)
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
