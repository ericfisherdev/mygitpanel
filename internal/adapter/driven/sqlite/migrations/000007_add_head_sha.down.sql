-- Requires SQLite 3.35.0+ (DROP COLUMN support). modernc.org/sqlite bundles 3.46.0+.
ALTER TABLE pull_requests DROP COLUMN head_sha;
