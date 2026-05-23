package handlers

import (
	"context"
	"net/http"
	"time"

	"dashfetchr/internal/app"
	"dashfetchr/internal/infra/http/response"
)

type Health struct {
	App *app.App
}

func (h Health) Liveness(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h Health) Readiness(w http.ResponseWriter, r *http.Request) {
	if h.App != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.App.PingDB(ctx); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not_ready",
				"error":  err.Error(),
			})
			return
		}
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
