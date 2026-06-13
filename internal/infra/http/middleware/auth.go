package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"dashfetchr/internal/auth"
	"dashfetchr/internal/infra/http/response"
	"dashfetchr/internal/ports"
)

// RequireRetailer authenticates the caller via an API key in the
// `Authorization: Bearer <key>` header (or the `X-API-Key` header), looks up
// the owning retailer, and stores it in the request context for handlers.
//
// Responds 401 when the key is missing/invalid and 403 when the retailer is
// known but deactivated.
func RequireRetailer(repo ports.RetailerRepository, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := apiKeyFromRequest(r)
			if key == "" {
				response.Error(w, http.StatusUnauthorized, "missing_api_key",
					"provide an API key via 'Authorization: Bearer <key>'")
				return
			}

			ret, err := repo.GetByAPIKeyHash(r.Context(), auth.HashAPIKey(key))
			if errors.Is(err, ports.ErrNotFound) {
				response.Error(w, http.StatusUnauthorized, "invalid_api_key", "unknown API key")
				return
			}
			if err != nil {
				log.Error("auth.lookup_failed", "err", err)
				response.Error(w, http.StatusInternalServerError, "auth_error", "could not verify API key")
				return
			}
			if !ret.IsActive {
				response.Error(w, http.StatusForbidden, "retailer_inactive", "retailer is deactivated")
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithRetailer(r.Context(), ret)))
		})
	}
}

// apiKeyFromRequest extracts the raw API key from the Authorization bearer
// token, falling back to the X-API-Key header.
func apiKeyFromRequest(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if rest, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}
