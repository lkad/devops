package config

import (
	"strings"
	"testing"
)

// makeConfig builds a minimal valid Config for the tests
// below. The defaults are the dev-friendly values the
// package ships; production-mode tests override the bits
// they care about.
func makeConfig() *Config {
	return &Config{
		App: AppConfig{
			Name: "devops-toolkit",
			Env:  "dev",
			Port: 3000,
		},
		Database: DatabaseConfig{
			Driver:   "sqlite",
			User:     "devops",
			Password: "secret-not-dev-default",
		},
		LDAP: LDAPConfig{
			URL: "ldap://localhost:389",
		},
		Logs: LogsConfig{Backend: "local"},
	}
}

// TestValidate_ProductionRequiresLDAPURL: the spec's
// "Required vs Optional Values" + "Production Rejects
// Hardcoded Passwords" scenarios. Production mode must
// have a non-empty LDAP URL; dev mode skips the check.
func TestValidate_ProductionRequiresLDAPURL(t *testing.T) {
	c := makeConfig()
	c.App.Env = "production"
	c.LDAP.URL = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error: production without LDAP URL")
	} else if !strings.Contains(err.Error(), "ldap.url") {
		t.Errorf("err = %v, want mention of ldap.url", err)
	}
}

// TestValidate_ProductionRequiresJWTSecret: production
// must not use the dev-default database password (the JWT
// secret is read directly from env in main.go, not the
// config file, so the config layer enforces the DB password
// instead — same hardening intent).
func TestValidate_ProductionRejectsDevDBPassword(t *testing.T) {
	c := makeConfig()
	c.App.Env = "production"
	c.LDAP.URL = "ldap://real.example.com"
	c.Database.User = "devops"
	c.Database.Password = "devops" // the dev default
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error: dev DB password in production")
	} else if !strings.Contains(err.Error(), "database.password") {
		t.Errorf("err = %v, want mention of database.password", err)
	}
}

// TestValidate_DevSkipsProductionChecks: in dev mode the
// default JWT secret + empty LDAP URL are accepted.
func TestValidate_DevSkipsProductionChecks(t *testing.T) {
	c := makeConfig()
	c.App.Env = "dev"
	if err := c.Validate(); err != nil {
		t.Errorf("dev config should validate, got: %v", err)
	}
}

// TestValidate_RejectsUnknownLogBackend: the spec's
// "Config Validation" scenario.
func TestValidate_RejectsUnknownLogBackend(t *testing.T) {
	c := makeConfig()
	c.Logs.Backend = "splunk"
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error for unknown backend")
	}
}

// TestValidate_RequiresAppName: a config with empty
// app.name must fail validation.
func TestValidate_RequiresAppName(t *testing.T) {
	c := makeConfig()
	c.App.Name = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error: empty app.name")
	}
}
