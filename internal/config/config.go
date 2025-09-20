package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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

	// Security configuration
	Security SecurityConfig
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
	LogLevel           string        `json:"log_level"`
	EnableProfiler     bool          `json:"enable_profiler"`
	EnableTracing      bool          `json:"enable_tracing"`
	EnablePIIRedaction bool          `json:"enable_pii_redaction"`
	ShutdownTimeout    time.Duration `json:"shutdown_timeout"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	// Default Cross-Origin policies
	DefaultCOEP string `json:"default_coep"` // Cross-Origin-Embedder-Policy: "", "require-corp", or "credentialless"
	DefaultCOOP string `json:"default_coop"` // Cross-Origin-Opener-Policy: "same-origin", "same-origin-allow-popups", or "unsafe-none"
	DefaultCORP string `json:"default_corp"` // Cross-Origin-Resource-Policy: "same-origin", "same-site", or "cross-origin"

	// API-specific policies (less restrictive for public APIs)
	APICOEP string `json:"api_coep"`
	APICOOP string `json:"api_coop"`
	APICORP string `json:"api_corp"`
}

// Validate validates the SecurityConfig values
func (sc SecurityConfig) Validate() error {
	validCOEP := []string{"", "require-corp", "credentialless"}
	validCOOP := []string{"", "same-origin", "same-origin-allow-popups", "unsafe-none"}
	validCORP := []string{"", "same-origin", "same-site", "cross-origin"}

	if err := validatePolicy("DefaultCOEP", sc.DefaultCOEP, validCOEP); err != nil {
		return err
	}
	if err := validatePolicy("DefaultCOOP", sc.DefaultCOOP, validCOOP); err != nil {
		return err
	}
	if err := validatePolicy("DefaultCORP", sc.DefaultCORP, validCORP); err != nil {
		return err
	}
	if err := validatePolicy("APICOEP", sc.APICOEP, validCOEP); err != nil {
		return err
	}
	if err := validatePolicy("APICOOP", sc.APICOOP, validCOOP); err != nil {
		return err
	}
	if err := validatePolicy("APICORP", sc.APICORP, validCORP); err != nil {
		return err
	}

	return nil
}

// validatePolicy validates a single policy value against allowed values
func validatePolicy(name, value string, allowed []string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("invalid %s value '%s', allowed values: %s", name, value, strings.Join(allowed, ", "))
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	return c.Security.Validate()
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
			LogLevel:           getEnv("LOG_LEVEL", "info"),
			EnableProfiler:     getBool("ENABLE_PROFILER", true),
			EnableTracing:      getBool("ENABLE_TRACING", true),
			EnablePIIRedaction: getBool("ENABLE_PII_REDACTION", false),
			ShutdownTimeout:    getDuration("SHUTDOWN_TIMEOUT", 5*time.Second),
		},
		Security: SecurityConfig{
			// Default strict policies for sensitive endpoints
			DefaultCOEP: getEnv("SECURITY_DEFAULT_COEP", "require-corp"),
			DefaultCOOP: getEnv("SECURITY_DEFAULT_COOP", "same-origin"),
			DefaultCORP: getEnv("SECURITY_DEFAULT_CORP", "same-origin"),

			// Less restrictive policies for API endpoints
			APICOEP: getEnv("SECURITY_API_COEP", ""), // Empty means header won't be set
			APICOOP: getEnv("SECURITY_API_COOP", "same-origin-allow-popups"),
			APICORP: getEnv("SECURITY_API_CORP", "cross-origin"),
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
			if intValue >= 0 {
				return intValue
			}
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
