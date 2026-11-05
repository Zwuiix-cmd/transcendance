package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"card-arena/api/internal/api"
	"card-arena/api/internal/config"
	"card-arena/api/internal/db"
	"card-arena/api/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg := config.Load()
	shutdownTelemetry, err := telemetry.Setup(cfg.OTELTracesStdout)
	if err != nil {
		slog.Error("could not initialize telemetry", "error", err)
		os.Exit(1)
	}
	defer shutdownTelemetry(context.Background())
	schemaPath := resolveSchema(cfg.SchemaPath)
	store, err := db.Open(cfg.DatabasePath, schemaPath)
	if err != nil {
		slog.Error("could not open database", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	if err = api.Bootstrap(store, cfg); err != nil {
		slog.Error("could not bootstrap API", "error", err)
		os.Exit(1)
	}
	if err = store.ResetTransientGames(); err != nil {
		slog.Error("could not reset transient games", "error", err)
		os.Exit(1)
	}
	apiServer := api.NewServer(cfg, store)
	defer apiServer.Close()
	handler := apiServer.Handler()
	if cfg.OTELTracesStdout {
		handler = otelhttp.NewHandler(handler, "card-arena-api")
	}
	server := &http.Server{Addr: ":" + cfg.Port, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("REST API listening", "port", cfg.Port, "database", cfg.DatabasePath, "debugAuth", cfg.DebugAuthEnabled)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("REST API stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("REST API shutdown failed", "error", err)
	}
}

func resolveSchema(configured string) string {
	candidates := []string{configured, "/app/schema.sql", filepath.Join("apps", "api", "db", "schema.sql"), filepath.Join("db", "schema.sql"), filepath.Join("src", "main", "resources", "db", "schema.sql")}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return configured
}
