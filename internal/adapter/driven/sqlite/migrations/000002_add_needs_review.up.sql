ALTER TABLE pull_requests ADD COLUMN needs_review INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_pull_requests_needs_review ON pull_requests(needs_review);
