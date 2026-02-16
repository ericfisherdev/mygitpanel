CREATE TABLE credentials (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    service    TEXT     NOT NULL,
    key        TEXT     NOT NULL,
    value      TEXT     NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service, key)
);
