package concurrentresponseenvelope_test

import (
	"bytes"
	"cityflood/internal/application"
	"cityflood/internal/httpapi"
	"cityflood/internal/storage"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type headerBarrierRecorder struct {
	header  http.Header
	body    bytes.Buffer
	status  int
	entered chan<- struct{}
	release <-chan struct{}
	once    sync.Once
}

func (r *headerBarrierRecorder) Header() http.Header {
	return r.header
}

func (r *headerBarrierRecorder) WriteHeader(status int) {
	r.status = status
	r.once.Do(func() {
		r.entered <- struct{}{}
		<-r.release
	})
}

func (r *headerBarrierRecorder) Write(p []byte) (int, error) {
	return r.body.Write(p)
}

func TestConcurrentResponsesKeepRequestDataIsolated(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(store)
	for _, id := range []string{"f-1", "f-2"} {
		_, err = service.CreateFacility(application.FacilityInput{
			FacilityID:     id,
			Name:           "调蓄设施" + id,
			DesignCapacity: 100,
			Actor:          "operator",
		}, "")
		if err != nil {
			t.Fatal(err)
		}
	}

	api := httpapi.New(service)
	entered := make(chan struct{})
	release := make(chan struct{})
	first := &headerBarrierRecorder{
		header:  make(http.Header),
		entered: entered,
		release: release,
	}
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		api.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/facilities/f-1", nil))
	}()

	<-entered
	second := httptest.NewRecorder()
	api.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/facilities/f-2", nil))
	close(release)
	<-firstDone

	var response struct {
		Data struct {
			FacilityID string `json:"facilityID"`
		} `json:"data"`
	}
	if err := json.Unmarshal(first.body.Bytes(), &response); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if response.Data.FacilityID != "f-1" {
		t.Fatalf("first concurrent response facilityID = %q, want f-1", response.Data.FacilityID)
	}
}
