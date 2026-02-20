package application

import (
	"context"
	"log/slog"

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

	// IsAgeUrgent: open longer than threshold days. A threshold of 0 means disabled.
	signals.IsAgeUrgent = thresholds.AgeUrgencyDays > 0 && pr.DaysSinceOpened() >= thresholds.AgeUrgencyDays

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
// Errors from the store are logged and fall back to defaults (non-fatal).
func (s *AttentionService) EffectiveThresholdsFor(ctx context.Context, repoFullName string) model.EffectiveThresholds {
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
	effective := model.EffectiveThresholds(global)

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

	return effective
}

// SignalsForPR computes attention signals for a single PR using pre-resolved thresholds.
// Callers should fetch thresholds once per unique repo via EffectiveThresholdsFor to avoid
// per-PR DB lookups. Returns zero-value AttentionSignals on review store error (non-fatal).
func (s *AttentionService) SignalsForPR(ctx context.Context, pr model.PullRequest, thresholds model.EffectiveThresholds) (model.AttentionSignals, error) {
	reviews, err := s.reviewStore.GetReviewsByPR(ctx, pr.ID)
	if err != nil {
		s.logger.Warn("failed to get reviews for attention signals", "pr_id", pr.ID, "error", err)
		return model.AttentionSignals{}, nil
	}

	// Collapse to each reviewer's latest review to avoid double-counting when
	// the same person has reviewed multiple times (e.g., approve → request changes → approve).
	latestByReviewer := make(map[string]model.Review, len(reviews))
	for _, r := range reviews {
		existing, seen := latestByReviewer[r.ReviewerLogin]
		if !seen || r.SubmittedAt.After(existing.SubmittedAt) {
			latestByReviewer[r.ReviewerLogin] = r
		}
	}

	// Count approvals and locate the authenticated user's review SHA in one pass.
	approvalCount := 0
	var userReviewSHA string
	for login, r := range latestByReviewer {
		if r.State == model.ReviewStateApproved && !r.IsBot {
			approvalCount++
		}
		if login == s.username {
			userReviewSHA = r.CommitID
		}
	}

	return ComputeAttentionSignals(pr, approvalCount, userReviewSHA, thresholds, s.username), nil
}
