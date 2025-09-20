package observability

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	dd_logrus "gopkg.in/DataDog/dd-trace-go.v1/contrib/sirupsen/logrus"
)

var log = logrus.New()

// Config holds observability configuration
type Config struct {
	EnablePIIRedaction bool
}

// config holds the current observability configuration
var config Config

func Init(logLevel string, cfg Config) {
	log.SetFormatter(&logrus.JSONFormatter{})
	log.Info("Logrus set to JSON formatter")

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
	log.Info("Logrus set to output to stdout")

	// Parse the provided log level, fallback to environment variable if empty, then to InfoLevel
	var level logrus.Level
	var err error

	if logLevel != "" {
		level, err = logrus.ParseLevel(logLevel)
	} else {
		// Fallback to environment variable for backward compatibility
		level, err = logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	}

	if err != nil {
		level = logrus.InfoLevel
	}

	log.SetLevel(level)

	// Store configuration
	config = cfg

	// Add Datadog context log hook
	log.AddHook(&dd_logrus.DDContextLogHook{})
}

func InfoWithContext(ctx context.Context, msg string) {
	log.WithContext(ctx).Info(msg)
}

func ErrorWithContext(ctx context.Context, msg string) {
	log.WithContext(ctx).Error(msg)
}

func WarnWithContext(ctx context.Context, msg string) {
	log.WithContext(ctx).Warn(msg)
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

// Hijack implements the http.Hijacker interface if the underlying ResponseWriter supports it
func (rw *responseWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

// Flush implements the http.Flusher interface if the underlying ResponseWriter supports it
func (rw *responseWrapper) Flush() {
	flusher, ok := rw.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
	// No-op if flushing is not supported
}

// Push implements the http.Pusher interface if the underlying ResponseWriter supports it
func (rw *responseWrapper) Push(target string, opts *http.PushOptions) error {
	pusher, ok := rw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

// RequestLoggingMiddleware provides comprehensive request/response logging
func RequestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code and size
		wrapper := newResponseWrapper(w)

		// Extract client info with PII redaction
		sanitizedQuery, sanitizedClientIP, sanitizedUserAgent := redactRequestFields(r, config)

		// Log incoming request
		log.WithContext(r.Context()).WithFields(logrus.Fields{
			"type":           "request_start",
			"method":         r.Method,
			"path":           r.URL.Path,
			"query":          sanitizedQuery,
			"client_ip":      sanitizedClientIP,
			"user_agent":     sanitizedUserAgent,
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
			"query":         sanitizedQuery,
			"status":        wrapper.statusCode,
			"status_class":  getStatusClass(wrapper.statusCode),
			"duration_ms":   float64(duration.Nanoseconds()) / 1000000.0,
			"response_size": wrapper.size,
			"client_ip":     sanitizedClientIP,
			"user_agent":    sanitizedUserAgent,
			"request_id":    getRequestID(r),
		})

		message := fmt.Sprintf("HTTP %s %s - %d - %v - %s",
			r.Method, r.URL.Path, wrapper.statusCode, duration, sanitizedClientIP)

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
	// Remove port if present using net.SplitHostPort to handle IPv6 addresses properly
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return strings.TrimSpace(host)
	}
	// If SplitHostPort fails (no port), use RemoteAddr as-is
	return strings.TrimSpace(ip)
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

// redactRequestFields sanitizes request fields to remove PII
// Returns sanitized query params, client IP, and user agent
func redactRequestFields(r *http.Request, cfg Config) (string, string, string) {
	if !cfg.EnablePIIRedaction {
		// Return original values when redaction is disabled
		return r.URL.RawQuery, getClientIP(r), r.Header.Get("User-Agent")
	}

	// Redact query parameters - allowlist only "page" and "limit"
	var sanitizedQuery string
	if r.URL.RawQuery != "" {
		queryValues, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			sanitizedQuery = "<invalid_query>"
		} else {
			sanitizedParams := url.Values{}
			for key, values := range queryValues {
				if key == "page" || key == "limit" {
					sanitizedParams[key] = values
				} else {
					sanitizedParams[key] = []string{"<redacted>"}
				}
			}
			sanitizedQuery = sanitizedParams.Encode()
		}
	}

	// Redact client IP - return first octet + ".0.0.0" for IPv4, or "redacted" for others
	clientIP := getClientIP(r)
	sanitizedIP := "redacted"
	if strings.Contains(clientIP, ".") && !strings.Contains(clientIP, ":") {
		// Looks like IPv4
		parts := strings.Split(clientIP, ".")
		if len(parts) == 4 {
			sanitizedIP = parts[0] + ".0.0.0"
		}
	}

	// Redact user agent - truncate to 100 chars max and replace disallowed values
	userAgent := r.Header.Get("User-Agent")
	sanitizedUserAgent := userAgent
	if len(userAgent) > 100 {
		sanitizedUserAgent = userAgent[:100]
	}
	if sanitizedUserAgent == "" {
		sanitizedUserAgent = "<redacted>"
	}

	return sanitizedQuery, sanitizedIP, sanitizedUserAgent
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
