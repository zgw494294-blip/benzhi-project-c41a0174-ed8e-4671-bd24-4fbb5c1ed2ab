package domain

import "time"

type Batch struct {
	BatchID                string               `json:"batchID"`
	FacilityID             string               `json:"facilityID"`
	InspectionWindow       string               `json:"inspectionWindow"`
	WindowStart            time.Time            `json:"windowStart"`
	WindowEnd              time.Time            `json:"windowEnd"`
	InspectorID            string               `json:"inspectorID"`
	Status                 string               `json:"status"`
	RiskLevel              string               `json:"riskLevel"`
	Items                  []CheckItem          `json:"items"`
	ItemHistory            []CheckItem          `json:"itemHistory,omitempty"`
	Assessments            []AssessmentSnapshot `json:"assessments,omitempty"`
	LatestValidationReport *ValidationReport    `json:"latestValidationReport,omitempty"`
	ReviewHistory          []ReviewAttempt      `json:"reviewHistory,omitempty"`
	LatestReviewResult     *ReviewResult        `json:"latestReviewResult,omitempty"`
	DefectIDs              []string             `json:"defectIDs"`
	ExpectedVersion        int                  `json:"expectedVersion"`
	FrozenRevision         int                  `json:"frozenRevision"`
	Version                int                  `json:"version"`
	CreatedAt              time.Time            `json:"createdAt"`
	SubmittedAt            *time.Time           `json:"submittedAt,omitempty"`
	ReviewedAt             *time.Time           `json:"reviewedAt,omitempty"`
	ReviewComment          string               `json:"reviewComment,omitempty"`
}

type AssessmentRule struct {
	Metric    string  `json:"metric"`
	Observed  float64 `json:"observed"`
	Threshold string  `json:"threshold"`
	Expected  float64 `json:"expected"`
	Result    string  `json:"result"`
	Severity  string  `json:"severity,omitempty"`
	Finding   string  `json:"finding,omitempty"`
}

type AssessmentSnapshot struct {
	Sequence            int              `json:"sequence"`
	InputRevision       int              `json:"inputRevision"`
	Rules               []AssessmentRule `json:"rules"`
	RiskLevel           string           `json:"riskLevel"`
	DefectIDs           []string         `json:"defectIDs"`
	AutoClosedDefectIDs []string         `json:"autoClosedDefectIDs,omitempty"`
	AutoClosedRevisions map[string]int   `json:"autoClosedRevisions,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
}
