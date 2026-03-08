package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/rest"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("go-mumble-server %s (commit: %s, built: %s)\n", version, commit, buildTime)
		os.Exit(0)
	}

	configPath := flag.String("config", "", "Path to mumble-server.toml")
	frontendEmbed := flag.Bool("frontend-embed", true, "Serve embedded web UI on REST port")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	cfg.FrontendEmbed = *frontendEmbed

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		slog.Error("database open failed", "err", err)
		os.Exit(1)
	}

	var feFS fs.FS
	if cfg.FrontendEmbed {
		feFS, err = getFrontendFS()
		if err != nil {
			slog.Warn("frontend embed unavailable, SPA routes will 404", "err", err)
		}
	}

	handler := rest.Router(db, cfg, feFS)
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.RESTPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	slog.Info("starting REST API", "addr", srv.Addr, "frontend_embed", cfg.FrontendEmbed)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("REST server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-waitForShutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}

func waitForShutdown() <-chan struct{} {
	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(done)
	}()
	return done
}
