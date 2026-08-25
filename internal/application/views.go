package application

import (
	"cityflood/internal/domain"
	"time"
)

type BatchView struct {
	*domain.Batch
	Defects          []DefectView               `json:"defects"`
	AssignedCount    int                        `json:"assignedCount"`
	OverdueCount     int                        `json:"overdueCount"`
	PendingReview    int                        `json:"pendingReview"`
	FacilityStatus   string                     `json:"facilityStatus,omitempty"`
	LatestAssessment *domain.AssessmentSnapshot `json:"latestAssessment,omitempty"`
	AssessmentDiff   *AssessmentDiff            `json:"assessmentDiff,omitempty"`
	TeamSummary      []TeamSummary              `json:"teamSummary"`
	OverdueDefectIDs []string                   `json:"overdueDefectIDs"`
	ReworkDefectIDs  []string                   `json:"reworkDefectIDs"`
}

type TeamSummary struct {
	MaintenanceTeam string     `json:"maintenanceTeam"`
	PendingCount    int        `json:"pendingCount"`
	NearestDueAt    *time.Time `json:"nearestDueAt,omitempty"`
}

type DefectView struct {
	*domain.Defect
	Overdue   bool `json:"overdue"`
	Warning   bool `json:"warning"`
	Escalated bool `json:"escalated"`
}

type AssessmentDiff struct {
	From               int      `json:"from"`
	To                 int      `json:"to"`
	RiskFrom           string   `json:"riskFrom"`
	RiskTo             string   `json:"riskTo"`
	Added              []string `json:"added"`
	Removed            []string `json:"removed"`
	Persistent         []string `json:"persistent"`
	AddedFailures      []string `json:"addedFailures"`
	RemovedFailures    []string `json:"removedFailures"`
	PersistentFailures []string `json:"persistentFailures"`
}
