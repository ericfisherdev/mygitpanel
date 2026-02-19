CREATE TABLE IF NOT EXISTS global_settings (
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
INSERT OR IGNORE INTO global_settings (key, value) VALUES
    ('review_count_threshold', '1'),
    ('age_urgency_days',       '7'),
    ('stale_review_enabled',   '1'),
    ('ci_failure_enabled',     '1');
CREATE TABLE IF NOT EXISTS repo_thresholds (
    repo_full_name       TEXT    NOT NULL PRIMARY KEY,
    review_count         INTEGER,
    age_urgency_days     INTEGER,
    stale_review_enabled INTEGER,
    ci_failure_enabled   INTEGER,
    FOREIGN KEY (repo_full_name) REFERENCES repositories(full_name) ON DELETE CASCADE
);
