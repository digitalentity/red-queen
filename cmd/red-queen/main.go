package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"redqueen/internal/app"
	"redqueen/internal/config"
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

	// 3. Initialize App
	application, err := app.New(logger, cfg)
	if err != nil {
		logger.Fatal("Failed to initialize application", zap.Error(err))
	}

	// 4. Start App
	if err := application.Start(); err != nil {
		logger.Fatal("Failed to start application", zap.Error(err))
	}

	// 5. Wait for Signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	// 6. Stop App
	if err := application.Stop(); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}
}
