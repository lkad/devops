package config

import (
	"os"
	"testing"
)

func TestEnvOrWarn_FromEnv(t *testing.T) {
	t.Setenv("TEST_SECRET_KEY", "real-value")
	got := EnvOrWarn("TEST_SECRET_KEY", "default")
	if got != "real-value" {
		t.Errorf("EnvOrWarn from env = %q, want %q", got, "real-value")
	}
}

func TestEnvOrWarn_Default(t *testing.T) {
	os.Unsetenv("TEST_SECRET_KEY_MISSING")
	got := EnvOrWarn("TEST_SECRET_KEY_MISSING", "default-value")
	if got != "default-value" {
		t.Errorf("EnvOrWarn default = %q, want %q", got, "default-value")
	}
}

func TestDevDefaults_NotEmpty(t *testing.T) {
	// Sanity: the dev-default constants must be non-empty so
	// production validation has something to compare against.
	cases := []struct {
		name, val string
	}{
		{"DevDefaultJWTSecret", DevDefaultJWTSecret},
		{"DevDefaultK8SCryptoKey", DevDefaultK8SCryptoKey},
		{"DevDefaultInfluxPassword", DevDefaultInfluxPassword},
		{"DevDefaultLDAPBindPassword", DevDefaultLDAPBindPassword},
		{"DevDefaultProberSSHKey", DevDefaultProberSSHKey},
	}
	for _, c := range cases {
		if c.val == "" {
			t.Errorf("%s is empty", c.name)
		}
	}
}
