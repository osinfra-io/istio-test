package security

import (
	"net/http"
)

// SecurityHeadersOptions defines configurable security header policies
type SecurityHeadersOptions struct {
	// Cross-Origin policies
	COEP string // Cross-Origin-Embedder-Policy: "", "require-corp", or "credentialless"
	COOP string // Cross-Origin-Opener-Policy: "same-origin", "same-origin-allow-popups", or "unsafe-none"
	CORP string // Cross-Origin-Resource-Policy: "same-origin", "same-site", or "cross-origin"

	// Other configurable headers (future extensibility)
	EnableStrictCSP bool   // Whether to use strict Content Security Policy
	CacheControl    string // Custom Cache-Control header (empty uses default)
}

// StrictSecurityOptions returns the most restrictive security options (original behavior)
func StrictSecurityOptions() SecurityHeadersOptions {
	return SecurityHeadersOptions{
		COEP:            "require-corp",
		COOP:            "same-origin",
		CORP:            "same-origin",
		EnableStrictCSP: true,
		CacheControl:    "", // Use default
	}
}

// APISecurityOptions returns less restrictive options suitable for public API endpoints
func APISecurityOptions() SecurityHeadersOptions {
	return SecurityHeadersOptions{
		COEP:            "", // Don't set COEP header to avoid blocking cross-origin requests
		COOP:            "same-origin-allow-popups",
		CORP:            "cross-origin", // Allow cross-origin requests
		EnableStrictCSP: false,
		CacheControl:    "", // Use default
	}
}

// CustomSecurityOptions creates options from config values
func CustomSecurityOptions(coep, coop, corp string) SecurityHeadersOptions {
	return SecurityHeadersOptions{
		COEP:            coep,
		COOP:            coop,
		CORP:            corp,
		EnableStrictCSP: false, // Default to less strict for custom configs
		CacheControl:    "",
	}
}

// SecurityMiddleware wraps an HTTP handler with security headers using strict options
func SecurityMiddleware(next http.Handler) http.Handler {
	return SecurityMiddlewareWithOptions(next, StrictSecurityOptions())
}

// SecurityMiddlewareWithOptions wraps an HTTP handler with configurable security headers
func SecurityMiddlewareWithOptions(next http.Handler, options SecurityHeadersOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set security headers with options
		setSecurityHeadersWithOptions(w, options)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// SecurityMiddlewareFunc wraps an HTTP handler function with security headers using strict options
func SecurityMiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return SecurityMiddlewareFuncWithOptions(next, StrictSecurityOptions())
}

// SecurityMiddlewareFuncWithOptions wraps an HTTP handler function with configurable security headers
func SecurityMiddlewareFuncWithOptions(next http.HandlerFunc, options SecurityHeadersOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set security headers with options
		setSecurityHeadersWithOptions(w, options)

		// Call the next handler
		next.ServeHTTP(w, r)
	}
}

// MethodValidationMiddleware ensures only specified HTTP methods are allowed using strict security options
func MethodValidationMiddleware(allowedMethods ...string) func(http.Handler) http.Handler {
	return MethodValidationMiddlewareWithOptions(StrictSecurityOptions(), allowedMethods...)
}

// MethodValidationMiddlewareWithOptions ensures only specified HTTP methods are allowed with configurable security headers
func MethodValidationMiddlewareWithOptions(options SecurityHeadersOptions, allowedMethods ...string) func(http.Handler) http.Handler {
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
				setSecurityHeadersWithOptions(w, options)
				w.Header().Set("Allow", joinMethods(allowedMethods))
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}

			// Method is allowed, proceed with security headers and next handler
			setSecurityHeadersWithOptions(w, options)
			next.ServeHTTP(w, r)
		})
	}
}

// MethodValidationMiddlewareFunc ensures only specified HTTP methods are allowed for handler functions using strict security options
func MethodValidationMiddlewareFunc(allowedMethods ...string) func(http.HandlerFunc) http.HandlerFunc {
	return MethodValidationMiddlewareFuncWithOptions(StrictSecurityOptions(), allowedMethods...)
}

// MethodValidationMiddlewareFuncWithOptions ensures only specified HTTP methods are allowed for handler functions with configurable security headers
func MethodValidationMiddlewareFuncWithOptions(options SecurityHeadersOptions, allowedMethods ...string) func(http.HandlerFunc) http.HandlerFunc {
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
				setSecurityHeadersWithOptions(w, options)
				w.Header().Set("Allow", joinMethods(allowedMethods))
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}

			// Method is allowed, proceed with security headers and next handler
			setSecurityHeadersWithOptions(w, options)
			next.ServeHTTP(w, r)
		}
	}
}

// setSecurityHeaders adds comprehensive security headers to the response
func setSecurityHeaders(w http.ResponseWriter) {
	// Use default strict options for backward compatibility
	setSecurityHeadersWithOptions(w, StrictSecurityOptions())
}

// setSecurityHeadersWithOptions adds security headers based on the provided options
func setSecurityHeadersWithOptions(w http.ResponseWriter, options SecurityHeadersOptions) {
	headers := w.Header()

	// Always set these fundamental security headers
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("X-Frame-Options", "DENY")
	headers.Set("X-XSS-Protection", "1; mode=block")
	headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	headers.Set("X-Permitted-Cross-Domain-Policies", "none")
	headers.Set("Server", "istio-test")

	// Content Security Policy - configurable strictness
	if options.EnableStrictCSP {
		headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	} else {
		// Less restrictive CSP for API endpoints
		headers.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
	}

	// Cache Control - configurable or default
	if options.CacheControl != "" {
		headers.Set("Cache-Control", options.CacheControl)
	} else {
		headers.Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
	}
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")

	// Cross-Origin policies - only set if specified (non-empty)
	if options.COEP != "" {
		headers.Set("Cross-Origin-Embedder-Policy", options.COEP)
	}

	if options.COOP != "" {
		headers.Set("Cross-Origin-Opener-Policy", options.COOP)
	}

	if options.CORP != "" {
		headers.Set("Cross-Origin-Resource-Policy", options.CORP)
	}
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

// SecureHandler wraps a handler function with both security headers and method validation using strict security options
func SecureHandler(allowedMethods []string, handler http.HandlerFunc) http.HandlerFunc {
	return SecureHandlerWithOptions(allowedMethods, handler, StrictSecurityOptions())
}

// SecureHandlerWithOptions wraps a handler function with both security headers and method validation using configurable security options
func SecureHandlerWithOptions(allowedMethods []string, handler http.HandlerFunc, options SecurityHeadersOptions) http.HandlerFunc {
	return MethodValidationMiddlewareFuncWithOptions(options, allowedMethods...)(SecurityMiddlewareFuncWithOptions(handler, options))
}
