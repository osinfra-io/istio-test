package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MockFetchMetadata struct{}

func (m *MockFetchMetadata) FetchMetadata(ctx context.Context, url string) (string, error) {
	switch url {
	case ClusterNameURL:
		return "test-cluster-name", nil
	case ClusterLocationURL:
		return "test-cluster-location", nil
	case InstanceZoneURL:
		return "projects/1234567890/zones/us-central1-a", nil
	default:
		return "", fmt.Errorf("unknown URL: %s", url)
	}
}

func metadataHandlerWrapper(fetcher MetadataFetcher) http.HandlerFunc {
	return MetadataHandler(fetcher.FetchMetadata)
}

func TestMain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/istio-test/metadata/", metadataHandlerWrapper(&MockFetchMetadata{}))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	os.Setenv("PORT", ts.Listener.Addr().String())
	defer os.Unsetenv("PORT")

	tests := []struct {
		url          string
		expectedCode int
		expectedBody string
		isJSON       bool
	}{
		{ts.URL + "/istio-test/metadata/cluster-name", http.StatusOK, `{"cluster-name":"test-cluster-name"}`, true},
		{ts.URL + "/istio-test/metadata/cluster-location", http.StatusOK, `{"cluster-location":"test-cluster-location"}`, true},
		{ts.URL + "/istio-test/metadata/instance-zone", http.StatusOK, `{"instance-zone":"us-central1-a"}`, true},
		{ts.URL + "/istio-test/metadata/unknown", http.StatusBadRequest, "Unknown metadata type\n", false},
	}

	// Run the test cases
	for _, test := range tests {
		resp, err := http.Get(test.url)
		assert.NoError(t, err)
		assert.Equal(t, test.expectedCode, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		defer resp.Body.Close()

		if test.isJSON {
			assert.JSONEq(t, test.expectedBody, string(body))
		} else {
			assert.Equal(t, test.expectedBody, string(body))
		}
	}
}

func TestHealthCheckHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	HealthCheckHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "OK", string(body))
}

func TestEnhancedHealthCheckHandler(t *testing.T) {
	t.Run("basic health check response structure", func(t *testing.T) {
		// Create a mock metadata client with a short timeout for testing
		mockClient := NewClient(1*time.Second, 1, 50*time.Millisecond, 500*time.Millisecond, 2.0)

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler := EnhancedHealthCheckHandler(mockClient)
		handler(w, req)

		resp := w.Result()
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var healthResp HealthResponse
		err := json.NewDecoder(resp.Body).Decode(&healthResp)
		assert.NoError(t, err)

		// Verify response structure
		assert.Contains(t, []HealthStatus{HealthStatusHealthy, HealthStatusDegraded, HealthStatusUnhealthy}, healthResp.Status)
		assert.Contains(t, healthResp.Checks, "metadata_service")
		assert.Contains(t, healthResp.Checks, "http_server")
		assert.NotEmpty(t, healthResp.Uptime)
		assert.Equal(t, "1.0.0", healthResp.Version)
		assert.NotZero(t, healthResp.Timestamp)

		// HTTP server check should always be healthy since we're responding
		assert.Equal(t, HealthStatusHealthy, healthResp.Checks["http_server"].Status)
		assert.Contains(t, healthResp.Checks["http_server"].Message, "responding")
	})
}

func TestDetermineOverallHealth(t *testing.T) {
	tests := []struct {
		name     string
		checks   map[string]HealthCheck
		expected HealthStatus
	}{
		{
			name: "all healthy",
			checks: map[string]HealthCheck{
				"service1": {Status: HealthStatusHealthy},
				"service2": {Status: HealthStatusHealthy},
			},
			expected: HealthStatusHealthy,
		},
		{
			name: "one degraded",
			checks: map[string]HealthCheck{
				"service1": {Status: HealthStatusHealthy},
				"service2": {Status: HealthStatusDegraded},
			},
			expected: HealthStatusDegraded,
		},
		{
			name: "one unhealthy",
			checks: map[string]HealthCheck{
				"service1": {Status: HealthStatusHealthy},
				"service2": {Status: HealthStatusUnhealthy},
			},
			expected: HealthStatusUnhealthy,
		},
		{
			name: "mixed with unhealthy",
			checks: map[string]HealthCheck{
				"service1": {Status: HealthStatusHealthy},
				"service2": {Status: HealthStatusDegraded},
				"service3": {Status: HealthStatusUnhealthy},
			},
			expected: HealthStatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineOverallHealth(tt.checks)
			assert.Equal(t, tt.expected, result)
		})
	}
}
