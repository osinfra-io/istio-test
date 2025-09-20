package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"istio-test/internal/observability"
	"istio-test/internal/security"
)

const (
	ClusterNameURL     = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/cluster-name"
	ClusterLocationURL = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/cluster-location"
	InstanceZoneURL    = "http://metadata.google.internal/computeMetadata/v1/instance/zone"
)

type MetadataFetcher interface {
	FetchMetadata(ctx context.Context, url string) (string, error)
}

// Client holds the HTTP client and configuration for metadata operations
type Client struct {
	httpClient      *http.Client
	maxRetries      int
	baseRetryDelay  time.Duration
	maxRetryDelay   time.Duration
	retryMultiplier float64
}

// NewClient creates a new metadata client with the given configuration
func NewClient(httpTimeout time.Duration, maxRetries int, baseRetryDelay, maxRetryDelay time.Duration, retryMultiplier float64) *Client {
	return &Client{
		httpClient:      &http.Client{Timeout: httpTimeout},
		maxRetries:      maxRetries,
		baseRetryDelay:  baseRetryDelay,
		maxRetryDelay:   maxRetryDelay,
		retryMultiplier: retryMultiplier,
	}
}

// Default client for backward compatibility
var defaultClient = NewClient(10*time.Second, 3, 100*time.Millisecond, 2*time.Second, 2.0)

// FetchMetadata fetches metadata using the default client (for backward compatibility)
func FetchMetadata(ctx context.Context, url string) (string, error) {
	return defaultClient.FetchMetadata(ctx, url)
}

// FetchMetadata fetches metadata from the given URL with retry logic
func (c *Client) FetchMetadata(ctx context.Context, url string) (string, error) {
	var lastErr error
	retryDelay := c.baseRetryDelay

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		// Check if context is already cancelled
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("error creating request: %w", err)
		}

		req.Header.Add("Metadata-Flavor", "Google")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error executing request: %w", err)
			if attempt < c.maxRetries-1 {
				observability.InfoWithContext(ctx, fmt.Sprintf("Metadata fetch attempt %d failed, retrying in %v: %v", attempt+1, retryDelay, err))
				time.Sleep(retryDelay)
				retryDelay = time.Duration(float64(retryDelay) * c.retryMultiplier)
				if retryDelay > c.maxRetryDelay {
					retryDelay = c.maxRetryDelay
				}
				continue
			}
			return "", fmt.Errorf("failed after %d attempts: %w", c.maxRetries, lastErr)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("failed to get metadata from %s, status code: %d, response: %s", url, resp.StatusCode, string(body))

			// Only retry on 5xx errors or 429 (rate limiting)
			if (resp.StatusCode >= 500 && resp.StatusCode < 600) || resp.StatusCode == http.StatusTooManyRequests {
				if attempt < c.maxRetries-1 {
					observability.InfoWithContext(ctx, fmt.Sprintf("Metadata fetch attempt %d failed with status %d, retrying in %v", attempt+1, resp.StatusCode, retryDelay))
					time.Sleep(retryDelay)
					retryDelay = time.Duration(float64(retryDelay) * c.retryMultiplier)
					if retryDelay > c.maxRetryDelay {
						retryDelay = c.maxRetryDelay
					}
					continue
				}
			}
			return "", fmt.Errorf("failed after %d attempts: %w", c.maxRetries, lastErr)
		}

		metadata, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error reading response body: %w", err)
		}

		// Success!
		if attempt > 0 {
			observability.InfoWithContext(ctx, fmt.Sprintf("Metadata fetch succeeded on attempt %d", attempt+1))
		}
		return string(metadata), nil
	}

	return "", fmt.Errorf("failed after %d attempts: %w", c.maxRetries, lastErr)
}

// HealthStatus represents the overall health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthResponse represents the health check response structure
type HealthResponse struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Version   string                 `json:"version"`
	Checks    map[string]HealthCheck `json:"checks"`
}

// HealthCheck represents an individual health check result
type HealthCheck struct {
	Status      HealthStatus `json:"status"`
	Message     string       `json:"message,omitempty"`
	Duration    string       `json:"duration"`
	LastChecked time.Time    `json:"last_checked"`
}

var startTime = time.Now()

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Set content type for health check
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		observability.ErrorWithContext(r.Context(), fmt.Sprintf("Error writing response: %v", err))
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// EnhancedHealthCheckHandler provides comprehensive health checks including dependencies
func EnhancedHealthCheckHandler(metadataClient *Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startCheck := time.Now()

		health := HealthResponse{
			Status:    HealthStatusHealthy,
			Timestamp: time.Now().UTC(),
			Uptime:    time.Since(startTime).String(),
			Version:   "1.0.0", // TODO: Get from build info or environment
			Checks:    make(map[string]HealthCheck),
		}

		// Check metadata service connectivity
		metadataCheck := checkMetadataService(r.Context(), metadataClient)
		health.Checks["metadata_service"] = metadataCheck

		// Check HTTP server responsiveness (implicit since we're responding)
		health.Checks["http_server"] = HealthCheck{
			Status:      HealthStatusHealthy,
			Message:     "HTTP server is responding",
			Duration:    time.Since(startCheck).String(),
			LastChecked: time.Now().UTC(),
		}

		// Determine overall health status
		health.Status = determineOverallHealth(health.Checks)

		// Marshal JSON response first to handle encoding errors before setting status
		jsonData, err := json.Marshal(health)
		if err != nil {
			observability.ErrorWithContext(r.Context(), fmt.Sprintf("Error encoding health response: %v", err))
			http.Error(w, "Failed to encode health response", http.StatusInternalServerError)
			return
		}

		// Set appropriate response headers and status code only after successful encoding
		w.Header().Set("Content-Type", "application/json")
		if health.Status == HealthStatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else if health.Status == HealthStatusDegraded {
			w.WriteHeader(http.StatusOK) // Still OK but with warnings
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Write the pre-encoded JSON data
		if _, err := w.Write(jsonData); err != nil {
			observability.ErrorWithContext(r.Context(), fmt.Sprintf("Error writing health response: %v", err))
		}

		// Log health check result
		observability.InfoWithContext(r.Context(), fmt.Sprintf("Health check completed: %s (took %v)", health.Status, time.Since(startCheck)))
	}
}

// checkMetadataService tests connectivity to the GCP metadata service
func checkMetadataService(ctx context.Context, metadataClient *Client) HealthCheck {
	checkStart := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Try to fetch cluster name as a connectivity test
	_, err := metadataClient.FetchMetadata(checkCtx, ClusterNameURL)
	duration := time.Since(checkStart)

	if err != nil {
		// Check if it's a timeout or other error
		if checkCtx.Err() == context.DeadlineExceeded {
			return HealthCheck{
				Status:      HealthStatusUnhealthy,
				Message:     "Metadata service timeout",
				Duration:    duration.String(),
				LastChecked: time.Now().UTC(),
			}
		}
		return HealthCheck{
			Status:      HealthStatusDegraded,
			Message:     fmt.Sprintf("Metadata service error: %v", err),
			Duration:    duration.String(),
			LastChecked: time.Now().UTC(),
		}
	}

	// Check response time
	if duration > 1*time.Second {
		return HealthCheck{
			Status:      HealthStatusDegraded,
			Message:     "Metadata service responding slowly",
			Duration:    duration.String(),
			LastChecked: time.Now().UTC(),
		}
	}

	return HealthCheck{
		Status:      HealthStatusHealthy,
		Message:     "Metadata service is responsive",
		Duration:    duration.String(),
		LastChecked: time.Now().UTC(),
	}
}

// determineOverallHealth calculates the overall health based on individual checks
func determineOverallHealth(checks map[string]HealthCheck) HealthStatus {
	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	for _, check := range checks {
		switch check.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		case HealthStatusUnhealthy:
			unhealthyCount++
		}
	}

	// If any critical checks are unhealthy, overall status is unhealthy
	if unhealthyCount > 0 {
		return HealthStatusUnhealthy
	}

	// If any checks are degraded, overall status is degraded
	if degradedCount > 0 {
		return HealthStatusDegraded
	}

	// All checks are healthy
	return HealthStatusHealthy
}

// SecureHealthCheckHandler returns a health check handler with security headers and method validation
func SecureHealthCheckHandler() http.HandlerFunc {
	return security.SecureHandler([]string{"GET", "HEAD"}, HealthCheckHandler)
}

// SecureHealthCheckHandlerWithOptions returns a health check handler with configurable security headers and method validation
func SecureHealthCheckHandlerWithOptions(options security.SecurityHeadersOptions) http.HandlerFunc {
	return security.SecureHandlerWithOptions([]string{"GET", "HEAD"}, HealthCheckHandler, options)
}

// SecureEnhancedHealthCheckHandler returns an enhanced health check handler with security headers and method validation
func SecureEnhancedHealthCheckHandler(metadataClient *Client) http.HandlerFunc {
	return security.SecureHandler([]string{"GET", "HEAD"}, EnhancedHealthCheckHandler(metadataClient))
}

// SecureEnhancedHealthCheckHandlerWithOptions returns an enhanced health check handler with configurable security headers and method validation
func SecureEnhancedHealthCheckHandlerWithOptions(metadataClient *Client, options security.SecurityHeadersOptions) http.HandlerFunc {
	return security.SecureHandlerWithOptions([]string{"GET", "HEAD"}, EnhancedHealthCheckHandler(metadataClient), options)
}

func MetadataHandler(fetchMetadataFunc func(ctx context.Context, url string) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		observability.InfoWithContext(r.Context(), fmt.Sprintf("Received request for %s", r.URL.Path))

		cleanPath := strings.TrimSuffix(r.URL.Path, "/")
		pathParts := strings.Split(cleanPath, "/")
		if len(pathParts) != 4 {
			observability.ErrorWithContext(r.Context(), fmt.Sprintf("Invalid request: %s", r.URL.Path))
			http.Error(w, "Invalid request: expected /istio-test/metadata/{type}", http.StatusBadRequest)
			return
		}

		metadataType := pathParts[3]
		var url string

		switch metadataType {
		case "cluster-name":
			url = ClusterNameURL
		case "cluster-location":
			url = ClusterLocationURL
		case "instance-zone":
			url = InstanceZoneURL
		default:
			observability.ErrorWithContext(r.Context(), fmt.Sprintf("Unknown metadata type: %s", metadataType))
			http.Error(w, "Unknown metadata type", http.StatusBadRequest)
			return
		}

		metadata, err := fetchMetadataFunc(r.Context(), url)
		if err != nil {
			observability.ErrorWithContext(r.Context(), fmt.Sprintf("Failed to fetch metadata: %v", err))
			http.Error(w, "Failed to fetch metadata", http.StatusBadGateway)
			return
		}
		if metadataType == "instance-zone" {
			instanceZoneParts := strings.Split(metadata, "/")
			if len(instanceZoneParts) > 0 {
				metadata = instanceZoneParts[len(instanceZoneParts)-1]
			} else {
				observability.ErrorWithContext(r.Context(), fmt.Sprintf("Unexpected format for instance-zone metadata: %s", metadata))
				http.Error(w, "Unexpected format for instance-zone metadata", http.StatusInternalServerError)
				return
			}
		}

		response := map[string]string{metadataType: metadata}
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not Found", http.StatusNotFound)
}

// SecureMetadataHandler returns a metadata handler with security headers and method validation
func SecureMetadataHandler(fetchMetadataFunc func(ctx context.Context, url string) (string, error)) http.HandlerFunc {
	return security.SecureHandler([]string{"GET"}, MetadataHandler(fetchMetadataFunc))
}

// SecureMetadataHandlerWithOptions returns a metadata handler with configurable security headers and method validation
func SecureMetadataHandlerWithOptions(fetchMetadataFunc func(ctx context.Context, url string) (string, error), options security.SecurityHeadersOptions) http.HandlerFunc {
	return security.SecureHandlerWithOptions([]string{"GET"}, MetadataHandler(fetchMetadataFunc), options)
}

// SecureNotFoundHandler returns a not found handler with security headers
func SecureNotFoundHandler() http.HandlerFunc {
	return security.SecurityMiddlewareFunc(NotFoundHandler)
}

// SecureNotFoundHandlerWithOptions returns a not found handler with configurable security headers
func SecureNotFoundHandlerWithOptions(options security.SecurityHeadersOptions) http.HandlerFunc {
	return security.SecurityMiddlewareFuncWithOptions(NotFoundHandler, options)
}
