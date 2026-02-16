package web

import (
	"fmt"
	"strings"
	"time"

	vm "github.com/ericfisherdev/mygitpanel/internal/adapter/driving/web/viewmodel"
	"github.com/ericfisherdev/mygitpanel/internal/application"
	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// toPRCardViewModels converts a slice of domain PullRequests to PRCardViewModels.
func toPRCardViewModels(prs []model.PullRequest) []vm.PRCardViewModel {
	cards := make([]vm.PRCardViewModel, 0, len(prs))
	for _, pr := range prs {
		cards = append(cards, toPRCardViewModel(pr))
	}
	return cards
}

// toPRCardViewModel converts a single domain PullRequest to a PRCardViewModel.
func toPRCardViewModel(pr model.PullRequest) vm.PRCardViewModel {
	labels := pr.Labels
	if labels == nil {
		labels = []string{}
	}

	return vm.PRCardViewModel{
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
	}
}

// toPRDetailViewModelWithWriteCaps creates a PR detail view model with write capability flags.
// username and hasCredentials control whether review/comment/draft-toggle forms are shown.
// Pass empty username and false for hasCredentials to disable write capabilities.
func toPRDetailViewModelWithWriteCaps(
	pr model.PullRequest,
	summary *application.PRReviewSummary,
	checkRuns []model.CheckRun,
	botUsernames []string,
	username string,
	hasCredentials bool,
) vm.PRDetailViewModel {
	card := toPRCardViewModel(pr)

	const shortSHALength = 7

	headSHA := pr.HeadSHA
	shortSHA := headSHA
	if len(shortSHA) > shortSHALength {
		shortSHA = shortSHA[:shortSHALength]
	}

	isCurrentUser := strings.EqualFold(pr.Author, username)
	canReview := hasCredentials && !isCurrentUser
	canToggleDraft := hasCredentials && pr.NodeID != ""

	reviewActionURL := fmt.Sprintf("/app/prs/%s/%d/review", pr.RepoFullName, pr.Number)
	commentURL := fmt.Sprintf("/app/prs/%s/%d/comment", pr.RepoFullName, pr.Number)
	draftToggleURL := fmt.Sprintf("/app/prs/%s/%d/draft", pr.RepoFullName, pr.Number)
	ignoreURL := fmt.Sprintf("/app/prs/%s/%d/ignore", pr.RepoFullName, pr.Number)

	detail := vm.PRDetailViewModel{
		PRCardViewModel: card,
		Branch:          pr.Branch,
		BaseBranch:      pr.BaseBranch,
		HeadSHA:         shortSHA,
		NodeID:          pr.NodeID,
		Additions:       pr.Additions,
		Deletions:       pr.Deletions,
		ChangedFiles:    pr.ChangedFiles,
		Reviews:         []vm.ReviewViewModel{},
		Threads:         []vm.ThreadViewModel{},
		IssueComments:   []vm.IssueCommentViewModel{},
		CheckRuns:       []vm.CheckRunViewModel{},
		Suggestions:     []vm.SuggestionViewModel{},
		CanReview:       canReview,
		CanToggleDraft:  canToggleDraft,
		IsCurrentUser:   isCurrentUser,
		RepoFullName:    pr.RepoFullName,
		PRNumber:        pr.Number,
		ReviewActionURL: reviewActionURL,
		CommentURL:      commentURL,
		DraftToggleURL:  draftToggleURL,
		IgnoreURL:       ignoreURL,
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
