package main

import (
	"net/http"
	"strconv"
	"strings"
)

type MemStorage struct {
	gauge   map[string]float64
	counter map[string]int64
}

type Storage interface {
	UpdateGauge(name string, value float64) error
	UpdateCounter(name string, value int64) error
}

func (m *MemStorage) UpdateGauge(name string, value float64) error {
	m.gauge[name] = value
	return nil
}

func (m *MemStorage) UpdateCounter(name string, value int64) error {
	m.counter[name] += value
	return nil
}
func (m *MemStorage) UpdatePage(res http.ResponseWriter, req *http.Request) {
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
		m.UpdateGauge(metricName, value)

	case "counter":
		value, err := strconv.ParseInt(valueStr, 10, 64)

		if err != nil {
			http.Error(res, "invalid counter value", http.StatusBadRequest)
			return
		}
		m.UpdateCounter(metricName, value)

	default:
		http.Error(res, "Unknown metrics", http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusOK)
}
func main() {
	mux := http.NewServeMux()
	MetricsStorage := &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
	mux.HandleFunc("/update", MetricsStorage.UpdatePage)
	mux.HandleFunc("/update/", MetricsStorage.UpdatePage)
	err := http.ListenAndServe(":8080", mux)

	if err != nil {
		panic(err)
	}
}
