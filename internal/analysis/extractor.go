package analysis

import (
	"fmt"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type Extractor struct{}

func NewExtractor() *Extractor {
	return &Extractor{}
}

type ExtractedAttr struct {
	Name       string
	Type       string
	SignalType string
	Value      string
}

func (e *Extractor) ExtractFromData(signalType string, data []byte) ([]ExtractedAttr, error) {
	switch signalType {
	case "metric":
		return e.extractMetrics(data)
	case "trace":
		return e.extractTraces(data)
	case "log":
		return e.extractLogs(data)
	default:
		return nil, fmt.Errorf("extractor: unknown signal type %q", signalType)
	}
}

func (e *Extractor) extractMetrics(data []byte) ([]ExtractedAttr, error) {
	req := pmetricotlp.NewExportRequest()
	if err := req.UnmarshalProto(data); err != nil {
		return nil, fmt.Errorf("extractor: unmarshal metrics: %w", err)
	}

	seen := make(map[string]ExtractedAttr)
	metrics := req.Metrics()

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		extractAttrs(rm.Resource().Attributes(), "metric", seen)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			extractAttrs(sm.Scope().Attributes(), "metric", seen)

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				seen[m.Name()] = ExtractedAttr{
					Name:       m.Name(),
					Type:       metricTypeStr(m.Type()),
					SignalType: "metric",
				}
				extractMetricDataPointAttrs(m, seen)
			}
		}
	}

	return mapToSlice(seen), nil
}

func extractMetricDataPointAttrs(m pmetric.Metric, seen map[string]ExtractedAttr) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			extractAttrs(dps.At(i).Attributes(), "metric", seen)
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			extractAttrs(dps.At(i).Attributes(), "metric", seen)
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			extractAttrs(dps.At(i).Attributes(), "metric", seen)
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			extractAttrs(dps.At(i).Attributes(), "metric", seen)
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			extractAttrs(dps.At(i).Attributes(), "metric", seen)
		}
	}
}

func (e *Extractor) extractTraces(data []byte) ([]ExtractedAttr, error) {
	req := ptraceotlp.NewExportRequest()
	if err := req.UnmarshalProto(data); err != nil {
		return nil, fmt.Errorf("extractor: unmarshal traces: %w", err)
	}

	seen := make(map[string]ExtractedAttr)
	traces := req.Traces()

	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		extractAttrs(rs.Resource().Attributes(), "trace", seen)

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			extractAttrs(ss.Scope().Attributes(), "trace", seen)

			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				extractAttrs(span.Attributes(), "trace", seen)
			}
		}
	}

	return mapToSlice(seen), nil
}

func (e *Extractor) extractLogs(data []byte) ([]ExtractedAttr, error) {
	req := plogotlp.NewExportRequest()
	if err := req.UnmarshalProto(data); err != nil {
		return nil, fmt.Errorf("extractor: unmarshal logs: %w", err)
	}

	seen := make(map[string]ExtractedAttr)
	logs := req.Logs()

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		extractAttrs(rl.Resource().Attributes(), "log", seen)

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			extractAttrs(sl.Scope().Attributes(), "log", seen)

			for k := 0; k < sl.LogRecords().Len(); k++ {
				record := sl.LogRecords().At(k)
				extractAttrs(record.Attributes(), "log", seen)
			}
		}
	}

	return mapToSlice(seen), nil
}

func extractAttrs(m pcommon.Map, signalType string, seen map[string]ExtractedAttr) {
	m.Range(func(k string, v pcommon.Value) bool {
		seen[k] = ExtractedAttr{
			Name:       k,
			Type:       valueTypeStr(v.Type()),
			SignalType: signalType,
			Value:      valueToStr(v),
		}
		return true
	})
}

func metricTypeStr(t pmetric.MetricType) string {
	switch t {
	case pmetric.MetricTypeGauge:
		return "gauge"
	case pmetric.MetricTypeSum:
		return "counter"
	case pmetric.MetricTypeHistogram:
		return "histogram"
	case pmetric.MetricTypeSummary:
		return "summary"
	case pmetric.MetricTypeExponentialHistogram:
		return "exponential_histogram"
	default:
		return "unknown"
	}
}

func valueTypeStr(t pcommon.ValueType) string {
	switch t {
	case pcommon.ValueTypeStr:
		return "string"
	case pcommon.ValueTypeInt:
		return "int"
	case pcommon.ValueTypeDouble:
		return "double"
	case pcommon.ValueTypeBool:
		return "bool"
	case pcommon.ValueTypeMap:
		return "map"
	case pcommon.ValueTypeSlice:
		return "slice"
	case pcommon.ValueTypeBytes:
		return "bytes"
	default:
		return "unknown"
	}
}

func valueToStr(v pcommon.Value) string {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return fmt.Sprintf("%d", v.Int())
	case pcommon.ValueTypeDouble:
		return fmt.Sprintf("%f", v.Double())
	case pcommon.ValueTypeBool:
		return fmt.Sprintf("%t", v.Bool())
	default:
		return ""
	}
}

func mapToSlice(m map[string]ExtractedAttr) []ExtractedAttr {
	result := make([]ExtractedAttr, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

func ToDictionaryEntries(attrs []ExtractedAttr, ts time.Time) []*dictionary.AttributeEntry {
	entries := make([]*dictionary.AttributeEntry, 0, len(attrs))
	for _, attr := range attrs {
		entries = append(entries, &dictionary.AttributeEntry{
			Name:        attr.Name,
			Type:        attr.Type,
			SignalTypes: []dictionary.SignalType{dictionary.SignalType(attr.SignalType)},
			FirstSeen:   ts,
			LastSeen:    ts,
			Status:      dictionary.StatusActive,
		})
	}
	return entries
}
