package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	methodKey    contextKey = "method"
	pathKey      contextKey = "path"
	userIDKey    contextKey = "log_user_id"
)

// Entry represents a single structured log line (Grafana/Loki compatible).
type Entry struct {
	Timestamp  string      `json:"timestamp"`
	Level      string      `json:"level"`
	RequestID  string      `json:"requestId"`
	Method     string      `json:"method"`
	Path       string      `json:"path"`
	Message    string      `json:"message"`
	Attributes interface{} `json:"attributes,omitempty"`
	Metrics    interface{} `json:"metrics,omitempty"`
	Error      interface{} `json:"error,omitempty"`
}

// ─── Context Helpers ───────────────────────────────────────────────────────────

// WithRequestID stores the request ID in context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// WithMethod stores the HTTP method in context.
func WithMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, methodKey, method)
}

// GetMethod retrieves the HTTP method from context.
func GetMethod(ctx context.Context) string {
	if v, ok := ctx.Value(methodKey).(string); ok {
		return v
	}
	return "INTERNAL"
}

// WithPath stores the request path in context.
func WithPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathKey, path)
}

// GetPath retrieves the request path from context.
func GetPath(ctx context.Context) string {
	if v, ok := ctx.Value(pathKey).(string); ok {
		return v
	}
	return ""
}

// WithUserID stores the user ID in the logging context.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID retrieves the user ID from the logging context.
func GetUserID(ctx context.Context) (int64, bool) {
	if v, ok := ctx.Value(userIDKey).(int64); ok {
		return v, true
	}
	return 0, false
}

// ─── Logging Functions ─────────────────────────────────────────────────────────

// emit writes a single JSON log line to stdout.
func emit(entry Entry) {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback: write a plain error message
		fmt.Fprintf(os.Stdout, `{"timestamp":"%s","level":"error","message":"logger marshal error: %s"}`+"\n",
			time.Now().UTC().Format(time.RFC3339Nano), err.Error())
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

// Info emits an info-level log with optional attributes.
func Info(ctx context.Context, message string, attributes interface{}) {
	emit(Entry{
		Level:      "info",
		RequestID:  GetRequestID(ctx),
		Method:     GetMethod(ctx),
		Path:       GetPath(ctx),
		Message:    message,
		Attributes: attributes,
	})
}

// Warn emits a warn-level log with optional metrics.
func Warn(ctx context.Context, message string, metrics interface{}) {
	emit(Entry{
		Level:     "warn",
		RequestID: GetRequestID(ctx),
		Method:    GetMethod(ctx),
		Path:      GetPath(ctx),
		Message:   message,
		Metrics:   metrics,
	})
}

// ErrorLog emits an error-level log with error details.
func ErrorLog(ctx context.Context, message string, errDetail interface{}) {
	emit(Entry{
		Level:     "error",
		RequestID: GetRequestID(ctx),
		Method:    GetMethod(ctx),
		Path:      GetPath(ctx),
		Message:   message,
		Error:     errDetail,
	})
}

// ErrorDetails is a structured error payload for error logs.
type ErrorDetails struct {
	Code    string `json:"code"`
	Details string `json:"details"`
	Stack   string `json:"stack,omitempty"`
}

// QueryAttributes holds DB query log attributes (message must be "Executed query").
type QueryAttributes struct {
	Query        string `json:"query"`
	DurationMs   int64  `json:"duration_ms"`
	RowsAffected int64  `json:"rows_affected"`
}

// SlowQueryMetrics holds warning metrics for slow queries.
type SlowQueryMetrics struct {
	ExecutionTimeMs int64 `json:"executionTimeMs"`
	ThresholdMs     int64 `json:"thresholdMs"`
}

// ─── Startup / Plain Logs ──────────────────────────────────────────────────────

// Infof emits a simple info log without request context (for startup messages).
func Infof(format string, args ...interface{}) {
	emit(Entry{
		Level:   "info",
		Method:  "INTERNAL",
		Path:    "System",
		Message: fmt.Sprintf(format, args...),
	})
}

// Fatalf emits an error log and exits the process.
func Fatalf(format string, args ...interface{}) {
	emit(Entry{
		Level:   "error",
		Method:  "INTERNAL",
		Path:    "System",
		Message: fmt.Sprintf(format, args...),
		Error: ErrorDetails{
			Code:    "FATAL",
			Details: fmt.Sprintf(format, args...),
		},
	})
	os.Exit(1)
}
