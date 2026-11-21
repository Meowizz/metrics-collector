package middleware

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/crypto"
	models "github.com/Meowizz/metrics-collector/internal/model"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

func mustDecryptTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := crypto.GenerateKeyRSA(2048)
	if err != nil {
		t.Fatalf("генерация ключа: %v", err)
	}
	return key
}

// captureHandler запоминает тело и ContentLength запроса, дошедшего до хендлера.
type captureHandler struct {
	called        bool
	body          []byte
	contentLength int64
}

func (h *captureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.body, _ = io.ReadAll(r.Body)
	h.contentLength = r.ContentLength
	w.WriteHeader(http.StatusOK)
}

func encryptedRequest(t *testing.T, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set(crypto.EncryptedHeader, "1")
	return req
}

func TestDecryptMiddleware_DecryptsMarkedRequest(t *testing.T) {
	key := mustDecryptTestKey(t)
	plain := []byte(`[{"id":"Alloc","type":"gauge","value":1.5}]`)

	enc, err := crypto.Encrypt(&key.PublicKey, plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	h := &captureHandler{}
	rec := httptest.NewRecorder()
	DecryptMiddleware(key)(h).ServeHTTP(rec, encryptedRequest(t, enc))

	if rec.Code != http.StatusOK {
		t.Fatalf("статус: ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	if !h.called {
		t.Fatal("запрос не дошёл до хендлера")
	}
	if !bytes.Equal(h.body, plain) {
		t.Errorf("хендлер получил %q, ожидали %q", h.body, plain)
	}
	if h.contentLength != int64(len(plain)) {
		t.Errorf("ContentLength: ожидали %d, получили %d", len(plain), h.contentLength)
	}
}

func TestDecryptMiddleware_PassesUnmarkedRequest(t *testing.T) {
	plain := []byte(`{"id":"Alloc","type":"gauge","value":1.5}`)

	// Без заголовка запрос проходит как есть — и с ключом, и без ключа.
	for name, key := range map[string]*rsa.PrivateKey{
		"ключ задан":    mustDecryptTestKey(t),
		"ключ не задан": nil,
	} {
		t.Run(name, func(t *testing.T) {
			h := &captureHandler{}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(plain))

			DecryptMiddleware(key)(h).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK || !h.called {
				t.Fatalf("незашифрованный запрос должен пройти: статус %d, хендлер вызван: %v", rec.Code, h.called)
			}
			if !bytes.Equal(h.body, plain) {
				t.Errorf("тело изменилось: %q", h.body)
			}
		})
	}
}

func TestDecryptMiddleware_Rejects(t *testing.T) {
	key := mustDecryptTestKey(t)
	otherKey := mustDecryptTestKey(t)

	encForOther, err := crypto.Encrypt(&otherKey.PublicKey, []byte("данные"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	encForKey, err := crypto.Encrypt(&key.PublicKey, []byte("данные"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	tampered := bytes.Clone(encForKey)
	tampered[len(tampered)-1] ^= 0x01

	cases := []struct {
		name string
		key  *rsa.PrivateKey
		body []byte
	}{
		{name: "ключ не задан", key: nil, body: encForKey},
		{name: "мусор вместо шифротекста", key: key, body: []byte("not encrypted at all")},
		{name: "пустое тело", key: key, body: nil},
		{name: "зашифровано чужим ключом", key: key, body: encForOther},
		{name: "изменён байт в пути", key: key, body: tampered},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &captureHandler{}
			rec := httptest.NewRecorder()

			DecryptMiddleware(tc.key)(h).ServeHTTP(rec, encryptedRequest(t, tc.body))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("статус: ожидали 400, получили %d", rec.Code)
			}
			if h.called {
				t.Error("запрос не должен доходить до хендлера")
			}
		})
	}
}

// TestDecryptMiddleware_FullChainWithSender проверяет связку агента и сервера целиком:
// настоящий Sender с шифрованием и хешем → DecryptMiddleware → GzipMiddleware → HashMiddleware → хендлер.
// Порядок middleware тот же, что в cmd/server/main.go.
func TestDecryptMiddleware_FullChainWithSender(t *testing.T) {
	key := mustDecryptTestKey(t)
	const hashKey = "secret"

	h := &captureHandler{}
	chain := DecryptMiddleware(key)(GzipMiddleware(HashMiddleware(h, hashKey)))

	srv := httptest.NewServer(chain)
	defer srv.Close()

	s := sender.NewSender(srv.URL, hashKey)
	s.EnableEncryption(&key.PublicKey)

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Alloc", Value: float64(42.5)},
		{Type: collector.Counter, Name: "PollCount", Value: int64(3)},
	}
	if err := s.SendBatch(metrics); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}

	if !h.called {
		t.Fatal("запрос не дошёл до хендлера")
	}
	var got []models.Metrics
	if err := json.Unmarshal(h.body, &got); err != nil {
		t.Fatalf("хендлер получил не JSON (%q): %v", h.body, err)
	}
	if len(got) != 2 || got[0].ID != "Alloc" || got[1].ID != "PollCount" {
		t.Errorf("неожиданные метрики: %+v", got)
	}
}
