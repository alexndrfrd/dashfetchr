package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"dashfetchr/internal/core/delivery"
	"dashfetchr/internal/ports"
)

// DeliveryRepo implements ports.DeliveryRepository.
type DeliveryRepo struct {
	pool *pgxpool.Pool
}

func NewDeliveryRepo(pool *pgxpool.Pool) *DeliveryRepo {
	return &DeliveryRepo{pool: pool}
}

func (r *DeliveryRepo) Save(ctx context.Context, d *delivery.Delivery) error {
	if d == nil {
		return fmt.Errorf("postgres: nil delivery")
	}
	pickup, err := marshalJSON(d.Pickup)
	if err != nil {
		return err
	}
	drop, err := marshalJSON(d.Drop)
	if err != nil {
		return err
	}
	var riderJSON []byte
	if d.Rider != nil {
		riderJSON, err = marshalJSON(d.Rider)
		if err != nil {
			return err
		}
	} else {
		riderJSON = []byte("null")
	}
	meta := []byte("{}")
	if d.Metadata != nil {
		meta, err = marshalJSON(d.Metadata)
		if err != nil {
			return err
		}
	}

	scheduled := toTimestamptzRange(d.ScheduledWindowStart, d.ScheduledWindowEnd)

	var idempotency *string
	if d.IdempotencyKey != "" {
		idempotency = &d.IdempotencyKey
	}
	var carrierExt *string
	if d.CarrierExternalID != "" {
		carrierExt = &d.CarrierExternalID
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO deliveries (
			id, awb_id, leg_number, carrier_id, carrier_version, carrier_external_id,
			pickup, drop, scheduled_window, state, state_reason, rider,
			price_quoted_minor, price_charged_minor, price_currency,
			estimated_pickup_at, estimated_drop_at, actual_pickup_at, actual_drop_at,
			idempotency_key, metadata, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23
		)
		ON CONFLICT (id) DO UPDATE SET
			carrier_id = EXCLUDED.carrier_id,
			carrier_version = EXCLUDED.carrier_version,
			carrier_external_id = EXCLUDED.carrier_external_id,
			pickup = EXCLUDED.pickup,
			drop = EXCLUDED.drop,
			scheduled_window = EXCLUDED.scheduled_window,
			state = EXCLUDED.state,
			state_reason = EXCLUDED.state_reason,
			rider = EXCLUDED.rider,
			price_quoted_minor = EXCLUDED.price_quoted_minor,
			price_charged_minor = EXCLUDED.price_charged_minor,
			price_currency = EXCLUDED.price_currency,
			estimated_pickup_at = EXCLUDED.estimated_pickup_at,
			estimated_drop_at = EXCLUDED.estimated_drop_at,
			actual_pickup_at = EXCLUDED.actual_pickup_at,
			actual_drop_at = EXCLUDED.actual_drop_at,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`,
		d.ID, d.AWBID, d.LegNumber, d.CarrierID, d.CarrierVersion, carrierExt,
		pickup, drop, scheduled,
		string(d.State), nullIfEmpty(d.StateReason), riderJSON,
		d.PriceQuotedMinor, nullableInt64(d.PriceChargedMinor), d.PriceCurrency,
		d.EstimatedPickupAt, d.EstimatedDropAt, d.ActualPickupAt, d.ActualDropAt,
		idempotency, meta, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: save delivery: %w", err)
	}
	return nil
}

func toTimestamptzRange(start, end *time.Time) pgtype.Range[pgtype.Timestamptz] {
	if start == nil || end == nil {
		return pgtype.Range[pgtype.Timestamptz]{}
	}
	return pgtype.Range[pgtype.Timestamptz]{
		Lower:     pgtype.Timestamptz{Time: start.UTC(), Valid: true},
		Upper:     pgtype.Timestamptz{Time: end.UTC(), Valid: true},
		LowerType: pgtype.Inclusive,
		UpperType: pgtype.Exclusive,
		Valid:     true,
	}
}

func fromTimestamptzRange(r pgtype.Range[pgtype.Timestamptz]) (start, end *time.Time) {
	if !r.Valid || !r.Lower.Valid {
		return nil, nil
	}
	s := r.Lower.Time.UTC()
	start = &s
	if r.Upper.Valid {
		e := r.Upper.Time.UTC()
		end = &e
	}
	return start, end
}

func nullableInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func (r *DeliveryRepo) GetByID(ctx context.Context, id uuid.UUID) (*delivery.Delivery, error) {
	row := r.pool.QueryRow(ctx, deliverySelectSQL+` WHERE id = $1`, id)
	return scanDelivery(row)
}

func (r *DeliveryRepo) GetByCarrierExternalID(ctx context.Context, carrierID, externalID string) (*delivery.Delivery, error) {
	row := r.pool.QueryRow(ctx, deliverySelectSQL+`
		WHERE carrier_id = $1 AND carrier_external_id = $2
	`, carrierID, externalID)
	return scanDelivery(row)
}

func (r *DeliveryRepo) ListByAWB(ctx context.Context, awbID uuid.UUID) ([]*delivery.Delivery, error) {
	rows, err := r.pool.Query(ctx, deliverySelectSQL+`
		WHERE awb_id = $1 ORDER BY leg_number
	`, awbID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list deliveries by awb: %w", err)
	}
	defer rows.Close()
	return collectDeliveries(rows)
}

func (r *DeliveryRepo) ListByState(ctx context.Context, state delivery.State, limit int) ([]*delivery.Delivery, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, deliverySelectSQL+`
		WHERE state = $1
		ORDER BY created_at ASC
		LIMIT $2
	`, string(state), limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list deliveries by state: %w", err)
	}
	defer rows.Close()
	return collectDeliveries(rows)
}

const deliverySelectSQL = `
	SELECT id, awb_id, leg_number, carrier_id, carrier_version, carrier_external_id,
		pickup, drop, scheduled_window, state, state_reason, rider,
		price_quoted_minor, price_charged_minor, price_currency,
		estimated_pickup_at, estimated_drop_at, actual_pickup_at, actual_drop_at,
		idempotency_key, metadata, created_at, updated_at
	FROM deliveries
`

func collectDeliveries(rows pgx.Rows) ([]*delivery.Delivery, error) {
	var out []*delivery.Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func scanDelivery(row pgx.Row) (*delivery.Delivery, error) {
	var d delivery.Delivery
	var pickupJSON, dropJSON, riderJSON, metaJSON []byte
	var state string
	var carrierExt, stateReason, idempotency *string
	var priceCharged *int64
	var window pgtype.Range[pgtype.Timestamptz]

	err := row.Scan(
		&d.ID, &d.AWBID, &d.LegNumber, &d.CarrierID, &d.CarrierVersion, &carrierExt,
		&pickupJSON, &dropJSON, &window, &state, &stateReason, &riderJSON,
		&d.PriceQuotedMinor, &priceCharged, &d.PriceCurrency,
		&d.EstimatedPickupAt, &d.EstimatedDropAt, &d.ActualPickupAt, &d.ActualDropAt,
		&idempotency, &metaJSON, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: scan delivery: %w", err)
	}
	if carrierExt != nil {
		d.CarrierExternalID = *carrierExt
	}
	if stateReason != nil {
		d.StateReason = *stateReason
	}
	if idempotency != nil {
		d.IdempotencyKey = *idempotency
	}
	if priceCharged != nil {
		d.PriceChargedMinor = *priceCharged
	}
	d.State = delivery.State(state)
	d.ScheduledWindowStart, d.ScheduledWindowEnd = fromTimestamptzRange(window)
	if err := unmarshalJSON(pickupJSON, &d.Pickup); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(dropJSON, &d.Drop); err != nil {
		return nil, err
	}
	if len(riderJSON) > 0 && string(riderJSON) != "null" {
		var rider delivery.Rider
		if err := unmarshalJSON(riderJSON, &rider); err != nil {
			return nil, err
		}
		d.Rider = &rider
	}
	if len(metaJSON) > 0 {
		if err := unmarshalJSON(metaJSON, &d.Metadata); err != nil {
			return nil, err
		}
	}
	return &d, nil
}
