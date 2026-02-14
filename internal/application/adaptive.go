package application

import (
	"time"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// ActivityTier represents the polling frequency classification for a repository
// based on how recently its PRs had activity.
type ActivityTier int

const (
	// TierHot indicates activity within the last hour. Polls every 2 minutes.
	TierHot ActivityTier = iota
	// TierActive indicates activity within the last day. Polls every 5 minutes.
	TierActive
	// TierWarm indicates activity within the last 7 days. Polls every 15 minutes.
	TierWarm
	// TierStale indicates no activity for 7+ days. Polls every 30 minutes.
	TierStale
)

// Polling intervals per activity tier.
const (
	intervalHot    = 2 * time.Minute
	intervalActive = 5 * time.Minute
	intervalWarm   = 15 * time.Minute
	intervalStale  = 30 * time.Minute
)

// String returns a human-readable name for the activity tier.
func (t ActivityTier) String() string {
	switch t {
	case TierHot:
		return "hot"
	case TierActive:
		return "active"
	case TierWarm:
		return "warm"
	case TierStale:
		return "stale"
	default:
		return "unknown"
	}
}

// tierInterval returns the polling interval for the given activity tier.
func tierInterval(tier ActivityTier) time.Duration {
	switch tier {
	case TierHot:
		return intervalHot
	case TierActive:
		return intervalActive
	case TierWarm:
		return intervalWarm
	case TierStale:
		return intervalStale
	default:
		return intervalActive
	}
}

// classifyActivity determines the activity tier based on the time elapsed
// since the last activity. A zero-value time is treated as TierStale.
func classifyActivity(lastActivity time.Time) ActivityTier {
	if lastActivity.IsZero() {
		return TierStale
	}

	elapsed := time.Since(lastActivity)

	switch {
	case elapsed < 1*time.Hour:
		return TierHot
	case elapsed < 24*time.Hour:
		return TierActive
	case elapsed < 7*24*time.Hour:
		return TierWarm
	default:
		return TierStale
	}
}

// repoSchedule tracks per-repository adaptive polling state.
type repoSchedule struct {
	tier       ActivityTier
	nextPollAt time.Time
	lastPolled time.Time
}

// ScheduleInfo is an exported view of a repo's adaptive polling schedule,
// used for observability and testing.
type ScheduleInfo struct {
	Tier       ActivityTier
	NextPollAt time.Time
	LastPolled time.Time
}

// freshestActivity finds the most recent LastActivityAt across all PRs.
// Returns the zero time if the slice is empty, which classifies as TierStale.
func freshestActivity(prs []model.PullRequest) time.Time {
	var newest time.Time
	for _, pr := range prs {
		if pr.LastActivityAt.After(newest) {
			newest = pr.LastActivityAt
		}
	}
	return newest
}
