package logger

import "testing"

func TestShouldMask_KnownKeys(t *testing.T) {
	cases := []string{
		"password", "PASSWORD",
		"kubeconfig", "KubeConfig",
		"bind_password", "BIND_PASSWORD",
		"token", "TOKEN",
		"secret", "secret_key",
		"jwt_secret", "JWT_SECRET",
		"k8s_crypto_key", "K8S_CRYPTO_KEY",
		"private_key", "privateKey",
		"ssh_key", "ssh-key",
		"client_secret", "client-secret",
		"api_key", "API_KEY",
		"ldap_bind_pw", "LDAP_BIND_PW",
		"tls_cert", "TLS_CERT",
	}
	for _, k := range cases {
		if !ShouldMask(k) {
			t.Errorf("ShouldMask(%q) = false, want true", k)
		}
	}
}

func TestShouldMask_UnknownKey(t *testing.T) {
	cases := []string{"username", "host", "port", "database"}
	for _, k := range cases {
		if ShouldMask(k) {
			t.Errorf("ShouldMask(%q) = true, want false", k)
		}
	}
}
