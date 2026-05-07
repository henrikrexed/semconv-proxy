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
	httpPort   int
	grpcPort   int
}

func New(httpPort, grpcPort int, fwd *exporter.Forwarder, buf *analysis.RingBuffer, logger *slog.Logger) *OTLPReceiver {
	return &OTLPReceiver{
		forwarder: fwd,
		buffer:    buf,
		logger:    logger,
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

	if r.forwarder != nil {
		metrics := pmetricotlp.NewExportRequest()
		if parseErr := metrics.UnmarshalProto(body); parseErr == nil {
			if fwdErr := r.forwarder.ForwardMetrics(req.Context(), metrics.Metrics()); fwdErr != nil {
				r.logger.Error("failed to forward metrics", "error", fwdErr)
			}
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

	if r.forwarder != nil {
		exportReq := ptraceotlp.NewExportRequest()
		if parseErr := exportReq.UnmarshalProto(body); parseErr == nil {
			if fwdErr := r.forwarder.ForwardTraces(req.Context(), exportReq.Traces()); fwdErr != nil {
				r.logger.Error("failed to forward traces", "error", fwdErr)
			}
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

	if r.forwarder != nil {
		exportReq := plogotlp.NewExportRequest()
		if parseErr := exportReq.UnmarshalProto(body); parseErr == nil {
			if fwdErr := r.forwarder.ForwardLogs(req.Context(), exportReq.Logs()); fwdErr != nil {
				r.logger.Error("failed to forward logs", "error", fwdErr)
			}
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
	if s.r.forwarder != nil {
		if err := s.r.forwarder.ForwardMetrics(ctx, req.Metrics()); err != nil {
			s.r.logger.Error("gRPC: failed to forward metrics", "error", err)
		}
	}
	return pmetricotlp.NewExportResponse(), nil
}

type traceGRPCServer struct {
	ptraceotlp.UnimplementedGRPCServer
	r *OTLPReceiver
}

func (s *traceGRPCServer) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	if s.r.forwarder != nil {
		if err := s.r.forwarder.ForwardTraces(ctx, req.Traces()); err != nil {
			s.r.logger.Error("gRPC: failed to forward traces", "error", err)
		}
	}
	return ptraceotlp.NewExportResponse(), nil
}

type logGRPCServer struct {
	plogotlp.UnimplementedGRPCServer
	r *OTLPReceiver
}

func (s *logGRPCServer) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	if s.r.forwarder != nil {
		if err := s.r.forwarder.ForwardLogs(ctx, req.Logs()); err != nil {
			s.r.logger.Error("gRPC: failed to forward logs", "error", err)
		}
	}
	return plogotlp.NewExportResponse(), nil
}
