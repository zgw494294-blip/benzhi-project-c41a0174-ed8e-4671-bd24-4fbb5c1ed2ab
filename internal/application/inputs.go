package application

import (
	"cityflood/internal/domain"
	"time"
)

type FacilityInput struct {
	FacilityID       string  `json:"facilityID"`
	Name             string  `json:"name"`
	District         string  `json:"district"`
	FacilityType     string  `json:"facilityType"`
	DesignCapacity   float64 `json:"designCapacity"`
	NormalWaterLevel float64 `json:"normalWaterLevel"`
	GateCount        int     `json:"gateCount"`
	PumpCount        int     `json:"pumpCount"`
	EmergencyRoute   string  `json:"emergencyRoute"`
	Actor            string  `json:"actor"`
}

type BatchInput struct {
	BatchID          string    `json:"batchID"`
	InspectionWindow string    `json:"inspectionWindow"`
	InspectorID      string    `json:"inspectorID"`
	WindowStart      time.Time `json:"windowStart"`
	WindowEnd        time.Time `json:"windowEnd"`
}

type ItemsInput struct {
	Items           []domain.CheckItem `json:"items"`
	ExpectedVersion int                `json:"expectedVersion"`
	Actor           string             `json:"actor"`
}

type RectifyInput struct {
	Assignee        string     `json:"assignee"`
	DueAt           *time.Time `json:"dueAt"`
	Measures        string     `json:"measures"`
	Responsible     string     `json:"responsible"`
	EvidenceRefs    []string   `json:"evidenceRefs"`
	ExpectedVersion int        `json:"expectedVersion"`
	Actor           string     `json:"actor"`
}

type ReviewInput struct {
	Pass            bool   `json:"pass"`
	Comment         string `json:"comment"`
	ExpectedVersion int    `json:"expectedVersion"`
	Actor           string `json:"actor"`
}

type FacilityStatusInput struct {
	Status          string    `json:"status"`
	Action          string    `json:"action,omitempty"`
	Reason          string    `json:"reason"`
	Actor           string    `json:"actor"`
	ExpectedVersion int       `json:"expectedVersion"`
	EffectiveAt     time.Time `json:"effectiveAt"`
}

type ItemRevisionInput struct {
	ItemID          string           `json:"itemID"`
	Reason          string           `json:"reason"`
	Item            domain.CheckItem `json:"item"`
	ExpectedVersion int              `json:"expectedVersion"`
	Actor           string           `json:"actor"`
	Metric          string           `json:"metric,omitempty"`
	Value           *float64         `json:"value,omitempty"`
	Unit            string           `json:"unit,omitempty"`
	Observation     string           `json:"observation,omitempty"`
	EvidenceRefs    []string         `json:"evidenceRefs,omitempty"`
	CapturedBy      string           `json:"capturedBy,omitempty"`
	CapturedAt      time.Time        `json:"capturedAt,omitempty"`
}

type DefectAssignment struct {
	DefectID        string     `json:"defectID"`
	MaintenanceTeam string     `json:"maintenanceTeam"`
	Responsible     string     `json:"responsible"`
	Assignee        string     `json:"assignee,omitempty"`
	DueAt           *time.Time `json:"dueAt"`
}

type BatchAssignmentInput struct {
	Assignments     []DefectAssignment  `json:"assignments"`
	Adjustments     []DueDateAdjustment `json:"adjustments,omitempty"`
	Action          string              `json:"action,omitempty"`
	ExpectedVersion int                 `json:"expectedVersion"`
	Actor           string              `json:"actor"`
}

type WithdrawInput struct {
	Revision        int    `json:"revision"`
	Reason          string `json:"reason"`
	Actor           string `json:"actor"`
	ExpectedVersion int    `json:"expectedVersion"`
}

type ReviewDecision struct {
	DefectID string `json:"defectID"`
	Pass     bool   `json:"pass"`
	Comment  string `json:"comment"`
	Revision int    `json:"revision"`
}

type ReviewDecisionsInput struct {
	Decisions       []ReviewDecision `json:"decisions"`
	ExpectedVersion int              `json:"expectedVersion"`
	Actor           string           `json:"actor"`
}

type DueDateAdjustment struct {
	DefectID    string     `json:"defectID"`
	DueAt       *time.Time `json:"dueAt"`
	Reason      string     `json:"reason"`
	Responsible string     `json:"responsible"`
}

type DueDateAdjustmentsInput struct {
	Adjustments     []DueDateAdjustment `json:"adjustments"`
	ExpectedVersion int                 `json:"expectedVersion"`
	Actor           string              `json:"actor"`
}
