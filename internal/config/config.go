// Package config loads runtime configuration from environment variables.
// All binaries (api, dispatcher, webhook-listener) share the same shape.
package config

import (
	"os"
	"strconv"
	"time"

	"dashfetchr/internal/carrier"
	"dashfetchr/internal/adapters/carriers/bolt"
)

// Config is the root configuration object.
type Config struct {
	Env      string // local | dev | staging | prod
	LogLevel string

	HTTP HTTPConfig
	DB   DBConfig

	Carriers carrier.Config
}

type HTTPConfig struct {
	APIAddr            string
	WebhookListenerAddr string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
}

type DBConfig struct {
	URL         string
	MaxConns    int32
	UseMemory   bool // true = in-memory repos (local dev without Postgres)
}

// Load reads configuration from the environment with sensible defaults
// for local development.
func Load() Config {
	return Config{
		Env:      env("ENV", "local"),
		LogLevel: env("LOG_LEVEL", "info"),
		HTTP: HTTPConfig{
			APIAddr:             env("API_ADDR", ":8080"),
			WebhookListenerAddr: env("WEBHOOK_ADDR", ":8081"),
			ReadTimeout:         durationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:        durationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second),
		},
		DB: DBConfig{
			URL:       env("DATABASE_URL", "postgres://dashfetchr:dashfetchr@localhost:5432/dashfetchr?sslmode=disable"),
			MaxConns:  int32(intEnv("DB_MAX_CONNS", 10)),
			UseMemory: boolEnv("USE_MEMORY_STORE", true),
		},
		Carriers: carrier.Config{
			Bolt: bolt.Config{
				BaseURL:       env("BOLT_BASE_URL", "https://node.bolt.eu"),
				APIKey:        env("BOLT_API_KEY", ""),
				WebhookSecret: env("BOLT_WEBHOOK_SECRET", "local-dev-secret"),
				ServiceArea:   env("BOLT_SERVICE_AREA", "bucharest"),
				Sandbox:       boolEnv("BOLT_SANDBOX", true),
			},
		},
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func boolEnv(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func durationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
