package agent

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

func TestBatchSender_AddAndFlush(t *testing.T) {
	var sentBatches int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&sentBatches, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := sender.NewSender(server.URL, "")
	bs := NewBatchSender(s, 3, 10*time.Second) // max batch size = 3, interval = 10s

	// Add 3 metrics to trigger flush
	metrics1 := []*collector.Metric{
		{Type: collector.Gauge, Name: "Test1", Value: float64(1.0)},
		{Type: collector.Gauge, Name: "Test2", Value: float64(2.0)},
		{Type: collector.Gauge, Name: "Test3", Value: float64(3.0)},
	}
	bs.Add(metrics1)

	// Give time for the flush to complete
	time.Sleep(100 * time.Millisecond)

	bs.FlushAndStop()

	count := atomic.LoadInt64(&sentBatches)
	if count < 1 {
		t.Errorf("expected at least 1 batch sent, got %d", count)
	}
}

func TestBatchSender_TimerFlush(t *testing.T) {
	var sentBatches int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&sentBatches, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := sender.NewSender(server.URL, "")
	// Use a very short interval (50ms)
	bs := NewBatchSender(s, 100, 50*time.Millisecond)

	// Add 1 metric (less than max batch size of 100)
	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "TimerTest", Value: float64(99.9)},
	}
	bs.Add(metrics)

	// Wait for timer to fire (50ms interval + buffer)
	time.Sleep(150 * time.Millisecond)

	bs.FlushAndStop()

	count := atomic.LoadInt64(&sentBatches)
	if count < 1 {
		t.Errorf("expected at least 1 batch sent via timer, got %d", count)
	}
}

func TestBatchSender_EmptyBuffer(t *testing.T) {
	var sentBatches int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&sentBatches, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := sender.NewSender(server.URL, "")
	bs := NewBatchSender(s, 3, 10*time.Second)

	// Don't add any metrics, just flush and stop
	bs.FlushAndStop()

	count := atomic.LoadInt64(&sentBatches)
	// Empty buffer should not trigger a send
	if count != 0 {
		t.Errorf("expected 0 batches sent for empty buffer, got %d", count)
	}
}

func TestWorkerPool(t *testing.T) {
	var totalSent int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&totalSent, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := sender.NewSender(server.URL, "")
	pool := NewWorkerPool(s, 2) // 2 workers

	pool.StartWorkingPool()

	// Send some metrics through the pool
	for i := 0; i < 5; i++ {
		metrics := []*collector.Metric{
			{Type: collector.Gauge, Name: "WorkerTest", Value: float64(i)},
		}
		pool.Ingest(metrics)
	}

	// Give workers time to process
	time.Sleep(200 * time.Millisecond)

	pool.Close()

	count := atomic.LoadInt64(&totalSent)
	if count < 5 {
		t.Errorf("expected at least 5 requests sent, got %d", count)
	}
}

func TestWorkerPool_EmptyBatch(t *testing.T) {
	var sentRequests int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&sentRequests, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := sender.NewSender(server.URL, "")
	pool := NewWorkerPool(s, 2)

	pool.StartWorkingPool()

	// Send empty batch - should not trigger a send
	pool.Ingest([]*collector.Metric{})

	time.Sleep(100 * time.Millisecond)

	pool.Close()

	count := atomic.LoadInt64(&sentRequests)
	if count != 0 {
		t.Errorf("expected 0 requests for empty batch, got %d", count)
	}
}

func TestBatchSender_Stop(t *testing.T) {
	s := sender.NewSender("http://localhost:9999", "") // unreachable URL
	bs := NewBatchSender(s, 100, 50*time.Millisecond)

	// Add some metrics but don't flush
	bs.Add([]*collector.Metric{
		{Type: collector.Gauge, Name: "StopTest", Value: float64(1.0)},
	})

	// Stop without flushing - should not panic
	bs.Stop()
}
