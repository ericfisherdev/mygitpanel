package application_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

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
		signals := application.ComputeAttentionSignals(pr, 0, "", thresholds, "alice")
		assert.True(t, signals.NeedsMoreReviews)
	})

	t.Run("1 approval with threshold 1 -> NeedsMoreReviews false", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.False(t, signals.NeedsMoreReviews)
	})

	t.Run("2 approvals with threshold 2 -> NeedsMoreReviews false", func(t *testing.T) {
		thresholds2 := thresholds
		thresholds2.ReviewCountThreshold = 2
		signals := application.ComputeAttentionSignals(pr, 2, "", thresholds2, "alice")
		assert.False(t, signals.NeedsMoreReviews)
	})

	t.Run("1 approval with threshold 2 -> NeedsMoreReviews true", func(t *testing.T) {
		thresholds2 := thresholds
		thresholds2.ReviewCountThreshold = 2
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds2, "alice")
		assert.True(t, signals.NeedsMoreReviews)
	})
}

func TestComputeAttentionSignals_IsAgeUrgent(t *testing.T) {
	thresholds := defaultThresholds() // AgeUrgencyDays = 7

	t.Run("8 days open with threshold 7 -> IsAgeUrgent true", func(t *testing.T) {
		pr := prWithAge(8)
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.True(t, signals.IsAgeUrgent)
	})

	t.Run("6 days open with threshold 7 -> IsAgeUrgent false", func(t *testing.T) {
		pr := prWithAge(6)
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.False(t, signals.IsAgeUrgent)
	})

	t.Run("exactly 7 days open with threshold 7 -> IsAgeUrgent true (>= comparison)", func(t *testing.T) {
		pr := prWithAge(7)
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.True(t, signals.IsAgeUrgent)
	})
}

func TestComputeAttentionSignals_HasStaleReview(t *testing.T) {
	thresholds := defaultThresholds() // StaleReviewEnabled = true
	pr := prWithAge(0)
	pr.HeadSHA = "newsha123"

	t.Run("different SHAs -> HasStaleReview true", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "oldsha456", thresholds, "alice")
		assert.True(t, signals.HasStaleReview)
	})

	t.Run("same SHA -> HasStaleReview false", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "newsha123", thresholds, "alice")
		assert.False(t, signals.HasStaleReview)
	})

	t.Run("empty reviewSHA (no review yet) -> HasStaleReview false", func(t *testing.T) {
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.False(t, signals.HasStaleReview)
	})

	t.Run("disabled in thresholds -> HasStaleReview false even with different SHAs", func(t *testing.T) {
		disabled := thresholds
		disabled.StaleReviewEnabled = false
		signals := application.ComputeAttentionSignals(pr, 1, "oldsha456", disabled, "alice")
		assert.False(t, signals.HasStaleReview)
	})
}

func TestComputeAttentionSignals_HasCIFailure(t *testing.T) {
	thresholds := defaultThresholds() // CIFailureEnabled = true

	t.Run("own PR with failing CI -> HasCIFailure true", func(t *testing.T) {
		pr := prWithAge(0)
		pr.Author = "alice"
		pr.CIStatus = model.CIStatusFailing
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.True(t, signals.HasCIFailure)
	})

	t.Run("other's PR with failing CI -> HasCIFailure false", func(t *testing.T) {
		pr := prWithAge(0)
		pr.Author = "bob"
		pr.CIStatus = model.CIStatusFailing
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.False(t, signals.HasCIFailure)
	})

	t.Run("own PR with passing CI -> HasCIFailure false", func(t *testing.T) {
		pr := prWithAge(0)
		pr.Author = "alice"
		pr.CIStatus = model.CIStatusPassing
		signals := application.ComputeAttentionSignals(pr, 1, "", thresholds, "alice")
		assert.False(t, signals.HasCIFailure)
	})

	t.Run("disabled in thresholds -> HasCIFailure false even for own failing PR", func(t *testing.T) {
		disabled := thresholds
		disabled.CIFailureEnabled = false
		pr := prWithAge(0)
		pr.Author = "alice"
		pr.CIStatus = model.CIStatusFailing
		signals := application.ComputeAttentionSignals(pr, 1, "", disabled, "alice")
		assert.False(t, signals.HasCIFailure)
	})
}

func TestComputeAttentionSignals_HasAny(t *testing.T) {
	t.Run("no signals -> HasAny false", func(t *testing.T) {
		signals := model.AttentionSignals{}
		assert.False(t, signals.HasAny())
	})

	t.Run("one signal -> HasAny true", func(t *testing.T) {
		signals := model.AttentionSignals{NeedsMoreReviews: true}
		assert.True(t, signals.HasAny())
	})
}

func TestComputeAttentionSignals_Severity(t *testing.T) {
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
