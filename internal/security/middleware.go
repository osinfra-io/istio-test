package security

import (
	"net/http"
)

// SecurityMiddleware wraps an HTTP handler with security headers
func SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set security headers
		setSecurityHeaders(w)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// SecurityMiddlewareFunc wraps an HTTP handler function with security headers
func SecurityMiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set security headers
		setSecurityHeaders(w)

		// Call the next handler
		next.ServeHTTP(w, r)
	}
}

// MethodValidationMiddleware ensures only specified HTTP methods are allowed
func MethodValidationMiddleware(allowedMethods ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the method is allowed
			methodAllowed := false
			for _, method := range allowedMethods {
				if r.Method == method {
					methodAllowed = true
					break
				}
			}

			if !methodAllowed {
				setSecurityHeaders(w)
				w.Header().Set("Allow", joinMethods(allowedMethods))
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}

			// Method is allowed, proceed with security headers and next handler
			setSecurityHeaders(w)
			next.ServeHTTP(w, r)
		})
	}
}

// MethodValidationMiddlewareFunc ensures only specified HTTP methods are allowed for handler functions
func MethodValidationMiddlewareFunc(allowedMethods ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Check if the method is allowed
			methodAllowed := false
			for _, method := range allowedMethods {
				if r.Method == method {
					methodAllowed = true
					break
				}
			}

			if !methodAllowed {
				setSecurityHeaders(w)
				w.Header().Set("Allow", joinMethods(allowedMethods))
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}

			// Method is allowed, proceed with security headers and next handler
			setSecurityHeaders(w)
			next.ServeHTTP(w, r)
		}
	}
}

// setSecurityHeaders adds comprehensive security headers to the response
func setSecurityHeaders(w http.ResponseWriter) {
	headers := w.Header()

	// Prevent MIME type sniffing
	headers.Set("X-Content-Type-Options", "nosniff")

	// Prevent clickjacking attacks
	headers.Set("X-Frame-Options", "DENY")

	// Enable XSS protection
	headers.Set("X-XSS-Protection", "1; mode=block")

	// Prevent information leakage through referrer header
	headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Content Security Policy for additional protection
	// This is restrictive but appropriate for an API service
	headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

	// Prevent caching of sensitive responses (can be overridden by specific handlers if needed)
	headers.Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")

	// Server identification (security through obscurity)
	headers.Set("Server", "istio-test")

	// Additional security headers for modern browsers
	headers.Set("X-Permitted-Cross-Domain-Policies", "none")
	headers.Set("Cross-Origin-Embedder-Policy", "require-corp")
	headers.Set("Cross-Origin-Opener-Policy", "same-origin")
	headers.Set("Cross-Origin-Resource-Policy", "same-origin")
}

// joinMethods joins allowed methods with comma separator for Allow header
func joinMethods(methods []string) string {
	if len(methods) == 0 {
		return ""
	}

	result := methods[0]
	for i := 1; i < len(methods); i++ {
		result += ", " + methods[i]
	}
	return result
}

// SecureHandler wraps a handler function with both security headers and method validation
func SecureHandler(allowedMethods []string, handler http.HandlerFunc) http.HandlerFunc {
	return MethodValidationMiddlewareFunc(allowedMethods...)(SecurityMiddlewareFunc(handler))
}
