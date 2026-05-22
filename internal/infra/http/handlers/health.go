package handlers

import (
	"net/http"

	"dashfetchr/internal/infra/http/response"
)

type Health struct{}

func (h Health) Liveness(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h Health) Readiness(w http.ResponseWriter, _ *http.Request) {
	// M1: check Postgres, Redis, carrier sandbox reachability.
	response.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
