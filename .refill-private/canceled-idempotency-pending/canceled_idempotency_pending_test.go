package canceled_idempotency_pending_test

import (
	"bytes"
	"cityflood/internal/application"
	"cityflood/internal/httpapi"
	"cityflood/internal/storage"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanceledIdempotentRequestDoesNotPoisonRetry(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(application.New(store)).Handler()
	body := []byte(`{"facilityID":"cancel-facility","name":"取消重试调蓄池","designCapacity":100,"normalWaterLevel":10,"gateCount":1,"pumpCount":1,"actor":"operator"}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/facilities", bytes.NewReader(body)).WithContext(ctx)
	firstRequest.Header.Set("Idempotency-Key", "cancel-retry-key")
	handler.ServeHTTP(first, firstRequest)
	if first.Code != http.StatusBadRequest {
		t.Fatalf("canceled request status = %d, want %d", first.Code, http.StatusBadRequest)
	}

	retry := httptest.NewRecorder()
	retryRequest := httptest.NewRequest(http.MethodPost, "/api/facilities", bytes.NewReader(body))
	retryRequest.Header.Set("Idempotency-Key", "cancel-retry-key")
	handler.ServeHTTP(retry, retryRequest)
	if retry.Code != http.StatusCreated {
		t.Fatalf("retry status = %d, want %d", retry.Code, http.StatusCreated)
	}

	lookup := httptest.NewRecorder()
	handler.ServeHTTP(lookup, httptest.NewRequest(http.MethodGet, "/api/facilities/cancel-facility", nil))
	if lookup.Code != http.StatusOK {
		t.Fatalf("facility lookup status = %d, want %d", lookup.Code, http.StatusOK)
	}
}
