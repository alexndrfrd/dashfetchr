package handlers

import (
	"io"
	"net/http"

	"dashfetchr/internal/app"
	"dashfetchr/internal/infra/http/response"
	"dashfetchr/internal/ports"
)

type Webhooks struct {
	App *app.App
}

func (h Webhooks) Bolt(w http.ResponseWriter, r *http.Request) {
	h.ingest(w, r, "bolt_food")
}

func (h Webhooks) ingest(w http.ResponseWriter, r *http.Request, carrierID ports.CarrierID) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "read_body", err.Error())
		return
	}
	headers := map[string]string{}
	for k := range r.Header {
		headers[k] = r.Header.Get(k)
	}
	if err := h.App.Webhook.Process(r.Context(), carrierID, raw, headers); err != nil {
		response.Error(w, http.StatusUnprocessableEntity, "webhook_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
