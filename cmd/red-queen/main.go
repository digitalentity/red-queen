package main

import (
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
	
	// Mock implementations for now
	analyzer := &ml.MockAnalyzer{}
	storageProvider := &storage.MockProvider{}
	notifiers := []notify.Notifier{&notify.MockNotifier{}}

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

	// 6. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	logger.Info("Shutting down Red Queen...")

	if err := ftpServer.Stop(); err != nil {
		logger.Error("Error during FTP server shutdown", zap.Error(err))
	}

	logger.Info("Shutdown complete")
}
