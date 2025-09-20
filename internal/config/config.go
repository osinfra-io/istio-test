package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the istio-test application
type Config struct {
	// Server configuration
	Server ServerConfig

	// Metadata service configuration
	Metadata MetadataConfig

	// Observability configuration
	Observability ObservabilityConfig
}

// ServerConfig holds HTTP server related configuration
type ServerConfig struct {
	Port         string        `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// MetadataConfig holds metadata service related configuration
type MetadataConfig struct {
	HTTPTimeout     time.Duration `json:"http_timeout"`
	MaxRetries      int           `json:"max_retries"`
	BaseRetryDelay  time.Duration `json:"base_retry_delay"`
	MaxRetryDelay   time.Duration `json:"max_retry_delay"`
	RetryMultiplier float64       `json:"retry_multiplier"`
}

// ObservabilityConfig holds observability related configuration
type ObservabilityConfig struct {
	LogLevel        string        `json:"log_level"`
	EnableProfiler  bool          `json:"enable_profiler"`
	EnableTracing   bool          `json:"enable_tracing"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// Load creates a new Config instance with values from environment variables
// and sensible defaults
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 5*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:  getDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Metadata: MetadataConfig{
			HTTPTimeout:     getDuration("METADATA_HTTP_TIMEOUT", 10*time.Second),
			MaxRetries:      getInt("METADATA_MAX_RETRIES", 3),
			BaseRetryDelay:  getDuration("METADATA_BASE_RETRY_DELAY", 100*time.Millisecond),
			MaxRetryDelay:   getDuration("METADATA_MAX_RETRY_DELAY", 2*time.Second),
			RetryMultiplier: getFloat("METADATA_RETRY_MULTIPLIER", 2.0),
		},
		Observability: ObservabilityConfig{
			LogLevel:        getEnv("LOG_LEVEL", "info"),
			EnableProfiler:  getBool("ENABLE_PROFILER", true),
			EnableTracing:   getBool("ENABLE_TRACING", true),
			ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 5*time.Second),
		},
	}
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getDuration parses a duration from an environment variable or returns a default value
func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			if duration > 0 {
				return duration
			}
		}
	}
	return defaultValue
}

// getInt parses an integer from an environment variable or returns a default value
func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getFloat parses a float from an environment variable or returns a default value
func getFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			if floatValue > 0 {
				return floatValue
			}
		}
	}
	return defaultValue
}

// getBool parses a boolean from an environment variable or returns a default value
func getBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
