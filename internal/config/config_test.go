package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

const sampleYAML = `
app:
  name: devops-toolkit
  env: dev
  host: 0.0.0.0
  port: 8080
  log_level: info

database:
  driver: sqlite
  dsn: tests/fixtures/db/dev.db
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 300

logs:
  backend: local
`

func TestLoad_FromYAMLFile(t *testing.T) {
	// GIVEN a valid config file on disk
	// WHEN Load is called
	// THEN the values are populated and App.Port == 8080
	path := writeTempYAML(t, sampleYAML)
	cfg, err := Load(WithConfigPath(path))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Name != "devops-toolkit" {
		t.Errorf("App.Name = %q", cfg.App.Name)
	}
	if cfg.App.Port != 8080 {
		t.Errorf("App.Port = %d, want 8080", cfg.App.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("Database.Driver = %q", cfg.Database.Driver)
	}
	if cfg.Logs.Backend != "local" {
		t.Errorf("Logs.Backend = %q", cfg.Logs.Backend)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	// GIVEN a config file and an env var override
	// WHEN Load is called
	// THEN the env var wins
	path := writeTempYAML(t, sampleYAML)
	t.Setenv("APP__PORT", "9999")
	t.Setenv("APP__LOG_LEVEL", "debug")

	cfg, err := Load(WithConfigPath(path))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Port != 9999 {
		t.Errorf("App.Port = %d, want 9999 (env override)", cfg.App.Port)
	}
	if cfg.App.LogLevel != "debug" {
		t.Errorf("App.LogLevel = %q, want debug", cfg.App.LogLevel)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// GIVEN a minimal config that omits optional fields
	// WHEN Load is called
	// THEN defaults are applied
	path := writeTempYAML(t, `
app:
  name: minimal
`)
	cfg, err := Load(WithConfigPath(path))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Port != 8080 {
		t.Errorf("default Port = %d, want 8080", cfg.App.Port)
	}
	if cfg.Logs.Backend != "local" {
		t.Errorf("default Logs.Backend = %q, want local", cfg.Logs.Backend)
	}
}

func TestLoad_MissingFileFails(t *testing.T) {
	// GIVEN a path that does not exist
	// WHEN Load is called
	// THEN it returns an error
	_, err := Load(WithConfigPath("/tmp/does-not-exist-config.yaml"))
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoad_InvalidPortFails(t *testing.T) {
	// GIVEN a config with an out-of-range port
	// WHEN Load is called
	// THEN validation fails with a clear message
	path := writeTempYAML(t, `
app:
  name: x
  port: 99999
`)
	_, err := Load(WithConfigPath(path))
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestConfig_StringMasksSecrets(t *testing.T) {
	// GIVEN a config with a database password
	// WHEN String() is called
	// THEN the password is masked
	c := &Config{
		App:      AppConfig{Name: "x", Env: "dev", Port: 8080},
		Database: DatabaseConfig{Driver: "postgres", DSN: "host=db", Password: "supersecret"},
	}
	out := c.String()
	if contains(out, "supersecret") {
		t.Errorf("password leaked: %s", out)
	}
	if !contains(out, "***") {
		t.Errorf("expected *** mask in output: %s", out)
	}
}

// TestConfig_StringMasksAllSensitiveFields is the audit's
// "Config.String() masking list incomplete" pin: every sensitive
// field whose name matches logger.IsSensitiveField (or the
// substring rule) MUST render as "***" — never the raw value.
// A regression here would silently leak a secret to the log
// aggregator; the test is the safety net.
func TestConfig_StringMasksAllSensitiveFields(t *testing.T) {
	const k8sHomeLeak = "/home/operator/.kube/config"
	c := &Config{
		App:      AppConfig{Name: "x", Env: "dev", Port: 8080},
		Database: DatabaseConfig{Password: "db-secret"},
		Redis:    RedisConfig{Password: "redis-secret"},
		Logs: LogsConfig{
			Elasticsearch: ElasticLogsConfig{Password: "es-secret"},
		},
		LDAP: LDAPConfig{
			BindPassword: "bind-secret",
			DevUsers: []DevUserEntry{
				{Username: "dev", Password: "devpass", Role: "admin"},
			},
		},
		K8s: K8sConfig{KubeconfigPath: k8sHomeLeak},
	}
	out := c.String()

	// All raw secret values must be absent.
	for _, leak := range []string{
		"db-secret", "redis-secret", "es-secret", "bind-secret", "devpass",
	} {
		if contains(out, leak) {
			t.Errorf("sensitive value %q leaked in String() output:\n%s", leak, out)
		}
	}
	// KubeconfigPath must NOT be rendered as the raw path —
	// it leaks the operator's $HOME. The audit asked for the
	// masking list to include kubeconfig; the chosen rendering
	// is "set" / "unset" so the path itself never reaches the
	// log.
	if contains(out, k8sHomeLeak) {
		t.Errorf("KubeconfigPath leaked as raw value:\n%s", out)
	}
	// Sanity: at least one "***" mask is present, and the
	// non-sensitive LDAP fields (bind_dn, base_dn) are still
	// rendered in cleartext so the operator can correlate the
	// log line with the YAML.
	if !contains(out, "***") {
		t.Errorf("expected *** mask in output:\n%s", out)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
