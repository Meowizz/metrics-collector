package sender

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Meowizz/metrics-collector/internal/collector"
)

func TestSender_Send(t *testing.T) {
	tests := []struct {
		name        string
		metrics     []*collector.Metric
		serverResp  int
		wantErr     bool
		wantURLPart string
	}{
		{
			name: "Send gauge metric successfully",
			metrics: []*collector.Metric{
				{Type: collector.Gauge, Name: "Alloc", Value: float64(123.45)},
			},
			serverResp:  http.StatusOK,
			wantErr:     false,
			wantURLPart: "/update/gauge/Alloc/123.45",
		},
		{
			name: "Send counter metric successfully",
			metrics: []*collector.Metric{
				{Type: collector.Counter, Name: "PollCount", Value: int64(42)},
			},
			serverResp:  http.StatusOK,
			wantErr:     false,
			wantURLPart: "/update/counter/PollCount/42",
		},
		{
			name: "Send multiple metrics",
			metrics: []*collector.Metric{
				{Type: collector.Gauge, Name: "Alloc", Value: float64(100.0)},
				{Type: collector.Gauge, Name: "Sys", Value: float64(200.0)},
				{Type: collector.Counter, Name: "PollCount", Value: int64(5)},
			},
			serverResp: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "Send empty metrics list",
			metrics:    []*collector.Metric{},
			serverResp: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "Unknown metric value type",
			metrics: []*collector.Metric{
				{Type: collector.Gauge, Name: "Test", Value: "string_value"},
			},
			serverResp: http.StatusOK,
			wantErr:    true,
		},
		{
			name: "Server returns error status",
			metrics: []*collector.Metric{
				{Type: collector.Gauge, Name: "Alloc", Value: float64(100.0)},
			},
			serverResp: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name: "Server returns bad request",
			metrics: []*collector.Metric{
				{Type: collector.Gauge, Name: "Alloc", Value: float64(100.0)},
			},
			serverResp: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				if r.Method != http.MethodPost {
					t.Errorf("Expected POST, got %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "text/plain" {
					t.Errorf("Expected Content-Type text/plain, got %s", r.Header.Get("Content-Type"))
				}

				if tt.wantURLPart != "" && !strings.Contains(r.URL.Path, tt.wantURLPart) {
					t.Errorf("Expected URL to contain %s, got %s", tt.wantURLPart, r.URL.Path)
				}

				w.WriteHeader(tt.serverResp)
			}))
			defer server.Close()

			s := NewSender(server.URL)

			err := s.Send(tt.metrics)

			if (err != nil) != tt.wantErr {
				t.Errorf("Sender.Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSender_Send_URLFormat(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSender(server.URL)

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Alloc", Value: float64(1234.567)},
	}

	err := s.Send(metrics)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedPrefix := "/update/gauge/Alloc/"
	if !strings.HasPrefix(receivedURL, expectedPrefix) {
		t.Errorf("Expected URL to start with %s, got %s", expectedPrefix, receivedURL)
	}
}

func TestSender_Send_Headers(t *testing.T) {
	var receivedContentType string
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSender(server.URL)

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Test", Value: float64(1.0)},
	}

	err := s.Send(metrics)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("Expected POST method, got %s", receivedMethod)
	}
	if receivedContentType != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", receivedContentType)
	}
}

func TestSender_Send_ValueFormatting(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSender(server.URL)

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Test", Value: float64(123.456)},
	}
	s.Send(metrics)
	if !strings.Contains(receivedURL, "123.456") {
		t.Errorf("Expected URL to contain 123.456, got %s", receivedURL)
	}

	metrics = []*collector.Metric{
		{Type: collector.Counter, Name: "Test", Value: int64(42)},
	}
	s.Send(metrics)
	if !strings.Contains(receivedURL, "42") {
		t.Errorf("Expected URL to contain 42, got %s", receivedURL)
	}
}
