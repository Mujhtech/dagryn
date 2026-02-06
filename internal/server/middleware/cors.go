package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns a middleware that handles Cross-Origin Resource Sharing.
func CORS() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		// Allow all origins in development, should be restricted in production
		AllowedOrigins: []string{"*"},
		// Allow common HTTP methods
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		// Allow common headers
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-API-Key",
			"X-Request-ID",
		},
		// Expose these headers to the client
		ExposedHeaders: []string{
			"Link",
			"X-Request-ID",
			"X-Total-Count",
		},
		// Allow credentials (cookies, authorization headers)
		AllowCredentials: true,
		// Cache preflight requests for 5 minutes
		MaxAge: 300,
	})
}

// CORSWithOrigins returns a CORS middleware with specific allowed origins.
func CORSWithOrigins(origins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-API-Key",
			"X-Request-ID",
		},
		ExposedHeaders: []string{
			"Link",
			"X-Request-ID",
			"X-Total-Count",
		},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
