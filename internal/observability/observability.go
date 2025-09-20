package observability

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	dd_logrus "gopkg.in/DataDog/dd-trace-go.v1/contrib/sirupsen/logrus"
)

var log = logrus.New()

func Init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	log.Info("Logrus set to JSON formatter")

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
	log.Info("Logrus set to output to stdout")

	// Only log the info severity or above
	level, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))

	if err != nil {
		level = logrus.InfoLevel
	}

	log.SetLevel(level)

	// Add Datadog context log hook
	log.AddHook(&dd_logrus.DDContextLogHook{})
}

func InfoWithContext(ctx context.Context, msg string) {
	log.WithContext(ctx).Info(msg)
}

func ErrorWithContext(ctx context.Context, msg string) {
	log.WithContext(ctx).Error(msg)
}

// responseWrapper wraps http.ResponseWriter to capture response status and size
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// newResponseWrapper creates a new response wrapper
func newResponseWrapper(w http.ResponseWriter) *responseWrapper {
	return &responseWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default to 200 if WriteHeader is not called
		size:           0,
	}
}

// WriteHeader captures the status code
func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size and writes the data
func (rw *responseWrapper) Write(data []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(data)
	rw.size += size
	return size, err
}

// RequestLoggingMiddleware provides comprehensive request/response logging
func RequestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code and size
		wrapper := newResponseWrapper(w)

		// Extract client info
		clientIP := getClientIP(r)
		userAgent := r.Header.Get("User-Agent")

		// Log incoming request
		log.WithContext(r.Context()).WithFields(logrus.Fields{
			"type":           "request_start",
			"method":         r.Method,
			"path":           r.URL.Path,
			"query":          r.URL.RawQuery,
			"client_ip":      clientIP,
			"user_agent":     userAgent,
			"request_id":     getRequestID(r),
			"content_length": r.ContentLength,
		}).Info("HTTP request started")

		// Process request
		next.ServeHTTP(wrapper, r)

		// Calculate duration
		duration := time.Since(start)

		// Determine log level based on status code and duration
		logLevel := determineLogLevel(wrapper.statusCode, duration)

		// Log response
		logEntry := log.WithContext(r.Context()).WithFields(logrus.Fields{
			"type":          "request_complete",
			"method":        r.Method,
			"path":          r.URL.Path,
			"query":         r.URL.RawQuery,
			"status":        wrapper.statusCode,
			"status_class":  getStatusClass(wrapper.statusCode),
			"duration_ms":   float64(duration.Nanoseconds()) / 1000000.0,
			"response_size": wrapper.size,
			"client_ip":     clientIP,
			"user_agent":    userAgent,
			"request_id":    getRequestID(r),
		})

		message := fmt.Sprintf("HTTP %s %s - %d - %v - %s",
			r.Method, r.URL.Path, wrapper.statusCode, duration, clientIP)

		switch logLevel {
		case logrus.ErrorLevel:
			logEntry.Error(message)
		case logrus.WarnLevel:
			logEntry.Warn(message)
		default:
			logEntry.Info(message)
		}
	})
}

// RequestLoggingMiddlewareFunc provides request/response logging for handler functions
func RequestLoggingMiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return RequestLoggingMiddleware(http.HandlerFunc(next)).ServeHTTP
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common in reverse proxy setups)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header (common in nginx setups)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		return ip[:idx]
	}
	return ip
}

// getRequestID extracts or generates a request ID for tracing
func getRequestID(r *http.Request) string {
	// Check common request ID headers
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		return reqID
	}
	if reqID := r.Header.Get("X-Correlation-ID"); reqID != "" {
		return reqID
	}
	if reqID := r.Header.Get("X-Trace-ID"); reqID != "" {
		return reqID
	}

	// If no request ID is found, generate a simple one based on timestamp
	// In production, you might want to use a proper UUID library
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// getStatusClass returns a human-readable status class
func getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "success"
	case statusCode >= 300 && statusCode < 400:
		return "redirect"
	case statusCode >= 400 && statusCode < 500:
		return "client_error"
	case statusCode >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

// determineLogLevel determines the appropriate log level based on response status and duration
func determineLogLevel(statusCode int, duration time.Duration) logrus.Level {
	// Log server errors as errors
	if statusCode >= 500 {
		return logrus.ErrorLevel
	}

	// Log client errors and slow requests as warnings
	if statusCode >= 400 || duration > 1*time.Second {
		return logrus.WarnLevel
	}

	// Everything else as info
	return logrus.InfoLevel
}
