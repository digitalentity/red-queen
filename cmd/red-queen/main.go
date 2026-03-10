package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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

func main() {
	// 1. Load Configuration
	cfgPath := os.Getenv("RED_QUEEN_CONFIG")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Initialize Logger
	logger, err := config.InitLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting Red Queen system")

	// 3. Initialize Domain Components
	zoneManager := zone.NewManager(cfg.Zones)
	
	// Initialize Analysis
	var analyzer ml.Analyzer
	switch cfg.ML.Provider {
	case "vertex-ai":
		vAnalyzer, err := ml.NewVertexAnalyzer(context.Background(), logger, cfg.ML)
		if err != nil {
			logger.Fatal("Failed to initialize Vertex AI analyzer", zap.Error(err))
		}
		defer vAnalyzer.Close()
		analyzer = vAnalyzer
		logger.Info("Using Vertex AI analyzer", zap.String("model", cfg.ML.ModelName))
	default:
		logger.Warn("Unknown or no ML provider configured, using mock")
		analyzer = &ml.MockAnalyzer{}
	}

	// Initialize Storage
	var storageProvider storage.Provider
	switch cfg.Storage.Provider {
	case "local":
		storageProvider = storage.NewLocalStorage(cfg.Storage.Local)
		logger.Info("Using local storage", zap.String("root_path", cfg.Storage.Local.RootPath))
	default:
		logger.Warn("Unknown or no storage provider configured, using mock")
		storageProvider = &storage.MockProvider{}
	}

	// Initialize Notifications
	var notifiers []notify.Notifier
	for _, ncfg := range cfg.Notifications {
		if !ncfg.Enabled {
			continue
		}

		switch ncfg.Type {
		case "webhook":
			notifiers = append(notifiers, notify.NewWebhookNotifier(ncfg))
			logger.Info("Enabled webhook notifier", zap.String("url", ncfg.URL))
		default:
			logger.Warn("Unknown notifier type", zap.String("type", ncfg.Type))
		}
	}

	if len(notifiers) == 0 {
		logger.Warn("No notifiers enabled, using mock")
		notifiers = append(notifiers, &notify.MockNotifier{})
	}

	// 4. Initialize Coordinator
	orchestrator := coordinator.NewCoordinator(logger, analyzer, storageProvider, notifiers)

	// 5. Initialize & Start FTP Server
	ftpServer := ftp.NewServer(logger, cfg.FTP, orchestrator, zoneManager)

	// Start server in a goroutine
	go func() {
		if err := ftpServer.Start(); err != nil {
			logger.Fatal("FTP server failed", zap.Error(err))
		}
	}()

	// 6. Initialize & Start REST API
	apiServer := api.NewServer(logger, cfg.API, cfg.Storage.Local)
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Fatal("REST API failed", zap.Error(err))
		}
	}()

	// 7. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	logger.Info("Shutting down Red Queen...")

	if err := ftpServer.Stop(); err != nil {
		logger.Error("Error during FTP server shutdown", zap.Error(err))
	}

	if err := apiServer.Stop(); err != nil {
		logger.Error("Error during API server shutdown", zap.Error(err))
	}

	logger.Info("Shutdown complete")
}
