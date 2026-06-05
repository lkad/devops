package alerts

import "testing"

// TestMask_RedactsSensitive verifies that keys whose name matches
// the logger.MaskValue list (password, token, secret, etc.) are
// blanked out, while non-sensitive keys pass through.
func TestMask_RedactsSensitive(t *testing.T) {
	c := Channel{
		Type: ChannelTypeSlack,
		Config: JSONMap{
			"webhook_url": "https://hooks.example.com/x/y/z",
			"api_token":   "very-secret-token",
			"password":    "hunter2",
			"channel":     "#alerts",
		},
	}
	masked := Mask(c)
	cfg := masked.Config
	if cfg["api_token"] != "***" {
		t.Errorf("api_token: got %q want ***", cfg["api_token"])
	}
	if cfg["password"] != "***" {
		t.Errorf("password: got %q want ***", cfg["password"])
	}
	if cfg["webhook_url"] != "https://hooks.example.com/x/y/z" {
		t.Errorf("webhook_url should pass through: got %q", cfg["webhook_url"])
	}
	if cfg["channel"] != "#alerts" {
		t.Errorf("channel should pass through: got %q", cfg["channel"])
	}
}

// TestMask_DoesNotMutateInput guards against a subtle bug: the
// caller may still hold a reference to the original Config, so
// Mask must return a fresh map (or at least a copy of the
// receiver) and never overwrite the source.
func TestMask_DoesNotMutateInput(t *testing.T) {
	c := Channel{
		Type: ChannelTypeEmail,
		Config: JSONMap{
			"smtp_password": "original",
		},
	}
	_ = Mask(c)
	if c.Config["smtp_password"] != "original" {
		t.Errorf("Mask mutated the input: %q", c.Config["smtp_password"])
	}
}
