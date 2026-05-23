package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"dashfetchr/internal/core/awb"
	"dashfetchr/internal/ports"
)

// AWBRepo implements ports.AWBRepository.
type AWBRepo struct {
	pool *pgxpool.Pool
}

func NewAWBRepo(pool *pgxpool.Pool) *AWBRepo {
	return &AWBRepo{pool: pool}
}

func (r *AWBRepo) Save(ctx context.Context, a *awb.AWB) error {
	if a == nil {
		return fmt.Errorf("postgres: nil awb")
	}
	ext, err := marshalJSON(a.ExternalAWBs)
	if err != nil {
		return err
	}
	pkg, err := marshalJSON(a.Package)
	if err != nil {
		return err
	}
	meta, err := marshalJSON(map[string]any{})
	if err != nil {
		return err
	}

	var customerID *uuid.UUID
	if a.CustomerID != nil {
		customerID = a.CustomerID
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO awbs (
			id, internal_awb, retailer_id, customer_id, external_awbs, package,
			declared_value_minor, declared_value_currency, state, state_reason, metadata,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			customer_id = EXCLUDED.customer_id,
			external_awbs = EXCLUDED.external_awbs,
			package = EXCLUDED.package,
			declared_value_minor = EXCLUDED.declared_value_minor,
			declared_value_currency = EXCLUDED.declared_value_currency,
			state = EXCLUDED.state,
			state_reason = EXCLUDED.state_reason,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`,
		a.ID, a.InternalAWB, a.RetailerID, customerID, ext, pkg,
		a.Package.DeclaredValueMinor, nullIfEmpty(a.Package.DeclaredValueCcy),
		string(a.State), nullIfEmpty(a.StateReason), meta,
		a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: save awb: %w", err)
	}
	return nil
}

func (r *AWBRepo) GetByID(ctx context.Context, id uuid.UUID) (*awb.AWB, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, internal_awb, retailer_id, customer_id, external_awbs, package,
			state, state_reason, created_at, updated_at
		FROM awbs WHERE id = $1
	`, id)
	return scanAWB(row)
}

func (r *AWBRepo) GetByInternalAWB(ctx context.Context, internalAWB string) (*awb.AWB, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, internal_awb, retailer_id, customer_id, external_awbs, package,
			state, state_reason, created_at, updated_at
		FROM awbs WHERE internal_awb = $1
	`, internalAWB)
	return scanAWB(row)
}

func scanAWB(row pgx.Row) (*awb.AWB, error) {
	var a awb.AWB
	var customerID *uuid.UUID
	var extJSON, pkgJSON []byte
	var state string

	err := row.Scan(
		&a.ID, &a.InternalAWB, &a.RetailerID, &customerID, &extJSON, &pkgJSON,
		&state, &a.StateReason, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: scan awb: %w", err)
	}
	a.CustomerID = customerID
	a.State = awb.State(state)
	if err := unmarshalJSON(extJSON, &a.ExternalAWBs); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(pkgJSON, &a.Package); err != nil {
		return nil, err
	}
	return &a, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
