CREATE TABLE IF NOT EXISTS jira_connections (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    display_name TEXT     NOT NULL,
    base_url     TEXT     NOT NULL,
    email        TEXT     NOT NULL,
    token        TEXT     NOT NULL DEFAULT '',
    is_default   INTEGER  NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS repo_jira_mapping (
    repo_full_name     TEXT    NOT NULL PRIMARY KEY,
    jira_connection_id INTEGER,
    FOREIGN KEY (repo_full_name) REFERENCES repositories(full_name) ON DELETE CASCADE,
    FOREIGN KEY (jira_connection_id) REFERENCES jira_connections(id) ON DELETE SET NULL
);
