package commitrollback

import (
	"cityflood/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

func TestCommitEventRollsBackMemoryWhenPersistenceFails(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "events.jsonl"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitEvent("facility.created", "f", "actor", map[string]any{}); err == nil {
		t.Fatal("expected persistence failure")
	}
	if got := len(store.Snapshot().Events); got != 0 {
		t.Fatalf("failed commit left %d event(s) in memory", got)
	}
	if err := os.Remove(filepath.Join(dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	eventID, err := store.CommitEvent("facility.created", "f", "actor", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if eventID != "evt-1" || store.Snapshot().Events[0].Sequence != 1 || store.Snapshot().Events[0].PrevHash != "" {
		t.Fatalf("retry did not restart the event chain: id=%s events=%+v", eventID, store.Snapshot().Events)
	}
}
