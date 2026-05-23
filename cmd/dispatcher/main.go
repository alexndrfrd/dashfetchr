// Command dispatcher is the background worker for scheduled dispatches.
// Polls pending deliveries and calls DispatchService (replace with SQS/river in prod).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dashfetchr/internal/app"
	"dashfetchr/internal/config"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("bootstrap failed", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interval := durationEnv("DISPATCH_POLL_INTERVAL", 30*time.Second)
	batch := intEnv("DISPATCH_BATCH_SIZE", 20)

	go runLoop(ctx, application, logger, interval, batch)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("dashfetchr.dispatcher stopped")
}

func runLoop(ctx context.Context, application *app.App, log *slog.Logger, interval time.Duration, batch int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := application.Dispatch.DispatchPending(ctx, batch)
			if err != nil {
				log.Error("dispatcher.tick_failed", "err", err)
				continue
			}
			if n > 0 {
				log.Info("dispatcher.dispatched", "count", n)
			} else {
				log.Debug("dispatcher.tick", "dispatched", 0)
			}
		}
	}
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

func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}
