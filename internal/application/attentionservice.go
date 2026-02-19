package application

import (
	"context"
	"log/slog"
	"sort"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// ComputeAttentionSignals evaluates a PR against effective thresholds and returns
// which attention signals are active. approvalCount is the number of approved reviews
// from non-bot reviewers. authenticatedUserReviewSHA is the commit SHA of the
// authenticated user's most recent review on this PR (empty string if no review exists).
func ComputeAttentionSignals(
	pr model.PullRequest,
	approvalCount int,
	authenticatedUserReviewSHA string,
	thresholds model.EffectiveThresholds,
	authenticatedUser string,
) model.AttentionSignals {
	signals := model.AttentionSignals{}

	// NeedsMoreReviews: fewer than threshold approvals.
	signals.NeedsMoreReviews = approvalCount < thresholds.ReviewCountThreshold

	// IsAgeUrgent: open longer than threshold days.
	signals.IsAgeUrgent = pr.DaysSinceOpened() >= thresholds.AgeUrgencyDays

	// HasStaleReview: user has reviewed, but not on the current head commit.
	// Non-empty reviewSHA means user has reviewed; if it doesn't match head, it's stale.
	signals.HasStaleReview = thresholds.StaleReviewEnabled &&
		authenticatedUserReviewSHA != "" &&
		authenticatedUserReviewSHA != pr.HeadSHA

	// HasCIFailure: the PR is authored by the authenticated user and CI is failing.
	signals.HasCIFailure = thresholds.CIFailureEnabled &&
		pr.Author == authenticatedUser &&
		pr.CIStatus == model.CIStatusFailing

	return signals
}

// AttentionService computes attention signals for PRs using threshold configuration
// and review data from the database.
type AttentionService struct {
	thresholdStore driven.ThresholdStore
	reviewStore    driven.ReviewStore
	username       string
	logger         *slog.Logger
}

// NewAttentionService creates a new AttentionService.
func NewAttentionService(ts driven.ThresholdStore, rs driven.ReviewStore, username string) *AttentionService {
	return &AttentionService{
		thresholdStore: ts,
		reviewStore:    rs,
		username:       username,
		logger:         slog.Default(),
	}
}

// EffectiveThresholdsFor returns the resolved thresholds for a repo (global + per-repo merge).
// On error, falls back to defaults (non-fatal).
func (s *AttentionService) EffectiveThresholdsFor(ctx context.Context, repoFullName string) (model.EffectiveThresholds, error) {
	global, err := s.thresholdStore.GetGlobalSettings(ctx)
	if err != nil {
		s.logger.Warn("failed to get global settings, using defaults", "error", err)
		global = model.DefaultGlobalSettings()
	}

	repoThreshold, err := s.thresholdStore.GetRepoThreshold(ctx, repoFullName)
	if err != nil {
		s.logger.Warn("failed to get repo threshold, using global defaults", "repo", repoFullName, "error", err)
		// Fall through with nil-pointer fields — global defaults will be used.
	}

	// Merge: repo override wins if non-nil.
	effective := model.EffectiveThresholds{
		ReviewCountThreshold: global.ReviewCountThreshold,
		AgeUrgencyDays:       global.AgeUrgencyDays,
		StaleReviewEnabled:   global.StaleReviewEnabled,
		CIFailureEnabled:     global.CIFailureEnabled,
	}

	if repoThreshold.ReviewCount != nil {
		effective.ReviewCountThreshold = *repoThreshold.ReviewCount
	}
	if repoThreshold.AgeUrgencyDays != nil {
		effective.AgeUrgencyDays = *repoThreshold.AgeUrgencyDays
	}
	if repoThreshold.StaleReviewEnabled != nil {
		effective.StaleReviewEnabled = *repoThreshold.StaleReviewEnabled
	}
	if repoThreshold.CIFailureEnabled != nil {
		effective.CIFailureEnabled = *repoThreshold.CIFailureEnabled
	}

	return effective, nil
}

// SignalsForPR computes attention signals for a single PR. It fetches the required
// review data from the review store. Returns zero-value AttentionSignals on error
// (non-fatal — signals degrade gracefully).
func (s *AttentionService) SignalsForPR(ctx context.Context, pr model.PullRequest) (model.AttentionSignals, error) {
	reviews, err := s.reviewStore.GetReviewsByPR(ctx, pr.ID)
	if err != nil {
		s.logger.Warn("failed to get reviews for attention signals", "pr_id", pr.ID, "error", err)
		return model.AttentionSignals{}, nil
	}

	// Count approvals from non-bot reviewers.
	approvalCount := 0
	for _, r := range reviews {
		if r.State == model.ReviewStateApproved && !r.IsBot {
			approvalCount++
		}
	}

	// Find the authenticated user's most recent review commit SHA.
	var userReviewSHA string
	userReviews := make([]model.Review, 0)
	for _, r := range reviews {
		if r.ReviewerLogin == s.username {
			userReviews = append(userReviews, r)
		}
	}
	if len(userReviews) > 0 {
		// Sort by SubmittedAt DESC and take the first.
		sort.Slice(userReviews, func(i, j int) bool {
			return userReviews[i].SubmittedAt.After(userReviews[j].SubmittedAt)
		})
		userReviewSHA = userReviews[0].CommitID
	}

	thresholds, err := s.EffectiveThresholdsFor(ctx, pr.RepoFullName)
	if err != nil {
		s.logger.Warn("failed to get effective thresholds, using defaults", "repo", pr.RepoFullName, "error", err)
		defaults := model.DefaultGlobalSettings()
		thresholds = model.EffectiveThresholds{
			ReviewCountThreshold: defaults.ReviewCountThreshold,
			AgeUrgencyDays:       defaults.AgeUrgencyDays,
			StaleReviewEnabled:   defaults.StaleReviewEnabled,
			CIFailureEnabled:     defaults.CIFailureEnabled,
		}
	}

	return ComputeAttentionSignals(pr, approvalCount, userReviewSHA, thresholds, s.username), nil
}
