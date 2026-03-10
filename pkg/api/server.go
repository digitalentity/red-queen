package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"redqueen/internal/config"
	"go.uber.org/zap"
)

type Server struct {
	logger *zap.Logger
	cfg    config.APIConfig
	storageCfg config.LocalConfig
	server *http.Server
}

func NewServer(logger *zap.Logger, cfg config.APIConfig, storageCfg config.LocalConfig) *Server {
	return &Server{
		logger: logger,
		cfg:    cfg,
		storageCfg: storageCfg,
	}
}

func (s *Server) Start() error {
	if !s.cfg.Enabled {
		s.logger.Info("REST API is disabled")
		return nil
	}

	mux := http.NewServeMux()

	// Artifacts endpoint
	// This serves files from the storage root_path
	// URL pattern: /artifacts/{date}/{zone}/{filename}
	fs := http.FileServer(http.Dir(s.storageCfg.RootPath))
	mux.Handle("/artifacts/", http.StripPrefix("/artifacts/", fs))

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: mux,
	}

	s.logger.Info("REST API starting", zap.Int("port", s.cfg.Port))
	
	// Check if storage directory exists
	if _, err := os.Stat(s.storageCfg.RootPath); os.IsNotExist(err) {
		s.logger.Warn("Storage root path does not exist", zap.String("path", s.storageCfg.RootPath))
	}

	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}
