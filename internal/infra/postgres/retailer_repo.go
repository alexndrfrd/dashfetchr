package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"dashfetchr/internal/core/retailer"
	"dashfetchr/internal/ports"
)

// RetailerRepo implements ports.RetailerRepository.
type RetailerRepo struct {
	pool *pgxpool.Pool
}

func NewRetailerRepo(pool *pgxpool.Pool) *RetailerRepo {
	return &RetailerRepo{pool: pool}
}

func (r *RetailerRepo) GetByAPIKeyHash(ctx context.Context, hash string) (*retailer.Retailer, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, slug, is_active
		FROM retailers WHERE api_key_hash = $1
	`, hash)

	var ret retailer.Retailer
	err := row.Scan(&ret.ID, &ret.Name, &ret.Slug, &ret.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: scan retailer: %w", err)
	}
	return &ret, nil
}
