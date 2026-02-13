CREATE TABLE IF NOT EXISTS review_comments (
    id INTEGER PRIMARY KEY,
    review_id INTEGER NOT NULL DEFAULT 0,
    pr_id INTEGER NOT NULL,
    author TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    line INTEGER NOT NULL DEFAULT 0,
    start_line INTEGER NOT NULL DEFAULT 0,
    side TEXT NOT NULL DEFAULT '',
    subject_type TEXT NOT NULL DEFAULT 'line',
    diff_hunk TEXT NOT NULL DEFAULT '',
    commit_id TEXT NOT NULL DEFAULT '',
    is_resolved INTEGER NOT NULL DEFAULT 0,
    is_outdated INTEGER NOT NULL DEFAULT 0,
    in_reply_to_id INTEGER,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE CASCADE
);
CREATE INDEX idx_review_comments_pr_id ON review_comments(pr_id);
CREATE INDEX idx_review_comments_in_reply_to ON review_comments(in_reply_to_id);
