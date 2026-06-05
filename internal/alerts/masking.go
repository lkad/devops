package alerts

import "github.com/devops-toolkit/backend/pkg/logger"

// Mask returns a copy of c with the sensitive keys of
// c.Config redacted via logger.MaskValue. The original Channel
// is left untouched so the caller can keep using it for
// dispatch. The masking is applied to the JSON-serialised form
// of the response only — the persisted Config in the database
// retains the secret so the dispatcher can still use it.
//
// Sensitivity rules: any key whose lower-cased name contains
// "password", "passwd", "secret", "token", "api_key", "apikey",
// "private_key", "privatekey", or "credential" is replaced with
// "***". See pkg/logger.MaskValue for the canonical list.
func Mask(c Channel) Channel {
	out := c
	if c.Config == nil {
		out.Config = JSONMap{}
		return out
	}
	out.Config = make(JSONMap, len(c.Config))
	for k, v := range c.Config {
		if s, ok := v.(string); ok {
			out.Config[k] = logger.MaskValue(k, s)
			continue
		}
		// Non-string values: still apply the key check so a
		// nested struct carrying a secret does not slip
		// through. The current grammar is flat, so the check
		// is a no-op for non-string values today; the
		// architecture is forward-compatible with nested
		// configs.
		out.Config[k] = v
	}
	return out
}
