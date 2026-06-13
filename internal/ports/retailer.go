package ports

import (
	"context"

	"dashfetchr/internal/core/retailer"
)

// RetailerRepository looks up retailers for authentication and ownership checks.
type RetailerRepository interface {
	// GetByAPIKeyHash returns the active retailer whose API key hashes to the
	// given value, or ports.ErrNotFound if none matches.
	GetByAPIKeyHash(ctx context.Context, hash string) (*retailer.Retailer, error)
}
