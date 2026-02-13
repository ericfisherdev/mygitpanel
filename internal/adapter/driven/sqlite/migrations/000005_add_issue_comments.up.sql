CREATE TABLE IF NOT EXISTS issue_comments (
    id INTEGER PRIMARY KEY,
    pr_id INTEGER NOT NULL,
    author TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    is_bot INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE CASCADE
);
CREATE INDEX idx_issue_comments_pr_id ON issue_comments(pr_id);
