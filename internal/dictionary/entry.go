package dictionary

import "time"

type SignalType string

const (
	SignalTypeMetric SignalType = "metric"
	SignalTypeTrace  SignalType = "trace"
	SignalTypeLog    SignalType = "log"
)

type EntryStatus string

const (
	StatusActive  EntryStatus = "active"
	StatusStale   EntryStatus = "stale"
	StatusExpired EntryStatus = "expired"
)

type AttributeEntry struct {
	Name           string       `json:"name" msgpack:"name"`
	Type           string       `json:"type" msgpack:"type"`
	SignalTypes    []SignalType `json:"signal_types" msgpack:"signal_types"`
	FirstSeen      time.Time    `json:"first_seen" msgpack:"first_seen"`
	LastSeen       time.Time    `json:"last_seen" msgpack:"last_seen"`
	Status         EntryStatus  `json:"status" msgpack:"status"`
	Cardinality    int64        `json:"cardinality" msgpack:"cardinality"`
	Classification *string      `json:"classification" msgpack:"classification"`
}
