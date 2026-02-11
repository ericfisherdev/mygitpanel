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
	gh       *gh.Client
	username string
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
		gh:       client,
		username: username,
	}
}

// NewClientWithHTTPClient creates a Client with a custom http.Client and base URL.
// This constructor is intended for testing, allowing injection of an httptest server.
func NewClientWithHTTPClient(httpClient *http.Client, baseURL, username string) (*Client, error) {
	client := gh.NewClient(httpClient)

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base URL: %w", err)
	}
	client.BaseURL = u

	return &Client{
		gh:       client,
		username: username,
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

// FetchReviews is a stub implementation for Phase 4.
func (c *Client) FetchReviews(_ context.Context, _ string, _ int) ([]model.Review, error) {
	return nil, nil
}

// FetchReviewComments is a stub implementation for Phase 4.
func (c *Client) FetchReviewComments(_ context.Context, _ string, _ int) ([]model.ReviewComment, error) {
	return nil, nil
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
