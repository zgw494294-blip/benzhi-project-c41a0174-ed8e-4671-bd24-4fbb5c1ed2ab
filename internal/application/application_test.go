package application

import (
	"cityflood/internal/domain"
	"cityflood/internal/storage"
	"errors"
	"testing"
	"time"
)

func TestIdempotencyAndVersion(t *testing.T) {
	st, _ := storage.New("")
	a := New(st)
	in := FacilityInput{FacilityID: "f", Name: "池", DesignCapacity: 10, NormalWaterLevel: 1, GateCount: 1, PumpCount: 1}
	f1, err := a.CreateFacility(in, "k")
	if err != nil {
		t.Fatal(err)
	}
	f2, err := a.CreateFacility(in, "k")
	if err != nil || f1.FacilityID != f2.FacilityID {
		t.Fatal("idempotency failed")
	}
	_, err = a.CreateBatch("f", BatchInput{BatchID: "b", InspectionWindow: "2026汛前", InspectorID: "u"}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = a.SubmitItems("b", ItemsInput{ExpectedVersion: 99})
	if err == nil {
		t.Fatal("expected conflict")
	}
}

func newAssessedService(t *testing.T) (*Service, *domain.Batch) {
	t.Helper()
	st, _ := storage.New("")
	a := New(st)
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	_, err := a.CreateFacility(FacilityInput{FacilityID: "f", Name: "调蓄池", DesignCapacity: 100, NormalWaterLevel: 10, GateCount: 2, PumpCount: 2, EmergencyRoute: "东门", Actor: "admin"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.CreateBatch("f", BatchInput{BatchID: "b", InspectionWindow: "2026汛前", InspectorID: "inspector"}, ""); err != nil {
		t.Fatal(err)
	}
	items := []domain.CheckItem{{ItemID: "w", Metric: "water_level", Value: 12, Unit: "m", Observation: "超线", EvidenceRefs: []string{"w"}, CapturedBy: "inspector"}, {ItemID: "g", Metric: "gate", Value: 2, Unit: "扇", Observation: "正常", EvidenceRefs: []string{"g"}, CapturedBy: "inspector"}, {ItemID: "p", Metric: "pump", Value: 1, Unit: "台", Observation: "不足", EvidenceRefs: []string{"p"}, CapturedBy: "inspector"}, {ItemID: "e", Metric: "emergency_route", Value: 1, Unit: "状态", Observation: "畅通", EvidenceRefs: []string{"e"}, CapturedBy: "inspector"}}
	b, _, err := a.SubmitItems("b", ItemsInput{Items: items, ExpectedVersion: 1, Actor: "inspector"})
	if err != nil {
		t.Fatal(err)
	}
	return a, b
}

func TestFacilityStatusAndRiskRevision(t *testing.T) {
	a, b := newAssessedService(t)
	f, err := a.ChangeFacilityStatus("f", FacilityStatusInput{Status: "suspended", Reason: "检修", Actor: "admin", ExpectedVersion: 1, EffectiveAt: a.now()})
	if err != nil || f.Version != 2 {
		t.Fatal(err)
	}
	if _, err = a.CreateBatch("f", BatchInput{BatchID: "blocked", InspectionWindow: "2027汛前", InspectorID: "u"}, ""); !errors.Is(err, domain.ErrState) {
		t.Fatalf("expected state, got %v", err)
	}
	if _, err = a.ChangeFacilityStatus("f", FacilityStatusInput{Status: "active", Reason: "完成", Actor: "admin", ExpectedVersion: 1, EffectiveAt: a.now()}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if _, err = a.ChangeFacilityStatus("f", FacilityStatusInput{Status: "active", Reason: "完成", Actor: "admin", ExpectedVersion: 2, EffectiveAt: a.now()}); err != nil {
		t.Fatal(err)
	}
	next := domain.CheckItem{Metric: "water_level", Value: 9, Unit: "m", Observation: "正常", EvidenceRefs: []string{"w2"}, CapturedBy: "inspector", CapturedAt: a.now()}
	b, _, err = a.ReviseItem("b", ItemRevisionInput{ItemID: "w", Reason: "修正误录", Item: next, ExpectedVersion: b.Version, Actor: "inspector"})
	if err != nil {
		t.Fatal(err)
	}
	if b.RiskLevel != "medium" || len(b.ItemHistory) != 1 || b.Items[0].Revision != 2 || len(b.Assessments) != 2 {
		t.Fatalf("unexpected revision result: %+v", b)
	}
	diff, err := a.GetAssessmentDiff("b", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.RemovedFailures) != 1 || diff.RemovedFailures[0] != "water_level" {
		t.Fatalf("unexpected diff: %+v", diff)
	}
}

func TestRectificationReviewPermitAndIdempotency(t *testing.T) {
	a, b := newAssessedService(t)
	due := a.now().Add(24 * time.Hour)
	b, err := a.AssignDefects("b", BatchAssignmentInput{Actor: "lead", ExpectedVersion: b.Version, Assignments: []DefectAssignment{{DefectID: "b-D1", MaintenanceTeam: "一班", Responsible: "alice", DueAt: &due}, {DefectID: "b-D2", MaintenanceTeam: "二班", Responsible: "bob", DueAt: &due}}})
	if err != nil {
		t.Fatal(err)
	}
	d1, err := a.RectifyDefect("b-D1", RectifyInput{Measures: "降水位", Responsible: "alice", EvidenceRefs: []string{"r1"}, Actor: "alice", ExpectedVersion: b.Version})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = a.GetBatch("b")
	if _, err = a.WithdrawRectification("b-D1", WithdrawInput{Revision: 1, Reason: "照片错误", Actor: "alice", ExpectedVersion: b.Version}); err != nil {
		t.Fatal(err)
	}
	b, _ = a.GetBatch("b")
	d1, err = a.RectifyDefect("b-D1", RectifyInput{Measures: "降水位", Responsible: "alice", EvidenceRefs: []string{"r2"}, Actor: "alice", ExpectedVersion: b.Version})
	if err != nil || len(d1.RectificationRevisions) != 2 {
		t.Fatal(err)
	}
	b, _ = a.GetBatch("b")
	if _, err = a.RectifyDefect("b-D2", RectifyInput{Measures: "修泵", Responsible: "bob", EvidenceRefs: []string{"p1"}, Actor: "bob", ExpectedVersion: b.Version}); err != nil {
		t.Fatal(err)
	}
	b, _ = a.GetBatch("b")
	b, err = a.ReviewDefects("b", ReviewDecisionsInput{Actor: "reviewer", ExpectedVersion: b.Version, Decisions: []ReviewDecision{{DefectID: "b-D1", Pass: true, Revision: 2}, {DefectID: "b-D2", Pass: false, Revision: 1, Comment: "继续整改"}}})
	if err != nil || b.Status != "rectification" {
		t.Fatal(err)
	}
	if _, err = a.RectifyDefect("b-D2", RectifyInput{Measures: "换泵", Responsible: "bob", EvidenceRefs: []string{"p2"}, Actor: "bob", ExpectedVersion: b.Version}); err != nil {
		t.Fatal(err)
	}
	b, _ = a.GetBatch("b")
	b, err = a.ReviewDefects("b", ReviewDecisionsInput{Actor: "reviewer", ExpectedVersion: b.Version, Decisions: []ReviewDecision{{DefectID: "b-D2", Pass: true, Revision: 2}}})
	if err != nil || b.Status != "reviewed" {
		t.Fatal(err)
	}
	p, err := a.IssuePermit("b", "reviewer", b.Version)
	if err != nil {
		t.Fatal(err)
	}
	v, err := a.VerifyPermit(p.PermitID)
	if err != nil || !v.Verified || v.Validity != "valid" {
		t.Fatalf("verification=%+v err=%v", v, err)
	}
	calls := 0
	fn := func() (any, error) { calls++; return map[string]any{"ok": true}, nil }
	if _, _, _, err = a.ExecuteIdempotent("key", "test", "b", "req", map[string]int{"v": 1}, 200, fn); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = a.ExecuteIdempotent("key", "test", "b", "req2", map[string]int{"v": 1}, 200, fn); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("idempotent calls=%d", calls)
	}
}
