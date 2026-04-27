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
	Auth     AuthConfig     `yaml:"auth"`
}

type AuthConfig struct {
	JWTSecret        string   `yaml:"jwt_secret"`
	SessionDuration  string   `yaml:"session_duration"`
	DevBypass        bool     `yaml:"dev_bypass"`
	DevUsername      string   `yaml:"dev_username"`
	DevPassword      string   `yaml:"dev_password"`
	DevRoles         []string `yaml:"dev_roles"`
}

type ServerConfig struct {
	Port     int    `yaml:"port"`
	Host     string `yaml:"host"`
	BasePath string `yaml:"base_path"`
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func (s ServerConfig) StripBasePath(path string) string {
	if s.BasePath == "" || s.BasePath == "/" {
		return path
	}
	// Strip base_path prefix if present
	if len(path) > len(s.BasePath) && path[:len(s.BasePath)] == s.BasePath {
		return path[len(s.BasePath):]
	}
	return path
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

func splitRoles(s string) []string {
	if s == "" {
		return nil
	}
	var roles []string
	for _, r := range splitString(s) {
		if r := trim(r); r != "" {
			roles = append(roles, r)
		}
	}
	return roles
}

func splitString(s string) []string {
	var result []string
	var curr []byte
	for _, c := range s {
		if c == ',' {
			if len(curr) > 0 {
				result = append(result, string(curr))
				curr = nil
			}
		} else {
			curr = append(curr, byte(c))
		}
	}
	if len(curr) > 0 {
		result = append(result, string(curr))
	}
	return result
}

func trim(s string) string {
	i, j := 0, len(s)-1
	for i <= j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j >= i && (s[j] == ' ' || s[j] == '\t') {
		j--
	}
	if i > j {
		return ""
	}
	return s[i : j+1]
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
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		cfg.Server.BasePath = basePath
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
	// Dev auth bypass
	if bypass := os.Getenv("DEVOPS_AUTH_BYPASS"); bypass == "true" {
		cfg.Auth.DevBypass = true
	}
	if devUser := os.Getenv("DEVOPS_DEV_USERNAME"); devUser != "" {
		cfg.Auth.DevUsername = devUser
	}
	if devPass := os.Getenv("DEVOPS_DEV_PASSWORD"); devPass != "" {
		cfg.Auth.DevPassword = devPass
	}
	if devRoles := os.Getenv("DEVOPS_DEV_ROLES"); devRoles != "" {
		cfg.Auth.DevRoles = splitRoles(devRoles)
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
	if cfg.Auth.SessionDuration == "" {
		cfg.Auth.SessionDuration = "8h"
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "devops-toolkit-change-in-production"
	}

	return &cfg, nil
}
