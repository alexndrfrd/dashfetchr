package ports

import (
	"context"
	"errors"
)

// NotificationPort abstracts outbound messaging providers (Twilio SMS,
// WhatsApp Business via 360dialog/Twilio, push, email).
type NotificationPort interface {
	Channel() NotificationChannel
	Send(ctx context.Context, req SendNotificationRequest) (*SendNotificationResult, error)
}

type NotificationChannel string

const (
	ChannelSMS      NotificationChannel = "sms"
	ChannelWhatsApp NotificationChannel = "whatsapp"
	ChannelEmail    NotificationChannel = "email"
	ChannelPush     NotificationChannel = "push"
)

type SendNotificationRequest struct {
	IdempotencyKey string
	To             string // phone E.164 for SMS/WhatsApp, email for email, device token for push
	Template       string // template ID configured with provider
	Variables      map[string]string
	Locale         string // "ro", "en"
	CTAURL         string
	Metadata       map[string]string
}

type SendNotificationResult struct {
	ProviderMessageID string
	State             string // queued|sent|delivered|failed
}

var (
	ErrNotificationProviderUnavailable = errors.New("notification: provider unavailable")
	ErrNotificationInvalidRecipient    = errors.New("notification: invalid recipient")
)
