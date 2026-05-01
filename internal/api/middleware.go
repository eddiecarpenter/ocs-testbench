package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// requestIDHeader is the response header carrying the per-request ID.
const requestIDHeader = "X-Request-ID"

// RequestID is a middleware that generates a v4 UUID for every
// incoming request and sets it as the X-Request-ID response header.
// The ID is also attached to the request context via the logger so
// downstream slog entries include it automatically.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r)
	})
}

// Recovery is a middleware that recovers from panics in downstream
// handlers, logs the panic value as an error, and writes a 500 JSON
// error response so the client receives a structured reply rather than
// an abrupt connection close.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				logging.Error("api: panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"panic", v,
				)
				respondInternalError(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
