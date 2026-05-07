package export

import (
	"fmt"
	"strings"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"gopkg.in/yaml.v3"
)

type WeaverExporter struct{}

func NewWeaverExporter() *WeaverExporter {
	return &WeaverExporter{}
}

type WeaverRegistry struct {
	Groups []WeaverGroup `yaml:"groups"`
}

type WeaverGroup struct {
	ID          string            `yaml:"id"`
	Type        string            `yaml:"type,omitempty"`
	MetricName  string            `yaml:"metric_name,omitempty"`
	Brief       string            `yaml:"brief"`
	Instrument  string            `yaml:"instrument,omitempty"`
	Unit        string            `yaml:"unit,omitempty"`
	Attributes  []WeaverAttribute `yaml:"attributes,omitempty"`
	Stability   string            `yaml:"stability"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

type WeaverAttribute struct {
	ID               string `yaml:"id"`
	Type             string `yaml:"type"`
	RequirementLevel string `yaml:"requirement_level"`
}

func (e *WeaverExporter) Export(entries []*dictionary.AttributeEntry) ([]byte, error) {
	registry := WeaverRegistry{Groups: []WeaverGroup{}}
	grouped := make(map[string][]*dictionary.AttributeEntry)

	for _, entry := range entries {
		for _, st := range entry.SignalTypes {
			key := string(st) + ":" + entry.Name
			grouped[key] = append(grouped[key], entry)
		}
	}

	for key, entries := range grouped {
		parts := strings.SplitN(key, ":", 2)
		signalType := parts[0]
		name := parts[1]
		if len(entries) == 0 {
			continue
		}
		entry := entries[0]

		group := WeaverGroup{
			ID:        fmt.Sprintf("%s.%s", signalType, strings.ReplaceAll(name, ".", "_")),
			Brief:     "Auto-discovered attribute",
			Stability: "experimental",
			Annotations: map[string]string{
				"semconv.proxy.status":     string(entry.Status),
				"semconv.proxy.first_seen": entry.FirstSeen.Format("2006-01-02T15:04:05Z"),
				"semconv.proxy.last_seen":  entry.LastSeen.Format("2006-01-02T15:04:05Z"),
			},
		}

		switch signalType {
		case "metric":
			group.Type = "metric_group"
			group.MetricName = name
		case "trace":
			group.Type = "span"
		case "log":
			group.Type = "log_record"
		}

		group.Attributes = append(group.Attributes, WeaverAttribute{
			ID:               name,
			Type:             entry.Type,
			RequirementLevel: "recommended",
		})

		registry.Groups = append(registry.Groups, group)
	}

	data, err := yaml.Marshal(registry)
	if err != nil {
		return nil, fmt.Errorf("export: marshal yaml: %w", err)
	}
	return data, nil
}
