package assessmentalias

import (
	"cityflood/internal/application"
	"cityflood/internal/domain"
	"cityflood/internal/storage"
	"testing"
	"time"
)

func TestAssessmentQueryDoesNotExposeMutableRuleSlices(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store)
	batch := &domain.Batch{BatchID: "b", Assessments: []domain.AssessmentSnapshot{{
		Sequence:  1,
		RiskLevel: "high",
		Rules:     []domain.AssessmentRule{{Metric: "water_level", Result: "fail"}},
		CreatedAt: time.Now(),
	}}}
	store.Snapshot().Batches[batch.BatchID] = batch

	first, err := app.GetAssessment("b", 1)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, ok := first.(domain.AssessmentSnapshot)
	if !ok || len(snapshot.Rules) != 1 {
		t.Fatalf("unexpected assessment result: %#v", first)
	}
	snapshot.Rules[0].Metric = "tampered"
	second, err := app.GetAssessment("b", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := second.(domain.AssessmentSnapshot).Rules[0].Metric; got == "tampered" {
		t.Fatal("assessment query exposed mutable internal rule slice")
	}
}
