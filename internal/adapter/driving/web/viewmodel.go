package web

import (
	"fmt"
	"strings"
	"time"

	vm "github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/viewmodel"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// toPRCardViewModel converts a single domain PullRequest to a PRCardViewModel.
// Pass model.AttentionSignals{} for zero-value signals (no signals active).
func toPRCardViewModel(pr model.PullRequest, signals model.AttentionSignals) vm.PRCardViewModel {
	labels := pr.Labels
	if labels == nil {
		labels = []string{}
	}

	return vm.PRCardViewModel{
		ID:                    pr.ID,
		Number:                pr.Number,
		Repository:            pr.RepoFullName,
		Title:                 pr.Title,
		Author:                pr.Author,
		Status:                string(pr.Status),
		IsDraft:               pr.IsDraft,
		NeedsReview:           pr.NeedsReview,
		CIStatus:              string(pr.CIStatus),
		MergeableStatus:       string(pr.MergeableStatus),
		DaysSinceOpened:       pr.DaysSinceOpened(),
		DaysSinceLastActivity: pr.DaysSinceLastActivity(),
		Labels:                labels,
		URL:                   pr.URL,
		DetailPath:            fmt.Sprintf("/app/prs/%s/%d", pr.RepoFullName, pr.Number),
		Attention:             signals,
	}
}

// toPRDetailViewModel converts domain data into a fully enriched PRDetailViewModel.
// Review enrichment failure is non-fatal: pass nil for summary/checkRuns if unavailable.
// authenticatedUser is used to set IsOwnPR; pass empty string if unauthenticated.
func toPRDetailViewModel(
	pr model.PullRequest,
	summary *application.PRReviewSummary,
	checkRuns []model.CheckRun,
	botUsernames []string,
	authenticatedUser string,
) vm.PRDetailViewModel {
	card := toPRCardViewModel(pr, model.AttentionSignals{})

	const shortSHALength = 7

	headSHA := pr.HeadSHA
	shortSHA := headSHA
	if len(shortSHA) > shortSHALength {
		shortSHA = shortSHA[:shortSHALength]
	}

	repoParts := strings.SplitN(pr.RepoFullName, "/", 2)
	owner, repoName := "", pr.RepoFullName
	if len(repoParts) == 2 {
		owner, repoName = repoParts[0], repoParts[1]
	}

	detail := vm.PRDetailViewModel{
		PRCardViewModel: card,
		Owner:           owner,
		RepoName:        repoName,
		Branch:          pr.Branch,
		BaseBranch:      pr.BaseBranch,
		HeadSHA:         shortSHA,
		Additions:       pr.Additions,
		Deletions:       pr.Deletions,
		ChangedFiles:    pr.ChangedFiles,
		IsOwnPR:         authenticatedUser != "" && pr.Author == authenticatedUser,
		Reviews:         []vm.ReviewViewModel{},
		Threads:         []vm.ThreadViewModel{},
		IssueComments:   []vm.IssueCommentViewModel{},
		CheckRuns:       []vm.CheckRunViewModel{},
		Suggestions:     []vm.SuggestionViewModel{},
	}

	if summary != nil {
		detail.Reviews = toReviewViewModels(summary.Reviews, headSHA, botUsernames)
		detail.Threads = toThreadViewModels(summary.Threads)
		detail.IssueComments = toIssueCommentViewModels(summary.IssueComments)
		detail.Suggestions = toSuggestionViewModels(summary.Suggestions)
		detail.ReviewStatus = string(summary.ReviewStatus)
		detail.HasBotReview = summary.HasBotReview
		detail.HasCoderabbitReview = summary.HasCoderabbitReview
		detail.AwaitingCoderabbit = summary.AwaitingCoderabbit
		detail.ResolvedThreads = summary.ResolvedThreadCount
		detail.UnresolvedThreads = summary.UnresolvedThreadCount
	}

	if len(checkRuns) > 0 {
		detail.CheckRuns = toCheckRunViewModels(checkRuns)
	}

	return detail
}

// toReviewViewModels converts domain Reviews to ReviewViewModels.
func toReviewViewModels(reviews []model.Review, headSHA string, botUsernames []string) []vm.ReviewViewModel {
	vms := make([]vm.ReviewViewModel, 0, len(reviews))
	for _, r := range reviews {
		isOutdated := r.CommitID != "" && r.CommitID != headSHA
		isNitpick := application.IsNitpickComment(r.ReviewerLogin, r.Body, botUsernames)

		vms = append(vms, vm.ReviewViewModel{
			ID:          r.ID,
			Reviewer:    r.ReviewerLogin,
			State:       string(r.State),
			Body:        r.Body,
			BodyHTML:    RenderMarkdown(r.Body),
			CommitID:    r.CommitID,
			SubmittedAt: r.SubmittedAt.UTC().Format(time.RFC3339),
			IsBot:       r.IsBot,
			IsOutdated:  isOutdated,
			IsNitpick:   isNitpick,
		})
	}
	return vms
}

// toThreadViewModels converts application CommentThreads to ThreadViewModels.
func toThreadViewModels(threads []application.CommentThread) []vm.ThreadViewModel {
	vms := make([]vm.ThreadViewModel, 0, len(threads))
	for _, t := range threads {
		replies := make([]vm.ReviewCommentViewModel, 0, len(t.Replies))
		for _, r := range t.Replies {
			replies = append(replies, toReviewCommentViewModel(r))
		}

		vms = append(vms, vm.ThreadViewModel{
			RootComment:  toReviewCommentViewModel(t.RootComment),
			Replies:      replies,
			IsResolved:   t.IsResolved,
			CommentCount: 1 + len(t.Replies),
		})
	}
	return vms
}

// toReviewCommentViewModel converts a domain ReviewComment to a ReviewCommentViewModel.
func toReviewCommentViewModel(c model.ReviewComment) vm.ReviewCommentViewModel {
	return vm.ReviewCommentViewModel{
		ID:           c.ID,
		Author:       c.Author,
		Body:         c.Body,
		BodyHTML:     RenderMarkdown(c.Body),
		FilePath:     c.Path,
		Line:         c.Line,
		StartLine:    c.StartLine,
		DiffHunk:     c.DiffHunk,
		DiffHunkHTML: RenderDiffHunk(c.DiffHunk),
		CommitID:     c.CommitID,
		IsOutdated:   c.IsOutdated,
		CreatedAt:    c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// toIssueCommentViewModels converts domain IssueComments to IssueCommentViewModels.
func toIssueCommentViewModels(comments []model.IssueComment) []vm.IssueCommentViewModel {
	vms := make([]vm.IssueCommentViewModel, 0, len(comments))
	for _, c := range comments {
		vms = append(vms, vm.IssueCommentViewModel{
			ID:        c.ID,
			Author:    c.Author,
			Body:      c.Body,
			BodyHTML:  RenderMarkdown(c.Body),
			IsBot:     c.IsBot,
			CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return vms
}

// toCheckRunViewModels converts domain CheckRuns to CheckRunViewModels.
func toCheckRunViewModels(runs []model.CheckRun) []vm.CheckRunViewModel {
	vms := make([]vm.CheckRunViewModel, 0, len(runs))
	for _, cr := range runs {
		vms = append(vms, vm.CheckRunViewModel{
			ID:         cr.ID,
			Name:       cr.Name,
			Status:     cr.Status,
			Conclusion: cr.Conclusion,
			IsRequired: cr.IsRequired,
			DetailsURL: cr.DetailsURL,
		})
	}
	return vms
}

// toSuggestionViewModels converts application Suggestions to SuggestionViewModels.
func toSuggestionViewModels(suggestions []application.Suggestion) []vm.SuggestionViewModel {
	vms := make([]vm.SuggestionViewModel, 0, len(suggestions))
	for _, s := range suggestions {
		vms = append(vms, vm.SuggestionViewModel{
			CommentID:    s.CommentID,
			FilePath:     s.FilePath,
			StartLine:    s.StartLine,
			EndLine:      s.EndLine,
			ProposedCode: s.ProposedCode,
		})
	}
	return vms
}
