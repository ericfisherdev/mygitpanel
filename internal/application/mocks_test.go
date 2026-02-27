package application

import (
	"context"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// testReviewStore is a configurable ReviewStore stub for white-box tests.
// Fields control the values returned by the corresponding Get methods;
// all write methods are no-ops.
type testReviewStore struct {
	reviews        []model.Review
	reviewComments []model.ReviewComment
	issueComments  []model.IssueComment
}

func (m *testReviewStore) UpsertReview(_ context.Context, _ model.Review) error { return nil }
func (m *testReviewStore) UpsertReviewComment(_ context.Context, _ model.ReviewComment) error {
	return nil
}
func (m *testReviewStore) UpsertIssueComment(_ context.Context, _ model.IssueComment) error {
	return nil
}
func (m *testReviewStore) GetReviewsByPR(_ context.Context, _ int64) ([]model.Review, error) {
	return m.reviews, nil
}
func (m *testReviewStore) GetReviewCommentsByPR(_ context.Context, _ int64) ([]model.ReviewComment, error) {
	return m.reviewComments, nil
}
func (m *testReviewStore) GetIssueCommentsByPR(_ context.Context, _ int64) ([]model.IssueComment, error) {
	return m.issueComments, nil
}
func (m *testReviewStore) UpdateCommentResolution(_ context.Context, _ int64, _ bool) error {
	return nil
}
func (m *testReviewStore) DeleteReviewsByPR(_ context.Context, _ int64) error { return nil }

// testBotConfigStore is a configurable BotConfigStore stub for white-box tests.
type testBotConfigStore struct {
	usernames []string
}

func (m *testBotConfigStore) Add(_ context.Context, config model.BotConfig) (model.BotConfig, error) {
	return config, nil
}
func (m *testBotConfigStore) Remove(_ context.Context, _ string) error             { return nil }
func (m *testBotConfigStore) ListAll(_ context.Context) ([]model.BotConfig, error) { return nil, nil }
func (m *testBotConfigStore) GetUsernames(_ context.Context) ([]string, error) {
	return m.usernames, nil
}

// testCheckStore is a configurable CheckStore stub for white-box tests.
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

// testPRStore is a configurable PRStore stub for white-box tests.
// GetByNumber returns the pr field; all other methods are no-ops.
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
