package middleware

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func getHash(body []byte, key string) string {
	h := sha256.New()
	h.Write(body)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func TestHashMiddleware_ValidHash(t *testing.T) {
	key := "test-secret-key"
	body := []byte(`{"id": "test", "type": "gauge", "value": 42.5}`)
	expectedHash := getHash(body, key)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	middleware := HashMiddleware(nextHandler, key)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HashSHA256", expectedHash)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Response should contain the hash header
	respHash := rr.Header().Get("HashSHA256")
	if respHash == "" {
		t.Error("expected HashSHA256 header in response")
	}
}

func TestHashMiddleware_InvalidHash(t *testing.T) {
	key := "test-secret-key"
	body := []byte(`{"id": "test", "type": "gauge", "value": 42.5}`)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	middleware := HashMiddleware(nextHandler, key)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HashSHA256", "invalid-hash-value")

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	bodyStr := rr.Body.String()
	if !strings.Contains(bodyStr, "hash mismatch") {
		t.Errorf("expected 'hash mismatch' in body, got: %s", bodyStr)
	}
}

func TestHashMiddleware_EmptyKey(t *testing.T) {
	body := []byte(`{"id": "test", "type": "gauge", "value": 42.5}`)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	// With empty key, hash validation is skipped
	middleware := HashMiddleware(nextHandler, "")

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No HashSHA256 header needed when key is empty

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 with empty key, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHashMiddleware_ResponseHash(t *testing.T) {
	key := "response-key"
	expectedResponseBody := `{"result": "success"}`

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedResponseBody))
	})

	middleware := HashMiddleware(nextHandler, key)

	body := []byte(`{}`)
	expectedHash := getHash(body, key)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HashSHA256", expectedHash)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	// Verify response hash
	respBody := rr.Body.Bytes()
	expectedRespHash := getHash(respBody, key)
	actualRespHash := rr.Header().Get("HashSHA256")

	if actualRespHash != expectedRespHash {
		t.Errorf("expected response hash %s, got %s", expectedRespHash, actualRespHash)
	}

	// Verify response body is correct
	if rr.Body.String() != expectedResponseBody {
		t.Errorf("expected body %s, got %s", expectedResponseBody, rr.Body.String())
	}
}

func TestHashMiddleware_MissingHashHeader(t *testing.T) {
	key := "test-key"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HashMiddleware(nextHandler, key)

	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	req.Header.Set("Content-Type", "application/json")
	// No HashSHA256 header

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing hash, got %d", rr.Code)
	}
}

func TestGzipMiddleware_RequestDecompress(t *testing.T) {
	var receivedBody string

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := GzipMiddleware(nextHandler)

	// Compress the request body
	body := []byte(`{"id": "test", "type": "gauge", "value": 42.5}`)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write(body)
	gw.Close()

	req := httptest.NewRequest(http.MethodPost, "/update", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if receivedBody != string(body) {
		t.Errorf("expected decompressed body %s, got %s", string(body), receivedBody)
	}
}

func TestGzipMiddleware_ResponseCompress(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "some response content that should be compressed"}`))
	})

	middleware := GzipMiddleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/update", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	// Response should be gzip compressed
	contentEncoding := rr.Header().Get("Content-Encoding")
	if contentEncoding != "gzip" {
		t.Errorf("expected Content-Encoding gzip, got %s", contentEncoding)
	}

	// Verify we can decompress the response
	gr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to read decompressed body: %v", err)
	}

	expectedBody := `{"data": "some response content that should be compressed"}`
	if string(decompressed) != expectedBody {
		t.Errorf("expected decompressed body %s, got %s", expectedBody, string(decompressed))
	}
}

func TestGzipMiddleware_NoCompression(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain text response"))
	})

	middleware := GzipMiddleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/update", nil)
	// No Accept-Encoding: gzip

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Response should NOT be gzip compressed
	contentEncoding := rr.Header().Get("Content-Encoding")
	if contentEncoding == "gzip" {
		t.Error("expected no Content-Encoding for non-JSON response")
	}

	// Body should be plain text
	if rr.Body.String() != "plain text response" {
		t.Errorf("expected plain text body, got: %s", rr.Body.String())
	}
}

func TestGzipMiddleware_InvalidGzipRequest(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := GzipMiddleware(nextHandler)

	// Send invalid gzip data
	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("not gzip data"))
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid gzip, got %d", rr.Code)
	}
}
