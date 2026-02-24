package middleware

import (
	"net"
	"net/http"
	"strings"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
)

// AuditContext is a middleware that extracts IP address, user-agent, and request ID
// from the request and adds them to the context for audit logging.
func AuditContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract client IP from X-Forwarded-For, X-Real-IP, or RemoteAddr.
		ip := extractClientIP(r)
		ctx = apiCtx.WithIPAddress(ctx, ip)

		// Extract user agent.
		ctx = apiCtx.WithUserAgent(ctx, r.UserAgent())

		// Extract request ID (chi middleware sets X-Request-Id).
		requestID := r.Header.Get("X-Request-Id")
		if requestID != "" {
			ctx = apiCtx.WithRequestID(ctx, requestID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (may contain multiple IPs).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (client) IP.
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr (strip port). Use net.SplitHostPort for proper IPv6 support.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
