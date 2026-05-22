package custody

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Repository is the persistence port for the ledger. It must be
// implemented by an adapter (e.g. Postgres) outside the core.
type Repository interface {
	// Append persists an event. Implementation MUST use the underlying
	// storage's append-only guarantees (e.g. triggers in Postgres) so
	// the event row cannot be modified after insertion.
	Append(ctx context.Context, e Event) error

	// LastByDelivery returns the most recent event for a delivery so the
	// next event can be chained to it. Returns (Event{}, false) if none.
	LastByDelivery(ctx context.Context, deliveryID uuid.UUID) (Event, bool, error)

	// ListByDelivery returns all events for a delivery in sequence order.
	ListByDelivery(ctx context.Context, deliveryID uuid.UUID) ([]Event, error)
}

// Ledger is the application-side façade for recording and verifying
// custody events. It enforces the hash chain.
type Ledger struct {
	repo Repository
}

func NewLedger(repo Repository) *Ledger { return &Ledger{repo: repo} }

// Record validates the event, computes its position in the chain, and
// persists it. The caller passes in an Event with everything filled
// EXCEPT EventID, SequenceNum, PrevHash, Hash, which Record assigns.
func (l *Ledger) Record(ctx context.Context, e Event) (Event, error) {
	if err := e.Validate(); err != nil {
		return Event{}, err
	}

	prev, hasPrev, err := l.repo.LastByDelivery(ctx, e.DeliveryID)
	if err != nil {
		return Event{}, fmt.Errorf("custody: read tail: %w", err)
	}

	e.EventID = uuid.New()
	if hasPrev {
		e.SequenceNum = prev.SequenceNum + 1
		e.PrevHash = prev.Hash
	} else {
		e.SequenceNum = 0
		e.PrevHash = genesisHash
	}
	e.Hash, err = computeHash(e)
	if err != nil {
		return Event{}, err
	}

	if err := l.repo.Append(ctx, e); err != nil {
		return Event{}, fmt.Errorf("custody: append: %w", err)
	}
	return e, nil
}

// List returns all events for a delivery in sequence order.
func (l *Ledger) List(ctx context.Context, deliveryID uuid.UUID) ([]Event, error) {
	return l.repo.ListByDelivery(ctx, deliveryID)
}

// Verify replays the chain for a delivery and returns nil if every link
// is intact. Returns ErrChainBroken or ErrTampered on integrity failures.
func (l *Ledger) Verify(ctx context.Context, deliveryID uuid.UUID) error {
	events, err := l.repo.ListByDelivery(ctx, deliveryID)
	if err != nil {
		return err
	}
	expected := genesisHash
	for i, e := range events {
		if e.SequenceNum != int64(i) {
			return ErrChainBroken{At: i, Reason: "sequence gap"}
		}
		if e.PrevHash != expected {
			return ErrChainBroken{At: i, Reason: "prev_hash mismatch"}
		}
		actual, err := computeHash(e)
		if err != nil {
			return err
		}
		if actual != e.Hash {
			return ErrTampered{At: i, EventID: e.EventID}
		}
		expected = e.Hash
	}
	return nil
}

// ----------------------------------------------------------------------------
// hashing
// ----------------------------------------------------------------------------

// genesisHash is the seed value used as PrevHash for the first event of
// each delivery. Anything constant works; we use a labeled SHA-256.
var genesisHash = func() string {
	sum := sha256.Sum256([]byte("dashfetchr:custody:genesis:v1"))
	return hex.EncodeToString(sum[:])
}()

// computeHash returns SHA256(prev_hash || canonical_json(payload)).
//
// Canonical JSON: keys sorted, no whitespace, no Hash field (avoid
// circular dependency), no SequenceNum (already in the chain via prev_hash).
func computeHash(e Event) (string, error) {
	payload := struct {
		EventID     uuid.UUID       `json:"event_id"`
		DeliveryID  uuid.UUID       `json:"delivery_id"`
		Type        EventType       `json:"type"`
		OccurredAt  int64           `json:"occurred_at_unix_ms"`
		CarrierTime *int64          `json:"carrier_time_unix_ms,omitempty"`
		Actor       Actor           `json:"actor"`
		Location    *GeoPoint       `json:"location,omitempty"`
		Photos      []PhotoRef      `json:"photos,omitempty"`
		Signature   *SignatureRef   `json:"signature,omitempty"`
		Reason      string          `json:"reason,omitempty"`
		Metadata    json.RawMessage `json:"metadata,omitempty"`
	}{
		EventID:    e.EventID,
		DeliveryID: e.DeliveryID,
		Type:       e.Type,
		OccurredAt: e.OccurredAt.UnixMilli(),
		Actor:      e.Actor,
		Location:   e.Location,
		Photos:     e.Photos,
		Signature:  e.Signature,
		Reason:     e.Reason,
		Metadata:   e.Metadata,
	}
	if e.CarrierTime != nil {
		ms := e.CarrierTime.UnixMilli()
		payload.CarrierTime = &ms
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write([]byte(e.PrevHash))
	h.Write([]byte{0x1f}) // delimiter
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ----------------------------------------------------------------------------
// errors
// ----------------------------------------------------------------------------

type ErrChainBroken struct {
	At     int
	Reason string
}

func (e ErrChainBroken) Error() string {
	return fmt.Sprintf("custody: chain broken at index %d: %s", e.At, e.Reason)
}

type ErrTampered struct {
	At      int
	EventID uuid.UUID
}

func (e ErrTampered) Error() string {
	return fmt.Sprintf("custody: tampered event at index %d (%s)", e.At, e.EventID)
}

var _ error = ErrChainBroken{}
var _ error = ErrTampered{}
var _ = errors.New
