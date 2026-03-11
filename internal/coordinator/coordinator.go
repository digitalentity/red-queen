package coordinator

import (
	"context"
	"os"
	"time"

	"redqueen/internal/ml"
	"redqueen/internal/models"
	"redqueen/internal/metrics"
	"redqueen/internal/notify"
	"redqueen/internal/storage"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CoordinatorConfig holds the configuration for the Coordinator.
type CoordinatorConfig struct {
	RetainFiles    bool
	Concurrency    int
	ProcessTimeout time.Duration
}

// Coordinator orchestrates the lifecycle of a surveillance event.
type Coordinator struct {
	logger    *zap.Logger
	analyzer  ml.Analyzer
	storage   storage.Provider
	notifiers []notify.Notifier
	config    CoordinatorConfig
	sem       chan struct{}
}

// NewCoordinator creates a new instance of the Coordinator.
func NewCoordinator(
	logger *zap.Logger,
	analyzer ml.Analyzer,
	storage storage.Provider,
	notifiers []notify.Notifier,
	cfg CoordinatorConfig,
) *Coordinator {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.ProcessTimeout <= 0 {
		cfg.ProcessTimeout = 5 * time.Minute
	}
	return &Coordinator{
		logger:    logger,
		analyzer:  analyzer,
		storage:   storage,
		notifiers: notifiers,
		config:    cfg,
		sem:       make(chan struct{}, cfg.Concurrency),
	}
}

// Process handles a new file upload by creating an event and running the analysis pipeline.
func (c *Coordinator) Process(ctx context.Context, filePath, ip, zone string) {
	// Acquire semaphore
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		c.logger.Warn("Context cancelled before process started", zap.String("path", filePath))
		os.Remove(filePath)
		return
	}

	startTime := time.Now()
	event := &models.Event{
		ID:        uuid.New().String(),
		FilePath:  filePath,
		Timestamp: time.Now(),
		CameraIP:  ip,
		Zone:      zone,
	}

	log := c.logger.With(
		zap.String("event_id", event.ID),
		zap.String("zone", event.Zone),
		zap.String("ip", event.CameraIP),
	)

	log.Info("Starting event processing")

	processCtx, cancel := context.WithTimeout(ctx, c.config.ProcessTimeout)
	defer cancel()

	// Ensure cleanup at the end
	defer func() {
		if c.config.RetainFiles {
			log.Debug("Retention enabled, keeping ephemeral file", zap.String("path", event.FilePath))
			return
		}

		if err := os.Remove(event.FilePath); err != nil && !os.IsNotExist(err) {
			log.Error("Failed to cleanup ephemeral file", zap.Error(err), zap.String("path", event.FilePath))
		} else {
			log.Debug("Ephemeral file cleaned up")
		}
	}()

	// 1. Analyze with Retry Logic
	mlStart := time.Now()
	analyzerName := c.analyzer.Name()
	result, err := c.analyzeWithRetry(processCtx, event, log)
	metrics.MLAnalysisDuration.WithLabelValues(analyzerName, event.Zone).Observe(time.Since(mlStart).Seconds())

	if err != nil {
		log.Error("Analysis failed after retries or encountered hard failure", zap.Error(err))
		metrics.EventsProcessed.WithLabelValues(event.Zone, "error").Inc()
		return
	}

	log.Info("Analysis completed", zap.Bool("is_threat", result.IsThreat), zap.Float64("confidence", result.Confidence))

	// 2. If no threat, we are done
	if !result.IsThreat {
		metrics.EventsProcessed.WithLabelValues(event.Zone, "success").Inc()
		return
	}

	metrics.MLThreatsDetected.WithLabelValues(event.Zone).Inc()

	// 3. Store Artifact
	artifactURL, err := c.storage.Save(processCtx, event)
	storageType := c.storage.Type()
	if err != nil {
		log.Error("Failed to store threat artifact", zap.Error(err), zap.String("storage_type", storageType))
		metrics.StorageOperations.WithLabelValues(storageType, "error").Inc()
		// We continue to notify even if storage fails, though the URL will be empty/invalid
	} else {
		metrics.StorageOperations.WithLabelValues(storageType, "success").Inc()
	}

	// 4. Notify
	for _, n := range c.notifiers {
		notifierType := n.Type()
		if err := n.Send(processCtx, event, result, artifactURL); err != nil {
			log.Error("Notification failed", zap.Error(err), zap.String("notifier_type", notifierType))
			metrics.NotificationsSent.WithLabelValues(notifierType, "error").Inc()
		} else {
			metrics.NotificationsSent.WithLabelValues(notifierType, "success").Inc()
		}
	}

	metrics.EventsProcessed.WithLabelValues(event.Zone, "success").Inc()
	log.Info("Event processing total duration", zap.Duration("duration", time.Since(startTime)))
}

func (c *Coordinator) analyzeWithRetry(ctx context.Context, event *models.Event, log *zap.Logger) (*ml.Result, error) {
	var result *ml.Result

	operation := func() error {
		var err error
		result, err = c.analyzer.Analyze(ctx, event)
		if err == nil {
			return nil
		}

		// Check if it's our custom AnalysisError
		if aErr, ok := err.(*ml.AnalysisError); ok {
			if aErr.Type == ml.ErrorHard {
				return backoff.Permanent(aErr)
			}
			log.Warn("Soft failure in ML analysis, retrying...", zap.Error(err))
			return err
		}

		// Unknown errors (network, etc) should be retried by default
		log.Warn("Unknown error in ML analysis, retrying...", zap.Error(err))
		return err
	}

	// Exponential backoff configuration
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 2 * time.Minute // Stop retrying after 2 minutes

	err := backoff.Retry(operation, backoff.WithContext(b, ctx))
	return result, err
}
