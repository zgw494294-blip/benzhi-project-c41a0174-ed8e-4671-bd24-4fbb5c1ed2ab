package idempotencycachescope_test

import (
	"bytes"
	"cityflood/internal/application"
	"cityflood/internal/httpapi"
	"cityflood/internal/storage"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdempotencyResultCacheIsScopedToKeyAndAggregate(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(store)
	for _, id := range []string{"f-1", "f-2"} {
		_, err := service.CreateFacility(application.FacilityInput{
			FacilityID:       id,
			Name:             "调蓄设施" + id,
			DesignCapacity:   100,
			NormalWaterLevel: 10,
			GateCount:        2,
			PumpCount:        2,
		}, "")
		if err != nil {
			t.Fatal(err)
		}
	}
	handler := httpapi.New(service).Handler()
	body := []byte(`{"status":"suspended","reason":"汛前检修","actor":"operator","expectedVersion":1}`)

	requestStatusChange := func(facilityID, idempotencyKey string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/facilities/"+facilityID+"/status", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status change for %s returned %d: %s", facilityID, res.Code, res.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		data, ok := response["data"].(map[string]any)
		if !ok {
			t.Fatalf("unexpected response data: %T", response["data"])
		}
		return data
	}

	first := requestStatusChange("f-1", "key-for-f-1")
	second := requestStatusChange("f-2", "key-for-f-2")
	if first["facilityID"] != "f-1" || second["facilityID"] != "f-2" {
		t.Fatalf("cached result crossed request scopes: first=%v second=%v", first["facilityID"], second["facilityID"])
	}
	facility, err := service.GetFacility("f-2")
	if err != nil {
		t.Fatal(err)
	}
	if facility.Status != "suspended" {
		t.Fatalf("second aggregate was not updated: status=%s", facility.Status)
	}
}
