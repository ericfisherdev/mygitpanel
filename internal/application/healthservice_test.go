package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// --- computeCombinedCIStatus tests (table-driven) ---

func TestComputeCombinedCIStatus(t *testing.T) {
	tests := []struct {
		name           string
		checkRuns      []model.CheckRun
		combinedStatus *model.CombinedStatus
		want           model.CIStatus
	}{
		{
			name: "all passing check runs + success status",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "success"},
				{Name: "lint", Status: "completed", Conclusion: "success"},
			},
			combinedStatus: &model.CombinedStatus{State: "success"},
			want:           model.CIStatusPassing,
		},
		{
			name: "one failing check run",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "success"},
				{Name: "test", Status: "completed", Conclusion: "failure"},
			},
			want: model.CIStatusFailing,
		},
		{
			name: "one pending check run (in_progress)",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "success"},
				{Name: "test", Status: "in_progress"},
			},
			want: model.CIStatusPending,
		},
		{
			name: "failing takes precedence over pending",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "failure"},
				{Name: "test", Status: "in_progress"},
			},
			want: model.CIStatusFailing,
		},
		{
			name:           "no check runs no combined status",
			checkRuns:      nil,
			combinedStatus: nil,
			want:           model.CIStatusUnknown,
		},
		{
			name:           "no check runs combined status failure",
			checkRuns:      nil,
			combinedStatus: &model.CombinedStatus{State: "failure", Statuses: []model.CommitStatus{{State: "failure"}}},
			want:           model.CIStatusFailing,
		},
		{
			name:           "no check runs combined status pending",
			checkRuns:      nil,
			combinedStatus: &model.CombinedStatus{State: "pending", Statuses: []model.CommitStatus{{State: "pending"}}},
			want:           model.CIStatusPending,
		},
		{
			name: "check runs passing combined status failing",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "success"},
			},
			combinedStatus: &model.CombinedStatus{State: "failure", Statuses: []model.CommitStatus{{State: "failure"}}},
			want:           model.CIStatusFailing,
		},
		{
			name: "neutral conclusion treated as passing",
			checkRuns: []model.CheckRun{
				{Name: "optional", Status: "completed", Conclusion: "neutral"},
			},
			want: model.CIStatusPassing,
		},
		{
			name: "skipped conclusion treated as passing",
			checkRuns: []model.CheckRun{
				{Name: "conditional", Status: "completed", Conclusion: "skipped"},
			},
			want: model.CIStatusPassing,
		},
		{
			name: "canceled conclusion treated as failing",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "canceled"},
			},
			want: model.CIStatusFailing,
		},
		{
			name: "timed_out conclusion treated as failing",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "timed_out"},
			},
			want: model.CIStatusFailing,
		},
		{
			name: "action_required conclusion treated as failing",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "action_required"},
			},
			want: model.CIStatusFailing,
		},
		{
			name: "queued status treated as pending",
			checkRuns: []model.CheckRun{
				{Name: "build", Status: "queued"},
			},
			want: model.CIStatusPending,
		},
		{
			name:           "empty combined status with no check runs is unknown",
			checkRuns:      nil,
			combinedStatus: &model.CombinedStatus{State: "", Statuses: nil},
			want:           model.CIStatusUnknown,
		},
		{
			name:           "combined status error state treated as failing",
			checkRuns:      nil,
			combinedStatus: &model.CombinedStatus{State: "error", Statuses: []model.CommitStatus{{State: "error"}}},
			want:           model.CIStatusFailing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCombinedCIStatus(tt.checkRuns, tt.combinedStatus)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- markRequiredChecks tests ---

func TestMarkRequiredChecks(t *testing.T) {
	t.Run("marks matching check runs as required", func(t *testing.T) {
		checkRuns := []model.CheckRun{
			{Name: "build", IsRequired: false},
			{Name: "lint", IsRequired: false},
			{Name: "optional-test", IsRequired: false},
		}

		markRequiredChecks(checkRuns, []string{"build", "lint"})

		assert.True(t, checkRuns[0].IsRequired, "build should be required")
		assert.True(t, checkRuns[1].IsRequired, "lint should be required")
		assert.False(t, checkRuns[2].IsRequired, "optional-test should not be required")
	})

	t.Run("nil required contexts leaves all as not required", func(t *testing.T) {
		checkRuns := []model.CheckRun{
			{Name: "build", IsRequired: false},
			{Name: "lint", IsRequired: false},
		}

		markRequiredChecks(checkRuns, nil)

		assert.False(t, checkRuns[0].IsRequired)
		assert.False(t, checkRuns[1].IsRequired)
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		checkRuns := []model.CheckRun{
			{Name: "Build", IsRequired: false},
			{Name: "LINT", IsRequired: false},
		}

		markRequiredChecks(checkRuns, []string{"build", "lint"})

		assert.True(t, checkRuns[0].IsRequired, "Build should match build (case-insensitive)")
		assert.True(t, checkRuns[1].IsRequired, "LINT should match lint (case-insensitive)")
	})

	t.Run("empty required contexts leaves all as not required", func(t *testing.T) {
		checkRuns := []model.CheckRun{
			{Name: "build", IsRequired: false},
		}

		markRequiredChecks(checkRuns, []string{})

		assert.False(t, checkRuns[0].IsRequired)
	})
}

// --- GetPRHealthSummary tests ---

// testCheckStore implements driven.CheckStore for testing.
type testCheckStore struct {
	runs []model.CheckRun
}

func (s *testCheckStore) ReplaceCheckRunsForPR(_ context.Context, _ int64, runs []model.CheckRun) error {
	s.runs = runs
	return nil
}

func (s *testCheckStore) GetCheckRunsByPR(_ context.Context, _ int64) ([]model.CheckRun, error) {
	return s.runs, nil
}

// testPRStore returns a PR with a pre-set CIStatus for GetPRHealthSummary tests.
type testPRStore struct {
	pr *model.PullRequest
}

func (s *testPRStore) Upsert(_ context.Context, _ model.PullRequest) error { return nil }
func (s *testPRStore) GetByRepository(_ context.Context, _ string) ([]model.PullRequest, error) {
	return nil, nil
}
func (s *testPRStore) GetByStatus(_ context.Context, _ model.PRStatus) ([]model.PullRequest, error) {
	return nil, nil
}
func (s *testPRStore) GetByNumber(_ context.Context, _ string, _ int) (*model.PullRequest, error) {
	return s.pr, nil
}
func (s *testPRStore) ListAll(_ context.Context) ([]model.PullRequest, error) { return nil, nil }
func (s *testPRStore) ListNeedingReview(_ context.Context) ([]model.PullRequest, error) {
	return nil, nil
}
func (s *testPRStore) ListIgnoredWithPRData(_ context.Context) ([]model.PullRequest, error) {
	return nil, nil
}
func (s *testPRStore) Delete(_ context.Context, _ string, _ int) error { return nil }

func TestGetPRHealthSummary(t *testing.T) {
	t.Run("returns check runs and stored CI status from PR", func(t *testing.T) {
		checkStore := &testCheckStore{
			runs: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "success"},
				{Name: "test", Status: "completed", Conclusion: "success"},
			},
		}
		prStore := &testPRStore{pr: &model.PullRequest{
			ID: 42, RepoFullName: "org/repo", Number: 1, CIStatus: model.CIStatusPassing,
		}}

		svc := NewHealthService(checkStore, prStore)
		summary, err := svc.GetPRHealthSummary(context.Background(), 42, "org/repo", 1)

		require.NoError(t, err)
		require.NotNil(t, summary)
		assert.Len(t, summary.CheckRuns, 2)
		assert.Equal(t, model.CIStatusPassing, summary.CIStatus)
	})

	t.Run("returns unknown when PR has no stored CI status", func(t *testing.T) {
		checkStore := &testCheckStore{runs: nil}
		prStore := &testPRStore{pr: &model.PullRequest{
			ID: 99, RepoFullName: "org/repo", Number: 2, CIStatus: model.CIStatusUnknown,
		}}

		svc := NewHealthService(checkStore, prStore)
		summary, err := svc.GetPRHealthSummary(context.Background(), 99, "org/repo", 2)

		require.NoError(t, err)
		require.NotNil(t, summary)
		assert.Empty(t, summary.CheckRuns)
		assert.Equal(t, model.CIStatusUnknown, summary.CIStatus)
	})

	t.Run("returns stored failing status even with passing check runs", func(t *testing.T) {
		// PR's stored CIStatus was computed during poll with both Checks API and
		// Status API — the Status API reported failure even though check runs passed.
		checkStore := &testCheckStore{
			runs: []model.CheckRun{
				{Name: "build", Status: "completed", Conclusion: "success"},
			},
		}
		prStore := &testPRStore{pr: &model.PullRequest{
			ID: 42, RepoFullName: "org/repo", Number: 1, CIStatus: model.CIStatusFailing,
		}}

		svc := NewHealthService(checkStore, prStore)
		summary, err := svc.GetPRHealthSummary(context.Background(), 42, "org/repo", 1)

		require.NoError(t, err)
		assert.Equal(t, model.CIStatusFailing, summary.CIStatus)
	})
}
