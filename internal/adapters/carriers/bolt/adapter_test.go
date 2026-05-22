package bolt_test

import (
	"testing"

	"dashfetchr/internal/adapters/carriers/bolt"
	"dashfetchr/tests/contract"
)

// TestBoltContract runs the shared carrier contract test suite against
// the Bolt adapter. Any new adapter (Wolt, Glovo, etc.) only needs to
// follow this same pattern; the suite is identical.
func TestBoltContract(t *testing.T) {
	adapter := bolt.New(bolt.Config{
		BaseURL:       "http://127.0.0.1:0", // never actually called; offline scenarios
		APIKey:        "test-key",
		WebhookSecret: "test-secret",
		ServiceArea:   "bucharest",
		Sandbox:       true,
	})

	contract.RunCarrierContract(t, adapter)
}
