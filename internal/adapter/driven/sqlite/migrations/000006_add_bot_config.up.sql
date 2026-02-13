CREATE TABLE IF NOT EXISTS bot_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Seed default bot usernames
INSERT INTO bot_config (username) VALUES ('coderabbitai');
INSERT INTO bot_config (username) VALUES ('github-actions[bot]');
INSERT INTO bot_config (username) VALUES ('copilot[bot]');
