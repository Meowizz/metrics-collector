package sender

import (
	"bytes"
	"compress/gzip"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/crypto"
	models "github.com/Meowizz/metrics-collector/internal/model"
)

// receivedRequest — то, что тестовый сервер увидел в запросе.
type receivedRequest struct {
	path    string
	header  http.Header
	rawBody []byte
}

// recordingServer запускает тестовый сервер, который сохраняет все запросы
// и отвечает статусами из statuses по очереди (после конца списка — 200).
func recordingServer(t *testing.T, statuses ...int) (*httptest.Server, func() []receivedRequest) {
	t.Helper()
	var (
		mu   sync.Mutex
		reqs []receivedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("чтение тела: %v", err)
		}
		mu.Lock()
		reqs = append(reqs, receivedRequest{path: r.URL.Path, header: r.Header.Clone(), rawBody: body})
		n := len(reqs)
		mu.Unlock()

		status := http.StatusOK
		if n <= len(statuses) {
			status = statuses[n-1]
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []receivedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]receivedRequest(nil), reqs...)
	}
}

// decryptAndGunzip повторяет обработку на сервере: расшифровка, затем распаковка.
func decryptAndGunzip(t *testing.T, key *rsa.PrivateKey, body []byte) []byte {
	t.Helper()
	compressed, err := crypto.Decrypt(key, body)
	if err != nil {
		t.Fatalf("расшифровка тела: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("тело после расшифровки не gzip: %v", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("распаковка: %v", err)
	}
	return plain
}

func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := crypto.GenerateKeyRSA(2048)
	if err != nil {
		t.Fatalf("генерация ключа: %v", err)
	}
	return key
}

var encryptTestMetrics = []*collector.Metric{
	{Type: collector.Gauge, Name: "Alloc", Value: float64(123.5)},
	{Type: collector.Counter, Name: "PollCount", Value: int64(7)},
}

func TestSendBatch_Encrypted(t *testing.T) {
	key := testRSAKey(t)
	const hashKey = "secret"

	srv, requests := recordingServer(t)
	s := NewSender(srv.URL, hashKey)
	s.EnableEncryption(&key.PublicKey)

	if err := s.SendBatch(encryptTestMetrics); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}

	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("ожидали 1 запрос, получили %d", len(reqs))
	}
	got := reqs[0]

	if got.path != "/updates/" {
		t.Errorf("путь: ожидали /updates/, получили %s", got.path)
	}
	if got.header.Get(crypto.EncryptedHeader) == "" {
		t.Errorf("нет заголовка %s", crypto.EncryptedHeader)
	}
	if got.header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding: ожидали gzip, получили %q", got.header.Get("Content-Encoding"))
	}
	if bytes.Contains(got.rawBody, []byte("PollCount")) {
		t.Error("в теле по сети виден открытый JSON")
	}

	plain := decryptAndGunzip(t, key, got.rawBody)

	var metrics []models.Metrics
	if err := json.Unmarshal(plain, &metrics); err != nil {
		t.Fatalf("тело после расшифровки и распаковки не JSON: %v", err)
	}
	if len(metrics) != 2 || metrics[0].ID != "Alloc" || metrics[1].ID != "PollCount" {
		t.Errorf("неожиданные метрики: %+v", metrics)
	}

	// Хеш считается от открытого JSON и должен сойтись после расшифровки и распаковки.
	if want := getExpectedHash(plain, hashKey); got.header.Get("HashSHA256") != want {
		t.Errorf("HashSHA256: ожидали %s, получили %s", want, got.header.Get("HashSHA256"))
	}
}

func TestSendBatch_NotEncryptedByDefault(t *testing.T) {
	srv, requests := recordingServer(t)
	s := NewSender(srv.URL, "")

	if err := s.SendBatch(encryptTestMetrics); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}

	got := requests()[0]
	if got.header.Get(crypto.EncryptedHeader) != "" {
		t.Errorf("без EnableEncryption не должно быть заголовка %s", crypto.EncryptedHeader)
	}
	zr, err := gzip.NewReader(bytes.NewReader(got.rawBody))
	if err != nil {
		t.Fatalf("без шифрования тело должно быть обычным gzip: %v", err)
	}
	plain, _ := io.ReadAll(zr)
	if !bytes.Contains(plain, []byte("PollCount")) {
		t.Error("в распакованном теле нет метрик")
	}
}

func TestSend_TextEndpointPassesThrough(t *testing.T) {
	key := testRSAKey(t)
	srv, requests := recordingServer(t)
	s := NewSender(srv.URL, "")
	s.EnableEncryption(&key.PublicKey)

	if err := s.Send(encryptTestMetrics[:1]); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := requests()[0]
	if got.header.Get(crypto.EncryptedHeader) != "" {
		t.Error("запрос без тела не должен помечаться как зашифрованный")
	}
	if len(got.rawBody) != 0 {
		t.Errorf("тело должно остаться пустым, получили %d байт", len(got.rawBody))
	}
	if got.path != "/update/gauge/Alloc/123.5" {
		t.Errorf("путь изменился: %s", got.path)
	}
}

func TestSendBatch_RetryReencrypts(t *testing.T) {
	key := testRSAKey(t)
	srv, requests := recordingServer(t, http.StatusInternalServerError)
	s := NewSender(srv.URL, "")
	s.EnableEncryption(&key.PublicKey)

	if err := s.SendBatch(encryptTestMetrics); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}

	reqs := requests()
	if len(reqs) != 2 {
		t.Fatalf("ожидали 2 попытки (500, затем 200), получили %d", len(reqs))
	}
	if bytes.Equal(reqs[0].rawBody, reqs[1].rawBody) {
		t.Error("повторная попытка должна шифроваться заново, с новым AES-ключом и nonce")
	}
	first := decryptAndGunzip(t, key, reqs[0].rawBody)
	second := decryptAndGunzip(t, key, reqs[1].rawBody)
	if !bytes.Equal(first, second) {
		t.Error("после расшифровки попытки должны содержать одинаковые данные")
	}
}

// roundTripFunc позволяет подставить функцию вместо следующего транспорта.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// closeTracker — тело запроса, которое запоминает, было ли закрыто.
type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error {
	c.closed = true
	return nil
}

func TestEncryptedTransport_RoundTrip(t *testing.T) {
	key := testRSAKey(t)
	okResponse := &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}

	t.Run("без тела запрос передаётся как есть", func(t *testing.T) {
		for _, body := range []io.ReadCloser{nil, http.NoBody} {
			req := httptest.NewRequest(http.MethodPost, "http://example.com/update/gauge/A/1", nil)
			req.Body = body

			var passed *http.Request
			rt := &EncryptedTransport{
				publicKey: &key.PublicKey,
				nextTransport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					passed = r
					return okResponse, nil
				}),
			}
			resp, err := rt.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip: %v", err)
			}
			resp.Body.Close()
			if passed != req {
				t.Errorf("тело %v: ожидали тот же запрос без клонирования", body)
			}
		}
	})

	t.Run("исходный запрос не меняется, тело закрывается", func(t *testing.T) {
		plain := []byte(`{"id":"Alloc"}`)
		body := &closeTracker{Reader: bytes.NewReader(plain)}
		req, err := http.NewRequest(http.MethodPost, "http://example.com/updates/", body)
		if err != nil {
			t.Fatal(err)
		}

		var sent *http.Request
		rt := &EncryptedTransport{
			publicKey: &key.PublicKey,
			nextTransport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				sent = r
				return okResponse, nil
			}),
		}
		resp, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip: %v", err)
		}
		resp.Body.Close()

		if !body.closed {
			t.Error("тело исходного запроса не закрыто")
		}
		if req.Header.Get(crypto.EncryptedHeader) != "" {
			t.Error("исходный запрос изменён: в нём появился заголовок шифрования")
		}
		if sent == req {
			t.Fatal("следующему транспорту передан исходный запрос, а не клон")
		}

		encrypted, err := io.ReadAll(sent.Body)
		if err != nil {
			t.Fatal(err)
		}
		if sent.ContentLength != int64(len(encrypted)) {
			t.Errorf("ContentLength %d не совпадает с длиной тела %d", sent.ContentLength, len(encrypted))
		}

		decrypted, err := crypto.Decrypt(key, encrypted)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if !bytes.Equal(decrypted, plain) {
			t.Error("расшифрованное тело не совпадает с исходным")
		}

		if sent.GetBody == nil {
			t.Fatal("GetBody не задан")
		}
		again, err := sent.GetBody()
		if err != nil {
			t.Fatal(err)
		}
		againBytes, _ := io.ReadAll(again)
		if !bytes.Equal(againBytes, encrypted) {
			t.Error("GetBody должен возвращать то же зашифрованное тело")
		}
	})
}
