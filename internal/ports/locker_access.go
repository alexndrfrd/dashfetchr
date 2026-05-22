package ports

import (
	"context"
	"errors"
)

// LockerAccessPort abstracts how DashFetchr obtains permission to open
// a third-party locker (Sameday easybox, etc.) to retrieve a package.
//
// Two implementations are expected:
//   - B2B (partner API): tokenized access granted by Sameday for our riders
//   - PIN forwarding (interim): the customer passes their pickup PIN to us
//     and we forward it to the rider via a single-use signed URL
type LockerAccessPort interface {
	Name() string

	// GrantAccess returns a one-time access credential for the rider to open
	// the locker. May be a PIN, QR code, or short-lived token depending on
	// the implementation.
	GrantAccess(ctx context.Context, req LockerAccessRequest) (*LockerAccessGrant, error)

	// RevokeAccess invalidates a previously granted credential (e.g. on cancel).
	RevokeAccess(ctx context.Context, grantID string) error
}

type LockerAccessRequest struct {
	LockerID       string
	ExternalAWB    string // tracking number with the locker network
	RiderID        string
	DeliveryID     string
	IdempotencyKey string
}

type LockerAccessGrant struct {
	GrantID       string
	Credential    string // PIN/QR/token (sensitive, never log)
	ExpiresAt     int64  // unix seconds
	OneTime       bool
	OpenInstructions string
}

var (
	ErrLockerNotFound       = errors.New("locker: not found")
	ErrLockerAccessDenied   = errors.New("locker: access denied")
	ErrLockerEmpty          = errors.New("locker: package not present")
	ErrLockerAlreadyOpened  = errors.New("locker: already opened")
)
