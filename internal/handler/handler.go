package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Meowizz/metrics-collector/internal/audit"
	models "github.com/Meowizz/metrics-collector/internal/model"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"github.com/go-chi/chi/v5"
)

// MetricsHandler — HTTP-хендлер для приёма, хранения и получения метрик.
// Поддерживает форматы text/plain, JSON и батч-обновления.
type MetricsHandler struct {
	storage repository.Storage
	audit   *audit.Subject
}

// NewMetricsHandler создаёт новый MetricsHandler с указанным хранилищем
// и опциональным субъектом аудита для уведомления об изменениях метрик.
// Возвращает указатель на инициализированный MetricsHandler.
func NewMetricsHandler(storage repository.Storage, auditSubject *audit.Subject) *MetricsHandler {
	return &MetricsHandler{
		storage: storage,
		audit:   auditSubject,
	}
}

func getClientIP(rq *http.Request) string {
	ip := rq.RemoteAddr

	if fwd := rq.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}

	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}

func (m *MetricsHandler) notifyAudit(rq *http.Request, metricNames []string) {
	if m.audit == nil {
		return
	}

	event := audit.Event{
		TS:      int(time.Now().Unix()),
		Metrics: metricNames,
		IP:      getClientIP(rq),
	}
	m.audit.Notify(event)
}

func (m *MetricsHandler) MainPage(rw http.ResponseWriter, rq *http.Request) {
	if rq.Method != http.MethodGet {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rw.Header().Set("Content-Type", "text/html")
	rw.WriteHeader(http.StatusOK)

	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Metrics Collector</title></head>
<body>
	<h1>Page for autotest Iter8</h1>
	<p>Server is running and serving metrics.</p>
</body>
</html>`

	rw.Write([]byte(htmlContent))
}

func (m *MetricsHandler) UpdatePage(res http.ResponseWriter, req *http.Request) {
	log.Printf("Получен запрос: %s %s", req.Method, req.URL.Path)
	if req.Method != http.MethodPost {
		http.Error(res, "", http.StatusMethodNotAllowed)
		return
	}

	metricType := chi.URLParam(req, "type")
	metricName := chi.URLParam(req, "name")
	valueStr := chi.URLParam(req, "value")

	switch metricType {
	case "gauge":
		value, err := strconv.ParseFloat(valueStr, 64)

		if err != nil {
			http.Error(res, "invalid gauge value", http.StatusBadRequest)
			return
		}
		m.storage.UpdateGauge(metricName, value)
		m.notifyAudit(req, []string{metricName})

	case "counter":
		value, err := strconv.ParseInt(valueStr, 10, 64)

		if err != nil {
			http.Error(res, "invalid counter value", http.StatusBadRequest)
			return
		}
		m.storage.UpdateCounter(metricName, value)
		m.notifyAudit(req, []string{metricName})

	default:
		http.Error(res, "Unknown metrics", http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusOK)
}

func (m *MetricsHandler) GetMetricValue(rw http.ResponseWriter, req *http.Request) {
	metricType := chi.URLParam(req, "type")
	metricName := chi.URLParam(req, "name")

	rw.Header().Set("Content-Type", "text/plain;charset=utf-8")

	switch metricType {
	case "gauge":
		val, ok := m.storage.GetGauge(metricName)

		if !ok {
			http.Error(rw, "Metric not found", http.StatusNotFound)
			return
		}

		rw.WriteHeader(http.StatusOK)
		fmt.Fprint(rw, val)

	case "counter":
		val, ok := m.storage.GetCounter(metricName)

		if !ok {
			http.Error(rw, "Metric not found", http.StatusNotFound)
			return
		}

		rw.WriteHeader(http.StatusOK)
		fmt.Fprint(rw, val)
	default:
		http.Error(rw, "Unknown metric type", http.StatusBadRequest)
	}
}

func (m *MetricsHandler) UpdateMetricJSON(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost || req.Header.Get("Content-Type") != "application/json" {
		http.Error(rw, "Not allowed method or invalid Content-Type", http.StatusMethodNotAllowed)
		return
	}

	var reqMetric models.Metrics

	if err := json.NewDecoder(req.Body).Decode(&reqMetric); err != nil {
		http.Error(rw, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resp := models.Metrics{
		ID:    reqMetric.ID,
		MType: reqMetric.MType,
	}

	switch reqMetric.MType {
	case models.Gauge:
		if reqMetric.Value == nil {
			http.Error(rw, "Value is required for gauge", http.StatusUnprocessableEntity)
			return
		}
		if err := m.storage.UpdateGauge(reqMetric.ID, *reqMetric.Value); err != nil {
			http.Error(rw, "Storage error", http.StatusInternalServerError)
			return
		}
		resp.Value = reqMetric.Value
		m.notifyAudit(req, []string{reqMetric.ID})
	case models.Counter:
		if reqMetric.Delta == nil {
			http.Error(rw, "Value is required for counter", http.StatusUnprocessableEntity)
			return
		}
		if err := m.storage.UpdateCounter(reqMetric.ID, *reqMetric.Delta); err != nil {
			http.Error(rw, "Storage error", http.StatusInternalServerError)
			return
		}
		resp.Delta = reqMetric.Delta
		m.notifyAudit(req, []string{reqMetric.ID})
	default:
		http.Error(rw, "Unsupported metric type", http.StatusUnprocessableEntity)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(resp)
}

func (m *MetricsHandler) ValueMetricJSON(rw http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodPost || req.Header.Get("Content-Type") != "application/json" {
		http.Error(rw, "Not allowed method or invalid Content-Type", http.StatusMethodNotAllowed)
		return
	}

	var reqMetric models.Metrics

	if err := json.NewDecoder(req.Body).Decode(&reqMetric); err != nil {
		http.Error(rw, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resp := models.Metrics{
		ID:    reqMetric.ID,
		MType: reqMetric.MType,
	}

	switch reqMetric.MType {
	case models.Gauge:
		value, exist := m.storage.GetGauge(reqMetric.ID)
		if !exist {
			http.Error(rw, "Metric not found", http.StatusNotFound)
			return
		}
		resp.Value = &value
	case models.Counter:
		value, exist := m.storage.GetCounter(reqMetric.ID)
		if !exist {
			http.Error(rw, "Metric not found", http.StatusNotFound)
			return
		}
		resp.Delta = &value

	default:
		http.Error(rw, "Unsupported metric type", http.StatusUnprocessableEntity)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(resp)

}

func (m *MetricsHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if err := m.storage.Ping(); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (m *MetricsHandler) UpdatesHandler(rw http.ResponseWriter, rq *http.Request) {

	if rq.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if rq.Header.Get("Content-Type") != "application/json" {
		http.Error(rw, "Invalid Content-Type", http.StatusBadRequest) // 400, не 405!
		return
	}

	var metric []models.Metrics
	if err := json.NewDecoder(rq.Body).Decode(&metric); err != nil {
		http.Error(rw, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(metric) == 0 {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		return
	}

	if err := m.storage.UpdateBatch(metric); err != nil {
		http.Error(rw, "Storage error", http.StatusInternalServerError)
		return
	}

	metricNames := make([]string, len(metric))
	for i, metric := range metric {
		metricNames[i] = metric.ID
	}
	m.notifyAudit(rq, metricNames)

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
}
func (m *MetricsHandler) RegisterRouters(r chi.Router) {
	r.Get("/", m.MainPage)
	r.Get("/value/{type}/{name}", m.GetMetricValue)
	r.Post("/update/{type}/{name}/{value}", m.UpdatePage)
	r.Post("/update", m.UpdateMetricJSON)
	r.Post("/value", m.ValueMetricJSON)
	r.Get("/ping", m.Ping)
	r.Post("/updates/", m.UpdatesHandler)
}
