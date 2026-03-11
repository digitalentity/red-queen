package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"redqueen/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Server struct {
	logger          *zap.Logger
	cfg             config.APIConfig
	artifactHandler http.Handler
	server          *http.Server
}

func NewServer(logger *zap.Logger, cfg config.APIConfig, artifactHandler http.Handler) *Server {
	return &Server{
		logger:          logger,
		cfg:             cfg,
		artifactHandler: artifactHandler,
	}
}

func (s *Server) Start() error {
	if !s.cfg.Enabled {
		s.logger.Info("REST API is disabled")
		return nil
	}

	mux := http.NewServeMux()

	// Artifacts endpoint
	if s.artifactHandler != nil {
		mux.Handle("/artifacts/", http.StripPrefix("/artifacts/", s.artifactHandler))
	}

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: mux,
	}

	s.logger.Info("REST API starting", zap.Int("port", s.cfg.Port))

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
