package application

import (
	"context"
	"strings"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// PRHealthSummary contains the enriched health view of a PR's CI/check state.
type PRHealthSummary struct {
	CheckRuns []model.CheckRun
	CIStatus  model.CIStatus
}

// HealthService provides enrichment methods that transform raw stored check
// data into structured output for the HTTP API. It depends only on port interfaces.
type HealthService struct {
	checkStore driven.CheckStore
	prStore    driven.PRStore
}

// NewHealthService creates a new HealthService with the required dependencies.
func NewHealthService(checkStore driven.CheckStore, prStore driven.PRStore) *HealthService {
	return &HealthService{
		checkStore: checkStore,
		prStore:    prStore,
	}
}

// GetPRHealthSummary assembles the health view for a PR by loading stored
// check runs and the PR's persisted CIStatus. The CIStatus was computed during
// the poll cycle with both Checks API and Status API data, so we use the stored
// value rather than recomputing from check runs alone.
func (s *HealthService) GetPRHealthSummary(ctx context.Context, prID int64, repoFullName string, number int) (*PRHealthSummary, error) {
	checkRuns, err := s.checkStore.GetCheckRunsByPR(ctx, prID)
	if err != nil {
		return nil, err
	}

	// Use the CIStatus persisted on the PR, which was computed during poll
	// with both Checks API and Status API data.
	ciStatus := model.CIStatusUnknown
	pr, err := s.prStore.GetByNumber(ctx, repoFullName, number)
	if err == nil && pr != nil {
		ciStatus = pr.CIStatus
	}

	return &PRHealthSummary{
		CheckRuns: checkRuns,
		CIStatus:  ciStatus,
	}, nil
}

// computeCombinedCIStatus aggregates check runs from the Checks API and the
// combined status from the Status API into a single CIStatus value.
// Priority: failing > pending > passing > unknown.
func computeCombinedCIStatus(checkRuns []model.CheckRun, combinedStatus *model.CombinedStatus) model.CIStatus {
	if len(checkRuns) == 0 && (combinedStatus == nil || len(combinedStatus.Statuses) == 0) {
		return model.CIStatusUnknown
	}

	var hasFailing, hasPending bool

	for _, cr := range checkRuns {
		if cr.Status == "completed" {
			switch cr.Conclusion {
			case "failure", "canceled", "cancelled", "timed_out", "action_required": //nolint:misspell // GitHub API uses British "cancelled"
				hasFailing = true
			case "success", "neutral", "skipped":
				// passing -- no flag needed
			}
		} else {
			// queued, in_progress, waiting, requested, pending
			hasPending = true
		}
	}

	if combinedStatus != nil {
		switch combinedStatus.State {
		case "failure", "error":
			hasFailing = true
		case "pending":
			hasPending = true
		case "success":
			// passing -- no flag needed
		}
	}

	if hasFailing {
		return model.CIStatusFailing
	}
	if hasPending {
		return model.CIStatusPending
	}
	return model.CIStatusPassing
}

// markRequiredChecks sets IsRequired = true on check runs whose Name matches
// any entry in requiredContexts (case-insensitive). If requiredContexts is nil
// (branch protection unavailable), all checks remain IsRequired = false.
func markRequiredChecks(checkRuns []model.CheckRun, requiredContexts []string) {
	if len(requiredContexts) == 0 {
		return
	}

	requiredSet := make(map[string]bool, len(requiredContexts))
	for _, ctx := range requiredContexts {
		requiredSet[strings.ToLower(ctx)] = true
	}

	for i := range checkRuns {
		if requiredSet[strings.ToLower(checkRuns[i].Name)] {
			checkRuns[i].IsRequired = true
		}
	}
}
