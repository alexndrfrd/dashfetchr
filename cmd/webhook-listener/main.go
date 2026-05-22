package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"dashfetchr/internal/app"
	"dashfetchr/internal/config"
	"dashfetchr/internal/infra/http/handlers"
	"dashfetchr/internal/infra/http/middleware"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("bootstrap failed", "err", err)
		os.Exit(1)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recover(logger))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))

	wh := handlers.Webhooks{App: application}
	r.Get("/healthz", handlers.Health{}.Liveness)
	r.Post("/webhooks/bolt", wh.Bolt)

	srv := &http.Server{Addr: cfg.HTTP.WebhookListenerAddr, Handler: r}

	go func() {
		logger.Info("dashfetchr.webhook-listener listening", "addr", cfg.HTTP.WebhookListenerAddr)
		_ = srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
