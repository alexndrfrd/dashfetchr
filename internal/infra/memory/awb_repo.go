package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"dashfetchr/internal/core/awb"
	"dashfetchr/internal/ports"
)

// AWBRepo is an in-memory AWBRepository for local development.
type AWBRepo struct {
	mu    sync.RWMutex
	byID  map[uuid.UUID]*awb.AWB
	byAWB map[string]uuid.UUID
}

func NewAWBRepo() *AWBRepo {
	return &AWBRepo{
		byID:  make(map[uuid.UUID]*awb.AWB),
		byAWB: make(map[string]uuid.UUID),
	}
}

func (r *AWBRepo) Save(ctx context.Context, a *awb.AWB) error {
	_ = ctx
	if a == nil {
		return fmt.Errorf("memory: nil awb")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[a.ID] = cloneAWB(a)
	r.byAWB[a.InternalAWB] = a.ID
	return nil
}

func (r *AWBRepo) GetByID(ctx context.Context, id uuid.UUID) (*awb.AWB, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return cloneAWB(a), nil
}

func (r *AWBRepo) GetByInternalAWB(ctx context.Context, internalAWB string) (*awb.AWB, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byAWB[internalAWB]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return cloneAWB(r.byID[id]), nil
}

func cloneAWB(a *awb.AWB) *awb.AWB {
	cp := *a
	cp.ExternalAWBs = append([]awb.ExternalRef(nil), a.ExternalAWBs...)
	return &cp
}
