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
	"github.com/utkugulgec/agenttape/internal/api"
	"github.com/utkugulgec/agenttape/internal/config"
	"github.com/utkugulgec/agenttape/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	pool, err := storage.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("server starting", "port", cfg.HTTPPort, "db", "connected")

	sessions := storage.NewSessionRepo(pool)
	spans := storage.NewSpanRepo(pool)

	r := chi.NewRouter()
	r.Use(api.RequestLogger)

	h := api.NewHandler(pool, sessions, spans)
	r.Get("/health", h.Health)
	r.Get("/sessions", h.ListSessions)
	r.Get("/sessions/{id}", h.GetSession)
	r.Get("/sessions/{id}/spans", h.ListSpans)
	r.Post("/v1/traces", h.IngestTraces)

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("server listening", "port", cfg.HTTPPort)
	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}
