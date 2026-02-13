package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	gh "github.com/google/go-github/v82/github"
	"github.com/gregjones/httpcache"

	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"

	"github.com/efisher/reviewhub/internal/domain/model"
	"github.com/efisher/reviewhub/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.GitHubClient = (*Client)(nil)

// Client implements the driven.GitHubClient port using the go-github library.
type Client struct {
	gh         *gh.Client
	username   string
	token      string // Stored for GraphQL Authorization header.
	graphqlURL string // "https://api.github.com/graphql" in production; derived from baseURL in tests.
}

// NewClient creates a new GitHub API client with the following transport stack:
//  1. httpcache (ETag-based conditional request caching)
//  2. go-github-ratelimit (secondary rate limit middleware, sleeps on 429)
//  3. go-github (GitHub REST API client with PAT auth)
func NewClient(token, username string) *Client {
	cacheTransport := httpcache.NewMemoryCacheTransport()
	rateLimitClient := github_ratelimit.NewClient(cacheTransport, nil)
	client := gh.NewClient(rateLimitClient).WithAuthToken(token)

	return &Client{
		gh:         client,
		username:   username,
		token:      token,
		graphqlURL: "https://api.github.com/graphql",
	}
}

// NewClientWithHTTPClient creates a Client with a custom http.Client and base URL.
// This constructor is intended for testing, allowing injection of an httptest server.
func NewClientWithHTTPClient(httpClient *http.Client, baseURL, username, token string) (*Client, error) {
	client := gh.NewClient(httpClient)

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base URL: %w", err)
	}
	client.BaseURL = u

	// Derive graphqlURL from baseURL so httptest servers can intercept GraphQL requests.
	graphqlU := *u
	graphqlU.Path = "/graphql"

	return &Client{
		gh:         client,
		username:   username,
		token:      token,
		graphqlURL: graphqlU.String(),
	}, nil
}

// FetchPullRequests retrieves all open pull requests for the given repository.
// It handles pagination automatically and maps go-github types to domain model types.
func (c *Client) FetchPullRequests(ctx context.Context, repoFullName string) ([]model.PullRequest, error) {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return nil, err
	}

	opts := &gh.PullRequestListOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}

	var allPRs []model.PullRequest

	for {
		prs, resp, err := c.gh.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("listing pull requests for %s (page %d): %w", repoFullName, opts.Page, err)
		}

		logRateLimit(resp, repoFullName, opts.Page, len(prs))

		for _, pr := range prs {
			allPRs = append(allPRs, mapPullRequest(pr, repoFullName))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if allPRs == nil {
		allPRs = []model.PullRequest{}
	}

	return allPRs, nil
}

// FetchReviews retrieves all reviews for a pull request.
// It handles pagination automatically and maps go-github types to domain model types.
func (c *Client) FetchReviews(ctx context.Context, repoFullName string, prNumber int) ([]model.Review, error) {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return nil, err
	}

	opts := &gh.ListOptions{PerPage: 100}
	var allReviews []model.Review

	for {
		reviews, resp, err := c.gh.PullRequests.ListReviews(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("listing reviews for %s#%d (page %d): %w", repoFullName, prNumber, opts.Page, err)
		}

		for _, r := range reviews {
			allReviews = append(allReviews, mapReview(r))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allReviews, nil
}

// FetchReviewComments retrieves all review comments (inline code comments) for a pull request.
// It handles pagination automatically and maps go-github types to domain model types.
func (c *Client) FetchReviewComments(ctx context.Context, repoFullName string, prNumber int) ([]model.ReviewComment, error) {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return nil, err
	}

	opts := &gh.PullRequestListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	var allComments []model.ReviewComment

	for {
		comments, resp, err := c.gh.PullRequests.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("listing review comments for %s#%d (page %d): %w", repoFullName, prNumber, opts.Page, err)
		}

		for _, comment := range comments {
			allComments = append(allComments, mapReviewComment(comment))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// FetchIssueComments retrieves all general PR-level comments (from the Issues API) for a pull request.
// It handles pagination automatically and maps go-github types to domain model types.
func (c *Client) FetchIssueComments(ctx context.Context, repoFullName string, prNumber int) ([]model.IssueComment, error) {
	owner, repo, err := splitRepo(repoFullName)
	if err != nil {
		return nil, err
	}

	opts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	var allComments []model.IssueComment

	for {
		comments, resp, err := c.gh.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("listing issue comments for %s#%d (page %d): %w", repoFullName, prNumber, opts.Page, err)
		}

		for _, comment := range comments {
			allComments = append(allComments, mapIssueComment(comment))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// mapReview converts a go-github PullRequestReview to a domain model Review.
func mapReview(r *gh.PullRequestReview) model.Review {
	return model.Review{
		ID:            r.GetID(),
		PRID:          0, // Caller assigns before storing; adapter has no knowledge of database ID.
		ReviewerLogin: r.GetUser().GetLogin(),
		State:         model.ReviewState(strings.ToLower(r.GetState())),
		Body:          r.GetBody(),
		CommitID:      r.GetCommitID(),
		SubmittedAt:   r.GetSubmittedAt().Time,
		IsBot:         false, // Bot detection happens in the enrichment service, not the adapter.
	}
}

// mapReviewComment converts a go-github PullRequestComment to a domain model ReviewComment.
func mapReviewComment(c *gh.PullRequestComment) model.ReviewComment {
	var inReplyTo *int64
	if c.InReplyTo != nil {
		val := c.GetInReplyTo()
		inReplyTo = &val
	}

	return model.ReviewComment{
		ID:          c.GetID(),
		ReviewID:    c.GetPullRequestReviewID(),
		PRID:        0, // Caller assigns before storing.
		Author:      c.GetUser().GetLogin(),
		Body:        c.GetBody(),
		Path:        c.GetPath(),
		Line:        c.GetLine(),
		StartLine:   c.GetStartLine(),
		Side:        c.GetSide(),
		SubjectType: c.GetSubjectType(),
		DiffHunk:    c.GetDiffHunk(),
		CommitID:    c.GetCommitID(),
		IsResolved:  false, // Set later from GraphQL data.
		IsOutdated:  false, // Set later by enrichment service.
		InReplyToID: inReplyTo,
		CreatedAt:   c.GetCreatedAt().Time,
		UpdatedAt:   c.GetUpdatedAt().Time,
	}
}

// mapIssueComment converts a go-github IssueComment to a domain model IssueComment.
func mapIssueComment(c *gh.IssueComment) model.IssueComment {
	return model.IssueComment{
		ID:        c.GetID(),
		PRID:      0, // Caller assigns before storing.
		Author:    c.GetUser().GetLogin(),
		Body:      c.GetBody(),
		IsBot:     false, // Enrichment service handles bot detection.
		CreatedAt: c.GetCreatedAt().Time,
		UpdatedAt: c.GetUpdatedAt().Time,
	}
}

// logRateLimit logs the GitHub API rate limit status after each call.
func logRateLimit(resp *gh.Response, endpoint string, page, count int) {
	if resp == nil {
		return
	}

	slog.Debug("github api call",
		"endpoint", endpoint,
		"page", page,
		"count", count,
		"rate_remaining", resp.Rate.Remaining,
		"rate_limit", resp.Rate.Limit,
	)

	if resp.Rate.Remaining < 100 {
		slog.Warn("github rate limit low",
			"remaining", resp.Rate.Remaining,
			"reset_in", time.Until(resp.Rate.Reset.Time).Round(time.Second),
		)
	}
}

// mapPullRequest converts a go-github PullRequest to a domain model PullRequest.
// It uses GetXxx() helper methods exclusively to avoid nil pointer panics.
func mapPullRequest(pr *gh.PullRequest, repoFullName string) model.PullRequest {
	status := model.PRStatusOpen
	if !pr.GetMergedAt().IsZero() {
		status = model.PRStatusMerged
	} else if pr.GetState() == "closed" {
		status = model.PRStatusClosed
	}

	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, l.GetName())
	}

	reviewers := make([]string, 0, len(pr.RequestedReviewers))
	for _, r := range pr.RequestedReviewers {
		reviewers = append(reviewers, r.GetLogin())
	}

	teamSlugs := make([]string, 0, len(pr.RequestedTeams))
	for _, t := range pr.RequestedTeams {
		teamSlugs = append(teamSlugs, t.GetSlug())
	}

	return model.PullRequest{
		Number:             pr.GetNumber(),
		RepoFullName:       repoFullName,
		Title:              pr.GetTitle(),
		Author:             pr.GetUser().GetLogin(),
		Status:             status,
		IsDraft:            pr.GetDraft(),
		URL:                pr.GetHTMLURL(),
		Branch:             pr.GetHead().GetRef(),
		BaseBranch:         pr.GetBase().GetRef(),
		HeadSHA:            pr.GetHead().GetSHA(),
		Labels:             labels,
		OpenedAt:           pr.GetCreatedAt().Time,
		UpdatedAt:          pr.GetUpdatedAt().Time,
		LastActivityAt:     pr.GetUpdatedAt().Time,
		RequestedReviewers: reviewers,
		RequestedTeamSlugs: teamSlugs,
	}
}

// splitRepo splits a "owner/repo" string into its two components.
func splitRepo(fullName string) (string, string, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo name %q: expected owner/repo", fullName)
	}
	return parts[0], parts[1], nil
}
