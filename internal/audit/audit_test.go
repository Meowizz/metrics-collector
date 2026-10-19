package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSubject_AttachAndNotify(t *testing.T) {
	subject := NewSubject()

	// Test that Notify with no observers does not panic
	subject.Notify(Event{TS: int64(time.Now().Unix())}, context.Background())

	// Create a mock observer that captures events
	var mu sync.Mutex
	var capturedEvents []Event
	done := make(chan struct{})

	mockObserver := &mockObserver{
		updateFn: func(event Event, ctx context.Context) {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
			select {
			case done <- struct{}{}:
			default:
			}
		},
	}

	subject.Attach(mockObserver)

	// Notify and wait for observer to receive
	event := Event{
		TS:      int64(time.Now().Unix()),
		Metrics: []string{"Alloc", "Sys"},
		IP:      "127.0.0.1",
	}
	subject.Notify(event, context.Background())

	// Wait for observer to receive (with timeout)
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for observer to receive event")
	}

	mu.Lock()
	if len(capturedEvents) != 1 {
		t.Errorf("expected 1 event, got %d", len(capturedEvents))
	}
	if len(capturedEvents) > 0 {
		if capturedEvents[0].IP != "127.0.0.1" {
			t.Errorf("expected IP 127.0.0.1, got %s", capturedEvents[0].IP)
		}
		if len(capturedEvents[0].Metrics) != 2 {
			t.Errorf("expected 2 metrics, got %d", len(capturedEvents[0].Metrics))
		}
	}
	mu.Unlock()
}

func TestSubject_MultipleObservers(t *testing.T) {
	subject := NewSubject()

	var mu sync.Mutex
	var count int
	done := make(chan struct{}, 3)

	for i := 0; i < 3; i++ {
		mockObserver := &mockObserver{
			updateFn: func(event Event, ctx context.Context) {
				mu.Lock()
				count++
				mu.Unlock()
				done <- struct{}{}
			},
		}
		subject.Attach(mockObserver)
	}

	event := Event{TS: int64(time.Now().Unix())}
	subject.Notify(event, context.Background())

	// Wait for all observers
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for observer")
		}
	}

	mu.Lock()
	if count != 3 {
		t.Errorf("expected 3 notifications, got %d", count)
	}
	mu.Unlock()
}

func TestFileAuditObserver(t *testing.T) {
	tempFile := t.TempDir() + "/audit.log"

	observer, err := NewFileAuditObserver(tempFile, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create FileAuditObserver: %v", err)
	}
	defer observer.Close()

	event := Event{
		TS:      1234567890,
		Metrics: []string{"Alloc", "Sys"},
		IP:      "192.168.1.1",
	}

	observer.Update(event, context.Background())

	// Read the file and verify content
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty audit file")
	}

	// Parse JSON to verify structure
	var parsedEvent Event
	if err := json.Unmarshal(data, &parsedEvent); err != nil {
		t.Fatalf("failed to parse audit event JSON: %v", err)
	}

	if parsedEvent.TS != 1234567890 {
		t.Errorf("expected TS 1234567890, got %d", parsedEvent.TS)
	}
	if len(parsedEvent.Metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(parsedEvent.Metrics))
	}
	if parsedEvent.IP != "192.168.1.1" {
		t.Errorf("expected IP 192.168.1.1, got %s", parsedEvent.IP)
	}
}

func TestFileAuditObserver_AppendMode(t *testing.T) {
	tempFile := t.TempDir() + "/audit_append.log"

	observer1, err := NewFileAuditObserver(tempFile, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create FileAuditObserver: %v", err)
	}

	event1 := Event{TS: 100, Metrics: []string{"Metric1"}, IP: "1.1.1.1"}
	observer1.Update(event1, context.Background())
	observer1.Close()

	// Open in append mode again
	observer2, err := NewFileAuditObserver(tempFile, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create second FileAuditObserver: %v", err)
	}
	defer observer2.Close()

	event2 := Event{TS: 200, Metrics: []string{"Metric2"}, IP: "2.2.2.2"}
	observer2.Update(event2, context.Background())

	// Read and verify both events are present
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	// Should have 2 lines (each event ends with \n)
	if lines != 2 {
		t.Errorf("expected 2 lines, got %d", lines)
	}
}

func TestHTTPAuditObserver(t *testing.T) {
	var receivedEvents []Event
	var mu sync.Mutex
	done := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
		done <- struct{}{}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	observer := NewHTTPObserver(server.URL, zap.NewNop())

	event := Event{
		TS:      int64(time.Now().Unix()),
		Metrics: []string{"TestMetric"},
		IP:      "10.0.0.1",
	}

	observer.Update(event, context.Background())

	// Wait for server to receive
	select {
	case <-done:
	case <-time.After(6 * time.Second): // 5s timeout + buffer
		t.Fatal("timeout waiting for HTTP observer to send event")
	}

	mu.Lock()
	if len(receivedEvents) != 1 {
		t.Errorf("expected 1 event, got %d", len(receivedEvents))
	}
	if len(receivedEvents) > 0 {
		if receivedEvents[0].IP != "10.0.0.1" {
			t.Errorf("expected IP 10.0.0.1, got %s", receivedEvents[0].IP)
		}
	}
	mu.Unlock()
}

func TestHTTPAuditObserver_ServerError(t *testing.T) {
	done := make(chan struct{})

	// Server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		done <- struct{}{}
	}))
	defer server.Close()

	observer := NewHTTPObserver(server.URL, zap.NewNop())

	event := Event{TS: int64(time.Now().Unix())}
	observer.Update(event, context.Background())

	// Wait for request to complete
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("timeout waiting for HTTP request")
	}
}

type mockObserver struct {
	updateFn func(Event, context.Context)
}

func (m *mockObserver) Update(event Event, ctx context.Context) {
	if m.updateFn != nil {
		m.updateFn(event, ctx)
	}
}
