package middleware

import (
	"net/http"
	"net/netip"
)

// CheckSubnetMiddleware — middleware для проверки, что IP-адрес из заголовка X-Real-IP
// входит в доверенную подсеть. Отвечает 403, если заголовок отсутствует, не разбирается
// как IP-адрес или адрес не входит в подсеть.
// Если подсеть не задана (нулевой netip.Prefix), пропускает все запросы без проверки.
func CheckSubnetMiddleware(subnet netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !subnet.IsValid() {
				next.ServeHTTP(w, r)
				return
			}
			ipAdr := r.Header.Get("X-Real-IP")
			addr, err := netip.ParseAddr(ipAdr)
			if err != nil {
				http.Error(w, "invalid X-Real-IP header", http.StatusForbidden)
				return
			}

			if !subnet.Contains(addr.Unmap()) {
				http.Error(w, "IP is not in trusted subnet", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})

	}
}
