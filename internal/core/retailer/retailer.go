// Package retailer holds the retailer aggregate: a B2B client (shop) that
// books concierge deliveries through the API.
package retailer

import "github.com/google/uuid"

// Retailer is the authenticated B2B caller of the API. Each retailer owns
// the AWBs and deliveries it creates.
type Retailer struct {
	ID       uuid.UUID
	Name     string
	Slug     string
	IsActive bool
}
