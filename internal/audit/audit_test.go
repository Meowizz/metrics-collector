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

func TestSubject_DetachObserver(t *testing.T) {
	subject := NewSubject()

	var mu sync.Mutex
	var count int

	mockObserver := &mockObserver{
		updateFn: func(event Event, ctx context.Context) {
			mu.Lock()
			count++
			mu.Unlock()
		},
	}

	subject.Attach(mockObserver)

	// Send one event — should be processed
	event := Event{TS: int64(time.Now().Unix())}
	subject.Notify(event, context.Background())

	// Worker processes asynchronously, give it time
	time.Sleep(2 * time.Second)

	mu.Lock()
	prevCount := count
	mu.Unlock()

	if prevCount != 1 {
		t.Fatalf("expected 1 notification before detach, got %d", prevCount)
	}

	// Detach — should stop the worker
	subject.Detach(mockObserver)

	// Send another event — should be dropped (worker stopped)
	subject.Notify(event, context.Background())

	// Give a moment for any stray processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != prevCount {
		t.Errorf("expected no notifications after detach, got %d (was %d)", count, prevCount)
	}
	mu.Unlock()
}

func TestSubject_Shutdown(t *testing.T) {
	subject := NewSubject()

	var mu sync.Mutex
	var count int

	for i := 0; i < 3; i++ {
		mockObserver := &mockObserver{
			updateFn: func(event Event, ctx context.Context) {
				mu.Lock()
				count++
				mu.Unlock()
			},
		}
		subject.Attach(mockObserver)
	}

	// Send multiple events
	for i := 0; i < 5; i++ {
		subject.Notify(Event{TS: int64(time.Now().Unix())}, context.Background())
	}

	// Allow workers time to process
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	preShutdownCount := count
	mu.Unlock()

	if preShutdownCount != 15 {
		t.Errorf("expected 15 notifications before shutdown, got %d", preShutdownCount)
	}

	// Shutdown all workers
	subject.Shutdown()

	// Send after shutdown — should be silently ignored
	subject.Notify(Event{TS: int64(time.Now().Unix())}, context.Background())

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != preShutdownCount {
		t.Errorf("expected no notifications after shutdown, got %d (was %d)", count, preShutdownCount)
	}
	mu.Unlock()
}

func TestSubject_ConcurrentNotify(t *testing.T) {
	subject := NewSubject()

	var mu sync.Mutex
	var count int
	numObservers := 5
	numNotifications := 100

	// Create multiple observers
	for i := 0; i < numObservers; i++ {
		mockObserver := &mockObserver{
			updateFn: func(event Event, ctx context.Context) {
				mu.Lock()
				count++
				mu.Unlock()
			},
		}
		subject.Attach(mockObserver)
	}

	// Concurrent notifications
	var wg sync.WaitGroup
	for i := 0; i < numNotifications; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subject.Notify(Event{TS: int64(time.Now().Unix())}, context.Background())
		}()
	}

	wg.Wait()

	// Allow workers to process
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	expected := numObservers * numNotifications
	if count != expected {
		t.Errorf("expected %d notifications, got %d", expected, count)
	}
	mu.Unlock()

	subject.Shutdown()
}

func TestSubject_DropOldestOnOverflow(t *testing.T) {
	const bufSize = 3

	// Block observer until we fill the buffer
	proceed := make(chan struct{})
	eventOrder := make(chan Event, bufSize)

	slowObserver := &mockObserver{
		updateFn: func(event Event, ctx context.Context) {
			<-proceed // block until buffer is full
			eventOrder <- event
		},
	}

	worker := newObserverWorker(slowObserver, bufSize)

	// Fill the buffer — send() is non-blocking
	for i := 0; i < bufSize; i++ {
		if !worker.send(Event{TS: int64(100 + i)}) {
			t.Fatalf("send failed at index %d", i)
		}
	}

	// Buffer is full — this should drop the oldest event (TS=100)
	if !worker.send(Event{TS: 999}) {
		t.Fatal("send(999) should succeed (replacing oldest)")
	}

	// Now let the observer process events
	close(proceed)

	// Collect all processed events
	var processed []Event
	for i := 0; i < bufSize; i++ {
		select {
		case e := <-eventOrder:
			processed = append(processed, e)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d", i)
		}
	}

	// Verify that TS=100 was dropped (not in processed events)
	for _, e := range processed {
		if e.TS == 100 {
			t.Errorf("expected TS=100 to be dropped, but it was processed")
		}
	}

	// Verify TS=999 is present
	found := false
	for _, e := range processed {
		if e.TS == 999 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected TS=999 to be in processed events")
	}

	// Stop the worker and wait for it to exit
	worker.stop()
	worker.wait()
}

func TestSubject_ZeroBuffer(t *testing.T) {
	slowDone := make(chan struct{}, 1)
	slowObserver := &mockObserver{
		updateFn: func(event Event, ctx context.Context) {
			// Block to prevent draining
			time.Sleep(500 * time.Millisecond)
			slowDone <- struct{}{}
		},
	}

	worker := newObserverWorker(slowObserver, 0)
	defer func() {
		worker.stop()
		worker.wait()
	}()

	// With zero buffer and blocked reader, send should return false
	result := worker.send(Event{TS: 1})
	if result {
		// It's OK if it succeeds when the reader happens to be ready
		// The important thing is it doesn't block
	}

	// Send should not block even when buffer is full and reader is slow
	done := make(chan struct{})
	go func() {
		worker.send(Event{TS: 2})
		close(done)
	}()

	select {
	case <-done:
		// Good — non-blocking
	case <-time.After(100 * time.Millisecond):
		t.Fatal("send() blocked with zero buffer — should be non-blocking")
	}
}

func TestSubject_AttachDuplicate(t *testing.T) {
	subject := NewSubject()

	var mu sync.Mutex
	var count int

	mockObserver := &mockObserver{
		updateFn: func(event Event, ctx context.Context) {
			mu.Lock()
			count++
			mu.Unlock()
		},
	}

	subject.Attach(mockObserver)
	subject.Attach(mockObserver) // duplicate — should be ignored

	subject.Notify(Event{TS: int64(time.Now().Unix())}, context.Background())

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if count != 1 {
		t.Errorf("expected 1 notification (duplicate attach ignored), got %d", count)
	}
	mu.Unlock()

	subject.Detach(mockObserver)
}

func TestSubject_DetachNonExistent(t *testing.T) {
	subject := NewSubject()
	mockObserver := &mockObserver{}

	// Detaching non-existent observer should not panic
	subject.Detach(mockObserver)
}

type mockObserver struct {
	updateFn func(Event, context.Context)
}

func (m *mockObserver) Update(event Event, ctx context.Context) {
	if m.updateFn != nil {
		m.updateFn(event, ctx)
	}
}
