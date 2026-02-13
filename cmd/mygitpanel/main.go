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

	githubadapter "github.com/ericfisherdev/mygitpanel/internal/adapter/driven/github"
	sqliteadapter "github.com/ericfisherdev/mygitpanel/internal/adapter/driven/sqlite"
	httphandler "github.com/ericfisherdev/mygitpanel/internal/adapter/driving/http"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/config"
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
	db, err := sqliteadapter.NewDB(ctx, cfg.DBPath)
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
	reviewStore := sqliteadapter.NewReviewRepo(db)
	checkStore := sqliteadapter.NewCheckRepo(db)
	botConfigStore := sqliteadapter.NewBotConfigRepo(db)

	// 6. Create GitHub client.
	ghClient := githubadapter.NewClient(cfg.GitHubToken, cfg.GitHubUsername)

	// 7. Create and start poll service.
	pollSvc := application.NewPollService(
		ghClient,
		prStore,
		repoStore,
		reviewStore,
		checkStore,
		cfg.GitHubUsername,
		cfg.GitHubTeams,
		cfg.PollInterval,
	)
	go pollSvc.Start(ctx)

	// 7b. Create review service.
	reviewSvc := application.NewReviewService(reviewStore, botConfigStore)

	// 7.5. Create HTTP handler and server.
	h := httphandler.NewHandler(prStore, repoStore, botConfigStore, reviewSvc, nil, pollSvc, cfg.GitHubUsername, slog.Default())
	mux := httphandler.NewServeMux(h, slog.Default())

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("http server starting", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
		}
	}()

	// 8. Log startup complete.
	slog.Info("mygitpanel started",
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

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	// 11. Log shutdown complete.
	slog.Info("shutdown complete")
	return nil
}
