package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/coordinator"
	"redqueen/internal/ftp"
	"redqueen/internal/ml"
	"redqueen/internal/notify"
	"redqueen/internal/storage"
	"redqueen/internal/zone"
	"redqueen/pkg/api"

	"go.uber.org/zap"
)

// App represents the entire Red Queen application lifecycle.
type App struct {
	logger      *zap.Logger
	cfg         *config.Config
	httpClient  *http.Client
	ftpServer   *ftp.Server
	apiServer   *api.Server
	coordinator *coordinator.Coordinator
	cancel      context.CancelFunc
}

// New creates and initializes a new App instance.
func New(logger *zap.Logger, cfg *config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 0. Initialize Shared HTTP Client
	httpTimeout := 30 * time.Second
	if cfg.HTTPClient.Timeout > 0 {
		httpTimeout = cfg.HTTPClient.Timeout
	}
	httpClient := &http.Client{
		Timeout: httpTimeout,
	}

	// 1. Initialize Domain Components
	zoneManager := zone.NewManager(cfg.Zones)

	// 2. Initialize Detection Pipeline (Analysis + Optional Prefilter)
	analysis, err := ml.NewAnalyzer(ctx, *cfg.Detection.Analysis, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize analysis stage: %w", err)
	}

	var analyzer ml.Analyzer = analysis
	if cfg.Detection.Prefilter != nil {
		prefilter, err := ml.NewAnalyzer(ctx, *cfg.Detection.Prefilter, logger)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to initialize prefilter stage: %w", err)
		}
		analyzer = ml.NewChainedAnalyzer(prefilter, analysis, logger)
		logger.Info("Multi-stage detection enabled",
			zap.String("prefilter", cfg.Detection.Prefilter.Provider),
			zap.String("analysis", cfg.Detection.Analysis.Provider))
	} else {
		logger.Info("Single-stage detection enabled",
			zap.String("analysis", cfg.Detection.Analysis.Provider))
	}

	// 3. Initialize Storage
	var providers []storage.Provider
	artifactHandler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Artifact serving not available", http.StatusNotFound)
	}))

	for _, pcfg := range cfg.Storage.Providers {
		switch pcfg.Type {
		case "local":
			p := storage.NewLocalStorage(pcfg.Local)
			providers = append(providers, p)
			// First local provider wins for HTTP artifact serving.
			if _, isDefault := artifactHandler.(http.HandlerFunc); isDefault {
				artifactHandler = http.FileServer(http.Dir(pcfg.Local.RootPath))
			}
			logger.Info("Storage: local enabled", zap.String("root_path", pcfg.Local.RootPath))
		case "google_drive":
			p, err := storage.NewGDriveStorage(ctx, pcfg.GoogleDrive)
			if err != nil {
				cancel()
				return nil, fmt.Errorf("failed to init google_drive storage: %w", err)
			}
			providers = append(providers, p)
			logger.Info("Storage: google_drive enabled", zap.String("folder_id", pcfg.GoogleDrive.FolderID))
		default:
			logger.Warn("Unknown storage provider type, skipping", zap.String("type", pcfg.Type))
		}
	}

	var storageProvider storage.Provider
	switch len(providers) {
	case 0:
		logger.Warn("No storage providers configured, using mock")
		storageProvider = &storage.MockProvider{}
	case 1:
		storageProvider = providers[0]
	default:
		storageProvider = storage.NewMultiProvider(providers, logger)
	}

	// 4. Initialize Notifications
	var notifiers []notify.Notifier
	for _, ncfg := range cfg.Notifications {
		if !ncfg.Enabled {
			continue
		}

		switch ncfg.Type {
		case "webhook":
			n := notify.NewWebhookNotifier(ncfg, httpClient)
			notifiers = append(notifiers, n)
			logger.Info("Enabled webhook notifier", zap.String("url", ncfg.URL), zap.String("condition", n.Condition()))
		case "homey":
			n := notify.NewHomeyNotifier(ncfg, httpClient)
			notifiers = append(notifiers, n)
			logger.Info("Enabled Homey notifier", zap.String("homey_id", ncfg.HomeyID), zap.String("event", ncfg.Event), zap.String("condition", n.Condition()))
		case "telegram":
			n := notify.NewTelegramNotifier(ncfg, httpClient)
			notifiers = append(notifiers, n)
			logger.Info("Enabled Telegram notifier", zap.Int64("chat_id", ncfg.ChatID), zap.String("condition", n.Condition()))
		default:
			logger.Warn("Unknown notifier type", zap.String("type", ncfg.Type))
		}
	}

	if len(notifiers) == 0 {
		logger.Warn("No notifiers enabled, using mock")
		notifiers = append(notifiers, &notify.MockNotifier{})
	}

	// 5. Initialize Coordinator
	processTimeout := 5 * time.Minute
	if cfg.ProcessTimeout > 0 {
		processTimeout = cfg.ProcessTimeout
	}

	orchestrator := coordinator.NewCoordinator(logger, analyzer, storageProvider, notifiers, coordinator.CoordinatorConfig{
		RetainFiles:    cfg.FTP.RetainFiles,
		AlwaysStore:    cfg.Storage.AlwaysStore,
		Concurrency:    cfg.Concurrency,
		ProcessTimeout: processTimeout,
	})
	if cfg.Storage.AlwaysStore {
		logger.Info("Storage: always_store enabled (retaining all events)")
	}

	// 6. Initialize Servers
	ftpServer := ftp.NewServer(ctx, logger, cfg.FTP, orchestrator, zoneManager)
	apiServer := api.NewServer(logger, cfg.API, artifactHandler)

	return &App{
		logger:      logger,
		cfg:         cfg,
		httpClient:  httpClient,
		ftpServer:   ftpServer,
		apiServer:   apiServer,
		coordinator: orchestrator,
		cancel:      cancel,
	}, nil
}

// Start starts the application services in background goroutines.
func (a *App) Start() error {
	a.logger.Info("Starting Red Queen system services")

	// Start FTP server
	go func() {
		if err := a.ftpServer.Start(); err != nil {
			a.logger.Fatal("FTP server failed", zap.Error(err))
		}
	}()

	// Start REST API
	go func() {
		if err := a.apiServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Fatal("REST API failed", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully shuts down all application services.
func (a *App) Stop() error {
	a.logger.Info("Shutting down Red Queen...")
	a.cancel()

	var errs []error

	// Stop the FTP server first so no new uploads are accepted.
	if err := a.ftpServer.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("error during FTP server shutdown: %w", err))
	}

	// Wait for all in-flight analysis goroutines to finish before stopping the API.
	a.coordinator.Wait()

	if err := a.apiServer.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("error during API server shutdown: %w", err))
	}

	if len(errs) > 0 {
		for _, err := range errs {
			a.logger.Error("Shutdown error", zap.Error(err))
		}
		return errors.New("errors occurred during shutdown")
	}

	a.logger.Info("Shutdown complete")
	return nil
}
