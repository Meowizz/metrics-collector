package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Meowizz/metrics-collector/internal/collector"
	models "github.com/Meowizz/metrics-collector/internal/model"
)

type Sender struct {
	serverURL string
	client    *http.Client
}

func NewSender(serverURL string) *Sender {
	return &Sender{
		serverURL: serverURL,
		client:    &http.Client{},
	}
}

func (s *Sender) Send(metrics []*collector.Metric) error {
	for _, metric := range metrics {
		var valueStr string
		switch v := metric.Value.(type) {
		case float64:
			valueStr = fmt.Sprintf("%f", v)
		case int64:
			valueStr = fmt.Sprintf("%d", v)
		default:
			return fmt.Errorf("Unkown matric value type: %T", v)
		}

		url := fmt.Sprintf("%s/update/%s/%s/%s", s.serverURL, metric.Type, metric.Name, valueStr)
		req, err := http.NewRequest("POST", url, nil)

		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "text/plain")

		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send metric %s: %w", metric.Name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status %d for metric %s",
				resp.StatusCode, metric.Name)
		}
	}
	return nil
}

func (s *Sender) SendJSON(metrics []*collector.Metric) error {
	for _, metric := range metrics {
		reqMetric := models.Metrics{
			ID:    metric.Name,
			MType: string(metric.Type),
		}

		switch v := metric.Value.(type) {
		case float64:
			reqMetric.Value = &v
		case int64:
			reqMetric.Delta = &v
		default:
			return fmt.Errorf("Unknown metric type: %T", v)
		}

		jsonBytes, err := json.Marshal(reqMetric)
		if err != nil {
			return fmt.Errorf("Failed to marshal metric %s: %w", metric.Name, err)

		}

		req, err := http.NewRequest(http.MethodPost, s.serverURL+"/update", bytes.NewReader(jsonBytes))
		if err != nil {
			return fmt.Errorf("Failed to create request for %s: %w", metric.Name, err)

		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("Failed to send metric %s: %w", metric.Name, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Server returned status %d for metric %s", resp.StatusCode, metric.Name)

		}
	}
	return nil
}
