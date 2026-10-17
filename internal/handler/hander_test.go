package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	models "github.com/Meowizz/metrics-collector/internal/model"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"github.com/go-chi/chi/v5"
)

type MockStorage struct {
	pingErr error
}

func (m *MockStorage) Ping() error                                  { return m.pingErr }
func (m *MockStorage) UpdateGauge(name string, value float64) error { return nil }
func (m *MockStorage) UpdateCounter(name string, value int64) error { return nil }
func (m *MockStorage) GetGauge(name string) (float64, bool)         { return 0, false }
func (m *MockStorage) GetCounter(name string) (int64, bool)         { return 0, false }
func (m *MockStorage) UpdateBatch(metrics []models.Metrics) error   { return nil }

func TestMetricsHandler_Ping(t *testing.T) {
	tests := []struct {
		name           string
		mockPingErr    error
		expectedStatus int
	}{
		{
			name:           "Успешный пинг (MemStorage или живая БД)",
			mockPingErr:    nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Ошибка пинга (БД недоступна)",
			mockPingErr:    errors.New("connection refused"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockStorage{pingErr: tt.mockPingErr}
			h := NewMetricsHandler(mockStore, nil)

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			rr := httptest.NewRecorder()

			h.Ping(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("ожидался статус %d, получен %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestMetricsHandler_UpdateAndGet(t *testing.T) {
	realStore := repository.NewMemStorage()
	h := NewMetricsHandler(realStore, nil)

	r := chi.NewRouter()
	h.RegisterRouters(r)

	reqUpdate := httptest.NewRequest(http.MethodPost, "/update/gauge/TestGauge/42.5", nil)
	rrUpdate := httptest.NewRecorder()
	r.ServeHTTP(rrUpdate, reqUpdate)

	if rrUpdate.Code != http.StatusOK {
		t.Errorf("UpdatePage expected status 200, got %d", rrUpdate.Code)
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/value/gauge/TestGauge", nil)
	rrGet := httptest.NewRecorder()
	r.ServeHTTP(rrGet, reqGet)

	if rrGet.Code != http.StatusOK {
		t.Errorf("GetMetricValue expected status 200, got %d", rrGet.Code)
	}

	body := strings.TrimSpace(rrGet.Body.String())
	if body != "42.5" {
		t.Errorf("GetMetricValue expected body '42.5', got '%s'", body)
	}
}

func TestMetricsHandler_UpdateMetricJSON(t *testing.T) {
	realStore := repository.NewMemStorage()
	h := NewMetricsHandler(realStore, nil)

	r := chi.NewRouter()
	h.RegisterRouters(r)

	jsonBody := strings.NewReader(`{"id": "MyCounter", "type": "counter", "delta": 10}`)

	req := httptest.NewRequest(http.MethodPost, "/update", jsonBody)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("UpdateMetricJSON expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
		return
	}

	val, ok := realStore.GetCounter("MyCounter")
	if !ok || val != 10 {
		t.Errorf("Expected counter 'MyCounter' to be 10 in storage, got %v, exists: %v", val, ok)
	}
}

func TestMetricsHandler_MainPage(t *testing.T) {
	realStore := repository.NewMemStorage()
	h := NewMetricsHandler(realStore, nil)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET returns HTML welcome page",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "Metrics Collector",
		},
		{
			name:           "POST not allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			rr := httptest.NewRecorder()

			h.MainPage(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.expectedBody != "" && !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain %q, got: %s", tt.expectedBody, rr.Body.String())
			}

			if tt.name == "GET returns HTML welcome page" {
				contentType := rr.Header().Get("Content-Type")
				if contentType != "text/html" {
					t.Errorf("expected Content-Type text/html, got %s", contentType)
				}
			}
		})
	}
}

func TestMetricsHandler_GetMetricValue(t *testing.T) {
	realStore := repository.NewMemStorage()
	realStore.UpdateGauge("TestGauge", 42.5)
	realStore.UpdateCounter("TestCounter", 100)
	h := NewMetricsHandler(realStore, nil)

	r := chi.NewRouter()
	h.RegisterRouters(r)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET gauge value success",
			url:            "/value/gauge/TestGauge",
			expectedStatus: http.StatusOK,
			expectedBody:   "42.5",
		},
		{
			name:           "GET counter value success",
			url:            "/value/counter/TestCounter",
			expectedStatus: http.StatusOK,
			expectedBody:   "100",
		},
		{
			name:           "GET gauge not found",
			url:            "/value/gauge/NonExistent",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Metric not found",
		},
		{
			name:           "GET counter not found",
			url:            "/value/counter/NonExistent",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Metric not found",
		},
		{
			name:           "GET unknown metric type",
			url:            "/value/unknown/TestMetric",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Unknown metric type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.expectedBody != "" && !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain %q, got: %s", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestMetricsHandler_ValueMetricJSON(t *testing.T) {
	realStore := repository.NewMemStorage()
	realStore.UpdateGauge("JSONGauge", 99.9)
	realStore.UpdateCounter("JSONCounter", 50)
	h := NewMetricsHandler(realStore, nil)

	r := chi.NewRouter()
	r.Post("/value", h.ValueMetricJSON)

	tests := []struct {
		name           string
		jsonBody       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "POST gauge value success",
			jsonBody:       `{"id": "JSONGauge", "type": "gauge"}`,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "99.9") {
					t.Errorf("expected body to contain 99.9, got: %s", body)
				}
			},
		},
		{
			name:           "POST counter value success",
			jsonBody:       `{"id": "JSONCounter", "type": "counter"}`,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "50") {
					t.Errorf("expected body to contain 50, got: %s", body)
				}
			},
		},
		{
			name:           "POST gauge not found",
			jsonBody:       `{"id": "NonExistent", "type": "gauge"}`,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "Metric not found") {
					t.Errorf("expected 'Metric not found', got: %s", body)
				}
			},
		},
		{
			name:           "POST counter not found",
			jsonBody:       `{"id": "NonExistent", "type": "counter"}`,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "Metric not found") {
					t.Errorf("expected 'Metric not found', got: %s", body)
				}
			},
		},
		{
			name:           "POST unknown metric type",
			jsonBody:       `{"id": "Test", "type": "unknown"}`,
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "Unsupported metric type") {
					t.Errorf("expected 'Unsupported metric type', got: %s", body)
				}
			},
		},
		{
			name:           "POST invalid JSON",
			jsonBody:       `not valid json`,
			expectedStatus: http.StatusBadRequest,
			checkResponse:  nil,
		},
		{
			name:           "POST wrong Content-Type",
			jsonBody:       `{"id": "Test", "type": "gauge"}`,
			expectedStatus: http.StatusMethodNotAllowed,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.name == "POST wrong Content-Type" {
				req = httptest.NewRequest(http.MethodPost, "/value", strings.NewReader(tt.jsonBody))
			} else {
				req = httptest.NewRequest(http.MethodPost, "/value", strings.NewReader(tt.jsonBody))
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
				return
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rr.Body.String())
			}
		})
	}
}

func TestMetricsHandler_UpdatesHandler(t *testing.T) {
	realStore := repository.NewMemStorage()
	h := NewMetricsHandler(realStore, nil)

	r := chi.NewRouter()
	r.Post("/updates", h.UpdatesHandler)

	tests := []struct {
		name           string
		jsonBody       string
		expectedStatus int
		checkStorage   func(t *testing.T)
	}{
		{
			name: "Успешный батч с Gauge и Counter",
			jsonBody: `[
				{"id": "BatchGauge1", "type": "gauge", "value": 99.9},
				{"id": "BatchCounter1", "type": "counter", "delta": 5}
			]`,
			expectedStatus: http.StatusOK,
			checkStorage: func(t *testing.T) {
				gVal, gOk := realStore.GetGauge("BatchGauge1")
				if !gOk || gVal != 99.9 {
					t.Errorf("Expected gauge 'BatchGauge1' to be 99.9, got %v (exists: %v)", gVal, gOk)
				}

				cVal, cOk := realStore.GetCounter("BatchCounter1")
				if !cOk || cVal != 5 {
					t.Errorf("Expected counter 'BatchCounter1' to be 5, got %v (exists: %v)", cVal, cOk)
				}
			},
		},
		{
			name:           "Пустой батч (должен вернуть 200 OK)",
			jsonBody:       `[]`,
			expectedStatus: http.StatusOK,
			checkStorage:   nil,
		},
		{
			name:           "Невалидный JSON",
			jsonBody:       `not a valid json array`,
			expectedStatus: http.StatusBadRequest,
			checkStorage:   nil,
		},
		{
			name:           "Неверный Content-Type",
			jsonBody:       `[{"id": "Test", "type": "gauge", "value": 10}]`,
			expectedStatus: http.StatusBadRequest,
			checkStorage:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(tt.jsonBody))

			if tt.name != "Неверный Content-Type" {
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("ожидался статус %d, получен %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
				return
			}

			if tt.checkStorage != nil {
				tt.checkStorage(t)
			}
		})
	}
}
