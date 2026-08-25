package domain

import (
	"testing"
	"time"
)

func TestRiskAndPermitLifecycle(t *testing.T) {
	now := time.Now()
	f, err := NewFacility("f", "池", "区", "lake", 100, 10, 2, 1, "clear", now)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewBatch("b", "f", "window", "u", now)
	if err := b.AddItems([]CheckItem{{ItemID: "w", Metric: "water_level", Value: 12, Unit: "m", Observation: "偏高", EvidenceRefs: []string{"photo-w"}, CapturedBy: "u"}, {ItemID: "g", Metric: "gate", Value: 2, Unit: "扇", Observation: "正常", EvidenceRefs: []string{"photo-g"}, CapturedBy: "u"}, {ItemID: "p", Metric: "pump", Value: 1, Unit: "台", Observation: "正常", EvidenceRefs: []string{"photo-p"}, CapturedBy: "u"}, {ItemID: "e", Metric: "emergency_route", Value: 1, Unit: "状态", Observation: "畅通", EvidenceRefs: []string{"photo-e"}, CapturedBy: "u"}}, now); err != nil {
		t.Fatal(err)
	}
	ds := Evaluate(b, f, now)
	if len(ds) != 1 || b.RiskLevel != "high" {
		t.Fatalf("risk=%s defects=%d", b.RiskLevel, len(ds))
	}
	if err := ds[0].Rectify("清障", "班组", []string{"photo-1"}, now); err != nil {
		t.Fatal(err)
	}
	if err := ds[0].Review(true, "", now); err != nil {
		t.Fatal(err)
	}
	b.Status = "reviewed"
	p, err := b.Freeze(map[string]*Defect{ds[0].DefectID: &ds[0]}, "reviewer", now)
	if err != nil || p.ContentHash == "" {
		t.Fatal(err)
	}
}

func TestRequiredItems(t *testing.T) {
	b, _ := NewBatch("b", "f", "w", "u", time.Now())
	err := b.AddItems([]CheckItem{{Metric: "water_level", CapturedBy: "u"}}, time.Now())
	if err == nil {
		t.Fatal("expected missing item error")
	}
}
