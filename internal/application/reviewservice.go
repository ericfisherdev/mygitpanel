package application

import (
	"context"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// suggestionPattern matches GitHub suggestion blocks in comment bodies.
// Example: ```suggestion\n<proposed code>\n```
var suggestionPattern = regexp.MustCompile("(?s)`{3,}suggestion[^\n]*\n(.*?)\n`{3,}")

// CommentThread groups a root review comment with its replies.
type CommentThread struct {
	RootComment model.ReviewComment
	Replies     []model.ReviewComment // Sorted by CreatedAt.
	IsResolved  bool                  // From root comment's IsResolved field.
}

// Suggestion represents a structured proposed code change extracted from a
// review comment's suggestion block.
type Suggestion struct {
	CommentID    int64
	FilePath     string
	StartLine    int // From comment's StartLine; if 0, uses Line.
	EndLine      int // From comment's Line.
	ProposedCode string
	OriginalBody string // Full comment body for context.
}

// PRReviewSummary is the complete enriched view of a PR's review state.
type PRReviewSummary struct {
	Reviews              []model.Review
	Threads              []CommentThread
	IssueComments        []model.IssueComment
	Suggestions          []Suggestion
	ReviewStatus         model.ReviewState
	HasBotReview         bool
	HasCoderabbitReview  bool
	AwaitingCoderabbit   bool
	ResolvedThreadCount  int
	UnresolvedThreadCount int
}

// ReviewService provides enrichment methods that transform raw stored review
// data into structured output for the HTTP API. It depends only on port interfaces.
type ReviewService struct {
	reviewStore    driven.ReviewStore
	botConfigStore driven.BotConfigStore
}

// NewReviewService creates a new ReviewService with the required dependencies.
func NewReviewService(
	reviewStore driven.ReviewStore,
	botConfigStore driven.BotConfigStore,
) *ReviewService {
	return &ReviewService{
		reviewStore:    reviewStore,
		botConfigStore: botConfigStore,
	}
}

// GetPRReviewSummary assembles the complete enriched view of a PR's review state.
// It loads reviews, comments, and bot config, then enriches with threading,
// suggestions, bot detection, outdated detection, and status aggregation.
func (s *ReviewService) GetPRReviewSummary(ctx context.Context, prID int64, headSHA string) (*PRReviewSummary, error) {
	botUsernames, err := s.botConfigStore.GetUsernames(ctx)
	if err != nil {
		return nil, err
	}

	reviews, err := s.reviewStore.GetReviewsByPR(ctx, prID)
	if err != nil {
		return nil, err
	}

	reviewComments, err := s.reviewStore.GetReviewCommentsByPR(ctx, prID)
	if err != nil {
		return nil, err
	}

	issueComments, err := s.reviewStore.GetIssueCommentsByPR(ctx, prID)
	if err != nil {
		return nil, err
	}

	// Mark reviews: IsBot and outdated.
	for i := range reviews {
		reviews[i].IsBot = isBotUser(reviews[i].ReviewerLogin, botUsernames)
		if isReviewOutdated(reviews[i], headSHA) {
			slog.Debug("review marked outdated",
				"review_id", reviews[i].ID,
				"commit_id", reviews[i].CommitID,
				"head_sha", headSHA,
			)
		}
	}

	// Mark issue comments: IsBot.
	for i := range issueComments {
		issueComments[i].IsBot = isBotUser(issueComments[i].Author, botUsernames)
	}

	threads := groupIntoThreads(reviewComments)
	suggestions := extractSuggestions(reviewComments)
	reviewStatus := aggregateReviewStatus(reviews, botUsernames)

	hasBotReview, hasCoderabbitReview, awaitingCoderabbit := computeBotFlags(reviews, botUsernames, headSHA)

	var resolvedCount, unresolvedCount int
	for _, t := range threads {
		if t.IsResolved {
			resolvedCount++
		} else {
			unresolvedCount++
		}
	}

	return &PRReviewSummary{
		Reviews:               reviews,
		Threads:               threads,
		IssueComments:         issueComments,
		Suggestions:           suggestions,
		ReviewStatus:          reviewStatus,
		HasBotReview:          hasBotReview,
		HasCoderabbitReview:   hasCoderabbitReview,
		AwaitingCoderabbit:    awaitingCoderabbit,
		ResolvedThreadCount:   resolvedCount,
		UnresolvedThreadCount: unresolvedCount,
	}, nil
}

// isBotUser checks if the login matches any configured bot username (case-insensitive).
func isBotUser(login string, botUsernames []string) bool {
	for _, bot := range botUsernames {
		if strings.EqualFold(login, bot) {
			return true
		}
	}
	return false
}

// IsNitpickComment returns true if the author is a bot AND the body contains
// a nitpick pattern indicator.
func IsNitpickComment(author, body string, botUsernames []string) bool {
	if !isBotUser(author, botUsernames) {
		return false
	}

	lowerBody := strings.ToLower(body)
	nitpickPatterns := []string{
		"**nitpick",
		"[nitpick]",
		"(nitpick)",
		"nitpick:",
		"nitpick (non-blocking)",
	}

	for _, pattern := range nitpickPatterns {
		if strings.Contains(lowerBody, pattern) {
			return true
		}
	}
	return false
}

// isReviewOutdated returns true if the review targets a commit other than
// the current PR head SHA.
func isReviewOutdated(review model.Review, headSHA string) bool {
	if review.CommitID == "" {
		return false
	}
	return review.CommitID != headSHA
}

// groupIntoThreads groups review comments by InReplyToID into conversation threads.
// Root comments (InReplyToID == nil) become thread roots. Replies are attached
// to their root's thread. Orphan replies (root not found) become their own thread.
// Threads are sorted by root comment's CreatedAt (oldest first).
func groupIntoThreads(comments []model.ReviewComment) []CommentThread {
	if len(comments) == 0 {
		return nil
	}

	// Index comments by ID for lookup.
	byID := make(map[int64]model.ReviewComment, len(comments))
	for _, c := range comments {
		byID[c.ID] = c
	}

	// Separate roots and replies.
	threadMap := make(map[int64]*CommentThread)
	var rootOrder []int64

	for _, c := range comments {
		if c.InReplyToID == nil {
			thread := &CommentThread{
				RootComment: c,
				IsResolved:  c.IsResolved,
			}
			threadMap[c.ID] = thread
			rootOrder = append(rootOrder, c.ID)
		}
	}

	// Attach replies to their roots, tracking orphans.
	for _, c := range comments {
		if c.InReplyToID == nil {
			continue
		}

		rootID := *c.InReplyToID
		if thread, ok := threadMap[rootID]; ok {
			thread.Replies = append(thread.Replies, c)
		} else {
			// Orphan reply: create a synthetic thread with this as root.
			orphanThread := &CommentThread{
				RootComment: c,
				IsResolved:  c.IsResolved,
			}
			threadMap[c.ID] = orphanThread
			rootOrder = append(rootOrder, c.ID)
		}
	}

	// Sort replies within each thread by CreatedAt.
	for _, thread := range threadMap {
		sort.Slice(thread.Replies, func(i, j int) bool {
			return thread.Replies[i].CreatedAt.Before(thread.Replies[j].CreatedAt)
		})
	}

	// Build result slice ordered by root's CreatedAt.
	threads := make([]CommentThread, 0, len(threadMap))
	for _, id := range rootOrder {
		threads = append(threads, *threadMap[id])
	}

	sort.Slice(threads, func(i, j int) bool {
		return threads[i].RootComment.CreatedAt.Before(threads[j].RootComment.CreatedAt)
	})

	return threads
}

// extractSuggestions parses GitHub suggestion blocks from review comment bodies.
func extractSuggestions(comments []model.ReviewComment) []Suggestion {
	var suggestions []Suggestion

	for _, c := range comments {
		matches := suggestionPattern.FindStringSubmatch(c.Body)
		if matches == nil {
			continue
		}

		startLine := c.StartLine
		if startLine == 0 {
			startLine = c.Line
		}

		suggestions = append(suggestions, Suggestion{
			CommentID:    c.ID,
			FilePath:     c.Path,
			StartLine:    startLine,
			EndLine:      c.Line,
			ProposedCode: matches[1],
			OriginalBody: c.Body,
		})
	}

	return suggestions
}

// aggregateReviewStatus computes the PR's overall review status from stored reviews.
// It uses only the latest review per non-bot human reviewer.
func aggregateReviewStatus(reviews []model.Review, botUsernames []string) model.ReviewState {
	// Find latest review per non-bot reviewer.
	latestByReviewer := make(map[string]model.Review)

	for _, r := range reviews {
		if isBotUser(r.ReviewerLogin, botUsernames) {
			continue
		}

		existing, ok := latestByReviewer[r.ReviewerLogin]
		if !ok || r.SubmittedAt.After(existing.SubmittedAt) {
			latestByReviewer[r.ReviewerLogin] = r
		}
	}

	if len(latestByReviewer) == 0 {
		return model.ReviewStatePending
	}

	allApproved := true
	for _, r := range latestByReviewer {
		if r.State == model.ReviewStateChangesRequested {
			return model.ReviewStateChangesRequested
		}
		if r.State != model.ReviewStateApproved {
			allApproved = false
		}
	}

	if allApproved {
		return model.ReviewStateApproved
	}

	return model.ReviewStateCommented
}

// computeBotFlags determines bot-related review flags.
func computeBotFlags(reviews []model.Review, botUsernames []string, headSHA string) (hasBotReview, hasCoderabbitReview, awaitingCoderabbit bool) {
	for _, r := range reviews {
		if isBotUser(r.ReviewerLogin, botUsernames) {
			hasBotReview = true
		}

		if isCoderabbitUser(r.ReviewerLogin, botUsernames) {
			hasCoderabbitReview = true
			if r.CommitID == headSHA {
				// CodeRabbit has reviewed the latest commit.
				awaitingCoderabbit = false
				return hasBotReview, hasCoderabbitReview, awaitingCoderabbit
			}
		}
	}

	// If CodeRabbit has reviewed but none match headSHA, still awaiting.
	if hasCoderabbitReview {
		awaitingCoderabbit = true
	}

	return hasBotReview, hasCoderabbitReview, awaitingCoderabbit
}

// isCoderabbitUser checks if a reviewer login matches any bot username containing "coderabbit".
func isCoderabbitUser(login string, botUsernames []string) bool {
	for _, bot := range botUsernames {
		if strings.Contains(strings.ToLower(bot), "coderabbit") &&
			strings.EqualFold(login, bot) {
			return true
		}
	}
	return false
}
