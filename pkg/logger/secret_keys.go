// Package logger — secret key list.
//
// This file is the canonical source of truth for "which
// config keys MUST be masked in logs / String()
// renderings?". It complements the substring matcher
// in MaskValue with an explicit, exact-match list so a
// field name like "jwt_secret" or "k8s_crypto_key" is
// caught deterministically even if the substring rule
// is later loosened.
//
// If you add a new secret-bearing field to
// internal/config, add its key here too.
package logger

import "strings"

// SecretKeys lists every config key whose value MUST be
// redacted when logged or rendered via String(). Matched
// case-insensitively (callers should lowercase the key
// before lookup).
//
// The list mirrors the substring match in MaskValue for
// the common cases (password, token, secret, …) and adds
// the project-specific names (jwt_secret, k8s_crypto_key,
// ldap_bind_pw, tls_cert, …) that the substring rule
// would otherwise miss.
var SecretKeys = map[string]bool{
	"password":       true,
	"kubeconfig":     true,
	"bind_password":  true,
	"token":          true,
	"secret":         true,
	"secret_key":     true,
	"jwt_secret":     true,
	"k8s_crypto_key": true,
	"private_key":    true,
	"privatekey":     true,
	"ssh_key":        true,
	"client_secret":  true,
	"api_key":        true,
	"ldap_bind_pw":   true,
	"tls_cert":       true,
}

// ShouldMask reports whether a config key's value should
// be masked in logs. The match is case-insensitive after
// lowercasing, and tolerant of underscores vs hyphens vs
// dots ("tls.cert" or "tls-cert" both match "tls_cert"
// after the normalization).
//
// Internally a thin wrapper over the SecretKeys map so
// the canonical source stays a single declaration.
func ShouldMask(key string) bool {
	k := normalizeKey(key)
	return SecretKeys[k]
}

// normalizeKey lower-cases the key and rewrites
// hyphens / dots to underscores so all three common
// separators map to the same canonical form.
func normalizeKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	k = strings.ReplaceAll(k, "-", "_")
	k = strings.ReplaceAll(k, ".", "_")
	return k
}
