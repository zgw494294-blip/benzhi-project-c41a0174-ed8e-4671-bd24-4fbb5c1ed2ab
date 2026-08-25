package domain

import (
	"encoding/json"
	"time"
)

type Permit struct {
	PermitID      string          `json:"permitID"`
	FacilityID    string          `json:"facilityID"`
	BatchID       string          `json:"batchID"`
	Revision      int             `json:"revision"`
	RiskLevel     string          `json:"riskLevel"`
	IssuedBy      string          `json:"issuedBy"`
	IssuedAt      time.Time       `json:"issuedAt"`
	ValidUntil    time.Time       `json:"validUntil"`
	ContentHash   string          `json:"contentHash"`
	AuditEventID  string          `json:"auditEventID"`
	FrozenContent json.RawMessage `json:"frozenContent,omitempty"`
}

type FrozenFacility struct {
	FacilityID       string  `json:"facilityID"`
	FacilityType     string  `json:"facilityType"`
	DesignCapacity   float64 `json:"designCapacity"`
	NormalWaterLevel float64 `json:"normalWaterLevel"`
	GateCount        int     `json:"gateCount"`
	PumpCount        int     `json:"pumpCount"`
	EmergencyRoute   string  `json:"emergencyRoute"`
	Version          int     `json:"version"`
}

type FrozenManifest struct {
	Facility FrozenFacility     `json:"facility"`
	Batch    *Batch             `json:"batch"`
	Defects  map[string]*Defect `json:"defects"`
}
