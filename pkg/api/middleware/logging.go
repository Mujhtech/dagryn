// Package middleware provides HTTP middleware for the Dagryn API.
package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// RequestID is a middleware that injects a request ID into the context.
var RequestID = middleware.RequestID

// RealIP is a middleware that sets the RemoteAddr to the real IP.
var RealIP = middleware.RealIP

// Recoverer is a middleware that recovers from panics.
var Recoverer = middleware.Recoverer

// Logger returns a middleware that logs HTTP requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			// Log request
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("duration", time.Since(start)).
				Str("request_id", middleware.GetReqID(r.Context())).
				Msg("HTTP request")
		}()

		next.ServeHTTP(ww, r)
	})
}

// Timeout returns a middleware that times out requests.
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return middleware.Timeout(timeout)
}
