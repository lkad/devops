// Package config loads application configuration from YAML files
// with environment variable overrides, validates the result, and
// renders a secrets-masked string view for logging.
//
// The shape mirrors the section structure in openspec/specs/config-management.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"

	"github.com/devops-toolkit/backend/pkg/logger"
)

// Config is the fully-populated application configuration.
type Config struct {
	App          AppConfig          `mapstructure:"app"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Logs         LogsConfig         `mapstructure:"logs"`
	LDAP         LDAPConfig         `mapstructure:"ldap"`
	Alerts       AlertsConfig       `mapstructure:"alerts"`
	K8s          K8sConfig          `mapstructure:"k8s"`
	PhysicalHost PhysicalHostConfig `mapstructure:"physicalhost"`
	WebSocket    WebSocketConfig    `mapstructure:"websocket"`
}

type AppConfig struct {
	Name     string `mapstructure:"name"`
	Env      string `mapstructure:"env"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	LogLevel string `mapstructure:"log_level"`
}

type DatabaseConfig struct {
	Driver         string `mapstructure:"driver"`
	DSN            string `mapstructure:"dsn"`
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	DBName         string `mapstructure:"dbname"`
	SSLMode        string `mapstructure:"sslmode"`
	MaxOpenConns   int    `mapstructure:"max_open_conns"`
	MaxIdleConns   int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int   `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type LogsConfig struct {
	Backend        string                 `mapstructure:"backend"`
	Local          LocalLogsConfig        `mapstructure:"local"`
	Elasticsearch  ElasticLogsConfig      `mapstructure:"elasticsearch"`
	Loki           LokiLogsConfig         `mapstructure:"loki"`
}

type LocalLogsConfig struct {
	Path string `mapstructure:"path"`
}

type ElasticLogsConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type LokiLogsConfig struct {
	URL string `mapstructure:"url"`
}

type LDAPConfig struct {
	URL          string         `mapstructure:"url"`
	BindDN       string         `mapstructure:"bind_dn"`
	BindPassword string         `mapstructure:"bind_password"`
	BaseDN       string         `mapstructure:"base_dn"`
	UserFilter   string         `mapstructure:"user_filter"`
	DevBypass    bool           `mapstructure:"dev_bypass"`
	DevUsers     []DevUserEntry `mapstructure:"dev_users"`
}

type DevUserEntry struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Role     string `mapstructure:"role"`
}

type AlertsConfig struct {
	Channels    []string             `mapstructure:"channels"`
	Suppression AlertsSuppression    `mapstructure:"suppression"`
}

type AlertsSuppression struct {
	EnableInMaintenance bool `mapstructure:"enable_in_maintenance"`
}

type K8sConfig struct {
	DefaultNamespace string `mapstructure:"default_namespace"`
	InCluster        bool   `mapstructure:"in_cluster"`
	KubeconfigPath   string `mapstructure:"kubeconfig_path"`
}

type PhysicalHostConfig struct {
	MonitoringInterval int `mapstructure:"monitoring_interval"`
	SSHTimeout         int `mapstructure:"ssh_timeout"`
}

type WebSocketConfig struct {
	PingInterval int `mapstructure:"ping_interval"`
	WriteTimeout int `mapstructure:"write_timeout"`
	ReadTimeout  int `mapstructure:"read_timeout"`
}

// LoadOptions configures a Load call. WithConfigPath is the only
// knob we need today; other knobs (env prefix, profile name) can be
// added without breaking callers.
type LoadOptions struct {
	Path string
}

// Option mutates LoadOptions.
type Option func(*LoadOptions)

// WithConfigPath sets the path to the YAML file. Empty string is an error.
func WithConfigPath(p string) Option {
	return func(o *LoadOptions) {
		o.Path = p
	}
}

// Load reads, parses, validates, and returns a Config. It fails
// loudly on a missing file, an unparseable file, or a validation error.
func Load(opts ...Option) (*Config, error) {
	o := &LoadOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.Path == "" {
		return nil, fmt.Errorf("config path is required")
	}
	if _, err := os.Stat(o.Path); err != nil {
		return nil, fmt.Errorf("config file not found at %q: %w", o.Path, err)
	}

	v := viper.New()
	v.SetConfigFile(o.Path)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &c, nil
}

// setDefaults applies sensible defaults for any field not in the YAML.
// Pulled from the config-management spec's "Default Config" scenario.
func setDefaults(v *viper.Viper) {
	v.SetDefault("app.host", "0.0.0.0")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.env", "dev")
	v.SetDefault("logs.backend", "local")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 300)
}

// Validate enforces invariants that YAML/env alone cannot guarantee.
// Returns the first violation with a message that points at the field.
func (c *Config) Validate() error {
	if c.App.Port < 0 || c.App.Port > 65535 {
		return fmt.Errorf("app.port %d is out of range 0-65535", c.App.Port)
	}
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}
	switch c.Logs.Backend {
	case "local", "elasticsearch", "loki":
		// ok
	case "":
		// empty is allowed (defaults to local via setDefaults)
	default:
		return fmt.Errorf("logs.backend %q is not supported (use local|elasticsearch|loki)", c.Logs.Backend)
	}
	if c.PhysicalHost.MonitoringInterval < 0 {
		return fmt.Errorf("physicalhost.monitoring_interval must be >= 0")
	}

	// Production-mode rules. Per the spec's
	// "Required vs Optional Values" scenario + the README's
	// "production must not have hardcoded passwords" rule.
	if c.App.Env == "production" {
		if c.LDAP.URL == "" {
			return fmt.Errorf("production requires ldap.url to be set")
		}
		if c.Database.Password == "devops" || c.Database.Password == "" {
			return fmt.Errorf("production requires a non-default database.password (got %q)", c.Database.Password)
		}
		// APP_JWT_SECRET and K8S_CRYPTO_KEY are not
		// stored in the YAML (they are env-only — see
		// cmd/devops-toolkit/main.go). The dev-bypass
		// gate is the only production-deny check we can
		// do at Validate time; the JWT/K8S-crypto
		// missing-env check is at startup in main.go.
		if c.LDAP.DevBypass {
			return fmt.Errorf("production must not enable ldap.dev_bypass (set ldap.dev_bypass: false or use a non-production env)")
		}
	}
	return nil
}

// String renders the config for diagnostic logging, masking any
// field whose name matches a sensitive pattern. It is not exhaustive;
// new sensitive fields must be added here (and to logger.MaskValue's
// patterns) when introduced.
func (c *Config) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "app={name=%s env=%s host=%s port=%d log_level=%s}\n",
		c.App.Name, c.App.Env, c.App.Host, c.App.Port, c.App.LogLevel)
	fmt.Fprintf(&b, "database={driver=%s host=%s port=%d user=%s password=%s dbname=%s}\n",
		c.Database.Driver, c.Database.Host, c.Database.Port, c.Database.User,
		logger.MaskValue("password", c.Database.Password), c.Database.DBName)
	fmt.Fprintf(&b, "logs={backend=%s}\n", c.Logs.Backend)
	fmt.Fprintf(&b, "ldap={url=%s dev_bypass=%v users=%d}\n",
		c.LDAP.URL, c.LDAP.DevBypass, len(c.LDAP.DevUsers))
	fmt.Fprintf(&b, "alerts={channels=%d suppression.enable_in_maintenance=%v}\n",
		len(c.Alerts.Channels), c.Alerts.Suppression.EnableInMaintenance)
	fmt.Fprintf(&b, "k8s={default_namespace=%s in_cluster=%v}\n",
		c.K8s.DefaultNamespace, c.K8s.InCluster)
	fmt.Fprintf(&b, "physicalhost={monitoring_interval=%d ssh_timeout=%d}\n",
		c.PhysicalHost.MonitoringInterval, c.PhysicalHost.SSHTimeout)
	fmt.Fprintf(&b, "websocket={ping=%d write=%d read=%d}\n",
		c.WebSocket.PingInterval, c.WebSocket.WriteTimeout, c.WebSocket.ReadTimeout)
	return b.String()
}
