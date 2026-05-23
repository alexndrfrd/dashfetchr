package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"dashfetchr/internal/ports"
)

// RoutingRepo implements ports.RoutingDecisionRepository.
type RoutingRepo struct {
	pool *pgxpool.Pool
}

func NewRoutingRepo(pool *pgxpool.Pool) *RoutingRepo {
	return &RoutingRepo{pool: pool}
}

func (r *RoutingRepo) Save(ctx context.Context, deliveryID uuid.UUID, record ports.RoutingDecisionRecord) error {
	candidates := []byte("[]")
	if record.Candidates != "" {
		candidates = []byte(record.Candidates)
	}
	chosen, err := marshalJSON(map[string]any{
		"carrier_id": record.ChosenID,
		"score":      record.Score,
	})
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO routing_decisions (delivery_id, strategy, candidates, chosen, reasoning)
		VALUES ($1, $2, $3, $4, $5)
	`, deliveryID, record.Strategy, candidates, chosen, record.Reasoning)
	if err != nil {
		return fmt.Errorf("postgres: save routing decision: %w", err)
	}
	return nil
}
