package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a thin transport over the Bolt Delivery REST API.
// It contains NO business logic; all mapping happens in the Mapper.
type Client struct {
	cfg  Config
	http *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// WebhookSecret exposes the shared secret used to verify inbound webhooks.
func (c *Client) WebhookSecret() string { return c.cfg.WebhookSecret }

// CreateOrder POSTs a new delivery order. The idempotencyKey is forwarded
// in a header so duplicate requests return the original order.
func (c *Client) CreateOrder(ctx context.Context, req boltCreateOrderRequest, idempotencyKey string) (*boltOrder, error) {
	var out boltOrder
	if err := c.do(ctx, http.MethodPost, "/v1/orders", req, &out, map[string]string{
		"Idempotency-Key": idempotencyKey,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOrder fetches an order by ID.
func (c *Client) GetOrder(ctx context.Context, orderID string) (*boltOrder, error) {
	var out boltOrder
	if err := c.do(ctx, http.MethodGet, "/v1/orders/"+orderID, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelOrder requests cancellation with a reason.
func (c *Client) CancelOrder(ctx context.Context, orderID, reason string) error {
	body := map[string]string{"reason": reason}
	return c.do(ctx, http.MethodPost, "/v1/orders/"+orderID+"/cancel", body, nil, nil)
}

// Estimate returns a quote.
func (c *Client) Estimate(ctx context.Context, req boltEstimateRequest) (*boltEstimateResponse, error) {
	var out boltEstimateResponse
	if err := c.do(ctx, http.MethodPost, "/v1/estimate", req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// do is the low-level HTTP wrapper. It encodes JSON request bodies,
// sets auth headers, and maps HTTP status codes to typed errors so the
// adapter layer can translate to ports.Err*.
func (c *Client) do(ctx context.Context, method, path string, body, out any, extraHeaders map[string]string) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range extraHeaders {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return &transportError{cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp.Body)
		return &httpError{
			Status: resp.StatusCode,
			Body:   string(buf),
		}
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("bolt: decode response: %w", err)
	}
	return nil
}

// transportError wraps network-level failures.
type transportError struct{ cause error }

func (e *transportError) Error() string { return "bolt transport: " + e.cause.Error() }
func (e *transportError) Unwrap() error { return e.cause }

// httpError represents a non-2xx response.
type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("bolt http %d: %s", e.Status, e.Body)
}

// mapClientError translates transport/HTTP failures to standardized
// domain errors so the routing engine can decide on fallback behavior.
func mapClientError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*transportError); ok {
		return wrapPorts(err, "carrier unavailable")
	}
	if he, ok := err.(*httpError); ok {
		switch {
		case he.Status == 401, he.Status == 403:
			return wrapPorts(err, "auth failed")
		case he.Status == 404:
			return wrapPorts(err, "shipment not found")
		case he.Status == 409:
			return wrapPorts(err, "conflict / already cancelled")
		case he.Status == 422:
			return wrapPorts(err, "invalid request")
		case he.Status == 429:
			return wrapPorts(err, "rate limited")
		case he.Status >= 500:
			return wrapPorts(err, "carrier unavailable")
		}
	}
	return err
}

// wrapPorts is a small helper to construct error chains that callers can
// inspect with errors.Is using the standard ports.Err* sentinels.
func wrapPorts(cause error, msg string) error {
	return &wrappedErr{cause: cause, msg: msg}
}

type wrappedErr struct {
	cause error
	msg   string
}

func (w *wrappedErr) Error() string { return w.msg + ": " + w.cause.Error() }
func (w *wrappedErr) Unwrap() error { return w.cause }
