package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

func TestClassifyActivity(t *testing.T) {
	tests := []struct {
		name     string
		elapsed  time.Duration
		wantTier ActivityTier
	}{
		{"30 minutes ago is hot", 30 * time.Minute, TierHot},
		{"59 minutes ago is hot (boundary)", 59 * time.Minute, TierHot},
		{"61 minutes ago is active (boundary)", 61 * time.Minute, TierActive},
		{"12 hours ago is active", 12 * time.Hour, TierActive},
		{"25 hours ago is warm", 25 * time.Hour, TierWarm},
		{"3 days ago is warm", 3 * 24 * time.Hour, TierWarm},
		{"8 days ago is stale", 8 * 24 * time.Hour, TierStale},
		{"zero time is stale", 0, TierStale},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lastActivity time.Time
			if tt.elapsed > 0 {
				lastActivity = time.Now().Add(-tt.elapsed)
			}
			got := classifyActivity(lastActivity)
			assert.Equal(t, tt.wantTier, got)
		})
	}
}

func TestTierInterval(t *testing.T) {
	tests := []struct {
		tier    ActivityTier
		wantDur time.Duration
	}{
		{TierHot, 2 * time.Minute},
		{TierActive, 5 * time.Minute},
		{TierWarm, 15 * time.Minute},
		{TierStale, 30 * time.Minute},
		{ActivityTier(99), 5 * time.Minute}, // unknown defaults to 5m
	}

	for _, tt := range tests {
		t.Run(tt.tier.String(), func(t *testing.T) {
			got := tierInterval(tt.tier)
			assert.Equal(t, tt.wantDur, got)
		})
	}
}

func TestFreshestActivity(t *testing.T) {
	t.Run("empty slice returns zero time", func(t *testing.T) {
		got := freshestActivity(nil)
		assert.True(t, got.IsZero())
	})

	t.Run("single PR returns its LastActivityAt", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		prs := []model.PullRequest{
			{LastActivityAt: now},
		}
		got := freshestActivity(prs)
		assert.Equal(t, now, got)
	})

	t.Run("multiple PRs returns the most recent", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		prs := []model.PullRequest{
			{LastActivityAt: now.Add(-2 * time.Hour)},
			{LastActivityAt: now},
			{LastActivityAt: now.Add(-5 * time.Hour)},
		}
		got := freshestActivity(prs)
		assert.Equal(t, now, got)
	})
}

func TestActivityTierString(t *testing.T) {
	tests := []struct {
		tier ActivityTier
		want string
	}{
		{TierHot, "hot"},
		{TierActive, "active"},
		{TierWarm, "warm"},
		{TierStale, "stale"},
		{ActivityTier(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tier.String())
		})
	}
}
