CREATE TABLE IF NOT EXISTS reviews (
    id INTEGER PRIMARY KEY,
    pr_id INTEGER NOT NULL,
    reviewer_login TEXT NOT NULL,
    state TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    commit_id TEXT NOT NULL DEFAULT '',
    submitted_at DATETIME NOT NULL,
    is_bot INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE CASCADE
);
CREATE INDEX idx_reviews_pr_id ON reviews(pr_id);
CREATE INDEX idx_reviews_reviewer ON reviews(reviewer_login);
