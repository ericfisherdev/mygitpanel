package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// attentionReviewStore implements driven.ReviewStore for AttentionService tests.
// Only GetReviewsByPR is used by AttentionService; remaining methods panic.
type attentionReviewStore struct {
	reviews []model.Review
	err     error
}

func (m *attentionReviewStore) GetReviewsByPR(_ context.Context, _ int64) ([]model.Review, error) {
	return m.reviews, m.err
}

func (m *attentionReviewStore) UpsertReview(_ context.Context, _ model.Review) error { panic("unused") }
func (m *attentionReviewStore) UpsertReviewComment(_ context.Context, _ model.ReviewComment) error {
	panic("unused")
}
func (m *attentionReviewStore) UpsertIssueComment(_ context.Context, _ model.IssueComment) error {
	panic("unused")
}
func (m *attentionReviewStore) GetReviewCommentsByPR(_ context.Context, _ int64) ([]model.ReviewComment, error) {
	panic("unused")
}
func (m *attentionReviewStore) GetIssueCommentsByPR(_ context.Context, _ int64) ([]model.IssueComment, error) {
	panic("unused")
}
func (m *attentionReviewStore) UpdateCommentResolution(_ context.Context, _ int64, _ bool) error {
	panic("unused")
}
func (m *attentionReviewStore) DeleteReviewsByPR(_ context.Context, _ int64) error { panic("unused") }

// attentionThresholdStore implements driven.ThresholdStore for AttentionService tests.
// Only GetGlobalSettings and GetRepoThreshold are used by AttentionService.
type attentionThresholdStore struct {
	global    model.GlobalSettings
	globalErr error
	repo      model.RepoThreshold
	repoErr   error
}

func (m *attentionThresholdStore) GetGlobalSettings(_ context.Context) (model.GlobalSettings, error) {
	return m.global, m.globalErr
}

func (m *attentionThresholdStore) GetRepoThreshold(_ context.Context, _ string) (model.RepoThreshold, error) {
	return m.repo, m.repoErr
}

func (m *attentionThresholdStore) SetGlobalSettings(_ context.Context, _ model.GlobalSettings) error {
	panic("unused")
}
func (m *attentionThresholdStore) SetRepoThreshold(_ context.Context, _ model.RepoThreshold) error {
	panic("unused")
}
func (m *attentionThresholdStore) DeleteRepoThreshold(_ context.Context, _ string) error {
	panic("unused")
}

// testAuthor is the GitHub username used as the authenticated user in test cases.
const testAuthor = "alice"

// prWithAge returns a PullRequest opened the given number of days ago.
func prWithAge(days int) model.PullRequest {
	return model.PullRequest{
		OpenedAt: time.Now().Add(-time.Duration(days) * 24 * time.Hour),
		Status:   model.PRStatusOpen,
	}
}

func defaultThresholds() model.EffectiveThresholds {
	return model.EffectiveThresholds{
		ReviewCountThreshold: 1,
		AgeUrgencyDays:       7,
		StaleReviewEnabled:   true,
		CIFailureEnabled:     true,
	}
}

func TestComputeAttentionSignals_NeedsMoreReviews(t *testing.T) {
	thresholds := defaultThresholds()
	pr := prWithAge(0)
	pr.HeadSHA = "abc123"

	t.Run("0 approvals with threshold 1 -> NeedsMoreReviews true", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 0, "", thresholds, testAuthor)
		assert.True(t, signals.NeedsMoreReviews)
	})

	t.Run("1 approval with threshold 1 -> NeedsMoreReviews false", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.False(t, signals.NeedsMoreReviews)
	})

	t.Run("2 approvals with threshold 2 -> NeedsMoreReviews false", func(t *testing.T) {
		thresholds2 := thresholds
		thresholds2.ReviewCountThreshold = 2
		signals := application.ComputeAttentionSignals(pr, 2, "", thresholds2, testAuthor)
		assert.False(t, signals.NeedsMoreReviews)
	})

	t.Run("1 approval with threshold 2 -> NeedsMoreReviews true", func(t *testing.T) {
		thresholds2 := thresholds
		thresholds2.ReviewCountThreshold = 2
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds2, testAuthor)
		assert.True(t, signals.NeedsMoreReviews)
	})
}

func TestComputeAttentionSignals_IsAgeUrgent(t *testing.T) {
	thresholds := defaultThresholds() // AgeUrgencyDays = 7

	t.Run("8 days open with threshold 7 -> IsAgeUrgent true", func(t *testing.T) {
		pr := prWithAge(8)
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.True(t, signals.IsAgeUrgent)
	})

	t.Run("6 days open with threshold 7 -> IsAgeUrgent false", func(t *testing.T) {
		pr := prWithAge(6)
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.False(t, signals.IsAgeUrgent)
	})

	t.Run("exactly 7 days open with threshold 7 -> IsAgeUrgent true (>= comparison)", func(t *testing.T) {
		pr := prWithAge(7)
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.True(t, signals.IsAgeUrgent)
	})

	t.Run("threshold 0 -> IsAgeUrgent false (disabled)", func(t *testing.T) {
		disabled := thresholds
		disabled.AgeUrgencyDays = 0
		pr := prWithAge(100)
		signals := application.ComputeAttentionSignals(pr, 1, "", disabled, testAuthor)
		assert.False(t, signals.IsAgeUrgent)
	})
}

func TestComputeAttentionSignals_HasStaleReview(t *testing.T) {
	thresholds := defaultThresholds() // StaleReviewEnabled = true
	pr := prWithAge(0)
	pr.HeadSHA = "newsha123"

	t.Run("different SHAs -> HasStaleReview true", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "oldsha456", thresholds, testAuthor)
		assert.True(t, signals.HasStaleReview)
	})

	t.Run("same SHA -> HasStaleReview false", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "newsha123", thresholds, testAuthor)
		assert.False(t, signals.HasStaleReview)
	})

	t.Run("empty reviewSHA (no review yet) -> HasStaleReview false", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.False(t, signals.HasStaleReview)
	})

	t.Run("disabled in thresholds -> HasStaleReview false even with different SHAs", func(t *testing.T) {
		disabled := thresholds
		disabled.StaleReviewEnabled = false
		signals := application.ComputeAttentionSignals(pr, 1, "oldsha456", disabled, testAuthor)
		assert.False(t, signals.HasStaleReview)
	})
}

func TestComputeAttentionSignals_HasCIFailure(t *testing.T) {
	thresholds := defaultThresholds() // CIFailureEnabled = true

	t.Run("own PR with failing CI -> HasCIFailure true", func(t *testing.T) {
		pr := prWithAge(0)
		pr.Author = testAuthor
		pr.CIStatus = model.CIStatusFailing
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.True(t, signals.HasCIFailure)
	})

	t.Run("other's PR with failing CI -> HasCIFailure false", func(t *testing.T) {
		pr := prWithAge(0)
		pr.Author = "bob"
		pr.CIStatus = model.CIStatusFailing
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.False(t, signals.HasCIFailure)
	})

	t.Run("own PR with passing CI -> HasCIFailure false", func(t *testing.T) {
		pr := prWithAge(0)
		pr.Author = testAuthor
		pr.CIStatus = model.CIStatusPassing
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, testAuthor)
		assert.False(t, signals.HasCIFailure)
	})

	t.Run("disabled in thresholds -> HasCIFailure false even for own failing PR", func(t *testing.T) {
		disabled := thresholds
		disabled.CIFailureEnabled = false
		pr := prWithAge(0)
		pr.Author = testAuthor
		pr.CIStatus = model.CIStatusFailing
		signals := application.ComputeAttentionSignals(pr, 1, "", disabled, testAuthor)
		assert.False(t, signals.HasCIFailure)
	})
}

func TestAttentionSignals_HasAny(t *testing.T) {
	t.Run("no signals -> HasAny false", func(t *testing.T) {
		signals := model.AttentionSignals{}
		assert.False(t, signals.HasAny())
	})

	t.Run("one signal -> HasAny true", func(t *testing.T) {
		signals := model.AttentionSignals{NeedsMoreReviews: true}
		assert.True(t, signals.HasAny())
	})
}

func TestAttentionSignals_Severity(t *testing.T) {
	t.Run("no signals -> severity 0", func(t *testing.T) {
		signals := model.AttentionSignals{}
		assert.Equal(t, 0, signals.Severity())
	})

	t.Run("one signal -> severity 1", func(t *testing.T) {
		signals := model.AttentionSignals{NeedsMoreReviews: true}
		assert.Equal(t, 1, signals.Severity())
	})

	t.Run("two signals -> severity 2", func(t *testing.T) {
		signals := model.AttentionSignals{NeedsMoreReviews: true, IsAgeUrgent: true}
		assert.Equal(t, 2, signals.Severity())
	})

	t.Run("four signals -> severity 4", func(t *testing.T) {
		signals := model.AttentionSignals{
			NeedsMoreReviews: true,
			IsAgeUrgent:      true,
			HasStaleReview:   true,
			HasCIFailure:     true,
		}
		assert.Equal(t, 4, signals.Severity())
	})
}

func TestSignalsForPR_ReviewerDeduplication(t *testing.T) {
	now := time.Now()
	pr := model.PullRequest{ID: 1, HeadSHA: "sha1", Status: model.PRStatusOpen, OpenedAt: now}
	thresholds := defaultThresholds() // ReviewCountThreshold = 1

	t.Run("latest approval wins over earlier request-changes from same reviewer", func(t *testing.T) {
		reviews := []model.Review{
			{ReviewerLogin: "bob", State: model.ReviewStateChangesRequested, SubmittedAt: now.Add(-2 * time.Hour), CommitID: "sha1"},
			{ReviewerLogin: "bob", State: model.ReviewStateApproved, SubmittedAt: now.Add(-1 * time.Hour), CommitID: "sha1"},
		}
		svc := application.NewAttentionService(
			&attentionThresholdStore{global: model.DefaultGlobalSettings()},
			&attentionReviewStore{reviews: reviews},
			testAuthor,
		)
		signals, err := svc.SignalsForPR(context.Background(), pr, thresholds)
		require.NoError(t, err)
		assert.False(t, signals.NeedsMoreReviews, "one approval from bob should satisfy threshold of 1")
	})

	t.Run("latest request-changes overrides earlier approval from same reviewer", func(t *testing.T) {
		reviews := []model.Review{
			{ReviewerLogin: "bob", State: model.ReviewStateApproved, SubmittedAt: now.Add(-2 * time.Hour), CommitID: "sha1"},
			{ReviewerLogin: "bob", State: model.ReviewStateChangesRequested, SubmittedAt: now.Add(-1 * time.Hour), CommitID: "sha1"},
		}
		svc := application.NewAttentionService(
			&attentionThresholdStore{global: model.DefaultGlobalSettings()},
			&attentionReviewStore{reviews: reviews},
			testAuthor,
		)
		signals, err := svc.SignalsForPR(context.Background(), pr, thresholds)
		require.NoError(t, err)
		assert.True(t, signals.NeedsMoreReviews, "request-changes should nullify the earlier approval")
	})

	t.Run("bots are excluded from approval count", func(t *testing.T) {
		reviews := []model.Review{
			{ReviewerLogin: "dependabot", State: model.ReviewStateApproved, SubmittedAt: now, CommitID: "sha1", IsBot: true},
		}
		svc := application.NewAttentionService(
			&attentionThresholdStore{global: model.DefaultGlobalSettings()},
			&attentionReviewStore{reviews: reviews},
			testAuthor,
		)
		signals, err := svc.SignalsForPR(context.Background(), pr, thresholds)
		require.NoError(t, err)
		assert.True(t, signals.NeedsMoreReviews, "bot approvals should not count toward threshold")
	})
}

func TestSignalsForPR_StoreError(t *testing.T) {
	pr := model.PullRequest{ID: 1, HeadSHA: "sha1", Status: model.PRStatusOpen, OpenedAt: time.Now()}
	thresholds := defaultThresholds()
	storeErr := errors.New("db unavailable")

	svc := application.NewAttentionService(
		&attentionThresholdStore{global: model.DefaultGlobalSettings()},
		&attentionReviewStore{err: storeErr},
		testAuthor,
	)
	signals, err := svc.SignalsForPR(context.Background(), pr, thresholds)
	assert.NoError(t, err, "review store errors should be swallowed (non-fatal)")
	assert.Equal(t, model.AttentionSignals{}, signals, "zero-value signals returned on store error")
}

func TestEffectiveThresholdsFor_RepoOverridePrecedence(t *testing.T) {
	repoCount := 3
	repoAge := 14
	repoStale := false
	repoCI := false

	ts := &attentionThresholdStore{
		global: model.GlobalSettings{ReviewCountThreshold: 1, AgeUrgencyDays: 7, StaleReviewEnabled: true, CIFailureEnabled: true},
		repo: model.RepoThreshold{
			ReviewCount:        &repoCount,
			AgeUrgencyDays:     &repoAge,
			StaleReviewEnabled: &repoStale,
			CIFailureEnabled:   &repoCI,
		},
	}
	svc := application.NewAttentionService(ts, &attentionReviewStore{}, testAuthor)
	effective := svc.EffectiveThresholdsFor(context.Background(), "owner/repo")

	assert.Equal(t, 3, effective.ReviewCountThreshold, "repo override should win over global")
	assert.Equal(t, 14, effective.AgeUrgencyDays, "repo override should win over global")
	assert.False(t, effective.StaleReviewEnabled, "repo override should win over global")
	assert.False(t, effective.CIFailureEnabled, "repo override should win over global")
}

func TestEffectiveThresholdsFor_GlobalFallbackOnStoreError(t *testing.T) {
	storeErr := errors.New("db unavailable")
	ts := &attentionThresholdStore{
		globalErr: storeErr,
		repoErr:   storeErr,
	}
	svc := application.NewAttentionService(ts, &attentionReviewStore{}, testAuthor)
	effective := svc.EffectiveThresholdsFor(context.Background(), "owner/repo")

	defaults := model.DefaultGlobalSettings()
	assert.Equal(t, defaults.ReviewCountThreshold, effective.ReviewCountThreshold)
	assert.Equal(t, defaults.AgeUrgencyDays, effective.AgeUrgencyDays)
	assert.Equal(t, defaults.StaleReviewEnabled, effective.StaleReviewEnabled)
	assert.Equal(t, defaults.CIFailureEnabled, effective.CIFailureEnabled)
}
