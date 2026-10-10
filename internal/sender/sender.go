package sender

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	models "github.com/Meowizz/metrics-collector/internal/model"
	"github.com/Meowizz/metrics-collector/pkg"
)

type Sender struct {
	serverURL string
	hashKey   string
	client    *http.Client
}

func NewSender(serverURL string, hashKey string) *Sender {
	client := &http.Client{
		Transport: &GzipTransport{Transport: http.DefaultTransport},
		Timeout:   10 * time.Second,
	}
	return &Sender{
		serverURL: serverURL,
		hashKey:   hashKey,
		client:    client,
	}
}

type GzipTransport struct {
	Transport http.RoundTripper
}

func computeHash(body []byte, key string) string {
	h := sha256.New()
	h.Write(body)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func (g *GzipTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Content-Encoding") == "gzip" ||
		!strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		return g.transport().RoundTrip(req)
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	req.Body.Close()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(bodyBytes); err != nil {
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}

	newReq := req.Clone(req.Context())
	newReq.Body = io.NopCloser(&buf)
	newReq.ContentLength = int64(buf.Len())
	newReq.Header.Set("Content-Encoding", "gzip")
	newReq.Header.Set("Content-Length", strconv.Itoa(buf.Len()))

	newReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}

	return g.transport().RoundTrip(newReq)
}

func (g *GzipTransport) transport() http.RoundTripper {
	if g.Transport != nil {
		return g.Transport
	}
	return http.DefaultTransport
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

		if s.hashKey != "" {
			req.Header.Set("HashSHA256", computeHash([]byte{}, s.hashKey))
		}

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
		var hash string

		if s.hashKey != "" {
			hash = computeHash(jsonBytes, s.hashKey)
		}

		err = pkg.DoWithRetry(func() error {
			req, err := http.NewRequest(http.MethodPost, s.serverURL+"/update", bytes.NewReader(jsonBytes))
			if err != nil {
				return fmt.Errorf("failed to create request for %s: %w", metric.Name, err)
			}
			req.Header.Set("Content-Type", "application/json")

			if hash != "" {
				req.Header.Set("HashSHA256", hash)
			}

			resp, err := s.client.Do(req)
			if err != nil {
				return fmt.Errorf("network error: %w", err)
			}

			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}

			if resp.StatusCode >= 500 {
				return fmt.Errorf("server error status: %d", resp.StatusCode)
			}

			return fmt.Errorf("client error status %d: %w", resp.StatusCode, pkg.ErrNotRetriable)
		})

		if err != nil {
			return fmt.Errorf("failed to send metric %s: %w", metric.Name, err)
		}
	}

	return nil
}

func (s *Sender) SendBatch(metrics []*collector.Metric) error {

	if len(metrics) == 0 {
		return nil
	}

	apiMetrics := make([]models.Metrics, 0, len(metrics))

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

		apiMetrics = append(apiMetrics, reqMetric)
	}

	jsonBytes, err := json.Marshal(apiMetrics)
	if err != nil {
		return fmt.Errorf("Failed to marshal metric to batch: %w", err)

	}
	var hash string

	if s.hashKey != "" {
		hash = computeHash(jsonBytes, s.hashKey)
	}

	return pkg.DoWithRetry(func() error {
		req, err := http.NewRequest(http.MethodPost, s.serverURL+"/updates", bytes.NewReader(jsonBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if hash != "" {
			req.Header.Set("HashSHA256", hash)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("network error: %w", err)
		}

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error status: %d", resp.StatusCode)
		}
		return fmt.Errorf("client error status %d: %w", resp.StatusCode, pkg.ErrNotRetriable)
	})

}
