package expireddefectwindow

import (
	"cityflood/internal/application"
	"cityflood/internal/domain"
	"cityflood/internal/storage"
	"testing"
	"time"
)

func TestExpiredWindowRejectsDefectAssignmentAndRectification(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store)
	now := time.Now()
	facility, err := domain.NewFacility("f", "调蓄池", "城区", "lake", 100, 10, 2, 1, "clear", now)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := domain.NewBatchWithWindow("b", "f", "2020汛前", "inspector", now.Add(-48*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	batch.Status = "rectification"
	defect := &domain.Defect{DefectID: "b-D1", BatchID: "b", Category: "pump", Severity: "medium", Status: "open", CreatedAt: now, UpdatedAt: now}
	batch.DefectIDs = []string{defect.DefectID}
	state := store.Snapshot()
	state.Facilities[facility.FacilityID] = facility
	state.Batches[batch.BatchID] = batch
	state.Defects[defect.DefectID] = defect

	due := now.Add(24 * time.Hour)
	if _, err := app.AssignDefects("b", application.BatchAssignmentInput{
		Actor: "lead", ExpectedVersion: batch.Version,
		Assignments: []application.DefectAssignment{{DefectID: defect.DefectID, MaintenanceTeam: "一班", Responsible: "alice", DueAt: &due}},
	}); err == nil {
		t.Fatal("expired batch accepted defect assignment")
	}

	if _, err := app.RectifyDefect(defect.DefectID, application.RectifyInput{
		Measures: "修泵", Responsible: "alice", EvidenceRefs: []string{"e1"}, Actor: "alice", ExpectedVersion: batch.Version,
	}); err == nil {
		t.Fatal("expired batch accepted defect rectification")
	}
}
