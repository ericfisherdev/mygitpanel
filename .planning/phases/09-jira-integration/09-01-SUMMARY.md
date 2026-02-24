---
phase: 09-jira-integration
plan: 01
subsystem: database, domain
tags: [jira, sqlite, aes-256-gcm, encryption, migrations, hexagonal]

# Dependency graph
requires:
  - phase: 08-review-workflows-and-attention-signals
    provides: credential encryption pattern (CredentialRepo), PR model with health signals
provides:
  - JiraConnection, JiraIssue, JiraComment domain models
  - JiraClient port interface with sentinel errors
  - JiraConnectionStore port interface (8 methods)
  - SQLite JiraConnectionRepo with AES-256-GCM token encryption
  - Migrations 000013 (jira_connections, repo_jira_mapping) and 000014 (jira_key column)
  - PRRepo updated to persist and scan jira_key
  - ExtractJiraKey application helper
affects: [09-02 jira-client-adapter, 09-03 settings-ui, 09-04 jiracard-handler]

# Tech tracking
tech-stack:
  added: []
  patterns: [per-repo encryption methods (SRP), zero-value-not-error for optional lookups, FK cascade with ON DELETE SET NULL for mappings]

key-files:
  created:
    - internal/domain/model/jiraconnection.go
    - internal/domain/port/driven/jiraclient.go
    - internal/domain/port/driven/jiraconnectionstore.go
    - internal/adapter/driven/sqlite/jiraconnectionrepo.go
    - internal/adapter/driven/sqlite/jiraconnectionrepo_test.go
    - internal/adapter/driven/sqlite/migrations/000013_add_jira_connections.up.sql
    - internal/adapter/driven/sqlite/migrations/000013_add_jira_connections.down.sql
    - internal/adapter/driven/sqlite/migrations/000014_add_jira_key_to_prs.up.sql
    - internal/adapter/driven/sqlite/migrations/000014_add_jira_key_to_prs.down.sql
    - internal/application/jirakey.go
    - internal/application/jirakey_test.go
  modified:
    - internal/domain/model/pullrequest.go
    - internal/adapter/driven/sqlite/prrepo.go

key-decisions:
  - "Duplicated encrypt/decrypt methods in JiraConnectionRepo (not shared with CredentialRepo) per SRP"
  - "GetByID and GetForRepo return zero-value JiraConnection (ID==0) + nil error when not found, matching project convention"
  - "repo_jira_mapping uses ON DELETE SET NULL so mapping rows survive connection deletion"

patterns-established:
  - "Zero-value-not-error: optional lookups return zero-value struct + nil error instead of sql.ErrNoRows"
  - "Atomic default switch: transaction clears all is_default then sets new one"

# Metrics
duration: 4min
completed: 2026-02-24
---

# Phase 9 Plan 1: Jira Data Foundation Summary

**Jira domain models, port interfaces, SQLite JiraConnectionRepo with AES-256-GCM encryption, PR jira_key persistence, and regex-based Jira key extraction**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-24T23:05:31Z
- **Completed:** 2026-02-24T23:09:22Z
- **Tasks:** 2
- **Files modified:** 13

## Accomplishments

- Domain models (JiraConnection, JiraIssue, JiraComment) and port interfaces (JiraClient, JiraConnectionStore) with zero external dependencies
- JiraConnectionRepo with full CRUD, repo-to-connection mapping, default fallback logic, and AES-256-GCM token encryption
- PRRepo updated across all 7 query paths (Upsert, GetByRepository, GetByStatus, GetByNumber, ListAll, ListNeedingReview, ListIgnoredWithPRData) to persist and scan jira_key
- ExtractJiraKey helper with branch-priority pattern matching and comprehensive table-driven tests

## Task Commits

Each task was committed atomically:

1. **Task 1: Domain models and port interfaces** - `19e71eb` (feat)
2. **Task 2: DB migrations, JiraConnectionRepo, PRRepo jira_key, ExtractJiraKey** - `b86868a` (feat)

## Files Created/Modified

- `internal/domain/model/jiraconnection.go` - JiraConnection, JiraIssue, JiraComment domain structs
- `internal/domain/port/driven/jiraclient.go` - JiraClient interface with GetIssue, AddComment, Ping + sentinel errors
- `internal/domain/port/driven/jiraconnectionstore.go` - JiraConnectionStore interface with 8 CRUD/mapping methods
- `internal/domain/model/pullrequest.go` - Added JiraKey string field
- `internal/adapter/driven/sqlite/migrations/000013_add_jira_connections.up.sql` - jira_connections and repo_jira_mapping tables
- `internal/adapter/driven/sqlite/migrations/000013_add_jira_connections.down.sql` - Drop tables
- `internal/adapter/driven/sqlite/migrations/000014_add_jira_key_to_prs.up.sql` - jira_key TEXT column on pull_requests
- `internal/adapter/driven/sqlite/migrations/000014_add_jira_key_to_prs.down.sql` - Irreversible (SQLite limitation)
- `internal/adapter/driven/sqlite/jiraconnectionrepo.go` - Full JiraConnectionStore implementation with encryption
- `internal/adapter/driven/sqlite/jiraconnectionrepo_test.go` - 11 test cases covering all operations
- `internal/adapter/driven/sqlite/prrepo.go` - Added jira_key to all INSERT/UPDATE/SELECT/Scan paths
- `internal/application/jirakey.go` - ExtractJiraKey regex helper
- `internal/application/jirakey_test.go` - 8 table-driven test cases

## Decisions Made

- Duplicated encrypt/decrypt methods in JiraConnectionRepo rather than sharing with CredentialRepo, per SRP (each repo owns its own encryption)
- GetByID and GetForRepo return zero-value JiraConnection + nil error when not found, consistent with project convention (not sql.ErrNoRows)
- repo_jira_mapping uses ON DELETE SET NULL so mapping rows persist (with NULL connection_id) after connection deletion

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All domain contracts and persistence in place for Plan 02 (Jira REST client adapter)
- JiraConnectionStore ready for Plan 03 (settings UI handlers)
- ExtractJiraKey and PRRepo.JiraKey ready for Plan 04 (JiraCard web handler wiring)

---
*Phase: 09-jira-integration*
*Completed: 2026-02-24*
