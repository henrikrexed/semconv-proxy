package analysis

import (
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
)

type Extractor struct{}

func NewExtractor() *Extractor {
	return &Extractor{}
}

type ExtractedMetric struct {
	Name        string
	Type        string
	Unit        string
	Temporality string
	Attributes  []ExtractedAttr
}

type ExtractedTrace struct {
	SpanName   string
	StatusCode string
	Attributes []ExtractedAttr
	ParentSpan string
}

type ExtractedLog struct {
	Severity   string
	Attributes []ExtractedAttr
}

func (e *Extractor) ExtractMetricAttributes(name, metricType, unit, temporality string, attrKeys map[string]string) []ExtractedAttr {
	var attrs []ExtractedAttr
	for k, v := range attrKeys {
		attrs = append(attrs, ExtractedAttr{
			Name:       k,
			Type:       v,
			SignalType: "metric",
		})
	}
	attrs = append(attrs, ExtractedAttr{
		Name:       name,
		Type:       metricType,
		SignalType: "metric",
	})
	return attrs
}

func (e *Extractor) ExtractTraceAttributes(spanName, statusCode string, attrKeys map[string]string) []ExtractedAttr {
	var attrs []ExtractedAttr
	for k, v := range attrKeys {
		attrs = append(attrs, ExtractedAttr{
			Name:       k,
			Type:       v,
			SignalType: "trace",
		})
	}
	return attrs
}

func (e *Extractor) ExtractLogAttributes(severity string, attrKeys map[string]string) []ExtractedAttr {
	var attrs []ExtractedAttr
	for k, v := range attrKeys {
		attrs = append(attrs, ExtractedAttr{
			Name:       k,
			Type:       v,
			SignalType: "log",
		})
	}
	return attrs
}

func (e *Extractor) ToAnalysisTask(signalType string, attrs []ExtractedAttr) *AnalysisTask {
	return &AnalysisTask{
		SignalType: SignalType(signalType),
		Timestamp:  time.Now(),
		Attributes: attrs,
	}
}

func (e *Extractor) ToDictionaryEntries(task *AnalysisTask) []*dictionary.AttributeEntry {
	now := task.Timestamp
	var entries []*dictionary.AttributeEntry
	for _, attr := range task.Attributes {
		entries = append(entries, &dictionary.AttributeEntry{
			Name:        attr.Name,
			Type:        attr.Type,
			SignalTypes: []dictionary.SignalType{dictionary.SignalType(attr.SignalType)},
			FirstSeen:   now,
			LastSeen:    now,
			Status:      dictionary.StatusActive,
			Cardinality: attr.Cardinality,
		})
	}
	return entries
}
