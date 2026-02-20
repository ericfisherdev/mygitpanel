// Package application contains use-case orchestration services.
package application

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
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
	ghClient      driven.GitHubClient
	prStore       driven.PRStore
	repoStore     driven.RepoStore
	reviewStore   driven.ReviewStore
	checkStore    driven.CheckStore
	username      string
	teamSlugs     []string
	interval      time.Duration
	refreshCh     chan refreshRequest
	tokenProvider func(ctx context.Context) (string, error) // optional; re-reads token each cycle
	clientFactory func(token string) driven.GitHubClient    // optional; creates a new GitHub client with the given token

	// branchProtectionCache caches required status check contexts per
	// "repo/branch" key during a poll cycle. Branch protection rarely changes,
	// so multiple PRs targeting the same base branch reuse a single API call.
	// Cleared at the start of each poll cycle.
	branchProtectionCache map[string][]string

	// schedulesMu protects the schedules map from concurrent access.
	// The Start goroutine writes schedules; Schedules() may read from another goroutine.
	schedulesMu sync.RWMutex
	// schedules holds per-repository adaptive polling state. Each repo is
	// classified into an activity tier that determines its polling frequency.
	schedules map[string]repoSchedule
}

// NewPollService creates a new PollService with all required dependencies.
// tokenProvider and clientFactory are both optional (may be nil). When both
// are provided, tokenProvider is called at the start of each poll cycle to
// obtain the current token; if the token is non-empty, clientFactory creates
// a new GitHubClient using that token, hot-swapping the GitHub client each cycle.
// The startup ghClient (created from the env var token) is used as a fallback
// when tokenProvider returns an empty string or an error.
func NewPollService(
	ghClient driven.GitHubClient,
	prStore driven.PRStore,
	repoStore driven.RepoStore,
	reviewStore driven.ReviewStore,
	checkStore driven.CheckStore,
	username string,
	teamSlugs []string,
	interval time.Duration,
	tokenProvider func(ctx context.Context) (string, error), // may be nil
	clientFactory func(token string) driven.GitHubClient, // may be nil
) *PollService {
	return &PollService{
		ghClient:      ghClient,
		prStore:       prStore,
		repoStore:     repoStore,
		reviewStore:   reviewStore,
		checkStore:    checkStore,
		username:      username,
		teamSlugs:     teamSlugs,
		interval:      interval,
		refreshCh:     make(chan refreshRequest),
		schedules:     make(map[string]repoSchedule),
		tokenProvider: tokenProvider,
		clientFactory: clientFactory,
	}
}

// Start begins the polling loop. It runs an immediate full poll to initialize
// schedules, then uses a 1-minute resolution ticker with per-repo adaptive
// scheduling. It also listens for manual refresh requests. Start blocks until
// the context is canceled.
func (s *PollService) Start(ctx context.Context) {
	// Initial poll fetches all repos and initializes adaptive schedules.
	if err := s.pollAll(ctx); err != nil {
		slog.Error("initial poll failed", "error", err)
	}

	// Initialize schedules for all repos after the initial full poll.
	s.initializeSchedules(ctx)

	// Use 1-minute resolution ticker. Per-repo adaptive schedules determine
	// which repos actually get polled on each tick.
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("poll service stopped")
			return
		case <-ticker.C:
			s.pollDueRepos(ctx)
		case req := <-s.refreshCh:
			req.done <- s.handleRefresh(ctx, req)
		}
	}
}

// Schedules returns a snapshot of the current adaptive polling schedules
// for all tracked repos. Used for observability and testing.
func (s *PollService) Schedules() map[string]ScheduleInfo {
	s.schedulesMu.RLock()
	defer s.schedulesMu.RUnlock()

	result := make(map[string]ScheduleInfo, len(s.schedules))
	for repo, sched := range s.schedules {
		result[repo] = ScheduleInfo{
			Tier:       sched.tier,
			NextPollAt: sched.nextPollAt,
			LastPolled: sched.lastPolled,
		}
	}
	return result
}

// RefreshRepo triggers a manual refresh for a specific repository, bypassing
// the polling interval. It blocks until the refresh completes or the context
// is canceled.
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

// maybeRefreshToken re-reads the GitHub token from the credential store and
// hot-swaps the GitHub client if a new non-empty token is found. The startup
// client is retained if tokenProvider is nil, returns an error, or returns
// an empty string.
func (s *PollService) maybeRefreshToken(ctx context.Context) {
	if s.tokenProvider == nil || s.clientFactory == nil {
		return
	}
	token, err := s.tokenProvider(ctx)
	if err != nil {
		slog.Debug("token provider error; retaining startup client", "error", err)
		return
	}
	if token == "" {
		return
	}
	s.ghClient = s.clientFactory(token)
	slog.Debug("github client hot-swapped with token from credential store")
}

// pollAll polls all watched repositories for open PRs.
func (s *PollService) pollAll(ctx context.Context) error {
	start := time.Now()

	// Re-read token from credential store each cycle; env var token is the fallback.
	s.maybeRefreshToken(ctx)

	// Reset per-cycle branch protection cache.
	s.branchProtectionCache = make(map[string][]string)

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
		} else {
			s.updateSchedule(ctx, repo.FullName)
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
// It fetches all PRs (open, closed, merged) and stores them unconditionally.
// NeedsReview is still computed to flag PRs where the user is a requested reviewer.
func (s *PollService) pollRepo(ctx context.Context, repoFullName string) error {
	prs, err := s.ghClient.FetchPullRequests(ctx, repoFullName, "all")
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
	var skippedUnchanged int

	for _, pr := range prs {
		fetchedNumbers[pr.Number] = true

		pr.NeedsReview = IsReviewRequestedFrom(pr, s.username, s.teamSlugs)

		if stored, ok := storedByNumber[pr.Number]; ok {
			if stored.UpdatedAt.Equal(pr.UpdatedAt) && stored.NeedsReview == pr.NeedsReview {
				skippedUnchanged++
				continue
			}
		}

		if err := s.prStore.Upsert(ctx, pr); err != nil {
			slog.Error("upsert failed", "repo", repoFullName, "pr", pr.Number, "error", err)
			continue
		}

		// Fetch review and health data for changed PRs. We need the stored PR's ID
		// (auto-increment) for foreign key references in review/check tables.
		storedPR, err := s.prStore.GetByNumber(ctx, pr.RepoFullName, pr.Number)
		if err != nil || storedPR == nil {
			slog.Error("failed to retrieve PR for review fetch", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
		} else {
			s.fetchReviewData(ctx, *storedPR)
			s.fetchHealthData(ctx, *storedPR)
		}
	}

	// Clean up stored open PRs that no longer appear in the API response.
	// Closed/merged PRs are terminal states and should not be deleted even if
	// absent from the fetch (they may be beyond the API's pagination window).
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

// fetchReviewData fetches reviews, review comments, issue comments, and thread
// resolution for a PR and stores them via ReviewStore. Each fetch step is
// independent -- partial failures are logged but do not abort the overall operation.
func (s *PollService) fetchReviewData(ctx context.Context, pr model.PullRequest) {
	reviews, err := s.ghClient.FetchReviews(ctx, pr.RepoFullName, pr.Number)
	if err != nil {
		slog.Error("fetch reviews failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
	} else {
		for _, review := range reviews {
			review.PRID = pr.ID
			if err := s.reviewStore.UpsertReview(ctx, review); err != nil {
				slog.Error("upsert review failed", "repo", pr.RepoFullName, "pr", pr.Number, "review", review.ID, "error", err)
			}
		}
	}

	comments, err := s.ghClient.FetchReviewComments(ctx, pr.RepoFullName, pr.Number)
	if err != nil {
		slog.Error("fetch review comments failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
	} else {
		for _, comment := range comments {
			comment.PRID = pr.ID
			if err := s.reviewStore.UpsertReviewComment(ctx, comment); err != nil {
				slog.Error("upsert review comment failed", "repo", pr.RepoFullName, "pr", pr.Number, "comment", comment.ID, "error", err)
			}
		}
	}

	issueComments, err := s.ghClient.FetchIssueComments(ctx, pr.RepoFullName, pr.Number)
	if err != nil {
		slog.Error("fetch issue comments failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
	} else {
		for _, ic := range issueComments {
			ic.PRID = pr.ID
			if err := s.reviewStore.UpsertIssueComment(ctx, ic); err != nil {
				slog.Error("upsert issue comment failed", "repo", pr.RepoFullName, "pr", pr.Number, "comment", ic.ID, "error", err)
			}
		}
	}

	resolutionMap, err := s.ghClient.FetchThreadResolution(ctx, pr.RepoFullName, pr.Number)
	if err != nil {
		slog.Error("fetch thread resolution failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
	} else {
		for commentID, isResolved := range resolutionMap {
			if err := s.reviewStore.UpdateCommentResolution(ctx, commentID, isResolved); err != nil {
				slog.Error("update comment resolution failed", "repo", pr.RepoFullName, "pr", pr.Number, "comment", commentID, "error", err)
			}
		}
	}

	slog.Debug("review data fetched",
		"repo", pr.RepoFullName,
		"pr", pr.Number,
		"reviews", len(reviews),
		"review_comments", len(comments),
		"issue_comments", len(issueComments),
	)
}

// fetchHealthData fetches check runs, combined status, PR detail, and required
// status checks for a PR and persists them. Each fetch step is independent --
// partial failures are logged but do not abort the overall operation.
func (s *PollService) fetchHealthData(ctx context.Context, pr model.PullRequest) {
	// Step 1: Fetch PR detail (diff stats + mergeable status).
	detail, err := s.ghClient.FetchPRDetail(ctx, pr.RepoFullName, pr.Number)
	if err != nil {
		slog.Error("fetch PR detail failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
	} else if detail != nil {
		pr.Additions = detail.Additions
		pr.Deletions = detail.Deletions
		pr.ChangedFiles = detail.ChangedFiles
		pr.MergeableStatus = detail.Mergeable
		if err := s.prStore.Upsert(ctx, pr); err != nil {
			slog.Error("upsert PR detail failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
		}
	}

	// Step 2: Fetch check runs.
	checkRuns, err := s.ghClient.FetchCheckRuns(ctx, pr.RepoFullName, pr.HeadSHA)
	if err != nil {
		slog.Error("fetch check runs failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
		return // Skip remaining check processing without check runs.
	}

	// Step 3: Fetch combined status (may fail independently).
	var combinedStatus *model.CombinedStatus
	combinedStatus, err = s.ghClient.FetchCombinedStatus(ctx, pr.RepoFullName, pr.HeadSHA)
	if err != nil {
		slog.Error("fetch combined status failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
		// Continue with nil combined status.
	}

	// Step 4: Fetch required status checks from branch protection (cached per branch per cycle).
	cacheKey := pr.RepoFullName + "/" + pr.BaseBranch
	requiredContexts, cached := s.branchProtectionCache[cacheKey]
	if !cached {
		requiredContexts, err = s.ghClient.FetchRequiredStatusChecks(ctx, pr.RepoFullName, pr.BaseBranch)
		if err != nil {
			slog.Error("fetch required status checks failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
			// Continue with nil requiredContexts -- all checks default to not required.
		}
		// Cache even nil results to avoid repeated 404/403 calls for the same branch.
		s.branchProtectionCache[cacheKey] = requiredContexts
	}

	// Step 5: Mark required checks.
	markRequiredChecks(checkRuns, requiredContexts)

	// Step 6: Set PRID on all check runs.
	for i := range checkRuns {
		checkRuns[i].PRID = pr.ID
	}

	// Step 7: Persist check runs (full replacement).
	if err := s.checkStore.ReplaceCheckRunsForPR(ctx, pr.ID, checkRuns); err != nil {
		slog.Error("replace check runs failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
	}

	// Step 8: Compute and persist combined CI status.
	ciStatus := computeCombinedCIStatus(checkRuns, combinedStatus)
	pr.CIStatus = ciStatus
	if err := s.prStore.Upsert(ctx, pr); err != nil {
		slog.Error("upsert CI status failed", "repo", pr.RepoFullName, "pr", pr.Number, "error", err)
	}

	slog.Debug("health data fetched",
		"repo", pr.RepoFullName,
		"pr", pr.Number,
		"check_runs", len(checkRuns),
		"ci_status", string(ciStatus),
		"mergeable_status", string(pr.MergeableStatus),
	)
}

// initializeSchedules sets up adaptive schedules for all repos after the
// initial full poll. This ensures every repo has a tier assignment before
// the adaptive ticker starts.
func (s *PollService) initializeSchedules(ctx context.Context) {
	repos, err := s.repoStore.ListAll(ctx)
	if err != nil {
		slog.Error("failed to list repos for schedule init", "error", err)
		return
	}

	for _, repo := range repos {
		s.updateSchedule(ctx, repo.FullName)
	}
}

// updateSchedule recalculates the activity tier and next poll time for a repo
// based on its freshest PR activity.
func (s *PollService) updateSchedule(ctx context.Context, repoFullName string) {
	prs, err := s.prStore.GetByRepository(ctx, repoFullName)
	if err != nil {
		slog.Error("failed to get PRs for schedule update", "repo", repoFullName, "error", err)
		return
	}

	latest := freshestActivity(prs)
	tier := classifyActivity(latest)
	nextPoll := time.Now().Add(tierInterval(tier))

	s.schedulesMu.Lock()
	s.schedules[repoFullName] = repoSchedule{
		tier:       tier,
		nextPollAt: nextPoll,
		lastPolled: time.Now(),
	}
	s.schedulesMu.Unlock()

	slog.Info("repo tier updated",
		"repo", repoFullName,
		"tier", tier.String(),
		"next_poll", nextPoll.Format(time.RFC3339),
	)
}

// pollDueRepos checks each repo's adaptive schedule and polls only those
// that are due. New repos without a schedule are polled immediately.
func (s *PollService) pollDueRepos(ctx context.Context) {
	// Re-read token from credential store each cycle; env var token is the fallback.
	s.maybeRefreshToken(ctx)

	// Reset per-cycle branch protection cache.
	s.branchProtectionCache = make(map[string][]string)

	repos, err := s.repoStore.ListAll(ctx)
	if err != nil {
		slog.Error("failed to list repos for adaptive poll", "error", err)
		return
	}

	var polled int
	for _, repo := range repos {
		if ctx.Err() != nil {
			return
		}

		s.schedulesMu.RLock()
		schedule, exists := s.schedules[repo.FullName]
		s.schedulesMu.RUnlock()

		if exists && time.Now().Before(schedule.nextPollAt) {
			continue // Not due yet.
		}

		if err := s.pollRepo(ctx, repo.FullName); err != nil {
			slog.Error("adaptive repo poll failed", "repo", repo.FullName, "error", err)
		} else {
			s.updateSchedule(ctx, repo.FullName)
		}
		polled++
	}

	slog.Info("adaptive poll cycle",
		"repos_checked", len(repos),
		"repos_polled", polled,
	)
}

// handleRefresh dispatches a manual refresh request. After polling, the repo's
// adaptive schedule is recalculated based on fresh activity data.
func (s *PollService) handleRefresh(ctx context.Context, req refreshRequest) error {
	if req.repoFullName != "" {
		err := s.pollRepo(ctx, req.repoFullName)
		if err == nil {
			s.updateSchedule(ctx, req.repoFullName)
		}
		return err
	}
	return s.pollAll(ctx)
}
