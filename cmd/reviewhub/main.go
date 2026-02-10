package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	sqliteadapter "github.com/efisher/reviewhub/internal/adapter/driven/sqlite"
	"github.com/efisher/reviewhub/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load configuration (fail fast on missing required env vars).
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.Info("config loaded",
		"listen_addr", cfg.ListenAddr,
		"db_path", cfg.DBPath,
		"poll_interval", cfg.PollInterval,
		"github_username", cfg.GitHubUsername,
	)

	// 2. Setup signal-based context (SIGINT, SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3. Open database (dual reader/writer with WAL mode).
	db, err := sqliteadapter.NewDB(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("error closing database", "error", closeErr)
		}
	}()
	slog.Info("database opened", "path", cfg.DBPath)

	// 4. Run migrations on writer connection.
	if err := sqliteadapter.RunMigrations(db.Writer); err != nil {
		return err
	}
	slog.Info("migrations complete")

	// 5. Wire adapters (used in Phase 2+ when services are added).
	prStore := sqliteadapter.NewPRRepo(db)
	repoStore := sqliteadapter.NewRepoRepo(db)
	_ = prStore
	_ = repoStore

	// 6. Log startup complete.
	slog.Info("reviewhub started", "listen_addr", cfg.ListenAddr)

	// 7. Wait for shutdown signal.
	<-ctx.Done()
	slog.Info("shutting down")

	// 8. Graceful shutdown with 10s timeout for future HTTP server drain.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Future phases will use shutdownCtx to drain the HTTP server.
	_ = shutdownCtx

	// 9. Log shutdown complete.
	slog.Info("shutdown complete")
	return nil
}
