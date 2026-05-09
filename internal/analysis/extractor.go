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

type ExtractionResult struct {
	DictionaryAttrs  []ExtractedAttr
	CardinalityAttrs []ExtractedAttr
}

func (e *Extractor) ExtractFromData(signalType string, data []byte) (*ExtractionResult, error) {
	var extractFn func([]byte) ([]ExtractedAttr, error)
	switch signalType {
	case "metric":
		extractFn = e.extractMetrics
	case "trace":
		extractFn = e.extractTraces
	case "log":
		extractFn = e.extractLogs
	default:
		return nil, fmt.Errorf("extractor: unknown signal type %q", signalType)
	}

	allAttrs, err := extractFn(data)
	if err != nil {
		return nil, err
	}

	deduped := dedupByName(allAttrs)

	return &ExtractionResult{
		DictionaryAttrs:  deduped,
		CardinalityAttrs: allAttrs,
	}, nil
}

func (e *Extractor) extractMetrics(data []byte) ([]ExtractedAttr, error) {
	req := pmetricotlp.NewExportRequest()
	if err := req.UnmarshalProto(data); err != nil {
		return nil, fmt.Errorf("extractor: unmarshal metrics: %w", err)
	}

	dedup := make(map[string]ExtractedAttr)
	vals := make(map[attribKey]struct{})
	var allAttrs []ExtractedAttr

	metrics := req.Metrics()

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		collectAttrs(rm.Resource().Attributes(), "metric", dedup, vals, &allAttrs)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			collectAttrs(sm.Scope().Attributes(), "metric", dedup, vals, &allAttrs)

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				metricAttr := ExtractedAttr{
					Name:       m.Name(),
					Type:       metricTypeStr(m.Type()),
					SignalType: "metric",
				}
				dedup[m.Name()] = metricAttr
				vk := attribKey{name: m.Name(), value: ""}
				if _, exists := vals[vk]; !exists {
					vals[vk] = struct{}{}
					allAttrs = append(allAttrs, metricAttr)
				}
				collectMetricDataPointAttrs(m, dedup, vals, &allAttrs)
			}
		}
	}

	return allAttrs, nil
}

func collectMetricDataPointAttrs(m pmetric.Metric, dedup map[string]ExtractedAttr, vals map[attribKey]struct{}, allAttrs *[]ExtractedAttr) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			collectAttrs(dps.At(i).Attributes(), "metric", dedup, vals, allAttrs)
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			collectAttrs(dps.At(i).Attributes(), "metric", dedup, vals, allAttrs)
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			collectAttrs(dps.At(i).Attributes(), "metric", dedup, vals, allAttrs)
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			collectAttrs(dps.At(i).Attributes(), "metric", dedup, vals, allAttrs)
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			collectAttrs(dps.At(i).Attributes(), "metric", dedup, vals, allAttrs)
		}
	}
}

func (e *Extractor) extractTraces(data []byte) ([]ExtractedAttr, error) {
	req := ptraceotlp.NewExportRequest()
	if err := req.UnmarshalProto(data); err != nil {
		return nil, fmt.Errorf("extractor: unmarshal traces: %w", err)
	}

	dedup := make(map[string]ExtractedAttr)
	vals := make(map[attribKey]struct{})
	var allAttrs []ExtractedAttr

	traces := req.Traces()

	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		collectAttrs(rs.Resource().Attributes(), "trace", dedup, vals, &allAttrs)

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			collectAttrs(ss.Scope().Attributes(), "trace", dedup, vals, &allAttrs)

			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				collectAttrs(span.Attributes(), "trace", dedup, vals, &allAttrs)
			}
		}
	}

	return allAttrs, nil
}

func (e *Extractor) extractLogs(data []byte) ([]ExtractedAttr, error) {
	req := plogotlp.NewExportRequest()
	if err := req.UnmarshalProto(data); err != nil {
		return nil, fmt.Errorf("extractor: unmarshal logs: %w", err)
	}

	dedup := make(map[string]ExtractedAttr)
	vals := make(map[attribKey]struct{})
	var allAttrs []ExtractedAttr

	logs := req.Logs()

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		collectAttrs(rl.Resource().Attributes(), "log", dedup, vals, &allAttrs)

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			collectAttrs(sl.Scope().Attributes(), "log", dedup, vals, &allAttrs)

			for k := 0; k < sl.LogRecords().Len(); k++ {
				record := sl.LogRecords().At(k)
				collectAttrs(record.Attributes(), "log", dedup, vals, &allAttrs)
			}
		}
	}

	return allAttrs, nil
}

type attribKey struct {
	name  string
	value string
}

func collectAttrs(m pcommon.Map, signalType string, dedup map[string]ExtractedAttr, vals map[attribKey]struct{}, allAttrs *[]ExtractedAttr) {
	m.Range(func(k string, v pcommon.Value) bool {
		attr := ExtractedAttr{
			Name:       k,
			Type:       valueTypeStr(v.Type()),
			SignalType: signalType,
			Value:      valueToStr(v),
		}
		dedup[k] = attr
		vk := attribKey{name: k, value: attr.Value}
		if _, exists := vals[vk]; !exists {
			vals[vk] = struct{}{}
			*allAttrs = append(*allAttrs, attr)
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

func dedupByName(attrs []ExtractedAttr) []ExtractedAttr {
	seen := make(map[string]struct{}, len(attrs))
	result := make([]ExtractedAttr, 0, len(attrs))
	for _, a := range attrs {
		if _, ok := seen[a.Name]; !ok {
			seen[a.Name] = struct{}{}
			result = append(result, a)
		}
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
