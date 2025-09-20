package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityMiddleware(t *testing.T) {
	// Test handler that just returns OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with security middleware
	secureHandler := SecurityMiddleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	secureHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify security headers
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"X-Xss-Protection":                  "1; mode=block", // Go canonicalizes X-XSS-Protection to X-Xss-Protection
		"Referrer-Policy":                   "strict-origin-when-cross-origin",
		"Content-Security-Policy":           "default-src 'none'; frame-ancestors 'none'",
		"Cache-Control":                     "no-cache, no-store, must-revalidate, private",
		"Pragma":                            "no-cache",
		"Expires":                           "0",
		"Server":                            "istio-test",
		"X-Permitted-Cross-Domain-Policies": "none",
		"Cross-Origin-Embedder-Policy":      "require-corp",
		"Cross-Origin-Opener-Policy":        "same-origin",
		"Cross-Origin-Resource-Policy":      "same-origin",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Expected header %s to be %q, got %q", header, expectedValue, actualValue)
		}
	}
}

func TestSecurityMiddlewareFunc(t *testing.T) {
	// Test handler function
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	// Wrap with security middleware
	secureHandler := SecurityMiddlewareFunc(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	secureHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify at least one security header is present
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Expected X-Content-Type-Options header to be set")
	}
}

func TestMethodValidationMiddleware(t *testing.T) {
	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create middleware that only allows GET and POST
	middleware := MethodValidationMiddleware("GET", "POST")
	secureHandler := middleware(testHandler)

	t.Run("allowed method GET", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for GET, got %d", w.Code)
		}

		// Should have security headers
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("Expected security headers to be set for allowed method")
		}
	})

	t.Run("allowed method POST", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for POST, got %d", w.Code)
		}
	})

	t.Run("disallowed method DELETE", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405 for DELETE, got %d", w.Code)
		}

		// Should have Allow header
		allowHeader := w.Header().Get("Allow")
		if allowHeader != "GET, POST" {
			t.Errorf("Expected Allow header to be 'GET, POST', got %q", allowHeader)
		}

		// Should still have security headers
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("Expected security headers to be set even for disallowed method")
		}
	})

	t.Run("disallowed method PUT", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405 for PUT, got %d", w.Code)
		}
	})
}

func TestMethodValidationMiddlewareFunc(t *testing.T) {
	// Test handler function
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	// Create middleware that only allows GET
	middleware := MethodValidationMiddlewareFunc("GET")
	secureHandler := middleware(testHandler)

	t.Run("allowed method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("disallowed method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}

		// Should have Allow header
		allowHeader := w.Header().Get("Allow")
		if allowHeader != "GET" {
			t.Errorf("Expected Allow header to be 'GET', got %q", allowHeader)
		}
	})
}

func TestSecureHandler(t *testing.T) {
	// Test handler function
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	// Create secure handler that allows only GET and HEAD
	secureHandler := SecureHandler([]string{"GET", "HEAD"}, testHandler)

	t.Run("allowed method GET", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Should have security headers
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("Expected security headers to be set")
		}
	})

	t.Run("allowed method HEAD", func(t *testing.T) {
		req := httptest.NewRequest("HEAD", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("disallowed method POST", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		secureHandler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}

		// Should have Allow header
		allowHeader := w.Header().Get("Allow")
		if allowHeader != "GET, HEAD" {
			t.Errorf("Expected Allow header to be 'GET, HEAD', got %q", allowHeader)
		}

		// Should still have security headers
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("Expected security headers to be set even for disallowed method")
		}
	})
}

func TestJoinMethods(t *testing.T) {
	tests := []struct {
		name     string
		methods  []string
		expected string
	}{
		{
			name:     "empty slice",
			methods:  []string{},
			expected: "",
		},
		{
			name:     "single method",
			methods:  []string{"GET"},
			expected: "GET",
		},
		{
			name:     "two methods",
			methods:  []string{"GET", "POST"},
			expected: "GET, POST",
		},
		{
			name:     "multiple methods",
			methods:  []string{"GET", "POST", "PUT", "DELETE"},
			expected: "GET, POST, PUT, DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinMethods(tt.methods)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSetSecurityHeaders(t *testing.T) {
	// Create a test response writer
	w := httptest.NewRecorder()

	// Call setSecurityHeaders
	setSecurityHeaders(w)

	// Define expected headers and their values (using canonicalized header names)
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"X-Xss-Protection":                  "1; mode=block", // Go canonicalizes X-XSS-Protection to X-Xss-Protection
		"Referrer-Policy":                   "strict-origin-when-cross-origin",
		"Content-Security-Policy":           "default-src 'none'; frame-ancestors 'none'",
		"Cache-Control":                     "no-cache, no-store, must-revalidate, private",
		"Pragma":                            "no-cache",
		"Expires":                           "0",
		"Server":                            "istio-test",
		"X-Permitted-Cross-Domain-Policies": "none",
		"Cross-Origin-Embedder-Policy":      "require-corp",
		"Cross-Origin-Opener-Policy":        "same-origin",
		"Cross-Origin-Resource-Policy":      "same-origin",
	}

	// Check that all expected headers are set with correct values
	foundHeaders := make([]string, 0)
	for header, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Header %s: expected %q, got %q", header, expectedValue, actualValue)
		} else {
			foundHeaders = append(foundHeaders, header)
		}
	}

	// Debug: print headers only if there's a mismatch
	if len(foundHeaders) != len(expectedHeaders) {
		t.Logf("Expected headers: %d", len(expectedHeaders))
		t.Logf("Found headers: %d", len(foundHeaders))
		t.Logf("All response headers:")
		for header, values := range w.Header() {
			t.Logf("  %s: %v", header, values)
		}
	}

	// Verify that we have the expected number of security headers
	headerCount := 0
	for header := range w.Header() {
		if _, isExpected := expectedHeaders[header]; isExpected {
			headerCount++
		}
	}

	if headerCount != len(expectedHeaders) {
		t.Errorf("Expected %d security headers, but found %d", len(expectedHeaders), headerCount)
	}
}
