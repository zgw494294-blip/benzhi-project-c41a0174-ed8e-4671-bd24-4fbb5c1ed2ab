package recoverysnapshotcache_test

import (
	"cityflood/internal/application"
	"cityflood/internal/domain"
	"cityflood/internal/storage"
	"errors"
	"testing"
)

func TestRecoveryReloadsSnapshotAfterLaterPersistence(t *testing.T) {
	dir := t.TempDir()
	writer, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(writer)
	createFacility(t, app, "facility-before-cache")

	warm, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = warm.Load(); err != nil {
		t.Fatal(err)
	}
	createFacility(t, app, "facility-after-cache")

	recovered, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = recovered.Load(); err != nil {
		t.Fatal(err)
	}
	recoveredApp := application.New(recovered)
	if _, err = recoveredApp.GetFacility("facility-after-cache"); errors.Is(err, domain.ErrNotFound) {
		t.Fatal("restart recovery reused a stale snapshot and lost the later facility")
	}
	if err != nil {
		t.Fatalf("unexpected recovery error: %v", err)
	}
}

func createFacility(t *testing.T, app *application.Service, id string) {
	t.Helper()
	_, err := app.CreateFacility(application.FacilityInput{
		FacilityID:       id,
		Name:             "缓存恢复测试设施",
		DesignCapacity:   100,
		NormalWaterLevel: 10,
		GateCount:        2,
		PumpCount:        2,
		EmergencyRoute:   "东门",
		Actor:            "operator",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
}
