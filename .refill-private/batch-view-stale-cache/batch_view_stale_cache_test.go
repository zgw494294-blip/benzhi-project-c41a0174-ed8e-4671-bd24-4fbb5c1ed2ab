package batch_view_stale_cache_test

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

type batchResponse struct {
	Data struct {
		Version   int    `json:"version"`
		RiskLevel string `json:"riskLevel"`
	} `json:"data"`
}

func TestBatchViewCacheRefreshesAfterAggregateVersionChange(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(application.New(store)).Handler()

	requestJSON(t, handler, http.MethodPost, "/api/facilities", map[string]any{
		"facilityID": "facility-cache", "name": "缓存核验池", "district": "东区",
		"facilityType": "pond", "designCapacity": 100, "normalWaterLevel": 10,
		"gateCount": 2, "pumpCount": 1, "emergencyRoute": "东门", "actor": "admin",
	}, http.StatusCreated, nil)
	requestJSON(t, handler, http.MethodPost, "/api/facilities/facility-cache/inspection-batches", map[string]any{
		"batchID": "batch-cache", "inspectionWindow": "2026汛前",
		"windowStart": "2026-01-01T00:00:00+08:00", "windowEnd": "2026-12-31T00:00:00+08:00",
		"inspectorID": "inspector",
	}, http.StatusCreated, nil)

	var before batchResponse
	requestJSON(t, handler, http.MethodGet, "/api/inspection-batches/batch-cache", nil, http.StatusOK, &before)
	requestJSON(t, handler, http.MethodPost, "/api/inspection-batches/batch-cache/items", map[string]any{
		"actor": "inspector",
		"items": []map[string]any{
			{"itemID": "water", "metric": "water_level", "value": 8, "unit": "m", "observation": "正常", "evidenceRefs": []string{"water-photo"}, "capturedBy": "inspector"},
			{"itemID": "gate", "metric": "gate", "value": 2, "unit": "扇", "observation": "正常", "evidenceRefs": []string{"gate-photo"}, "capturedBy": "inspector"},
			{"itemID": "pump", "metric": "pump", "value": 1, "unit": "台", "observation": "正常", "evidenceRefs": []string{"pump-photo"}, "capturedBy": "inspector"},
			{"itemID": "route", "metric": "emergency_route", "value": 1, "unit": "状态", "observation": "畅通", "evidenceRefs": []string{"route-photo"}, "capturedBy": "inspector"},
		},
	}, http.StatusOK, nil)

	var after batchResponse
	requestJSON(t, handler, http.MethodGet, "/api/inspection-batches/batch-cache", nil, http.StatusOK, &after)
	if after.Data.Version <= before.Data.Version || after.Data.RiskLevel != "low" {
		t.Fatalf("批次写入后查询仍返回旧缓存: beforeVersion=%d afterVersion=%d afterRisk=%q", before.Data.Version, after.Data.Version, after.Data.RiskLevel)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int, out any) {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s: status=%d body=%s", method, path, recorder.Code, recorder.Body.String())
	}
	if out != nil {
		if err := json.NewDecoder(recorder.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
}
