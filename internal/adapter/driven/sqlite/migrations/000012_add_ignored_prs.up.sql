CREATE TABLE ignored_prs (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    repo_full_name TEXT     NOT NULL,
    pr_number      INTEGER  NOT NULL,
    ignored_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_full_name, pr_number)
);
