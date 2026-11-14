package sender

import (
	"bytes"
	"net"
	"net/http"
	"testing"
)

var testAgentIP = net.ParseIP("10.1.2.3")

func TestSetRealIP_AllEndpoints(t *testing.T) {
	srv, requests := recordingServer(t)
	s := NewSender(srv.URL, "")
	s.SetRealIP(testAgentIP)

	if err := s.Send(encryptTestMetrics); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := s.SendJSON(encryptTestMetrics); err != nil {
		t.Fatalf("SendJSON: %v", err)
	}
	if err := s.SendBatch(encryptTestMetrics); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}

	reqs := requests()
	if len(reqs) == 0 {
		t.Fatal("сервер не получил ни одного запроса")
	}
	for _, r := range reqs {
		if got := r.header.Get("X-Real-IP"); got != "10.1.2.3" {
			t.Errorf("%s: X-Real-IP: ожидали 10.1.2.3, получили %q", r.path, got)
		}
	}
}

func TestSetRealIP_WithEncryption(t *testing.T) {
	key := testRSAKey(t)

	srv, requests := recordingServer(t)
	s := NewSender(srv.URL, "")
	s.EnableEncryption(&key.PublicKey)
	s.SetRealIP(testAgentIP)

	if err := s.SendBatch(encryptTestMetrics); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}

	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("ожидали 1 запрос, получили %d", len(reqs))
	}
	got := reqs[0]

	// SetRealIP оборачивает цепочку, а не заменяет её: шифрование и gzip должны остаться.
	if got.header.Get("X-Real-IP") != "10.1.2.3" {
		t.Errorf("X-Real-IP: ожидали 10.1.2.3, получили %q", got.header.Get("X-Real-IP"))
	}
	if bytes.Contains(got.rawBody, []byte("PollCount")) {
		t.Error("в теле по сети виден открытый JSON: шифрование потерялось")
	}
	decryptAndGunzip(t, key, got.rawBody)
}

func TestSendBatch_NoRealIPByDefault(t *testing.T) {
	srv, requests := recordingServer(t)
	s := NewSender(srv.URL, "")

	if err := s.SendBatch(encryptTestMetrics); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}
	if got := requests()[0].header.Get("X-Real-IP"); got != "" {
		t.Errorf("без SetRealIP не должно быть заголовка X-Real-IP, получили %q", got)
	}
}

func TestIPTransport_DoesNotModifyRequest(t *testing.T) {
	var gotIP string
	next := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotIP = r.Header.Get("X-Real-IP")
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: r}, nil
	})

	req, err := http.NewRequest(http.MethodPost, "http://example.invalid/updates/", nil)
	if err != nil {
		t.Fatal(err)
	}
	tr := &IPTransport{nextTransport: next, ip: testAgentIP}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	resp.Body.Close()

	if gotIP != "10.1.2.3" {
		t.Errorf("следующий транспорт получил X-Real-IP %q, ожидали 10.1.2.3", gotIP)
	}
	if req.Header.Get("X-Real-IP") != "" {
		t.Error("RoundTrip изменил исходный запрос")
	}
}
