// Package auth holds the primitives for authenticating API callers:
// API-key hashing and request-context propagation of the caller identity.
//
// API keys are high-entropy random strings (e.g. "df_live_<random>"), so a
// fast SHA-256 hash is sufficient for storage: it lets the DB look the key up
// by equality while never persisting the raw secret. (bcrypt would prevent the
// equality lookup and buys nothing for non-guessable keys.)
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"dashfetchr/internal/core/retailer"
)

// HashAPIKey returns the hex-encoded SHA-256 of a raw API key. The result is
// what gets stored in retailers.api_key_hash and compared at lookup time.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

type contextKey int

const retailerCtxKey contextKey = iota

// WithRetailer returns a copy of ctx carrying the authenticated retailer.
func WithRetailer(ctx context.Context, r *retailer.Retailer) context.Context {
	return context.WithValue(ctx, retailerCtxKey, r)
}

// RetailerFromContext returns the authenticated retailer, or (nil, false) if
// the request was not authenticated.
func RetailerFromContext(ctx context.Context) (*retailer.Retailer, bool) {
	r, ok := ctx.Value(retailerCtxKey).(*retailer.Retailer)
	return r, ok && r != nil
}
