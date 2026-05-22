package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"dashfetchr/internal/core/delivery"
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
		return nil, fmt.Errorf("memory: delivery %s not found", id)
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

func cloneDelivery(d *delivery.Delivery) *delivery.Delivery {
	cp := *d
	if d.Rider != nil {
		r := *d.Rider
		cp.Rider = &r
	}
	return &cp
}
