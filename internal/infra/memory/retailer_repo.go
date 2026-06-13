package memory

import (
	"context"

	"github.com/google/uuid"

	"dashfetchr/internal/auth"
	"dashfetchr/internal/core/retailer"
	"dashfetchr/internal/ports"
)

// DevAPIKey is the API key for the seeded dev retailer in in-memory mode.
// It matches the hash inserted by migration 0003 so curl examples work the
// same with or without Postgres. Local development only — never in production.
const DevAPIKey = "df_dev_pilot_2026"

// DevRetailerID is the seeded pilot retailer used across local demos.
var DevRetailerID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// RetailerRepo is an in-memory RetailerRepository for local development,
// pre-seeded with the dev pilot retailer.
type RetailerRepo struct {
	byKeyHash map[string]*retailer.Retailer
}

func NewRetailerRepo() *RetailerRepo {
	dev := &retailer.Retailer{
		ID:       DevRetailerID,
		Name:     "Dev Pilot Retailer",
		Slug:     "dev-pilot",
		IsActive: true,
	}
	return &RetailerRepo{
		byKeyHash: map[string]*retailer.Retailer{
			auth.HashAPIKey(DevAPIKey): dev,
		},
	}
}

func (r *RetailerRepo) GetByAPIKeyHash(ctx context.Context, hash string) (*retailer.Retailer, error) {
	_ = ctx
	ret, ok := r.byKeyHash[hash]
	if !ok {
		return nil, ports.ErrNotFound
	}
	cp := *ret
	return &cp, nil
}
