package domain

import "time"

type DueDateRevision struct {
	DueAt       *time.Time `json:"dueAt"`
	Reason      string     `json:"reason"`
	Responsible string     `json:"responsible"`
	ChangedAt   time.Time  `json:"changedAt"`
}

type DefectEscalation struct {
	At     time.Time `json:"at"`
	To     string    `json:"to"`
	Reason string    `json:"reason"`
}

type RectificationRevision struct {
	Revision         int        `json:"revision"`
	Measures         string     `json:"measures"`
	CompletedAt      time.Time  `json:"completedAt"`
	Responsible      string     `json:"responsible"`
	EvidenceRefs     []string   `json:"evidenceRefs"`
	Result           string     `json:"result"`
	Status           string     `json:"status,omitempty"`
	WithdrawnAt      *time.Time `json:"withdrawnAt,omitempty"`
	WithdrawalReason string     `json:"withdrawalReason,omitempty"`
	ReviewDecision   string     `json:"reviewDecision,omitempty"`
	ReviewComment    string     `json:"reviewComment,omitempty"`
	ReviewedBy       string     `json:"reviewedBy,omitempty"`
	ReviewedAt       *time.Time `json:"reviewedAt,omitempty"`
}

type Defect struct {
	DefectID               string                  `json:"defectID"`
	BatchID                string                  `json:"batchID"`
	Category               string                  `json:"category"`
	Severity               string                  `json:"severity"`
	Finding                string                  `json:"finding"`
	Assignee               string                  `json:"assignee"`
	MaintenanceTeam        string                  `json:"maintenanceTeam,omitempty"`
	Responsible            string                  `json:"responsible,omitempty"`
	DueAt                  *time.Time              `json:"dueAt,omitempty"`
	DueDateHistory         []DueDateRevision       `json:"dueDateHistory,omitempty"`
	EscalatedAt            *time.Time              `json:"escalatedAt,omitempty"`
	EscalatedTo            string                  `json:"escalatedTo,omitempty"`
	EscalationHistory      []DefectEscalation      `json:"escalationHistory,omitempty"`
	AutoClosedAtAssessment int                     `json:"autoClosedAtAssessment,omitempty"`
	AutoClosedByRevision   int                     `json:"autoClosedByRevision,omitempty"`
	RectificationRevisions []RectificationRevision `json:"rectificationRevisions"`
	EvidenceRefs           []string                `json:"evidenceRefs"`
	Status                 string                  `json:"status"`
	ReviewComment          string                  `json:"reviewComment,omitempty"`
	ReviewedRevision       int                     `json:"reviewedRevision,omitempty"`
	ReviewDecision         string                  `json:"reviewDecision,omitempty"`
	CreatedAt              time.Time               `json:"createdAt"`
	UpdatedAt              time.Time               `json:"updatedAt"`
}
