package auditeventalias

import (
	"cityflood/internal/application"
	"cityflood/internal/storage"
	"testing"
)

func TestAuditEventQueryDoesNotExposeMutableData(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(store)
	if _, err := service.CreateFacility(application.FacilityInput{
		FacilityID:       "facility-1",
		Name:             "调蓄池",
		DesignCapacity:   100,
		NormalWaterLevel: 10,
		GateCount:        2,
		PumpCount:        2,
		EmergencyRoute:   "东门",
		Actor:            "巡检员",
	}, ""); err != nil {
		t.Fatal(err)
	}

	events := service.ListEvents()
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	events[0].Data["summary"] = "被查询方篡改"

	refetched := service.ListEvents()
	if got := refetched[0].Data["summary"]; got != "创建设施档案" {
		t.Fatalf("audit event data was mutated through query result: %v", got)
	}
}
