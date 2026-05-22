package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"dashfetchr/internal/ports"
)

type RoutingRepo struct {
	mu   sync.Mutex
	logs map[uuid.UUID][]ports.RoutingDecisionRecord
}

func NewRoutingRepo() *RoutingRepo {
	return &RoutingRepo{logs: make(map[uuid.UUID][]ports.RoutingDecisionRecord)}
}

func (r *RoutingRepo) Save(ctx context.Context, deliveryID uuid.UUID, record ports.RoutingDecisionRecord) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs[deliveryID] = append(r.logs[deliveryID], record)
	return nil
}
