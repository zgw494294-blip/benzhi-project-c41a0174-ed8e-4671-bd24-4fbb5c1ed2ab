package domain

import "time"

type AuditEvent struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	AggregateID   string         `json:"aggregateID"`
	At            time.Time      `json:"at"`
	Actor         string         `json:"actor"`
	Data          map[string]any `json:"data,omitempty"`
	PrevHash      string         `json:"prevHash"`
	Hash          string         `json:"hash"`
	SchemaVersion int            `json:"schemaVersion"`
	Sequence      int64          `json:"sequence"`
}
