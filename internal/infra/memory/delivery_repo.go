package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"dashfetchr/internal/core/delivery"
	"dashfetchr/internal/ports"
)

// DeliveryRepo is an in-memory DeliveryRepository.
type DeliveryRepo struct {
	mu      sync.RWMutex
	byID    map[uuid.UUID]*delivery.Delivery
	byAWB   map[uuid.UUID][]uuid.UUID
}

func NewDeliveryRepo() *DeliveryRepo {
	return &DeliveryRepo{
		byID:  make(map[uuid.UUID]*delivery.Delivery),
		byAWB: make(map[uuid.UUID][]uuid.UUID),
	}
}

func (r *DeliveryRepo) Save(ctx context.Context, d *delivery.Delivery) error {
	_ = ctx
	if d == nil {
		return fmt.Errorf("memory: nil delivery")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[d.ID] = cloneDelivery(d)
	found := false
	for _, id := range r.byAWB[d.AWBID] {
		if id == d.ID {
			found = true
			break
		}
	}
	if !found {
		r.byAWB[d.AWBID] = append(r.byAWB[d.AWBID], d.ID)
	}
	return nil
}

func (r *DeliveryRepo) GetByID(ctx context.Context, id uuid.UUID) (*delivery.Delivery, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return cloneDelivery(d), nil
}

func (r *DeliveryRepo) ListByAWB(ctx context.Context, awbID uuid.UUID) ([]*delivery.Delivery, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byAWB[awbID]
	out := make([]*delivery.Delivery, 0, len(ids))
	for _, id := range ids {
		if d, ok := r.byID[id]; ok {
			out = append(out, cloneDelivery(d))
		}
	}
	return out, nil
}

func (r *DeliveryRepo) GetByCarrierExternalID(ctx context.Context, carrierID, externalID string) (*delivery.Delivery, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, d := range r.byID {
		if d.CarrierID == carrierID && d.CarrierExternalID == externalID {
			return cloneDelivery(d), nil
		}
	}
	return nil, ports.ErrNotFound
}

func (r *DeliveryRepo) ListByState(ctx context.Context, state delivery.State, limit int) ([]*delivery.Delivery, error) {
	_ = ctx
	if limit <= 0 {
		limit = 50
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*delivery.Delivery, 0)
	for _, d := range r.byID {
		if d.State == state {
			out = append(out, cloneDelivery(d))
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func cloneDelivery(d *delivery.Delivery) *delivery.Delivery {
	cp := *d
	if d.Rider != nil {
		r := *d.Rider
		cp.Rider = &r
	}
	return &cp
}
