package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Auth     AuthConfig     `yaml:"auth"`
	Logs     LogsConfig     `yaml:"logs"`
	Metrics  MetricsConfig  `yaml:"metrics"`
	Alerts   AlertsConfig   `yaml:"alerts"`
	Physical PhysicalConfig `yaml:"physicalhost"`
	K8s      K8sConfig      `yaml:"k8s"`
	WebSocket WebSocketConfig `yaml:"websocket"`
}

type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Env     string `yaml:"env"`
}

type ServerConfig struct {
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

type DatabaseConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Name            string `yaml:"name"`
	MaxConnections  int    `yaml:"max_connections"`
	SSLMode         string `yaml:"ssl_mode"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

type AuthConfig struct {
	JWT_SECRET  string      `yaml:"jwt_secret"`
	TokenExpiry time.Duration `yaml:"token_expiry"`
	LDAP        LDAPConfig   `yaml:"ldap"`
}

type LDAPConfig struct {
	Enabled       bool              `yaml:"enabled"`
	Host          string            `yaml:"host"`
	Port          int               `yaml:"port"`
	BaseDN        string            `yaml:"base_dn"`
	BindDN        string            `yaml:"bind_dn"`
	BindPassword  string            `yaml:"bind_password"`
	UserFilter    string            `yaml:"user_filter"`
	GroupMapping  map[string]string `yaml:"group_mapping"`
}

type LogsConfig struct {
	StorageBackend  string               `yaml:"storage_backend"`
	Level           string               `yaml:"level"`
	RetentionDays   int                  `yaml:"retention_days"`
	Elasticsearch   ElasticsearchConfig  `yaml:"elasticsearch"`
	Loki            LokiConfig           `yaml:"loki"`
}

type ElasticsearchConfig struct {
	URL      string `yaml:"url"`
	Index    string `yaml:"index"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type LokiConfig struct {
	URL    string `yaml:"url"`
	Tenant string `yaml:"tenant"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type AlertsConfig struct {
	Enabled   bool           `yaml:"enabled"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Slack     SlackConfig    `yaml:"slack"`
	Webhook   WebhookConfig  `yaml:"webhook"`
	Email     EmailConfig    `yaml:"email"`
}

type RateLimitConfig struct {
	WindowSeconds int `yaml:"window_seconds"`
	MaxAlerts    int `yaml:"max_alerts"`
}

type SlackConfig struct {
	Enabled        bool   `yaml:"enabled"`
	DefaultChannel string `yaml:"default_channel"`
}

type WebhookConfig struct {
	Enabled bool `yaml:"enabled"`
}

type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	From     string `yaml:"from"`
}

type PhysicalConfig struct {
	SSHPoolSize           int           `yaml:"ssh_pool_size"`
	HealthCheckInterval   time.Duration `yaml:"health_check_interval"`
	DataFreshnessThreshold time.Duration `yaml:"data_freshness_threshold"`
	SSHTimeout            time.Duration `yaml:"ssh_timeout"`
}

type K8sConfig struct {
	DefaultKubeconfigPath string `yaml:"default_kubeconfig_path"`
}

type WebSocketConfig struct {
	ReadBufferSize  int           `yaml:"read_buffer_size"`
	WriteBufferSize int           `yaml:"write_buffer_size"`
	PingInterval    time.Duration `yaml:"ping_interval"`
	PingTimeout     time.Duration `yaml:"ping_timeout"`
	MaxMessageSize  int           `yaml:"max_message_size"`
	Channels        []string      `yaml:"channels"`
}

var cfg *Config

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the config
	expanded := expandEnvVars(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg = &config
	return cfg, nil
}

func Get() *Config {
	return cfg
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandEnvVars(content string) string {
	return envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		varName := match[2 : len(match)-1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.Username, c.Password, c.Name, c.SSLMode,
	)
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
