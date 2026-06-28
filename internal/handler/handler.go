package handler

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Meowizz/metrics-collector/internal/repository"
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

	path := strings.Split(req.URL.Path, "/")

	if len(path) != 5 {
		http.Error(res, "Invalid path", http.StatusNotFound)
		return
	}
	if path[3] == "" {
		http.Error(res, "metric name is required", http.StatusNotFound)
		return
	}

	metricType := path[2]
	metricName := path[3]
	valueStr := path[4]

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
