package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ericfisherdev/mygitpanel/internal/domain/model"
	"github.com/ericfisherdev/mygitpanel/internal/domain/port/driven"
)

// Compile-time interface satisfaction check.
var _ driven.ReviewStore = (*ReviewRepo)(nil)

// ReviewRepo is the SQLite implementation of the ReviewStore port interface.
type ReviewRepo struct {
	db *DB
}

// NewReviewRepo creates a new ReviewRepo backed by the given DB.
func NewReviewRepo(db *DB) *ReviewRepo {
	return &ReviewRepo{db: db}
}

// UpsertReview inserts or updates a review by its GitHub ID.
func (r *ReviewRepo) UpsertReview(ctx context.Context, review model.Review) error {
	const query = `
		INSERT INTO reviews (id, pr_id, reviewer_login, state, body, commit_id, submitted_at, is_bot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			pr_id = excluded.pr_id,
			reviewer_login = excluded.reviewer_login,
			state = excluded.state,
			body = excluded.body,
			commit_id = excluded.commit_id,
			submitted_at = excluded.submitted_at,
			is_bot = excluded.is_bot
	`

	isBot := 0
	if review.IsBot {
		isBot = 1
	}

	_, err := r.db.Writer.ExecContext(ctx, query,
		review.ID, review.PRID, review.ReviewerLogin, string(review.State),
		review.Body, review.CommitID, review.SubmittedAt.UTC(), isBot,
	)
	if err != nil {
		return fmt.Errorf("upsert review %d: %w", review.ID, err)
	}

	return nil
}

// UpsertReviewComment inserts or updates a review comment by its GitHub ID.
func (r *ReviewRepo) UpsertReviewComment(ctx context.Context, comment model.ReviewComment) error {
	const query = `
		INSERT INTO review_comments (
			id, review_id, pr_id, author, body, path, line, start_line,
			side, subject_type, diff_hunk, commit_id, is_resolved, is_outdated,
			in_reply_to_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			review_id = excluded.review_id,
			pr_id = excluded.pr_id,
			author = excluded.author,
			body = excluded.body,
			path = excluded.path,
			line = excluded.line,
			start_line = excluded.start_line,
			side = excluded.side,
			subject_type = excluded.subject_type,
			diff_hunk = excluded.diff_hunk,
			commit_id = excluded.commit_id,
			is_resolved = excluded.is_resolved,
			is_outdated = excluded.is_outdated,
			in_reply_to_id = excluded.in_reply_to_id,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`

	isResolved := 0
	if comment.IsResolved {
		isResolved = 1
	}

	isOutdated := 0
	if comment.IsOutdated {
		isOutdated = 1
	}

	var inReplyToID any
	if comment.InReplyToID != nil {
		inReplyToID = *comment.InReplyToID
	}

	_, err := r.db.Writer.ExecContext(ctx, query,
		comment.ID, comment.ReviewID, comment.PRID, comment.Author,
		comment.Body, comment.Path, comment.Line, comment.StartLine,
		comment.Side, comment.SubjectType, comment.DiffHunk, comment.CommitID,
		isResolved, isOutdated, inReplyToID,
		comment.CreatedAt.UTC(), comment.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert review comment %d: %w", comment.ID, err)
	}

	return nil
}

// UpsertIssueComment inserts or updates an issue comment by its GitHub ID.
func (r *ReviewRepo) UpsertIssueComment(ctx context.Context, comment model.IssueComment) error {
	const query = `
		INSERT INTO issue_comments (id, pr_id, author, body, is_bot, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			pr_id = excluded.pr_id,
			author = excluded.author,
			body = excluded.body,
			is_bot = excluded.is_bot,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`

	isBot := 0
	if comment.IsBot {
		isBot = 1
	}

	_, err := r.db.Writer.ExecContext(ctx, query,
		comment.ID, comment.PRID, comment.Author, comment.Body,
		isBot, comment.CreatedAt.UTC(), comment.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert issue comment %d: %w", comment.ID, err)
	}

	return nil
}

// GetReviewsByPR returns all reviews for the given PR, ordered by submitted_at.
func (r *ReviewRepo) GetReviewsByPR(ctx context.Context, prID int64) ([]model.Review, error) {
	const query = `
		SELECT id, pr_id, reviewer_login, state, body, commit_id, submitted_at, is_bot
		FROM reviews
		WHERE pr_id = ?
		ORDER BY submitted_at
	`

	rows, err := r.db.Reader.QueryContext(ctx, query, prID)
	if err != nil {
		return nil, fmt.Errorf("query reviews for PR %d: %w", prID, err)
	}
	defer rows.Close()

	var reviews []model.Review
	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, *review)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviews: %w", err)
	}

	return reviews, nil
}

// GetReviewCommentsByPR returns all review comments for the given PR, ordered by created_at.
func (r *ReviewRepo) GetReviewCommentsByPR(ctx context.Context, prID int64) ([]model.ReviewComment, error) {
	const query = `
		SELECT id, review_id, pr_id, author, body, path, line, start_line,
		       side, subject_type, diff_hunk, commit_id, is_resolved, is_outdated,
		       in_reply_to_id, created_at, updated_at
		FROM review_comments
		WHERE pr_id = ?
		ORDER BY created_at
	`

	rows, err := r.db.Reader.QueryContext(ctx, query, prID)
	if err != nil {
		return nil, fmt.Errorf("query review comments for PR %d: %w", prID, err)
	}
	defer rows.Close()

	var comments []model.ReviewComment
	for rows.Next() {
		comment, err := scanReviewComment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review comment: %w", err)
		}
		comments = append(comments, *comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review comments: %w", err)
	}

	return comments, nil
}

// GetIssueCommentsByPR returns all issue comments for the given PR, ordered by created_at.
func (r *ReviewRepo) GetIssueCommentsByPR(ctx context.Context, prID int64) ([]model.IssueComment, error) {
	const query = `
		SELECT id, pr_id, author, body, is_bot, created_at, updated_at
		FROM issue_comments
		WHERE pr_id = ?
		ORDER BY created_at
	`

	rows, err := r.db.Reader.QueryContext(ctx, query, prID)
	if err != nil {
		return nil, fmt.Errorf("query issue comments for PR %d: %w", prID, err)
	}
	defer rows.Close()

	var comments []model.IssueComment
	for rows.Next() {
		comment, err := scanIssueComment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue comment: %w", err)
		}
		comments = append(comments, *comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue comments: %w", err)
	}

	return comments, nil
}

// UpdateCommentResolution sets the is_resolved flag on a review comment.
func (r *ReviewRepo) UpdateCommentResolution(ctx context.Context, commentID int64, isResolved bool) error {
	const query = `UPDATE review_comments SET is_resolved = ? WHERE id = ?`

	resolved := 0
	if isResolved {
		resolved = 1
	}

	_, err := r.db.Writer.ExecContext(ctx, query, resolved, commentID)
	if err != nil {
		return fmt.Errorf("update comment resolution %d: %w", commentID, err)
	}

	return nil
}

// DeleteReviewsByPR removes all reviews, review comments, and issue comments
// associated with the given PR.
func (r *ReviewRepo) DeleteReviewsByPR(ctx context.Context, prID int64) error {
	const deleteReviews = `DELETE FROM reviews WHERE pr_id = ?`
	const deleteReviewComments = `DELETE FROM review_comments WHERE pr_id = ?`
	const deleteIssueComments = `DELETE FROM issue_comments WHERE pr_id = ?`

	if _, err := r.db.Writer.ExecContext(ctx, deleteReviews, prID); err != nil {
		return fmt.Errorf("delete reviews for PR %d: %w", prID, err)
	}

	if _, err := r.db.Writer.ExecContext(ctx, deleteReviewComments, prID); err != nil {
		return fmt.Errorf("delete review comments for PR %d: %w", prID, err)
	}

	if _, err := r.db.Writer.ExecContext(ctx, deleteIssueComments, prID); err != nil {
		return fmt.Errorf("delete issue comments for PR %d: %w", prID, err)
	}

	return nil
}

func scanReview(s scanner) (*model.Review, error) {
	var review model.Review
	var state string
	var isBot int
	var submittedAt string

	err := s.Scan(
		&review.ID, &review.PRID, &review.ReviewerLogin, &state,
		&review.Body, &review.CommitID, &submittedAt, &isBot,
	)
	if err != nil {
		return nil, err
	}

	review.State = model.ReviewState(state)
	review.IsBot = isBot != 0

	review.SubmittedAt, err = parseTime(submittedAt)
	if err != nil {
		return nil, fmt.Errorf("parse submitted_at: %w", err)
	}

	return &review, nil
}

func scanReviewComment(s scanner) (*model.ReviewComment, error) {
	var comment model.ReviewComment
	var isResolved, isOutdated int
	var inReplyToID sql.NullInt64
	var createdAt, updatedAt string

	err := s.Scan(
		&comment.ID, &comment.ReviewID, &comment.PRID, &comment.Author,
		&comment.Body, &comment.Path, &comment.Line, &comment.StartLine,
		&comment.Side, &comment.SubjectType, &comment.DiffHunk, &comment.CommitID,
		&isResolved, &isOutdated, &inReplyToID, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	comment.IsResolved = isResolved != 0
	comment.IsOutdated = isOutdated != 0

	if inReplyToID.Valid {
		id := inReplyToID.Int64
		comment.InReplyToID = &id
	}

	comment.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	comment.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &comment, nil
}

func scanIssueComment(s scanner) (*model.IssueComment, error) {
	var comment model.IssueComment
	var isBot int
	var createdAt, updatedAt string

	err := s.Scan(
		&comment.ID, &comment.PRID, &comment.Author, &comment.Body,
		&isBot, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	comment.IsBot = isBot != 0

	comment.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	comment.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &comment, nil
}
