package storage

import (
	"cityflood/internal/domain"
	"encoding/json"
	"time"
)

type State struct {
	Facilities      map[string]*domain.Facility  `json:"facilities"`
	Batches         map[string]*domain.Batch     `json:"batches"`
	Defects         map[string]*domain.Defect    `json:"defects"`
	Permits         map[string]*domain.Permit    `json:"permits"`
	Events          []domain.AuditEvent          `json:"events"`
	Idempotency     map[string]json.RawMessage   `json:"idempotency"`
	IdempotencyMeta map[string]IdempotencyRecord `json:"idempotencyMeta,omitempty"`
}

type IdempotencyRecord struct {
	Key         string          `json:"key"`
	Command     string          `json:"command"`
	AggregateID string          `json:"aggregateID"`
	RequestHash string          `json:"requestHash"`
	Status      string          `json:"status"`
	HTTPStatus  int             `json:"httpStatus"`
	Result      json.RawMessage `json:"result,omitempty"`
	RequestID   string          `json:"requestID,omitempty"`
	CompletedAt time.Time       `json:"completedAt,omitempty"`
}
