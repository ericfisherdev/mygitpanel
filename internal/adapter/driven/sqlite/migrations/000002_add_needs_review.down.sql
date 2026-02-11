DROP INDEX IF EXISTS idx_pull_requests_needs_review;
ALTER TABLE pull_requests DROP COLUMN needs_review;
