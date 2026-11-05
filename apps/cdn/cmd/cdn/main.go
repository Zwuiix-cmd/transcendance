package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"card-arena/cdn/internal/config"
	"card-arena/cdn/internal/files"
	"card-arena/cdn/internal/httpapi"
)

func main() {
	cfg := config.Load()
	store, err := files.NewStoreWithGenerator(
		cfg.StorageDir,
		cfg.PublicBaseURL,
		cfg.AvatarGeneratorScript,
	)
	if err != nil {
		slog.Error("could not create file storage", "error", err, "path", cfg.StorageDir)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(store, cfg.InternalAPIKey, cfg.StaticAssetsDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("file CDN listening", "port", cfg.Port, "storage", cfg.StorageDir)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("file CDN stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("file CDN shutdown failed", "error", err)
	}
}
