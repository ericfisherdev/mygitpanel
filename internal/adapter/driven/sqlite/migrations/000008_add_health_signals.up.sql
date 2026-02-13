ALTER TABLE pull_requests ADD COLUMN additions INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pull_requests ADD COLUMN deletions INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pull_requests ADD COLUMN changed_files INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pull_requests ADD COLUMN mergeable_status TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE pull_requests ADD COLUMN ci_status TEXT NOT NULL DEFAULT 'unknown';
