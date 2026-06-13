package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"dashfetchr/internal/auth"
	"dashfetchr/internal/infra/http/response"
)

// RequireRider authenticates a carrier rider app via an
// `Authorization: Bearer {carrier_id}:{secret}` header.
//
// secrets maps carrier_id → shared secret (configured per-carrier in env).
// On success the authenticated carrier_id is stored in the request context.
func RequireRider(secrets map[string]string, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := apiKeyFromRequest(r)
			if raw == "" {
				response.Error(w, http.StatusUnauthorized, "missing_rider_token",
					"provide a rider token via 'Authorization: Bearer {carrier_id}:{secret}'")
				return
			}

			carrierID, secret, ok := strings.Cut(raw, ":")
			if !ok || carrierID == "" || secret == "" {
				response.Error(w, http.StatusUnauthorized, "invalid_rider_token",
					"token must be in the form {carrier_id}:{secret}")
				return
			}

			expected, known := secrets[carrierID]
			if !known || secret != expected {
				log.Warn("rider.auth_failed", "carrier_id", carrierID)
				response.Error(w, http.StatusUnauthorized, "invalid_rider_token", "unknown carrier or wrong secret")
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithCarrierID(r.Context(), carrierID)))
		})
	}
}
