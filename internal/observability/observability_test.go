package observability

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type TestHook struct {
	Entries []*logrus.Entry
}

func (hook *TestHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *TestHook) Fire(entry *logrus.Entry) error {
	hook.Entries = append(hook.Entries, entry)
	return nil
}

func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected logrus.Level
	}{
		{
			name:     "default log level (empty string)",
			logLevel: "",
			expected: logrus.InfoLevel,
		},
		{
			name:     "debug log level",
			logLevel: "debug",
			expected: logrus.DebugLevel,
		},
		{
			name:     "warn log level",
			logLevel: "warn",
			expected: logrus.WarnLevel,
		},
		{
			name:     "error log level",
			logLevel: "error",
			expected: logrus.ErrorLevel,
		},
		{
			name:     "invalid log level",
			logLevel: "invalid",
			expected: logrus.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh logger instance for testing to avoid interference
			testLogger := logrus.New()
			originalLogger := log
			log = testLogger // Temporarily replace the global logger

			// Add a test hook to capture log entries
			hook := &TestHook{}
			log.AddHook(hook)

			// Ensure env doesn't influence the default-level case
			t.Setenv("LOG_LEVEL", "")
			// Initialize the logger
			Init(tt.logLevel, Config{EnablePIIRedaction: false})

			// Check if the logger is set to JSON formatter
			_, ok := log.Formatter.(*logrus.JSONFormatter)
			assert.True(t, ok, "Expected JSONFormatter")

			// Check if the logger level is set to expected level
			assert.Equal(t, tt.expected, log.Level, "Expected log level to be %v", tt.expected)

			// Check init log messages (allow future additions)
			assert.GreaterOrEqual(t, len(hook.Entries), 2, "Expected at least two log entries during initialization")
			if len(hook.Entries) >= 2 {
				assert.Contains(t, hook.Entries[0].Message, "Logrus set to JSON formatter", "Expected first log message to contain 'Logrus set to JSON formatter'")
				assert.Contains(t, hook.Entries[1].Message, "Logrus set to output to stdout", "Expected second log message to contain 'Logrus set to output to stdout'")
			}

			// Restore the original logger
			log = originalLogger
		})
	}
}

func TestInfoWithContext(t *testing.T) {
	// Add a test hook to capture log entries
	hook := &TestHook{}
	log.AddHook(hook)

	// Create a context
	ctx := context.Background()

	// Log an info message with context
	InfoWithContext(ctx, "test info message")

	// Check if the message is logged
	assert.Len(t, hook.Entries, 1, "Expected one log entry")
	assert.Contains(t, hook.Entries[0].Message, "test info message", "Expected log message to contain 'test info message'")
}

func TestErrorWithContext(t *testing.T) {
	// Add a test hook to capture log entries
	hook := &TestHook{}
	log.AddHook(hook)

	// Create a context
	ctx := context.Background()

	// Log an error message with context
	ErrorWithContext(ctx, "test error message")

	// Check if the message is logged
	assert.Len(t, hook.Entries, 1, "Expected one log entry")
	assert.Contains(t, hook.Entries[0].Message, "test error message", "Expected log message to contain 'test error message'")
}

func TestRequestLoggingMiddleware(t *testing.T) {
	// Add a test hook to capture log entries
	hook := &TestHook{}
	log.AddHook(hook)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with logging middleware
	loggedHandler := RequestLoggingMiddleware(testHandler)

	t.Run("successful request logging", func(t *testing.T) {
		// Clear previous entries
		hook.Entries = []*logrus.Entry{}

		req := httptest.NewRequest("GET", "/test?param=value", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.Header.Set("X-Request-ID", "test-req-123")
		w := httptest.NewRecorder()

		loggedHandler.ServeHTTP(w, req)

		// Should have at least 2 log entries: request start and request complete
		assert.GreaterOrEqual(t, len(hook.Entries), 2, "Expected at least 2 log entries")

		// Find request start entry
		var startEntry *logrus.Entry
		var completeEntry *logrus.Entry
		for _, entry := range hook.Entries {
			if entry.Data["type"] == "request_start" {
				startEntry = entry
			}
			if entry.Data["type"] == "request_complete" {
				completeEntry = entry
			}
		}

		// Verify request start entry
		assert.NotNil(t, startEntry, "Expected request start entry")
		assert.Equal(t, "GET", startEntry.Data["method"])
		assert.Equal(t, "/test", startEntry.Data["path"])
		assert.Equal(t, "param=value", startEntry.Data["query"])
		assert.Equal(t, "test-agent", startEntry.Data["user_agent"])
		assert.Equal(t, "test-req-123", startEntry.Data["request_id"])

		// Verify request complete entry
		assert.NotNil(t, completeEntry, "Expected request complete entry")
		assert.Equal(t, "GET", completeEntry.Data["method"])
		assert.Equal(t, "/test", completeEntry.Data["path"])
		assert.Equal(t, 200, completeEntry.Data["status"])
		assert.Equal(t, "success", completeEntry.Data["status_class"])
		assert.Contains(t, completeEntry.Data, "duration_ms")
		assert.Equal(t, 13, completeEntry.Data["response_size"]) // "test response" = 13 bytes
	})

	t.Run("error response logging", func(t *testing.T) {
		// Clear previous entries
		hook.Entries = []*logrus.Entry{}

		errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		})
		loggedErrorHandler := RequestLoggingMiddleware(errorHandler)

		req := httptest.NewRequest("POST", "/error", nil)
		w := httptest.NewRecorder()

		loggedErrorHandler.ServeHTTP(w, req)

		// Find the complete entry
		var completeEntry *logrus.Entry
		for _, entry := range hook.Entries {
			if entry.Data["type"] == "request_complete" {
				completeEntry = entry
			}
		}

		assert.NotNil(t, completeEntry, "Expected request complete entry")
		assert.Equal(t, 500, completeEntry.Data["status"])
		assert.Equal(t, "server_error", completeEntry.Data["status_class"])
		assert.Equal(t, logrus.ErrorLevel, completeEntry.Level, "Expected error level for 500 status")
	})

	t.Run("client error logging", func(t *testing.T) {
		// Clear previous entries
		hook.Entries = []*logrus.Entry{}

		clientErrorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		})
		loggedClientErrorHandler := RequestLoggingMiddleware(clientErrorHandler)

		req := httptest.NewRequest("GET", "/bad", nil)
		w := httptest.NewRecorder()

		loggedClientErrorHandler.ServeHTTP(w, req)

		// Find the complete entry
		var completeEntry *logrus.Entry
		for _, entry := range hook.Entries {
			if entry.Data["type"] == "request_complete" {
				completeEntry = entry
			}
		}

		assert.NotNil(t, completeEntry, "Expected request complete entry")
		assert.Equal(t, 400, completeEntry.Data["status"])
		assert.Equal(t, "client_error", completeEntry.Data["status_class"])
		assert.Equal(t, logrus.WarnLevel, completeEntry.Level, "Expected warn level for 400 status")
	})
}

func TestRedactRequestFields(t *testing.T) {
	tests := []struct {
		name              string
		enableRedaction   bool
		rawQuery          string
		clientIP          string
		userAgent         string
		expectedQuery     string
		expectedClientIP  string
		expectedUserAgent string
	}{
		{
			name:              "redaction disabled - returns original values",
			enableRedaction:   false,
			rawQuery:          "page=1&limit=10&secret=password",
			clientIP:          "192.168.1.100",
			userAgent:         "Mozilla/5.0 (Test Browser)",
			expectedQuery:     "page=1&limit=10&secret=password",
			expectedClientIP:  "192.168.1.100",
			expectedUserAgent: "Mozilla/5.0 (Test Browser)",
		},
		{
			name:              "redaction enabled - allowlisted query params",
			enableRedaction:   true,
			rawQuery:          "page=1&limit=10&secret=password",
			clientIP:          "192.168.1.100",
			userAgent:         "Mozilla/5.0 (Test Browser)",
			expectedQuery:     "limit=10&page=1&secret=%3Credacted%3E",
			expectedClientIP:  "192.0.0.0",
			expectedUserAgent: "Mozilla/5.0 (Test Browser)",
		},
		{
			name:              "redaction enabled - long user agent truncated",
			enableRedaction:   true,
			rawQuery:          "",
			clientIP:          "10.0.0.50",
			userAgent:         strings.Repeat("A", 150),
			expectedQuery:     "",
			expectedClientIP:  "10.0.0.0",
			expectedUserAgent: strings.Repeat("A", 100),
		},
		{
			name:              "redaction enabled - IPv6 and empty user agent",
			enableRedaction:   true,
			rawQuery:          "token=abc123",
			clientIP:          "2001:db8::1",
			userAgent:         "",
			expectedQuery:     "token=%3Credacted%3E",
			expectedClientIP:  "redacted",
			expectedUserAgent: "<redacted>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest("GET", "http://example.com/test?"+tt.rawQuery, nil)
			req.Header.Set("User-Agent", tt.userAgent)
			req.RemoteAddr = tt.clientIP + ":12345"

			cfg := Config{EnablePIIRedaction: tt.enableRedaction}

			sanitizedQuery, sanitizedClientIP, sanitizedUserAgent := redactRequestFields(req, cfg)

			assert.Equal(t, tt.expectedQuery, sanitizedQuery, "Query should match expected")
			assert.Equal(t, tt.expectedClientIP, sanitizedClientIP, "Client IP should match expected")
			assert.Equal(t, tt.expectedUserAgent, sanitizedUserAgent, "User Agent should match expected")
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Forwarded-For header",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1, 10.0.0.1"},
			remoteAddr: "127.0.0.1:8080",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Real-IP header",
			headers:    map[string]string{"X-Real-IP": "192.168.1.2"},
			remoteAddr: "127.0.0.1:8080",
			expected:   "192.168.1.2",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.3:8080",
			expected:   "192.168.1.3",
		},
		{
			name:       "RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.4",
			expected:   "192.168.1.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := getClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStatusClass(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{200, "success"},
		{201, "success"},
		{299, "success"},
		{301, "redirect"},
		{302, "redirect"},
		{399, "redirect"},
		{400, "client_error"},
		{404, "client_error"},
		{499, "client_error"},
		{500, "server_error"},
		{503, "server_error"},
		{100, "unknown"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			result := getStatusClass(tt.statusCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineLogLevel(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		duration   time.Duration
		expected   logrus.Level
	}{
		{"server error", 500, 100 * time.Millisecond, logrus.ErrorLevel},
		{"client error", 400, 100 * time.Millisecond, logrus.WarnLevel},
		{"slow request", 200, 2 * time.Second, logrus.WarnLevel},
		{"normal request", 200, 100 * time.Millisecond, logrus.InfoLevel},
		{"fast success", 201, 50 * time.Millisecond, logrus.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineLogLevel(tt.statusCode, tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}
