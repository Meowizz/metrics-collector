package middleware

import (
	"context"
	"net/http"
	"net/netip"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)


type ContextKey string
// ClientIPKey — ключ для сохранения IP клиента в контексте.
const ClientIPKey ContextKey = "client-ip"

// isIPTrustedInSubnet — крошечная общая функция проверки.
// Возвращает true, если IP валиден и входит в подсеть.
func isIPTrustedInSubnet(ipStr string, subnet netip.Prefix) bool {
	if !subnet.IsValid() {
		return true
	}

	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return false
	}
	return subnet.Contains(addr.Unmap())
}

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

// GRPCAuthInterceptor — gRPC interceptor, который использует ТОЧНО ТУ ЖЕ проверку.
func GRPCAuthInterceptor(subnet netip.Prefix) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var ipStr string

		if md, ok := metadata.FromIncomingContext(ctx); ok {
			ips := md.Get("x-real-ip")
			if len(ips) > 0 {
				ipStr = ips[0]
			}
		}

		if !isIPTrustedInSubnet(ipStr, subnet) {
			return nil, status.Errorf(codes.PermissionDenied, "access denied for IP: %s", ipStr)
		}

		if ipStr != "" {
			ctx = context.WithValue(ctx, ClientIPKey, ipStr)
		}

		return handler(ctx, req)
	}
}
