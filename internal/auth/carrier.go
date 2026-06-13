package auth

import "context"

type carrierCtxKey int

const carrierIDCtxKey carrierCtxKey = iota

// WithCarrierID returns a copy of ctx carrying the authenticated carrier ID.
func WithCarrierID(ctx context.Context, carrierID string) context.Context {
	return context.WithValue(ctx, carrierIDCtxKey, carrierID)
}

// CarrierIDFromContext returns the authenticated carrier ID, or ("", false) if
// the request was not authenticated as a carrier/rider.
func CarrierIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(carrierIDCtxKey).(string)
	return id, ok && id != ""
}
