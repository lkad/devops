package ldap

import (
	"os"
	"strconv"
	"time"
)

// Config holds LDAP connection configuration from environment variables
type Config struct {
	// LDAP server URL (e.g., ldap://localhost:389)
	URL string

	// Base DN for searches (e.g., dc=example,dc=com)
	BaseDN string

	// Admin DN for binding (e.g., cn=admin,dc=example,dc=com)
	AdminDN string

	// Admin password
	AdminPassword string

	// User search base (relative to BaseDN)
	UserSearchBase string

	// Group search base (relative to BaseDN)
	GroupSearchBase string

	// Connection pool settings
	PoolSize int

	// Timeout settings
	ConnectTimeout time.Duration
	SearchTimeout  time.Duration

	// Retry settings
	MaxRetries int

	// Rate limiting
	RateLimitPerSecond int
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		URL:               getEnv("LDAP_URL", "ldap://localhost:389"),
		BaseDN:            getEnv("LDAP_BASE_DN", "dc=example,dc=com"),
		AdminDN:           getEnv("LDAP_ADMIN_DN", "cn=admin,dc=example,dc=com"),
		AdminPassword:     getEnv("LDAP_ADMIN_PASSWORD", ""),
		UserSearchBase:    getEnv("LDAP_USER_SEARCH_BASE", "ou=Users"),
		GroupSearchBase:   getEnv("LDAP_GROUP_SEARCH_BASE", "ou=Groups"),
		PoolSize:          getEnvInt("LDAP_POOL_SIZE", 10),
		ConnectTimeout:    getEnvDuration("LDAP_CONNECT_TIMEOUT", 10*time.Second),
		SearchTimeout:     getEnvDuration("LDAP_SEARCH_TIMEOUT", 30*time.Second),
		MaxRetries:        getEnvInt("LDAP_MAX_RETRIES", 3),
		RateLimitPerSecond: getEnvInt("LDAP_RATE_LIMIT_PER_SECOND", 100),
	}
}

// Validate checks if the configuration has all required fields
func (c *Config) Validate() error {
	if c.URL == "" {
		return &ConfigError{Field: "URL", Message: "LDAP_URL is required"}
	}
	if c.BaseDN == "" {
		return &ConfigError{Field: "BaseDN", Message: "LDAP_BASE_DN is required"}
	}
	if c.AdminDN == "" {
		return &ConfigError{Field: "AdminDN", Message: "LDAP_ADMIN_DN is required"}
	}
	// AdminPassword can be empty for anonymous binds
	return nil
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config error: " + e.Field + " - " + e.Message
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}