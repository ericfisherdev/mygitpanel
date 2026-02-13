ALTER TABLE pull_requests DROP COLUMN additions;
ALTER TABLE pull_requests DROP COLUMN deletions;
ALTER TABLE pull_requests DROP COLUMN changed_files;
ALTER TABLE pull_requests DROP COLUMN mergeable_status;
ALTER TABLE pull_requests DROP COLUMN ci_status;
