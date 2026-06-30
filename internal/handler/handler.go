package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Meowizz/metrics-collector/internal/repository"
	"github.com/go-chi/chi/v5"
)

type MetricsHandler struct {
	storage repository.Storage
}

func NewMetricsHandler(storage repository.Storage) *MetricsHandler {
	return &MetricsHandler{storage: storage}
}

func (m *MetricsHandler) UpdatePage(res http.ResponseWriter, req *http.Request) {
	log.Printf("Получен запрос: %s %s", req.Method, req.URL.Path)
	if req.Method != http.MethodPost {
		http.Error(res, "", http.StatusMethodNotAllowed)
		return
	}

	if req.Header.Get("Content-Type") != "text/plain" {
		http.Error(res, "Content-Type must be text/plain", http.StatusBadRequest)
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

	case "counter":
		value, err := strconv.ParseInt(valueStr, 10, 64)

		if err != nil {
			http.Error(res, "invalid counter value", http.StatusBadRequest)
			return
		}
		m.storage.UpdateCounter(metricName, value)

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

func (m *MetricsHandler) RegisterRouters(r chi.Router) {
	r.Get("/value/{type}/{name}", m.GetMetricValue)
	r.Post("/update/{type}/{name}/{value}", m.UpdatePage)
}
