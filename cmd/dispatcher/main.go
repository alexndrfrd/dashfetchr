// Command dispatcher is the background worker for scheduled dispatches.
// Skeleton: polls pending deliveries and calls DispatchService.
// M1: replace with SQS consumer or river job queue.
package main

import (
	"context"
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

	go runLoop(ctx, application, logger)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("dashfetchr.dispatcher stopped")
}

func runLoop(ctx context.Context, _ *app.App, log *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Debug("dispatcher.tick", "note", "implement pending delivery scan in M1")
		}
	}
}
