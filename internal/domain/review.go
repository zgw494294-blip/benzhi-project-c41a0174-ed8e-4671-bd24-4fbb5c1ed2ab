package domain

import "time"

type ReviewAttempt struct {
	At        time.Time `json:"at"`
	Reviewer  string    `json:"reviewer"`
	Decisions []string  `json:"decisions"`
	Accepted  bool      `json:"accepted"`
	Reason    string    `json:"reason,omitempty"`
}

type ReworkEntry struct {
	DefectID         string   `json:"defectID"`
	Revision         int      `json:"revision"`
	Responsible      string   `json:"responsible"`
	RequiredEvidence []string `json:"requiredEvidence"`
	Reason           string   `json:"reason"`
}

type ReviewResult struct {
	At              time.Time     `json:"at"`
	Accepted        bool          `json:"accepted"`
	ClosedDefectIDs []string      `json:"closedDefectIDs,omitempty"`
	Rework          []ReworkEntry `json:"rework,omitempty"`
}
