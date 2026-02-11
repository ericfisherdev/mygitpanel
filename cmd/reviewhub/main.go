package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	githubadapter "github.com/efisher/reviewhub/internal/adapter/driven/github"
	sqliteadapter "github.com/efisher/reviewhub/internal/adapter/driven/sqlite"
	"github.com/efisher/reviewhub/internal/application"
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

	// 5. Wire adapters.
	prStore := sqliteadapter.NewPRRepo(db)
	repoStore := sqliteadapter.NewRepoRepo(db)

	// 6. Create GitHub client.
	ghClient := githubadapter.NewClient(cfg.GitHubToken, cfg.GitHubUsername)

	// 7. Create and start poll service.
	pollSvc := application.NewPollService(
		ghClient,
		prStore,
		repoStore,
		cfg.GitHubUsername,
		cfg.GitHubTeams,
		cfg.PollInterval,
	)
	go pollSvc.Start(ctx)

	// 8. Log startup complete.
	slog.Info("reviewhub started",
		"listen_addr", cfg.ListenAddr,
		"poll_interval", cfg.PollInterval,
		"teams", cfg.GitHubTeams,
	)

	// 9. Wait for shutdown signal.
	<-ctx.Done()
	slog.Info("shutting down")

	// 10. Graceful shutdown with 10s timeout for future HTTP server drain.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Future phases will use shutdownCtx to drain the HTTP server.
	_ = shutdownCtx

	// 11. Log shutdown complete.
	slog.Info("shutdown complete")
	return nil
}
