package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ptt-fleet/services/api-server/internal/config"
	"ptt-fleet/services/api-server/internal/db"
	"ptt-fleet/services/api-server/internal/httpserver"
	"ptt-fleet/services/api-server/internal/ws"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := db.New(ctx, cfg.DatabaseURL, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer store.Close()

	realtimeHub := ws.NewHub()
	defer realtimeHub.CloseAll()

	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           httpserver.NewRouter(cfg, store, realtimeHub),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("api-server listening on %s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-stopCh:
		log.Println("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	return server.Shutdown(shutdownCtx)
}
