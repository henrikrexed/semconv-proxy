package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/henrikrexed/semconv-proxy/internal/analysis"
	"github.com/henrikrexed/semconv-proxy/internal/api"
	"github.com/henrikrexed/semconv-proxy/internal/cardinality"
	"github.com/henrikrexed/semconv-proxy/internal/config"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/export"
	"github.com/henrikrexed/semconv-proxy/internal/exporter"
	"github.com/henrikrexed/semconv-proxy/internal/health"
	"github.com/henrikrexed/semconv-proxy/internal/lifecycle"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
	"github.com/henrikrexed/semconv-proxy/internal/receiver"
	"github.com/henrikrexed/semconv-proxy/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfg *config.Config

func main() {
	cmd := &cobra.Command{
		Use:   "semconv-proxy",
		Short: "Collector Semantic Convention Proxy",
		RunE:  run,
	}

	cfg = config.Default()

	cmd.Flags().StringVar(&cfg.ConfigFile, "config", "", "path to YAML config file")
	cmd.Flags().StringVar(&cfg.BackendEndpoint, "backend-endpoint", cfg.BackendEndpoint, "OTLP backend endpoint (host:port)")
	cmd.Flags().IntVar(&cfg.OTLPHTTPPort, "otlp-http-port", cfg.OTLPHTTPPort, "OTLP/HTTP listen port")
	cmd.Flags().IntVar(&cfg.OTLPGRPCPort, "otlp-grpc-port", cfg.OTLPGRPCPort, "OTLP/gRPC listen port")
	cmd.Flags().IntVar(&cfg.APIPort, "api-port", cfg.APIPort, "REST API listen port")
	cmd.Flags().StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level (debug/info/warn/error)")
	cmd.Flags().StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "data directory for Pebble storage")
	cmd.Flags().IntVar(&cfg.RingBufferSize, "ring-buffer-size", cfg.RingBufferSize, "ring buffer capacity")
	cmd.Flags().IntVar(&cfg.WorkerCount, "worker-count", cfg.WorkerCount, "analysis worker count")
	cmd.Flags().IntVar(&cfg.ShardCount, "shard-count", cfg.ShardCount, "dictionary shard count")
	cmd.Flags().IntVar(&cfg.GlobalBudget, "global-budget", cfg.GlobalBudget, "global attribute budget")
	cmd.Flags().IntVar(&cfg.PerAttrCap, "per-attr-cap", cfg.PerAttrCap, "per-attribute cardinality cap")
	cmd.Flags().BoolVar(&cfg.BackendInsecure, "backend-insecure", cfg.BackendInsecure, "use insecure connection for backend")

	_ = viper.BindPFlag("backend-endpoint", cmd.Flags().Lookup("backend-endpoint"))
	_ = viper.BindPFlag("otlp-http-port", cmd.Flags().Lookup("otlp-http-port"))
	_ = viper.BindPFlag("otlp-grpc-port", cmd.Flags().Lookup("otlp-grpc-port"))
	_ = viper.BindPFlag("api-port", cmd.Flags().Lookup("api-port"))
	_ = viper.BindPFlag("log-level", cmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("data-dir", cmd.Flags().Lookup("data-dir"))

	viper.SetEnvPrefix("SEMCONV_PROXY")
	viper.AutomaticEnv()
	_ = viper.BindEnv("backend-endpoint")
	_ = viper.BindEnv("otlp-http-port")
	_ = viper.BindEnv("otlp-grpc-port")
	_ = viper.BindEnv("api-port")
	_ = viper.BindEnv("log-level")
	_ = viper.BindEnv("data-dir")

	cobra.OnInitialize(func() {
		if cfg.ConfigFile != "" {
			viper.SetConfigFile(cfg.ConfigFile)
			if err := viper.ReadInConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: error reading config file: %v\n", err)
			}
		}
	})

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("starting semconv-proxy",
		"backend_endpoint", cfg.BackendEndpoint,
		"http_port", cfg.OTLPHTTPPort,
		"grpc_port", cfg.OTLPGRPCPort,
		"api_port", cfg.APIPort,
	)

	if err := config.EnsureDataDir(cfg.DataDir); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := prometheus.NewRegistry()
	_ = metrics.New(registry)

	healthAgg := health.NewAggregator(logger)
	healthAgg.Register("dictionary")
	healthAgg.Register("receiver")
	healthAgg.Register("api")
	healthAgg.Register("storage")

	dict, err := dictionary.New(&dictionary.Config{
		ShardCount:   cfg.ShardCount,
		GlobalBudget: cfg.GlobalBudget,
		PerAttrCap:   cfg.PerAttrCap,
	}, logger)
	if err != nil {
		return fmt.Errorf("failed to create dictionary: %w", err)
	}

	tracker := cardinality.NewTracker(cfg.GlobalBudget, cfg.PerAttrCap, logger)
	weaverExporter := export.NewWeaverExporter()

	persister, err := storage.NewPersister(
		cfg.DataDir+"/pebble",
		cfg.PersistInterval,
		cfg.PersistBatchSize,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create persister: %w", err)
	}
	persister.Start(ctx)

	entries, err := persister.LoadAll()
	if err != nil {
		slog.Warn("failed to load persisted dictionary, starting fresh", "error", err)
	} else {
		for _, entry := range entries {
			dict.Upsert(entry)
		}
		slog.Info("loaded dictionary from storage", "entries", len(entries))
	}
	healthAgg.Update("dictionary", health.StatusOK)

	ringBuf := analysis.NewRingBuffer(cfg.RingBufferSize)
	workerPool := analysis.NewWorkerPool(cfg.WorkerCount, ringBuf.Channel(), dict)
	workerPool.Start(ctx)

	fwd := exporter.New(cfg.BackendEndpoint, cfg.BackendInsecure, logger)
	recv := receiver.New(cfg.OTLPHTTPPort, cfg.OTLPGRPCPort, fwd, ringBuf, logger)

	apiServer := api.NewServer(cfg.APIPort, dict, tracker, weaverExporter, logger)

	coordinator := lifecycle.NewCoordinator(logger, cfg.ShutdownTimeout)

	coordinator.Add(&lifecycleFunc{
		startFn: func(ctx context.Context) error { return nil },
		stopFn: func(ctx context.Context) error {
			entries := dict.List(nil)
			for _, e := range entries {
				persister.Enqueue(e)
			}
			persister.Stop()
			return nil
		},
	})

	coordinator.Add(&lifecycleFunc{
		startFn: func(ctx context.Context) error {
			err := recv.Start(ctx)
			if err != nil {
				return err
			}
			healthAgg.Update("receiver", health.StatusOK)
			return nil
		},
		stopFn: func(ctx context.Context) error {
			healthAgg.Update("receiver", health.StatusDegraded)
			return recv.Stop(ctx)
		},
	})

	coordinator.Add(&lifecycleFunc{
		startFn: func(ctx context.Context) error { return nil },
		stopFn: func(ctx context.Context) error {
			workerPool.Stop()
			return nil
		},
	})

	coordinator.Add(&lifecycleFunc{
		startFn: func(ctx context.Context) error {
			err := apiServer.Start(ctx)
			if err != nil {
				return err
			}
			healthAgg.Update("api", health.StatusOK)
			return nil
		},
		stopFn: func(ctx context.Context) error {
			healthAgg.Update("api", health.StatusDegraded)
			apiServer.Stop(ctx)
			return nil
		},
	})

	if err := coordinator.StartAll(ctx); err != nil {
		slog.Error("failed to start components", "error", err)
		cancel()
		coordinator.StopAll(context.Background())
		return err
	}

	slog.Info("semconv-proxy started successfully")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		slog.Info("received signal, shutting down", "signal", sig)
	}

	cancel()
	coordinator.StopAll(context.Background())

	slog.Info("semconv-proxy stopped")
	return nil
}

type lifecycleFunc struct {
	startFn func(ctx context.Context) error
	stopFn  func(ctx context.Context) error
}

func (l lifecycleFunc) Start(ctx context.Context) error {
	return l.startFn(ctx)
}

func (l lifecycleFunc) Stop(ctx context.Context) error {
	return l.stopFn(ctx)
}
