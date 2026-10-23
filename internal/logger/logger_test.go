package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitialize(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantError bool
	}{
		{
			name:      "Debug level",
			level:     "debug",
			wantError: false,
		},
		{
			name:      "Info level",
			level:     "info",
			wantError: false,
		},
		{
			name:      "Warn level",
			level:     "warn",
			wantError: false,
		},
		{
			name:      "Error level",
			level:     "error",
			wantError: false,
		},
		{
			name:      "Invalid level",
			level:     "invalid_level_xyz",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Initialize(tt.level)
			if (err != nil) != tt.wantError {
				t.Errorf("Initialize(%q) error = %v, wantError %v", tt.level, err, tt.wantError)
			}
		})
	}
}

func TestInitialize_WithValidLevels(t *testing.T) {
	validLevels := []string{"debug", "debug", "info", "warn", "error", "dEbUg"}

	for _, level := range validLevels {
		err := Initialize(level)
		if err != nil {
			t.Errorf("Initialize(%q) unexpected error: %v", level, err)
		}
	}
}

func TestWithLogging(t *testing.T) {
	// Create a handler that logs via the logger middleware
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	// Initialize logger to avoid output
	_ = Initialize("error")

	middleware := WithLogging(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got '%s'", rr.Body.String())
	}
}

func TestWithLogging_MethodNotAllowed(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	_ = Initialize("error")
	middleware := WithLogging(nextHandler)

	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rr.Code)
	}
}

func TestWithLogging_ResponseSize(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testBody := "this is a test response body"
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testBody))
	})

	_ = Initialize("error")
	middleware := WithLogging(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestLoggingResponseWriter_Write(t *testing.T) {
	next := httptest.NewRecorder()
	lw := &loggingResponseWriter{
		ResponseWriter: next,
		responseData: &responseData{
			status: http.StatusOK,
			size:   0,
		},
	}

	n, err := lw.Write([]byte("test data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 9 {
		t.Errorf("expected to write 9 bytes, got %d", n)
	}
	if lw.responseData.size != 9 {
		t.Errorf("expected responseData.size to be 9, got %d", lw.responseData.size)
	}
}

func TestLoggingResponseWriter_WriteHeader(t *testing.T) {
	next := httptest.NewRecorder()
	lw := &loggingResponseWriter{
		ResponseWriter: next,
		responseData: &responseData{
			status: http.StatusOK,
			size:   0,
		},
	}

	lw.WriteHeader(http.StatusCreated)
	if lw.responseData.status != http.StatusCreated {
		t.Errorf("expected status 201, got %d", lw.responseData.status)
	}
}
