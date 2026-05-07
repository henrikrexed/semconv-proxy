package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/cardinality"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/export"
)

type Server struct {
	httpServer *http.Server
	dict       *dictionary.Dictionary
	tracker    *cardinality.Tracker
	exporter   *export.WeaverExporter
	logger     *slog.Logger
	ready      bool
}

func NewServer(port int, dict *dictionary.Dictionary, tracker *cardinality.Tracker, exporter *export.WeaverExporter, logger *slog.Logger) *Server {
	s := &Server{
		dict:     dict,
		tracker:  tracker,
		exporter: exporter,
		logger:   logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/dictionary", s.handleDictionary)
	mux.HandleFunc("/api/v1/dictionary/", s.handleDictionaryEntry)
	mux.HandleFunc("/api/v1/cardinality", s.handleCardinality)
	mux.HandleFunc("/api/v1/export", s.handleExport)
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)

	var handler http.Handler = mux
	handler = loggingMiddleware(logger)(handler)
	handler = recoveryMiddleware(logger)(handler)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("api: listen: %w", err)
	}
	s.ready = true
	go func() {
		s.logger.Info("API server listening", "addr", s.httpServer.Addr)
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("API server error", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = s.httpServer.Shutdown(shutdownCtx)
}

func (s *Server) SetReady(ready bool) {
	s.ready = ready
}
