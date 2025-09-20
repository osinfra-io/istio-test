package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"PORT", "SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT", "SERVER_IDLE_TIMEOUT",
		"METADATA_HTTP_TIMEOUT", "METADATA_MAX_RETRIES", "METADATA_BASE_RETRY_DELAY",
		"METADATA_MAX_RETRY_DELAY", "METADATA_RETRY_MULTIPLIER",
		"LOG_LEVEL", "ENABLE_PROFILER", "ENABLE_TRACING", "SHUTDOWN_TIMEOUT",
	}

	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		// Restore original environment
		for _, env := range envVars {
			if val, exists := originalEnv[env]; exists && val != "" {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	t.Run("default values", func(t *testing.T) {
		conf := Load()

		// Test server defaults
		if conf.Server.Port != "8080" {
			t.Errorf("Expected default port 8080, got %s", conf.Server.Port)
		}
		if conf.Server.ReadTimeout != 5*time.Second {
			t.Errorf("Expected default read timeout 5s, got %v", conf.Server.ReadTimeout)
		}
		if conf.Server.WriteTimeout != 10*time.Second {
			t.Errorf("Expected default write timeout 10s, got %v", conf.Server.WriteTimeout)
		}
		if conf.Server.IdleTimeout != 60*time.Second {
			t.Errorf("Expected default idle timeout 60s, got %v", conf.Server.IdleTimeout)
		}

		// Test metadata defaults
		if conf.Metadata.HTTPTimeout != 10*time.Second {
			t.Errorf("Expected default HTTP timeout 10s, got %v", conf.Metadata.HTTPTimeout)
		}
		if conf.Metadata.MaxRetries != 3 {
			t.Errorf("Expected default max retries 3, got %d", conf.Metadata.MaxRetries)
		}
		if conf.Metadata.BaseRetryDelay != 100*time.Millisecond {
			t.Errorf("Expected default base retry delay 100ms, got %v", conf.Metadata.BaseRetryDelay)
		}
		if conf.Metadata.MaxRetryDelay != 2*time.Second {
			t.Errorf("Expected default max retry delay 2s, got %v", conf.Metadata.MaxRetryDelay)
		}
		if conf.Metadata.RetryMultiplier != 2.0 {
			t.Errorf("Expected default retry multiplier 2.0, got %f", conf.Metadata.RetryMultiplier)
		}

		// Test observability defaults
		if conf.Observability.LogLevel != "info" {
			t.Errorf("Expected default log level 'info', got %s", conf.Observability.LogLevel)
		}
		if !conf.Observability.EnableProfiler {
			t.Errorf("Expected default enable profiler true, got %t", conf.Observability.EnableProfiler)
		}
		if !conf.Observability.EnableTracing {
			t.Errorf("Expected default enable tracing true, got %t", conf.Observability.EnableTracing)
		}
		if conf.Observability.ShutdownTimeout != 5*time.Second {
			t.Errorf("Expected default shutdown timeout 5s, got %v", conf.Observability.ShutdownTimeout)
		}
	})

	t.Run("environment variable overrides", func(t *testing.T) {
		// Set environment variables
		os.Setenv("PORT", "9090")
		os.Setenv("SERVER_READ_TIMEOUT", "30s")
		os.Setenv("SERVER_WRITE_TIMEOUT", "45s")
		os.Setenv("SERVER_IDLE_TIMEOUT", "120s")
		os.Setenv("METADATA_HTTP_TIMEOUT", "15s")
		os.Setenv("METADATA_MAX_RETRIES", "5")
		os.Setenv("METADATA_BASE_RETRY_DELAY", "200ms")
		os.Setenv("METADATA_MAX_RETRY_DELAY", "5s")
		os.Setenv("METADATA_RETRY_MULTIPLIER", "1.5")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("ENABLE_PROFILER", "false")
		os.Setenv("ENABLE_TRACING", "false")
		os.Setenv("SHUTDOWN_TIMEOUT", "30s")

		conf := Load()

		// Test server overrides
		if conf.Server.Port != "9090" {
			t.Errorf("Expected port 9090, got %s", conf.Server.Port)
		}
		if conf.Server.ReadTimeout != 30*time.Second {
			t.Errorf("Expected read timeout 30s, got %v", conf.Server.ReadTimeout)
		}
		if conf.Server.WriteTimeout != 45*time.Second {
			t.Errorf("Expected write timeout 45s, got %v", conf.Server.WriteTimeout)
		}
		if conf.Server.IdleTimeout != 120*time.Second {
			t.Errorf("Expected idle timeout 120s, got %v", conf.Server.IdleTimeout)
		}

		// Test metadata overrides
		if conf.Metadata.HTTPTimeout != 15*time.Second {
			t.Errorf("Expected HTTP timeout 15s, got %v", conf.Metadata.HTTPTimeout)
		}
		if conf.Metadata.MaxRetries != 5 {
			t.Errorf("Expected max retries 5, got %d", conf.Metadata.MaxRetries)
		}
		if conf.Metadata.BaseRetryDelay != 200*time.Millisecond {
			t.Errorf("Expected base retry delay 200ms, got %v", conf.Metadata.BaseRetryDelay)
		}
		if conf.Metadata.MaxRetryDelay != 5*time.Second {
			t.Errorf("Expected max retry delay 5s, got %v", conf.Metadata.MaxRetryDelay)
		}
		if conf.Metadata.RetryMultiplier != 1.5 {
			t.Errorf("Expected retry multiplier 1.5, got %f", conf.Metadata.RetryMultiplier)
		}

		// Test observability overrides
		if conf.Observability.LogLevel != "debug" {
			t.Errorf("Expected log level 'debug', got %s", conf.Observability.LogLevel)
		}
		if conf.Observability.EnableProfiler {
			t.Errorf("Expected enable profiler false, got %t", conf.Observability.EnableProfiler)
		}
		if conf.Observability.EnableTracing {
			t.Errorf("Expected enable tracing false, got %t", conf.Observability.EnableTracing)
		}
		if conf.Observability.ShutdownTimeout != 30*time.Second {
			t.Errorf("Expected shutdown timeout 30s, got %v", conf.Observability.ShutdownTimeout)
		}
	})

	t.Run("invalid environment values fallback to defaults", func(t *testing.T) {
		// Set invalid environment variables
		os.Setenv("PORT", "") // Empty string should use default
		os.Setenv("SERVER_READ_TIMEOUT", "invalid")
		os.Setenv("METADATA_MAX_RETRIES", "not-a-number")
		os.Setenv("METADATA_RETRY_MULTIPLIER", "invalid-float")
		os.Setenv("ENABLE_PROFILER", "maybe")

		conf := Load()

		// Should fallback to defaults for invalid values
		if conf.Server.Port != "8080" {
			t.Errorf("Expected fallback to default port 8080, got %s", conf.Server.Port)
		}
		if conf.Server.ReadTimeout != 5*time.Second {
			t.Errorf("Expected fallback to default read timeout 5s, got %v", conf.Server.ReadTimeout)
		}
		if conf.Metadata.MaxRetries != 3 {
			t.Errorf("Expected fallback to default max retries 3, got %d", conf.Metadata.MaxRetries)
		}
		if conf.Metadata.RetryMultiplier != 2.0 {
			t.Errorf("Expected fallback to default retry multiplier 2.0, got %f", conf.Metadata.RetryMultiplier)
		}
		if !conf.Observability.EnableProfiler {
			t.Errorf("Expected fallback to default enable profiler true, got %t", conf.Observability.EnableProfiler)
		}
	})
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "existing environment variable",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "non-existing environment variable",
			key:          "NON_EXISTING_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "empty environment variable",
			key:          "EMPTY_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			original := os.Getenv(tt.key)
			defer func() {
				if original != "" {
					os.Setenv(tt.key, original)
				} else {
					os.Unsetenv(tt.key)
				}
			}()

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetDuration(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue time.Duration
		envValue     string
		expected     time.Duration
	}{
		{
			name:         "valid duration",
			key:          "TEST_DURATION",
			defaultValue: 5 * time.Second,
			envValue:     "10s",
			expected:     10 * time.Second,
		},
		{
			name:         "invalid duration",
			key:          "TEST_DURATION",
			defaultValue: 5 * time.Second,
			envValue:     "invalid",
			expected:     5 * time.Second,
		},
		{
			name:         "empty duration",
			key:          "TEST_DURATION",
			defaultValue: 5 * time.Second,
			envValue:     "",
			expected:     5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			original := os.Getenv(tt.key)
			defer func() {
				if original != "" {
					os.Setenv(tt.key, original)
				} else {
					os.Unsetenv(tt.key)
				}
			}()

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getDuration(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		expected     int
	}{
		{
			name:         "valid integer",
			key:          "TEST_INT",
			defaultValue: 5,
			envValue:     "10",
			expected:     10,
		},
		{
			name:         "invalid integer",
			key:          "TEST_INT",
			defaultValue: 5,
			envValue:     "not-a-number",
			expected:     5,
		},
		{
			name:         "empty integer",
			key:          "TEST_INT",
			defaultValue: 5,
			envValue:     "",
			expected:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			original := os.Getenv(tt.key)
			defer func() {
				if original != "" {
					os.Setenv(tt.key, original)
				} else {
					os.Unsetenv(tt.key)
				}
			}()

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getInt(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue float64
		envValue     string
		expected     float64
	}{
		{
			name:         "valid float",
			key:          "TEST_FLOAT",
			defaultValue: 1.5,
			envValue:     "2.5",
			expected:     2.5,
		},
		{
			name:         "invalid float",
			key:          "TEST_FLOAT",
			defaultValue: 1.5,
			envValue:     "not-a-float",
			expected:     1.5,
		},
		{
			name:         "empty float",
			key:          "TEST_FLOAT",
			defaultValue: 1.5,
			envValue:     "",
			expected:     1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			original := os.Getenv(tt.key)
			defer func() {
				if original != "" {
					os.Setenv(tt.key, original)
				} else {
					os.Unsetenv(tt.key)
				}
			}()

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getFloat(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue bool
		envValue     string
		expected     bool
	}{
		{
			name:         "true value",
			key:          "TEST_BOOL",
			defaultValue: false,
			envValue:     "true",
			expected:     true,
		},
		{
			name:         "false value",
			key:          "TEST_BOOL",
			defaultValue: true,
			envValue:     "false",
			expected:     false,
		},
		{
			name:         "1 value",
			key:          "TEST_BOOL",
			defaultValue: false,
			envValue:     "1",
			expected:     true,
		},
		{
			name:         "0 value",
			key:          "TEST_BOOL",
			defaultValue: true,
			envValue:     "0",
			expected:     false,
		},
		{
			name:         "invalid bool",
			key:          "TEST_BOOL",
			defaultValue: true,
			envValue:     "maybe",
			expected:     true,
		},
		{
			name:         "empty bool",
			key:          "TEST_BOOL",
			defaultValue: true,
			envValue:     "",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			original := os.Getenv(tt.key)
			defer func() {
				if original != "" {
					os.Setenv(tt.key, original)
				} else {
					os.Unsetenv(tt.key)
				}
			}()

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getBool(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %t, got %t", tt.expected, result)
			}
		})
	}
}
