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
			h := NewMetricsHandler(mockStore)

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
	h := NewMetricsHandler(realStore)

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
	h := NewMetricsHandler(realStore)
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
func TestMetricsHandler_UpdatesHandler(t *testing.T) {

	realStore := repository.NewMemStorage()
	h := NewMetricsHandler(realStore)

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
