package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sudok8s/hookr/internal/api"
	"github.com/sudok8s/hookr/internal/dispatcher"
	"github.com/sudok8s/hookr/internal/queue"
	"github.com/sudok8s/hookr/internal/store"
)

func main() {
	// --- Structured logger (goes to stdout, ELK-friendly in Phase 2) ---
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// --- Config (env vars with sensible defaults) ---
	addr := envOr("HOOKR_ADDR", ":8080")
	dbPath := envOr("HOOKR_DB", "hookr.db")
	workers := 5 // TODO: parse from env

	// --- Dependencies ---
	db, err := store.New(dbPath)
	if err != nil {
		logger.Error("failed to open store", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	q := queue.New(1000)

	disp := dispatcher.New(q, db, logger, workers)

	handler := api.NewHandler(db, q)
	router := api.NewRouter(handler)

	// --- HTTP server ---
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --- Start dispatcher in background ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go disp.Start(ctx)

	// --- Start HTTP server in background ---
	go func() {
		logger.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// --- Graceful shutdown on SIGTERM / SIGINT ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	logger.Info("shutting down...")
	cancel() // stop dispatcher workers

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
	logger.Info("bye")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
