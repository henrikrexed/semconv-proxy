package receiver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/analysis"
	"github.com/henrikrexed/semconv-proxy/internal/exporter"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
)

type OTLPReceiver struct {
	httpServer *http.Server
	grpcServer *grpc.Server
	forwarder  *exporter.Forwarder
	buffer     *analysis.RingBuffer
	logger     *slog.Logger
	m          *metrics.Metrics
	httpPort   int
	grpcPort   int
}

func New(httpPort, grpcPort int, fwd *exporter.Forwarder, buf *analysis.RingBuffer, logger *slog.Logger) *OTLPReceiver {
	return NewWithMetrics(httpPort, grpcPort, fwd, buf, logger, nil)
}

func NewWithMetrics(httpPort, grpcPort int, fwd *exporter.Forwarder, buf *analysis.RingBuffer, logger *slog.Logger, m *metrics.Metrics) *OTLPReceiver {
	return &OTLPReceiver{
		forwarder: fwd,
		buffer:    buf,
		logger:    logger,
		m:         m,
		httpPort:  httpPort,
		grpcPort:  grpcPort,
	}
}

func (r *OTLPReceiver) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/metrics", r.handleHTTPMetrics)
	mux.HandleFunc("/v1/traces", r.handleHTTPTraces)
	mux.HandleFunc("/v1/logs", r.handleHTTPLogs)

	r.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", r.httpPort),
		Handler: mux,
	}

	ln, err := net.Listen("tcp", r.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("receiver: http listen: %w", err)
	}

	go func() {
		r.logger.Info("OTLP/HTTP receiver listening", "addr", r.httpServer.Addr)
		if serveErr := r.httpServer.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			r.logger.Error("HTTP server error", "error", serveErr)
		}
	}()

	grpcLn, err := net.Listen("tcp", fmt.Sprintf(":%d", r.grpcPort))
	if err != nil {
		return fmt.Errorf("receiver: grpc listen: %w", err)
	}
	r.grpcServer = grpc.NewServer()

	pmetricotlp.RegisterGRPCServer(r.grpcServer, &metricGRPCServer{r: r})
	ptraceotlp.RegisterGRPCServer(r.grpcServer, &traceGRPCServer{r: r})
	plogotlp.RegisterGRPCServer(r.grpcServer, &logGRPCServer{r: r})

	go func() {
		r.logger.Info("OTLP/gRPC receiver listening", "addr", grpcLn.Addr())
		if err := r.grpcServer.Serve(grpcLn); err != nil {
			r.logger.Error("gRPC server error", "error", err)
		}
	}()

	return nil
}

func (r *OTLPReceiver) Stop(ctx context.Context) error {
	if r.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = r.httpServer.Shutdown(shutdownCtx)
	}
	if r.grpcServer != nil {
		r.grpcServer.GracefulStop()
	}
	return nil
}

func (r *OTLPReceiver) handleHTTPMetrics(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.m != nil {
		r.m.SignalsReceived.WithLabelValues("metric", "http").Inc()
	}

	if r.forwarder != nil {
		if fwdErr := r.forwarder.ForwardMetrics(req.Context(), body); fwdErr != nil {
			r.logger.Error("failed to forward metrics", "error", fwdErr)
		}
	}

	r.enqueueAnalysis("metric", body)

	respBytes, _ := pmetricotlp.NewExportResponse().MarshalProto()
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

func (r *OTLPReceiver) handleHTTPTraces(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.m != nil {
		r.m.SignalsReceived.WithLabelValues("trace", "http").Inc()
	}

	if r.forwarder != nil {
		if fwdErr := r.forwarder.ForwardTraces(req.Context(), body); fwdErr != nil {
			r.logger.Error("failed to forward traces", "error", fwdErr)
		}
	}

	r.enqueueAnalysis("trace", body)

	respBytes, _ := ptraceotlp.NewExportResponse().MarshalProto()
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

func (r *OTLPReceiver) handleHTTPLogs(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.m != nil {
		r.m.SignalsReceived.WithLabelValues("log", "http").Inc()
	}

	if r.forwarder != nil {
		if fwdErr := r.forwarder.ForwardLogs(req.Context(), body); fwdErr != nil {
			r.logger.Error("failed to forward logs", "error", fwdErr)
		}
	}

	r.enqueueAnalysis("log", body)

	respBytes, _ := plogotlp.NewExportResponse().MarshalProto()
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

func (r *OTLPReceiver) enqueueAnalysis(signalType string, data []byte) {
	if r.buffer != nil {
		r.buffer.Write(&analysis.AnalysisTask{
			SignalType: analysis.SignalType(signalType),
			Timestamp:  time.Now(),
			Data:       data,
		})
	}
}

type metricGRPCServer struct {
	pmetricotlp.UnimplementedGRPCServer
	r *OTLPReceiver
}

func (s *metricGRPCServer) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	protoBytes, err := req.MarshalProto()
	if err == nil {
		if s.r.m != nil {
			s.r.m.SignalsReceived.WithLabelValues("metric", "grpc").Inc()
		}
		if s.r.forwarder != nil {
			if fwdErr := s.r.forwarder.ForwardMetrics(ctx, protoBytes); fwdErr != nil {
				s.r.logger.Error("gRPC: failed to forward metrics", "error", fwdErr)
			}
		}
		s.r.enqueueAnalysis("metric", protoBytes)
	}
	return pmetricotlp.NewExportResponse(), nil
}

type traceGRPCServer struct {
	ptraceotlp.UnimplementedGRPCServer
	r *OTLPReceiver
}

func (s *traceGRPCServer) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	protoBytes, err := req.MarshalProto()
	if err == nil {
		if s.r.m != nil {
			s.r.m.SignalsReceived.WithLabelValues("trace", "grpc").Inc()
		}
		if s.r.forwarder != nil {
			if fwdErr := s.r.forwarder.ForwardTraces(ctx, protoBytes); fwdErr != nil {
				s.r.logger.Error("gRPC: failed to forward traces", "error", fwdErr)
			}
		}
		s.r.enqueueAnalysis("trace", protoBytes)
	}
	return ptraceotlp.NewExportResponse(), nil
}

type logGRPCServer struct {
	plogotlp.UnimplementedGRPCServer
	r *OTLPReceiver
}

func (s *logGRPCServer) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	protoBytes, err := req.MarshalProto()
	if err == nil {
		if s.r.m != nil {
			s.r.m.SignalsReceived.WithLabelValues("log", "grpc").Inc()
		}
		if s.r.forwarder != nil {
			if fwdErr := s.r.forwarder.ForwardLogs(ctx, protoBytes); fwdErr != nil {
				s.r.logger.Error("gRPC: failed to forward logs", "error", fwdErr)
			}
		}
		s.r.enqueueAnalysis("log", protoBytes)
	}
	return plogotlp.NewExportResponse(), nil
}
