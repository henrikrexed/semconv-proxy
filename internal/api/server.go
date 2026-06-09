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
	"github.com/henrikrexed/semconv-proxy/internal/health"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
	"github.com/henrikrexed/semconv-proxy/internal/semconv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	httpServer *http.Server
	dict       *dictionary.Dictionary
	tracker    *cardinality.Tracker
	exporter   *export.WeaverExporter
	logger     *slog.Logger
	healthAgg  *health.Aggregator
	m          *metrics.Metrics
	semconv    *semconv.Registry
	ready      bool
}

func NewServer(port int, dict *dictionary.Dictionary, tracker *cardinality.Tracker, exporter *export.WeaverExporter, logger *slog.Logger, healthAgg *health.Aggregator, registry *prometheus.Registry, m *metrics.Metrics) *Server {
	s := &Server{
		dict:      dict,
		tracker:   tracker,
		exporter:  exporter,
		logger:    logger,
		healthAgg: healthAgg,
		m:         m,
	}

	// Load the embedded official semantic-convention snapshot. A failure here is
	// non-fatal: the community endpoints return 503 and the rest of the proxy
	// keeps serving.
	if reg, err := semconv.Load(); err != nil {
		logger.Error("failed to load embedded semconv registry", "error", err)
	} else {
		s.semconv = reg
		logger.Info("loaded semconv registry", "items", reg.Len(), "source", reg.URL())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/dictionary", s.handleDictionary)
	mux.HandleFunc("/api/v1/dictionary/", s.handleDictionaryEntry)
	mux.HandleFunc("/api/v1/cardinality", s.handleCardinality)
	mux.HandleFunc("/api/v1/export", s.handleExport)
	mux.HandleFunc("/api/v1/semconv/community", s.handleCommunitySearch)
	mux.HandleFunc("/api/v1/semconv/community/", s.handleCommunityEntry)
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.Handle("/", uiHandler())

	var handler http.Handler = mux
	handler = loggingMiddleware(logger)(handler)
	handler = metricsMiddleware(s.m)(handler)
	handler = recoveryMiddleware(logger)(handler)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
	return s
}

func (s *Server) SetMetrics(m *metrics.Metrics) {
	s.m = m
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
	s.ready = false
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = s.httpServer.Shutdown(shutdownCtx)
}

func (s *Server) SetReady(ready bool) {
	s.ready = ready
}
