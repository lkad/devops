// Package config — dev-default secret constants and the
// envOrWarn helper that centralises the "fall back to a
// dev default with a warning log" pattern.
//
// Why this lives in the config package: the config layer
// is the natural home for any "value the app reads but
// must not log in cleartext" constants. main.go imports
// them as named constants; production validation in
// (*Config).Validate() compares against them so an
// operator cannot accidentally ship with a dev fallback
// still wired up.
package config

import (
	"log"
	"os"
)

// Dev-default secret constants. Centralized so the
// "rotate the dev fallback" change has one home and so
// production validation in (*Config).Validate() can
// compare against a known-bad value.
//
// NEVER set these to anything sensitive. In production,
// env != "production" is false, so config.Validate() and
// the startup guard in main.go will fail-fast if any of
// these are still in use.
const (
	// DevDefaultJWTSecret is the APP_JWT_SECRET fallback
	// when the env var is unset AND the app is not running
	// in production. Rotate in lockstep with the auth +
	// WS hub routes (the two signers must agree).
	DevDefaultJWTSecret = "dev-secret-do-not-use-in-prod"

	// DevDefaultK8SCryptoKey is the K8S_CRYPTO_KEY
	// fallback. The literal is 32 bytes (the AES-256 key
	// size) so the SHA-256 derivation branch is not
	// triggered in dev.
	DevDefaultK8SCryptoKey = "dev-k8s-crypto-key-32-bytes-long-xx"

	// DevDefaultInfluxPassword is the INFLUX_TOKEN
	// fallback. InfluxDB uses a "token" for auth, not a
	// password, but the masked-keys list treats them
	// interchangeably.
	DevDefaultInfluxPassword = "dev-influx-token"

	// DevDefaultLDAPBindPassword is the LDAP_BIND_PASSWORD
	// fallback used when the LDAP client needs a service
	// account to bind before searching.
	DevDefaultLDAPBindPassword = "dev-ldap-bind"

	// DevDefaultProberSSHKey is the PROBER_SSH_KEY
	// fallback used by the physical-host prober to SSH
	// onto monitored hosts and run metric collectors.
	DevDefaultProberSSHKey = "dev-prober-key"
)

// EnvOrWarn returns the value of envName if set, otherwise
// the supplied devDefault. When falling back to devDefault,
// logs a warning tagged with envName so operators can grep
// for accidental dev-defaults in production logs.
//
// Exported because main.go (cmd/devops-toolkit) is the
// canonical consumer: it reads APP_JWT_SECRET / K8S_CRYPTO_KEY
// directly from the environment and the production guard
// ("refuse to start with a dev default") lives there, not
// in the config struct.
func EnvOrWarn(envName, devDefault string) string {
	if v := os.Getenv(envName); v != "" {
		return v
	}
	log.Printf("WARN: using dev default for %s — DO NOT use in production", envName)
	return devDefault
}
