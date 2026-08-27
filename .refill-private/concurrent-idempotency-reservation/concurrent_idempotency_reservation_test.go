package concurrent_idempotency_reservation_test

import (
	"cityflood/internal/application"
	"cityflood/internal/storage"
	"sync/atomic"
	"testing"
)

type callResult struct {
	status int
	err    error
}

func TestConcurrentIdempotencyReservesBeforeBusinessExecution(t *testing.T) {
	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(store)

	var executions atomic.Int32
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	results := make(chan callResult, 2)
	command := func() (any, error) {
		if executions.Add(1) == 1 {
			close(firstEntered)
			<-releaseFirst
		}
		return map[string]any{"accepted": true}, nil
	}
	call := func(requestID string) {
		_, status, _, callErr := service.ExecuteIdempotent(
			"same-key",
			"batch.review",
			"batch-1",
			requestID,
			map[string]any{"expectedVersion": 7},
			200,
			command,
		)
		results <- callResult{status: status, err: callErr}
	}

	go call("request-1")
	<-firstEntered
	go call("request-2")

	second := <-results
	close(releaseFirst)
	first := <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("并发幂等调用不应返回错误: first=%v second=%v", first.err, second.err)
	}
	if got := executions.Load(); got != 1 {
		t.Fatalf("同一幂等键在首个命令完成前执行了 %d 次业务回调", got)
	}
}
