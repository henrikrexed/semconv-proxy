package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	SignalsReceived  *prometheus.CounterVec
	SignalsForwarded *prometheus.CounterVec
	SignalsDropped   *prometheus.CounterVec

	DictionaryEntries           prometheus.Gauge
	DictionaryAttributesAdded   prometheus.Counter
	DictionaryAttributesChanged prometheus.Counter
	DictionaryAttributesRemoved prometheus.Counter

	PipelineRingBufferSize prometheus.Gauge
	PipelineLag            prometheus.Gauge
	PipelineDrops          prometheus.Counter
	PipelineProcessingTime *prometheus.HistogramVec

	StoragePersistDuration *prometheus.HistogramVec
	StorageDiskSize        prometheus.Gauge

	APIRequests        *prometheus.CounterVec
	APIRequestDuration *prometheus.HistogramVec

	CardinalityHighAttrs  prometheus.Gauge
	CardinalityBudgetUtil prometheus.Gauge
}

func New(registry *prometheus.Registry) *Metrics {
	factory := promauto.With(registry)
	ns := "semconv_proxy"

	return &Metrics{
		SignalsReceived: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "signals_received_total",
			Help:      "Total OTLP signals received",
		}, []string{"signal_type", "protocol"}),
		SignalsForwarded: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "signals_forwarded_total",
			Help:      "Total OTLP signals forwarded to backend",
		}, []string{"signal_type", "protocol"}),
		SignalsDropped: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "signals_dropped_total",
			Help:      "Total OTLP signals dropped",
		}, []string{"signal_type", "protocol"}),

		DictionaryEntries: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "dictionary_entries",
			Help:      "Current number of dictionary entries",
		}),
		DictionaryAttributesAdded: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "dictionary_attributes_added_total",
			Help:      "Total attributes added to dictionary",
		}),
		DictionaryAttributesChanged: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "dictionary_attributes_changed_total",
			Help:      "Total attributes changed in dictionary",
		}),
		DictionaryAttributesRemoved: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "dictionary_attributes_removed_total",
			Help:      "Total attributes removed from dictionary",
		}),

		PipelineRingBufferSize: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "pipeline_ring_buffer_size",
			Help:      "Current ring buffer size",
		}),
		PipelineLag: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "pipeline_lag",
			Help:      "Analysis pipeline lag",
		}),
		PipelineDrops: factory.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "pipeline_drops_total",
			Help:      "Total dropped analysis tasks",
		}),
		PipelineProcessingTime: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "pipeline_processing_duration_seconds",
			Help:      "Analysis pipeline processing duration",
		}, []string{"stage"}),

		StoragePersistDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "storage_persist_duration_seconds",
			Help:      "Pebble persist duration",
		}, []string{"operation"}),
		StorageDiskSize: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "storage_disk_size_bytes",
			Help:      "Pebble disk size in bytes",
		}),

		APIRequests: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "api_request_total",
			Help:      "Total API requests",
		}, []string{"path", "method", "status"}),
		APIRequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "api_request_duration_seconds",
			Help:      "API request duration",
		}, []string{"path"}),

		CardinalityHighAttrs: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "cardinality_high_attributes",
			Help:      "Number of high-cardinality attributes",
		}),
		CardinalityBudgetUtil: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "cardinality_budget_utilization",
			Help:      "Cardinality budget utilization percentage",
		}),
	}
}
