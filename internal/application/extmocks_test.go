package application_test

import (
	"context"
	"sync"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
)

// mockReviewStore records review-store write calls and returns configurable
// values from GetReviewsByPR. Used by both attention-service and poll-service tests.
type mockReviewStore struct {
	mu                     sync.Mutex
	upsertedReviews        []model.Review
	upsertedReviewComments []model.ReviewComment
	upsertedIssueComments  []model.IssueComment
	updatedResolutions     map[int64]bool
	// stubReviews and stubErr configure the return value of GetReviewsByPR.
	stubReviews []model.Review
	stubErr     error
}

func newMockReviewStore() *mockReviewStore {
	return &mockReviewStore{
		updatedResolutions: make(map[int64]bool),
	}
}

func (m *mockReviewStore) UpsertReview(_ context.Context, review model.Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertedReviews = append(m.upsertedReviews, review)
	return nil
}

func (m *mockReviewStore) UpsertReviewComment(_ context.Context, comment model.ReviewComment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertedReviewComments = append(m.upsertedReviewComments, comment)
	return nil
}

func (m *mockReviewStore) UpsertIssueComment(_ context.Context, comment model.IssueComment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertedIssueComments = append(m.upsertedIssueComments, comment)
	return nil
}

func (m *mockReviewStore) GetReviewsByPR(_ context.Context, _ int64) ([]model.Review, error) {
	return m.stubReviews, m.stubErr
}

func (m *mockReviewStore) GetReviewCommentsByPR(_ context.Context, _ int64) ([]model.ReviewComment, error) {
	return nil, nil
}

func (m *mockReviewStore) GetIssueCommentsByPR(_ context.Context, _ int64) ([]model.IssueComment, error) {
	return nil, nil
}

func (m *mockReviewStore) UpdateCommentResolution(_ context.Context, commentID int64, isResolved bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedResolutions[commentID] = isResolved
	return nil
}

func (m *mockReviewStore) DeleteReviewsByPR(_ context.Context, _ int64) error { return nil }

func (m *mockReviewStore) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertedReviews = nil
	m.upsertedReviewComments = nil
	m.upsertedIssueComments = nil
	m.updatedResolutions = make(map[int64]bool)
}

// noopPRStoreMixin provides nil-returning no-op implementations for the PRStore
// methods that poll-service tests do not need to verify. Embed this in mock PR
// store types to avoid duplicating these stubs across mockPRStore and
// adaptiveMockPRStore.
type noopPRStoreMixin struct{}

func (*noopPRStoreMixin) GetByStatus(_ context.Context, _ model.PRStatus) ([]model.PullRequest, error) {
	return nil, nil
}
func (*noopPRStoreMixin) ListAll(_ context.Context) ([]model.PullRequest, error) { return nil, nil }
func (*noopPRStoreMixin) ListNeedingReview(_ context.Context) ([]model.PullRequest, error) {
	return nil, nil
}
func (*noopPRStoreMixin) ListIgnoredWithPRData(_ context.Context) ([]model.PullRequest, error) {
	return nil, nil
}
