CREATE TABLE repo_settings (
    repo_full_name       TEXT    NOT NULL PRIMARY KEY,
    required_review_count INTEGER NOT NULL DEFAULT 2,
    urgency_days         INTEGER NOT NULL DEFAULT 7,
    FOREIGN KEY (repo_full_name) REFERENCES repositories(full_name) ON DELETE CASCADE
);
