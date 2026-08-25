package domain

import "time"

type Facility struct {
	FacilityID       string    `json:"facilityID"`
	Name             string    `json:"name"`
	District         string    `json:"district"`
	FacilityType     string    `json:"facilityType"`
	DesignCapacity   float64   `json:"designCapacity"`
	NormalWaterLevel float64   `json:"normalWaterLevel"`
	GateCount        int       `json:"gateCount"`
	PumpCount        int       `json:"pumpCount"`
	EmergencyRoute   string    `json:"emergencyRoute"`
	Status           string    `json:"status"`
	StatusReason     string    `json:"statusReason,omitempty"`
	StatusChangedAt  time.Time `json:"statusChangedAt,omitempty"`
	Version          int       `json:"version"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
