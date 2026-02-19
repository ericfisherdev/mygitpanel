---
phase: 08-review-workflows-and-attention-signals
plan: 01
subsystem: database
tags: [sqlite, domain-model, migrations, hexagonal-architecture]

requires:
  - phase: 07-gui-foundation
    provides: existing SQLite schema with migrations 000001-000009
provides:
  - Credential domain model and CredentialStore port/adapter
  - RepoSettings domain model and RepoSettingsStore port/adapter
  - IgnoredPR domain model and IgnoreStore port/adapter
  - PullRequest.NodeID field persisted via migration
affects: [08-02, 08-03, 08-04]

tech-stack:
  added: []
  patterns: [idempotent-upsert, nil-nil-for-missing, compile-time-interface-checks]

key-files:
  created:
    - internal/domain/model/credential.go
    - internal/domain/model/reposettings.go
    - internal/domain/model/ignoredpr.go
    - internal/domain/port/driven/credentialstore.go
    - internal/domain/port/driven/reposettingsstore.go
    - internal/domain/port/driven/ignorestore.go
    - internal/adapter/driven/sqlite/credentialrepo.go
    - internal/adapter/driven/sqlite/reposettingsrepo.go
    - internal/adapter/driven/sqlite/ignorerepo.go
  modified:
    - internal/domain/model/pullrequest.go
    - internal/adapter/driven/sqlite/prrepo.go

key-decisions:
  - "CredentialStore.Get returns empty string for missing keys (not an error) — consistent with nil-nil pattern"
  - "IgnoreStore.Ignore uses ON CONFLICT DO NOTHING for idempotency"
  - "RepoSettings foreign key to repositories with ON DELETE CASCADE"

patterns-established:
  - "Credential store pattern: Set/Get/GetAll/Delete with service+key composite"
  - "Settings store pattern: GetSettings returns nil for missing (caller applies defaults)"

duration: 3min
completed: 2026-02-16
---

# Phase 8 Plan 1: Data Foundation Summary

**Credential, RepoSettings, and IgnoredPR domain models with SQLite adapters and NodeID on PullRequest via 4 new migrations**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-16T04:09:16Z
- **Completed:** 2026-02-16T04:12:36Z
- **Tasks:** 2
- **Files modified:** 22

## Accomplishments

- 3 new domain models (Credential, RepoSettings, IgnoredPR) with zero external dependencies
- 3 new port interfaces (CredentialStore, RepoSettingsStore, IgnoreStore) with documented error contracts
- 3 new SQLite adapters with compile-time interface checks and 18 passing tests
- PullRequest.NodeID field added and integrated into all PR repo queries
- 4 migration pairs (000010-000013) for credentials, repo_settings, ignored_prs, and node_id

## Task Commits

Each task was committed atomically:

1. **Task 1: Domain models, port interfaces, and migrations** - `8469868` (feat)
2. **Task 2: SQLite adapters with tests and PR repo NodeID integration** - `954d19f` (feat)

## Files Created/Modified

- `internal/domain/model/credential.go` - Credential entity (service/key/value)
- `internal/domain/model/reposettings.go` - Per-repo attention thresholds
- `internal/domain/model/ignoredpr.go` - PR ignore list entity
- `internal/domain/model/pullrequest.go` - Added NodeID field
- `internal/domain/port/driven/credentialstore.go` - CredentialStore port interface
- `internal/domain/port/driven/reposettingsstore.go` - RepoSettingsStore port interface
- `internal/domain/port/driven/ignorestore.go` - IgnoreStore port interface
- `internal/adapter/driven/sqlite/credentialrepo.go` - CredentialStore SQLite adapter
- `internal/adapter/driven/sqlite/credentialrepo_test.go` - 7 tests for credential CRUD
- `internal/adapter/driven/sqlite/reposettingsrepo.go` - RepoSettingsStore SQLite adapter
- `internal/adapter/driven/sqlite/reposettingsrepo_test.go` - 3 tests for settings CRUD
- `internal/adapter/driven/sqlite/ignorerepo.go` - IgnoreStore SQLite adapter
- `internal/adapter/driven/sqlite/ignorerepo_test.go` - 6 tests for ignore list operations
- `internal/adapter/driven/sqlite/prrepo.go` - Added node_id to Upsert, scanPR, all SELECTs
- `internal/adapter/driven/sqlite/migrations/000010_add_credentials.up.sql` - credentials table
- `internal/adapter/driven/sqlite/migrations/000010_add_credentials.down.sql` - drop credentials
- `internal/adapter/driven/sqlite/migrations/000011_add_repo_settings.up.sql` - repo_settings table
- `internal/adapter/driven/sqlite/migrations/000011_add_repo_settings.down.sql` - drop repo_settings
- `internal/adapter/driven/sqlite/migrations/000012_add_ignored_prs.up.sql` - ignored_prs table
- `internal/adapter/driven/sqlite/migrations/000012_add_ignored_prs.down.sql` - drop ignored_prs
- `internal/adapter/driven/sqlite/migrations/000013_add_node_id.up.sql` - ALTER TABLE add node_id
- `internal/adapter/driven/sqlite/migrations/000013_add_node_id.down.sql` - recreate table without node_id

## Decisions Made

- CredentialStore.Get returns ("", nil) for missing keys, consistent with existing nil-nil-for-missing pattern
- IgnoreStore.Ignore uses ON CONFLICT DO NOTHING for idempotent ignore operations
- RepoSettings has foreign key to repositories(full_name) with ON DELETE CASCADE
- Migration 000013 down uses table-rebuild strategy since SQLite does not reliably support DROP COLUMN

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All domain models and persistence adapters ready for Phase 8 plans 02-04
- Credential store ready for GitHub token management UI
- RepoSettings store ready for per-repo attention thresholds
- IgnoreStore ready for PR ignore list feature
- NodeID persisted and ready for GraphQL draft toggle mutations

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-16*
