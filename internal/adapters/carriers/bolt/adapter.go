// Package bolt is the carrier adapter for Bolt Food (Bolt Delivery B2B).
//
// This package is the CANONICAL TEMPLATE for adding new carriers. Anyone
// integrating Wolt, Glovo, FAN, DHL, etc. should mirror this layout:
//
//	adapter.go        — implements ports.CarrierPort (thin: delegates)
//	client.go         — HTTP client (transport only, no business logic)
//	mapper.go         — Anti-Corruption Layer (Bolt <-> domain types)
//	types.go          — Bolt-specific types from their OpenAPI
//	webhook.go        — signature verification + raw parsing
//	capabilities.go   — declares what Bolt can do
//	adapter_test.go   — runs the shared contract test suite
//
// THE ONLY PACKAGE THAT IMPORTS BOLT TYPES IS THIS ONE.
// Nothing in internal/core may know that Bolt exists.
package bolt

import (
	"context"
	"encoding/json"

	"dashfetchr/internal/ports"
)

// Adapter implements ports.CarrierPort for Bolt Food.
type Adapter struct {
	cfg    Config
	client *Client
	mapper *Mapper
	caps   ports.CarrierCapabilities
}

// Config holds environment-specific settings for the adapter.
type Config struct {
	BaseURL       string // e.g. https://node.bolt.eu or sandbox
	APIKey        string
	WebhookSecret string
	RiderSecret   string // shared secret for Bolt rider apps calling /custody
	ServiceArea   string // e.g. "bucharest"
	Sandbox       bool
}

// New constructs a configured adapter.
func New(cfg Config) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: NewClient(cfg),
		mapper: NewMapper(cfg.ServiceArea),
		caps:   buildCapabilities(cfg),
	}
}

// Capabilities is what the routing engine sees. Stable, in-memory.
func (a *Adapter) Capabilities() ports.CarrierCapabilities {
	return a.caps
}

// CreateShipment translates the domain request to a Bolt API call.
// The mapper is the only place that knows Bolt's shape.
func (a *Adapter) CreateShipment(ctx context.Context, req ports.ShipmentRequest) (*ports.ShipmentResponse, error) {
	boltReq, err := a.mapper.ToBoltCreateOrder(req)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.CreateOrder(ctx, boltReq, req.IdempotencyKey)
	if err != nil {
		return nil, mapClientError(err)
	}
	return a.mapper.FromBoltOrder(resp), nil
}

// GetShipmentStatus polls Bolt for the latest status (used for reconciliation).
func (a *Adapter) GetShipmentStatus(ctx context.Context, externalID string) (*ports.ShipmentStatus, error) {
	order, err := a.client.GetOrder(ctx, externalID)
	if err != nil {
		return nil, mapClientError(err)
	}
	return a.mapper.FromBoltOrderStatus(order), nil
}

// CancelShipment requests cancellation; Bolt may refuse if rider is at pickup.
func (a *Adapter) CancelShipment(ctx context.Context, externalID string, reason string) error {
	return mapClientError(a.client.CancelOrder(ctx, externalID, reason))
}

// GetEstimate fetches a quote without creating an order.
func (a *Adapter) GetEstimate(ctx context.Context, req ports.EstimateRequest) (*ports.Estimate, error) {
	boltReq, err := a.mapper.ToBoltEstimate(req)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Estimate(ctx, boltReq)
	if err != nil {
		return nil, mapClientError(err)
	}
	return a.mapper.FromBoltEstimate(resp), nil
}

// ParseWebhook is called by the webhook listener with the raw POST body.
// MUST verify signature first; returns domain events on success.
func (a *Adapter) ParseWebhook(ctx context.Context, raw []byte, headers map[string]string) ([]ports.DomainEvent, error) {
	if err := verifySignature(raw, headers, a.cfg.WebhookSecret); err != nil {
		return nil, ports.ErrInvalidSignature
	}
	var wh boltWebhookPayload
	if err := json.Unmarshal(raw, &wh); err != nil {
		return nil, ports.ErrInvalidRequest
	}
	return a.mapper.ToDomainEvents(wh), nil
}
