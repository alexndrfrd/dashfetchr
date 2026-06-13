package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"dashfetchr/internal/auth"
	"dashfetchr/internal/core/retailer"
	"dashfetchr/internal/infra/http/middleware"
	"dashfetchr/internal/ports"
)

// stubRetailers is a minimal ports.RetailerRepository for the middleware test.
type stubRetailers struct {
	byHash map[string]*retailer.Retailer
}

func (s stubRetailers) GetByAPIKeyHash(_ context.Context, hash string) (*retailer.Retailer, error) {
	r, ok := s.byHash[hash]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return r, nil
}

func TestRequireRetailer(t *testing.T) {
	const activeKey = "df_test_active"
	const inactiveKey = "df_test_inactive"

	active := &retailer.Retailer{ID: uuid.New(), Slug: "shop", IsActive: true}
	repo := stubRetailers{byHash: map[string]*retailer.Retailer{
		auth.HashAPIKey(activeKey):   active,
		auth.HashAPIKey(inactiveKey): {ID: uuid.New(), Slug: "old", IsActive: false},
	}}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Inner handler asserts the retailer reached the context on success.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := auth.RetailerFromContext(r.Context())
		if !ok || got.ID != active.ID {
			t.Errorf("retailer not in context: ok=%v", ok)
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequireRetailer(repo, log)(inner)

	tests := []struct {
		name       string
		authHeader string
		want       int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"unknown key", "Bearer df_test_nope", http.StatusUnauthorized},
		{"inactive retailer", "Bearer " + inactiveKey, http.StatusForbidden},
		{"valid key", "Bearer " + activeKey, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/bookings", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
