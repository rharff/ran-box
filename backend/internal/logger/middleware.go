package logger

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Flush supports streaming responses.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Middleware injects requestId, method, and path into the context,
// logs the incoming request and the completed response.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate or reuse X-Request-Id header
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Inject into context
		ctx := WithRequestID(r.Context(), requestID)
		ctx = WithMethod(ctx, r.Method)
		ctx = WithPath(ctx, r.URL.Path)

		// Set response header for tracing
		w.Header().Set("X-Request-Id", requestID)

		// Log incoming request
		Info(ctx, "Incoming request", map[string]interface{}{
			"remote_addr":    r.RemoteAddr,
			"user_agent":     r.UserAgent(),
			"content_length": r.ContentLength,
			"query":          r.URL.RawQuery,
		})

		wrapped := newResponseWriter(w)
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		duration := time.Since(start)

		// Log completed response
		attrs := map[string]interface{}{
			"status_code":  wrapped.statusCode,
			"duration_ms":  duration.Milliseconds(),
			"bytes_written": wrapped.written,
		}

		if wrapped.statusCode >= 500 {
			ErrorLog(ctx, fmt.Sprintf("Request completed with server error %d", wrapped.statusCode), ErrorDetails{
				Code:    fmt.Sprintf("HTTP_%d", wrapped.statusCode),
				Details: fmt.Sprintf("%s %s responded %d in %dms", r.Method, r.URL.Path, wrapped.statusCode, duration.Milliseconds()),
			})
		} else if wrapped.statusCode >= 400 {
			Warn(ctx, fmt.Sprintf("Request completed with client error %d", wrapped.statusCode), attrs)
		} else {
			Info(ctx, "Request completed", attrs)
		}

		// Slow request warning (> 5 seconds)
		if duration.Milliseconds() > 5000 {
			Warn(ctx, "Slow request detected", SlowQueryMetrics{
				ExecutionTimeMs: duration.Milliseconds(),
				ThresholdMs:     5000,
			})
		}
	})
}
