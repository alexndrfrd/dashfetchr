package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"dashfetchr/internal/core/custody"
)

// CustodyRepo implements custody.Repository.
type CustodyRepo struct {
	pool *pgxpool.Pool
}

func NewCustodyRepo(pool *pgxpool.Pool) *CustodyRepo {
	return &CustodyRepo{pool: pool}
}

func (r *CustodyRepo) Append(ctx context.Context, e custody.Event) error {
	actor, err := marshalJSON(e.Actor)
	if err != nil {
		return err
	}
	var location []byte
	if e.Location != nil {
		location, err = marshalJSON(e.Location)
		if err != nil {
			return err
		}
	}
	photos, err := marshalJSON(e.Photos)
	if err != nil {
		return err
	}
	if e.Photos == nil {
		photos = []byte("[]")
	}
	var signature []byte
	if e.Signature != nil {
		signature, err = marshalJSON(e.Signature)
		if err != nil {
			return err
		}
	}
	meta := []byte("{}")
	if len(e.Metadata) > 0 {
		meta = e.Metadata
	}

	tag, err := r.pool.Exec(ctx, `
		INSERT INTO custody_events (
			event_id, delivery_id, sequence_num, type, occurred_at, carrier_time,
			actor, location, photos, signature, reason, metadata, prev_hash, hash
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (event_id) DO NOTHING
	`,
		e.EventID, e.DeliveryID, e.SequenceNum, string(e.Type), e.OccurredAt, e.CarrierTime,
		actor, location, photos, signature, nullIfEmpty(e.Reason), meta, e.PrevHash, e.Hash,
	)
	if err != nil {
		return fmt.Errorf("postgres: append custody: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil // idempotent duplicate event_id
	}
	return nil
}

func (r *CustodyRepo) LastByDelivery(ctx context.Context, deliveryID uuid.UUID) (custody.Event, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT event_id, delivery_id, sequence_num, type, occurred_at, carrier_time,
			actor, location, photos, signature, reason, metadata, prev_hash, hash
		FROM custody_events
		WHERE delivery_id = $1
		ORDER BY sequence_num DESC
		LIMIT 1
	`, deliveryID)
	e, err := scanCustodyEvent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return custody.Event{}, false, nil
	}
	if err != nil {
		return custody.Event{}, false, err
	}
	return e, true, nil
}

func (r *CustodyRepo) ListByDelivery(ctx context.Context, deliveryID uuid.UUID) ([]custody.Event, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_id, delivery_id, sequence_num, type, occurred_at, carrier_time,
			actor, location, photos, signature, reason, metadata, prev_hash, hash
		FROM custody_events
		WHERE delivery_id = $1
		ORDER BY sequence_num ASC
	`, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list custody: %w", err)
	}
	defer rows.Close()

	var out []custody.Event
	for rows.Next() {
		e, err := scanCustodyEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanCustodyEvent(row pgx.Row) (custody.Event, error) {
	var e custody.Event
	var actorJSON, locationJSON, photosJSON, signatureJSON, metaJSON []byte
	var typ string
	var reason *string

	err := row.Scan(
		&e.EventID, &e.DeliveryID, &e.SequenceNum, &typ, &e.OccurredAt, &e.CarrierTime,
		&actorJSON, &locationJSON, &photosJSON, &signatureJSON, &reason, &metaJSON,
		&e.PrevHash, &e.Hash,
	)
	if err != nil {
		return custody.Event{}, fmt.Errorf("postgres: scan custody: %w", err)
	}
	e.Type = custody.EventType(typ)
	if reason != nil {
		e.Reason = *reason
	}
	if err := unmarshalJSON(actorJSON, &e.Actor); err != nil {
		return custody.Event{}, err
	}
	if len(locationJSON) > 0 && string(locationJSON) != "null" {
		var loc custody.GeoPoint
		if err := unmarshalJSON(locationJSON, &loc); err != nil {
			return custody.Event{}, err
		}
		e.Location = &loc
	}
	if err := unmarshalJSON(photosJSON, &e.Photos); err != nil {
		return custody.Event{}, err
	}
	if len(signatureJSON) > 0 && string(signatureJSON) != "null" {
		var sig custody.SignatureRef
		if err := unmarshalJSON(signatureJSON, &sig); err != nil {
			return custody.Event{}, err
		}
		e.Signature = &sig
	}
	if len(metaJSON) > 0 && string(metaJSON) != "null" && string(metaJSON) != "{}" {
		e.Metadata = metaJSON
	}
	return e, nil
}
