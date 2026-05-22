package ports

import (
	"context"
	"errors"
)

// PaymentPort abstracts payment providers (Stripe, Netopia, etc.).
type PaymentPort interface {
	Name() string

	// CreateIntent reserves funds; returns a client_secret for the PWA to confirm.
	CreateIntent(ctx context.Context, req CreatePaymentIntentRequest) (*PaymentIntent, error)

	// Capture completes a previously authorized payment (after successful delivery).
	Capture(ctx context.Context, providerPaymentID string) error

	// Refund issues a full or partial refund.
	Refund(ctx context.Context, providerPaymentID string, amount Money, reason string) error

	// VerifyWebhook validates and parses a provider webhook into a PaymentEvent.
	VerifyWebhook(ctx context.Context, raw []byte, headers map[string]string) (*PaymentEvent, error)
}

type CreatePaymentIntentRequest struct {
	IdempotencyKey string
	Amount         Money
	CustomerRef    string
	DeliveryID     string
	Description    string
	Metadata       map[string]string
}

type PaymentIntent struct {
	ProviderPaymentID string
	ClientSecret      string
	State             PaymentState
}

type PaymentState string

const (
	PaymentPending    PaymentState = "pending"
	PaymentAuthorized PaymentState = "authorized"
	PaymentCaptured   PaymentState = "captured"
	PaymentFailed     PaymentState = "failed"
	PaymentRefunded   PaymentState = "refunded"
)

type PaymentEvent struct {
	ProviderPaymentID string
	DeliveryID        string
	State             PaymentState
	Amount            Money
	Provider          string
}

var (
	ErrPaymentInvalidRequest = errors.New("payment: invalid request")
	ErrPaymentDeclined       = errors.New("payment: declined")
	ErrPaymentNotFound       = errors.New("payment: not found")
)
