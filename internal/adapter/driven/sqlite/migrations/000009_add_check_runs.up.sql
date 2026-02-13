CREATE TABLE check_runs (
    id            INTEGER PRIMARY KEY,
    pr_id         INTEGER NOT NULL,
    name          TEXT    NOT NULL,
    status        TEXT    NOT NULL DEFAULT '',
    conclusion    TEXT    NOT NULL DEFAULT '',
    is_required   INTEGER NOT NULL DEFAULT 0,
    details_url   TEXT    NOT NULL DEFAULT '',
    started_at    DATETIME,
    completed_at  DATETIME,
    FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE CASCADE
);

CREATE INDEX idx_check_runs_pr_id ON check_runs(pr_id);
