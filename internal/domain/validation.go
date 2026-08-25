package domain

import "time"

type ValidationIssue struct {
	Path       string `json:"path"`
	Metric     string `json:"metric,omitempty"`
	Code       string `json:"code"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion"`
}

type ValidationReport struct {
	GeneratedAt        time.Time         `json:"generatedAt"`
	Valid              bool              `json:"valid"`
	Total              int               `json:"total"`
	Passed             int               `json:"passed"`
	Completeness       int               `json:"completeness"`
	EvidenceReferences int               `json:"evidenceReferences"`
	Issues             []ValidationIssue `json:"issues,omitempty"`
}

type ValidationError struct{ Report ValidationReport }

func (e *ValidationError) Error() string { return "现场读数校验未通过" }
