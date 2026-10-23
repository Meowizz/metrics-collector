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
	"sync"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	models "github.com/Meowizz/metrics-collector/internal/model"
	"github.com/Meowizz/metrics-collector/pkg"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var jsonBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// Sender — отправитель метрик на сервер через HTTP с поддержкой gzip-сжатия
// и хеширования запросов.
type Sender struct {
	serverURL string
	hashKey   string
	client    *http.Client
}

// NewSender создаёт новый Sender для отправки метрик на указанный serverURL
// с использованием заданного hashKey для SHA256-хеширования.
// Возвращает указатель на инициализированный Sender с HTTP-клиентом со временем ожидания 10с.
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

// GzipTransport — HTTP-транспорт, автоматически сжимающий тело JSON-запросов через gzip.
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

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	gw := gzip.NewWriter(buf)
	if _, err := gw.Write(bodyBytes); err != nil {
		gw.Close()
		bufferPool.Put(buf)
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := gw.Close(); err != nil {
		bufferPool.Put(buf)
		return nil, fmt.Errorf("gzip close: %w", err)
	}

	compressedData := make([]byte, buf.Len())
	copy(compressedData, buf.Bytes())
	bufferPool.Put(buf)

	newReq := req.Clone(req.Context())
	newReq.Body = io.NopCloser(bytes.NewReader(compressedData))
	newReq.ContentLength = int64(len(compressedData))
	newReq.Header.Set("Content-Encoding", "gzip")
	newReq.Header.Set("Content-Length", strconv.Itoa(len(compressedData)))

	newReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(compressedData)), nil
	}

	return g.transport().RoundTrip(newReq)
}

func (g *GzipTransport) transport() http.RoundTripper {
	if g.Transport != nil {
		return g.Transport
	}
	return http.DefaultTransport
}

// Send отправляет каждую метрику отдельным POST-запросом на URL-путь
// /update/{type}/{name}/{value} в формате text/plain.
func (s *Sender) Send(metrics []*collector.Metric) error {
	for _, metric := range metrics {
		var valueStr string
		switch v := metric.Value.(type) {
		case float64:
			valueStr = strconv.FormatFloat(v, 'f', -1, 64)
		case int64:
			valueStr = strconv.FormatInt(v, 10)
		default:
			return fmt.Errorf("unkown matric value type: %T", v)
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

// SendJSON отправляет каждую метрику отдельным POST-запросом на /update
// в формате JSON с использованием DoWithRetry для повторных попыток.
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
			return fmt.Errorf("unknown metric type: %T", v)
		}

		jsonBytes, err := json.Marshal(reqMetric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric %s: %w", metric.Name, err)

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

// SendBatch отправляет пакет метрик одним POST-запросом на /updates
// в формате JSON-массива с использованием DoWithRetry для повторных попыток.
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
			return fmt.Errorf("unknown metric type: %T", v)
		}

		apiMetrics = append(apiMetrics, reqMetric)
	}

	buf := jsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := json.NewEncoder(buf).Encode(apiMetrics); err != nil {
		jsonBufferPool.Put(buf)
		return fmt.Errorf("failed to marshal metric to batch: %w", err)
	}

	jsonBytes := buf.Bytes()

	var hash string
	if s.hashKey != "" {
		hash = computeHash(jsonBytes, s.hashKey)
	}

	err := pkg.DoWithRetry(func() error {
		req, reqErr := http.NewRequest(http.MethodPost, s.serverURL+"/updates", bytes.NewReader(jsonBytes))
		if reqErr != nil {
			return reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		if hash != "" {
			req.Header.Set("HashSHA256", hash)
		}

		resp, doErr := s.client.Do(req)
		if doErr != nil {
			return fmt.Errorf("network error: %w", doErr)
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

	jsonBufferPool.Put(buf)

	return err
}
