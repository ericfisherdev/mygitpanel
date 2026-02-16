CREATE TABLE pull_requests_backup (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    number           INTEGER NOT NULL,
    repo_full_name   TEXT    NOT NULL,
    title            TEXT    NOT NULL,
    author           TEXT    NOT NULL,
    status           TEXT    NOT NULL DEFAULT 'open',
    is_draft         INTEGER NOT NULL DEFAULT 0,
    needs_review     INTEGER NOT NULL DEFAULT 0,
    url              TEXT    NOT NULL DEFAULT '',
    branch           TEXT    NOT NULL DEFAULT '',
    base_branch      TEXT    NOT NULL DEFAULT '',
    labels           TEXT    NOT NULL DEFAULT '[]',
    head_sha         TEXT    NOT NULL DEFAULT '',
    additions        INTEGER NOT NULL DEFAULT 0,
    deletions        INTEGER NOT NULL DEFAULT 0,
    changed_files    INTEGER NOT NULL DEFAULT 0,
    mergeable_status TEXT    NOT NULL DEFAULT 'unknown',
    ci_status        TEXT    NOT NULL DEFAULT 'unknown',
    opened_at        DATETIME NOT NULL,
    updated_at       DATETIME NOT NULL,
    last_activity_at DATETIME NOT NULL,
    UNIQUE(repo_full_name, number)
);

INSERT INTO pull_requests_backup
SELECT id, number, repo_full_name, title, author, status, is_draft, needs_review,
       url, branch, base_branch, labels, head_sha,
       additions, deletions, changed_files, mergeable_status, ci_status,
       opened_at, updated_at, last_activity_at
FROM pull_requests;

DROP TABLE pull_requests;

ALTER TABLE pull_requests_backup RENAME TO pull_requests;
