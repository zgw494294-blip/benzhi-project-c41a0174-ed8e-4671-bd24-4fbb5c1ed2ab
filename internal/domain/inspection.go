package domain

import (
	"encoding/json"
	"time"
)

type CheckItem struct {
	ItemID         string    `json:"itemID"`
	BatchID        string    `json:"batchID"`
	Metric         string    `json:"metric"`
	Value          float64   `json:"value"`
	Unit           string    `json:"unit"`
	Observation    string    `json:"observation"`
	EvidenceRefs   []string  `json:"evidenceRefs"`
	CapturedBy     string    `json:"capturedBy"`
	CapturedAt     time.Time `json:"capturedAt"`
	RuleResult     string    `json:"ruleResult"`
	Revision       int       `json:"revision"`
	SupersededBy   int       `json:"supersededBy,omitempty"`
	RevisionReason string    `json:"revisionReason,omitempty"`
	decoded        bool
	valuePresent   bool
}

func (i *CheckItem) UnmarshalJSON(data []byte) error {
	type alias CheckItem
	var value struct {
		*alias
		Value *float64 `json:"value"`
	}
	value.alias = (*alias)(i)
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	i.decoded = true
	if value.Value != nil {
		i.Value = *value.Value
		i.valuePresent = true
	}
	return nil
}
