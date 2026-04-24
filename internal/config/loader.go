package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	LDAP     LDAPConfig     `yaml:"ldap"`
	Logs     LogsConfig     `yaml:"logs"`
	K8s      K8sConfig      `yaml:"k8s"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"sslmode"`
}

func (d DatabaseConfig) DSN() string {
	ssl := d.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, ssl)
}

type LDAPConfig struct {
	URL          string `yaml:"url"`
	BaseDN       string `yaml:"base_dn"`
	AdminDN      string `yaml:"admin_dn"`
	AdminPass    string `yaml:"admin_pass"`
	UserSearch   string `yaml:"user_search"`
	GroupSearch  string `yaml:"group_search"`
	BindDN       string `yaml:"bind_dn"`
	BindPassword string `yaml:"bind_password"`
}

type LogsConfig struct {
	Backend       string `yaml:"backend"`
	RetentionDays int    `yaml:"retention_days"`
	Path          string `yaml:"path"`
}

type K8sConfig struct {
	Provider string `yaml:"provider"`
	KubePath string `yaml:"kubeconfig_path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Override from environment variables
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}
	if host := os.Getenv("HOST"); host != "" {
		cfg.Server.Host = host
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		cfg.Database.Host = dbHost
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		cfg.Database.User = dbUser
	}
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		cfg.Database.Password = dbPass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.Database.Name = dbName
	}
	if ldapURL := os.Getenv("LDAP_URL"); ldapURL != "" {
		cfg.LDAP.URL = ldapURL
	}
	if backend := os.Getenv("LOG_STORAGE_BACKEND"); backend != "" {
		cfg.Logs.Backend = backend
	}

	// Defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 3000
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Logs.RetentionDays == 0 {
		cfg.Logs.RetentionDays = 30
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.K8s.Provider == "" {
		cfg.K8s.Provider = "k3d"
	}

	return &cfg, nil
}
