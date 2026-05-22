package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"dashfetchr/internal/core/custody"
)

// CustodyRepo implements custody.Repository in memory.
type CustodyRepo struct {
	mu       sync.RWMutex
	byDel    map[uuid.UUID][]custody.Event
}

func NewCustodyRepo() *CustodyRepo {
	return &CustodyRepo{byDel: make(map[uuid.UUID][]custody.Event)}
}

func (r *CustodyRepo) Append(ctx context.Context, e custody.Event) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	events := r.byDel[e.DeliveryID]
	for _, existing := range events {
		if existing.EventID == e.EventID {
			return nil // idempotent
		}
	}
	r.byDel[e.DeliveryID] = append(events, e)
	return nil
}

func (r *CustodyRepo) LastByDelivery(ctx context.Context, deliveryID uuid.UUID) (custody.Event, bool, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := r.byDel[deliveryID]
	if len(events) == 0 {
		return custody.Event{}, false, nil
	}
	return events[len(events)-1], true, nil
}

func (r *CustodyRepo) ListByDelivery(ctx context.Context, deliveryID uuid.UUID) ([]custody.Event, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := r.byDel[deliveryID]
	out := make([]custody.Event, len(events))
	copy(out, events)
	return out, nil
}

// VerifyAll is a dev helper (not part of the interface).
func (r *CustodyRepo) VerifyAll(deliveryID uuid.UUID) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.byDel[deliveryID]; !ok {
		return fmt.Errorf("memory: no events for %s", deliveryID)
	}
	return nil
}
