package bolt

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

// verifySignature implements Bolt's webhook signature scheme.
//
// Bolt signs the raw body with HMAC-SHA256 using the shared webhook
// secret and places the hex digest in the X-Bolt-Signature header,
// prefixed with "sha256=".
//
// We compare in constant time to avoid timing attacks.
//
// NOTE: The exact header name and format is documented in Bolt's B2B
// integration guide and may differ between sandbox and prod. Update
// here if their docs change; nothing else in the codebase needs to know.
func verifySignature(body []byte, headers map[string]string, secret string) error {
	if secret == "" {
		// Misconfiguration: refuse to verify (fail closed).
		return errors.New("bolt: webhook secret not configured")
	}

	provided := headerLookup(headers, "X-Bolt-Signature")
	if provided == "" {
		return errors.New("bolt: missing X-Bolt-Signature header")
	}
	provided = strings.TrimPrefix(provided, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return errors.New("bolt: signature mismatch")
	}
	return nil
}

// headerLookup performs a case-insensitive header lookup.
func headerLookup(h map[string]string, key string) string {
	if v, ok := h[key]; ok {
		return v
	}
	low := strings.ToLower(key)
	for k, v := range h {
		if strings.ToLower(k) == low {
			return v
		}
	}
	return ""
}
