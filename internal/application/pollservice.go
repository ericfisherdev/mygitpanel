package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/efisher/reviewhub/internal/domain/model"
	"github.com/efisher/reviewhub/internal/domain/port/driven"
)

// refreshRequest represents a manual refresh trigger.
type refreshRequest struct {
	repoFullName string
	prNumber     int
	done         chan error
}

// PollService orchestrates periodic GitHub polling, PR discovery,
// and persistence.
type PollService struct {
	ghClient  driven.GitHubClient
	prStore   driven.PRStore
	repoStore driven.RepoStore
	username  string
	teamSlugs []string
	interval  time.Duration
	refreshCh chan refreshRequest
}

// NewPollService creates a new PollService with all required dependencies.
func NewPollService(
	ghClient driven.GitHubClient,
	prStore driven.PRStore,
	repoStore driven.RepoStore,
	username string,
	teamSlugs []string,
	interval time.Duration,
) *PollService {
	return &PollService{
		ghClient:  ghClient,
		prStore:   prStore,
		repoStore: repoStore,
		username:  username,
		teamSlugs: teamSlugs,
		interval:  interval,
		refreshCh: make(chan refreshRequest),
	}
}

// Start begins the polling loop. It runs an immediate poll, then polls on the
// configured interval. It also listens for manual refresh requests. Start blocks
// until the context is cancelled.
func (s *PollService) Start(ctx context.Context) {
	if err := s.pollAll(ctx); err != nil {
		slog.Error("initial poll failed", "error", err)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("poll service stopped")
			return
		case <-ticker.C:
			if err := s.pollAll(ctx); err != nil {
				slog.Error("poll cycle failed", "error", err)
			}
		case req := <-s.refreshCh:
			req.done <- s.handleRefresh(ctx, req)
		}
	}
}

// RefreshRepo triggers a manual refresh for a specific repository, bypassing
// the polling interval. It blocks until the refresh completes or the context
// is cancelled.
func (s *PollService) RefreshRepo(ctx context.Context, repoFullName string) error {
	done := make(chan error, 1)
	req := refreshRequest{
		repoFullName: repoFullName,
		done:         done,
	}

	select {
	case s.refreshCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RefreshPR triggers a manual refresh for a specific PR's repository.
// The PR number is logged for audit but the full repository is re-fetched
// since we do not have a single-PR fetch endpoint in the port interface.
func (s *PollService) RefreshPR(ctx context.Context, repoFullName string, prNumber int) error {
	slog.Info("manual PR refresh requested", "repo", repoFullName, "pr_number", prNumber)

	done := make(chan error, 1)
	req := refreshRequest{
		repoFullName: repoFullName,
		prNumber:     prNumber,
		done:         done,
	}

	select {
	case s.refreshCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pollAll polls all watched repositories for open PRs.
func (s *PollService) pollAll(ctx context.Context) error {
	start := time.Now()

	repos, err := s.repoStore.ListAll(ctx)
	if err != nil {
		return err
	}

	var pollErrors int
	for _, repo := range repos {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := s.pollRepo(ctx, repo.FullName); err != nil {
			slog.Error("repo poll failed", "repo", repo.FullName, "error", err)
			pollErrors++
		}
	}

	slog.Info("poll cycle complete",
		"repos", len(repos),
		"errors", pollErrors,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return nil
}

// pollRepo is the core PR discovery logic for a single repository.
func (s *PollService) pollRepo(ctx context.Context, repoFullName string) error {
	prs, err := s.ghClient.FetchPullRequests(ctx, repoFullName)
	if err != nil {
		return err
	}

	storedPRs, err := s.prStore.GetByRepository(ctx, repoFullName)
	if err != nil {
		return err
	}

	storedByNumber := make(map[int]model.PullRequest, len(storedPRs))
	for _, sp := range storedPRs {
		storedByNumber[sp.Number] = sp
	}

	fetchedNumbers := make(map[int]bool, len(prs))
	var relevant, skippedUnchanged int

	for _, pr := range prs {
		fetchedNumbers[pr.Number] = true

		isAuthored := strings.EqualFold(pr.Author, s.username)
		isReviewRequested := IsReviewRequestedFrom(pr, s.username, s.teamSlugs)

		if !isAuthored && !isReviewRequested {
			continue
		}

		relevant++

		if stored, ok := storedByNumber[pr.Number]; ok {
			if stored.UpdatedAt.Equal(pr.UpdatedAt) {
				skippedUnchanged++
				continue
			}
		}

		if err := s.prStore.Upsert(ctx, pr); err != nil {
			slog.Error("upsert failed", "repo", repoFullName, "pr", pr.Number, "error", err)
		}
	}

	var cleanedUp int
	for _, stored := range storedPRs {
		if !fetchedNumbers[stored.Number] && stored.Status == model.PRStatusOpen {
			if err := s.prStore.Delete(ctx, repoFullName, stored.Number); err != nil {
				slog.Error("stale cleanup failed", "repo", repoFullName, "pr", stored.Number, "error", err)
			} else {
				cleanedUp++
				slog.Info("cleaned up stale PR", "repo", repoFullName, "pr", stored.Number)
			}
		}
	}

	slog.Info("repo polled",
		"repo", repoFullName,
		"fetched", len(prs),
		"relevant", relevant,
		"skipped_unchanged", skippedUnchanged,
		"cleaned_up", cleanedUp,
	)

	return nil
}

// IsReviewRequestedFrom checks if a PR has a review request for the given user
// or any of the given team slugs.
func IsReviewRequestedFrom(pr model.PullRequest, username string, teamSlugs []string) bool {
	for _, reviewer := range pr.RequestedReviewers {
		if strings.EqualFold(reviewer, username) {
			return true
		}
	}

	for _, prTeam := range pr.RequestedTeamSlugs {
		for _, slug := range teamSlugs {
			if strings.EqualFold(prTeam, slug) {
				return true
			}
		}
	}

	return false
}

// handleRefresh dispatches a manual refresh request.
func (s *PollService) handleRefresh(ctx context.Context, req refreshRequest) error {
	if req.repoFullName != "" {
		return s.pollRepo(ctx, req.repoFullName)
	}
	return s.pollAll(ctx)
}
