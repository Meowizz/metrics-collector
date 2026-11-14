package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestCheckSubnetMiddleware(t *testing.T) {
	ipv4 := netip.MustParsePrefix("10.0.0.0/8")
	ipv6 := netip.MustParsePrefix("2001:db8::/32")

	tests := []struct {
		name     string
		subnet   netip.Prefix
		realIP   string
		wantCode int
	}{
		{"подсеть не задана, без заголовка", netip.Prefix{}, "", http.StatusOK},
		{"подсеть не задана, мусор в заголовке", netip.Prefix{}, "not-an-ip", http.StatusOK},
		{"IPv4 внутри подсети", ipv4, "10.1.2.3", http.StatusOK},
		{"IPv4 в форме IPv6 внутри подсети", ipv4, "::ffff:10.1.2.3", http.StatusOK},
		{"IPv4 вне подсети", ipv4, "192.168.0.1", http.StatusForbidden},
		{"IPv6 в IPv4-подсети", ipv4, "2001:db8::1", http.StatusForbidden},
		{"IPv6 внутри подсети", ipv6, "2001:db8::1", http.StatusOK},
		{"IPv6 вне подсети", ipv6, "2001:db9::1", http.StatusForbidden},
		{"нет заголовка", ipv4, "", http.StatusForbidden},
		{"мусор в заголовке", ipv4, "not-an-ip", http.StatusForbidden},
		{"адрес с маской", ipv4, "10.1.2.3/32", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/updates/", nil)
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}

			h := &captureHandler{}
			rec := httptest.NewRecorder()
			CheckSubnetMiddleware(tt.subnet)(h).ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("статус: ожидали %d, получили %d (%s)", tt.wantCode, rec.Code, rec.Body.String())
			}
			if wantCalled := tt.wantCode == http.StatusOK; h.called != wantCalled {
				t.Errorf("вызов хендлера: ожидали %v, получили %v", wantCalled, h.called)
			}
		})
	}
}
