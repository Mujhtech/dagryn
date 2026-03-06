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

// uploadBodyThreshold is the Content-Length above which a request is treated as
// an upload and given a longer timeout.
const uploadBodyThreshold int64 = 1 << 20 // 1 MB

// AdaptiveTimeout applies a standard timeout for most requests but a longer
// timeout for uploads (PUT/POST with a body larger than 1 MB) and SSE streams.
func AdaptiveTimeout(standard, upload time.Duration) func(http.Handler) http.Handler {
	stdMW := middleware.Timeout(standard)
	uploadMW := middleware.Timeout(upload)
	return func(next http.Handler) http.Handler {
		stdHandler := stdMW(next)
		uploadHandler := uploadMW(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isLongLivedRequest(r) {
				uploadHandler.ServeHTTP(w, r)
				return
			}
			stdHandler.ServeHTTP(w, r)
		})
	}
}

// isLongLivedRequest returns true for requests that need a longer timeout:
// large uploads, binary streams, and SSE endpoints.
func isLongLivedRequest(r *http.Request) bool {
	// Large request bodies (cache/artifact uploads)
	if (r.Method == http.MethodPut || r.Method == http.MethodPost) && r.ContentLength > uploadBodyThreshold {
		return true
	}

	// Multipart form uploads (artifacts) — Content-Length includes all parts
	ct := r.Header.Get("Content-Type")
	if r.Method == http.MethodPost && len(ct) > 0 {
		if len(ct) >= 9 && ct[:9] == "multipart" {
			return true
		}
	}

	// SSE streams and download endpoints need extended write time
	accept := r.Header.Get("Accept")
	return accept == "text/event-stream"
}
