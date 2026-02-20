---
phase: 08-review-workflows-and-attention-signals
plan: 01
subsystem: database
tags: [sqlite, aes-256-gcm, migrations, hexagonal, ports-and-adapters, credentials, thresholds]

# Dependency graph
requires:
  - phase: 07-gui-foundation
    provides: hexagonal adapter patterns established, SQLite migration infrastructure
provides:
  - Credential domain model and CredentialStore port with AES-256-GCM SQLite implementation
  - Threshold domain models (GlobalSettings, RepoThreshold) and ThresholdStore port/implementation
  - IgnoredPR type and IgnoreStore port with IgnoreRepo SQLite implementation
  - GitHubWriter port interface for write operations (Reviews, Comments, Draft toggle, ValidateToken)
  - AttentionSignals and EffectiveThresholds transient domain models
  - SQLite migrations 000010-000012 for credentials, thresholds, and ignored_prs tables
  - Optional MYGITPANEL_GITHUB_TOKEN (warn-not-fail) and MYGITPANEL_SECRET_KEY config
affects:
  - 08-02-PLAN.md
  - 08-03-PLAN.md
  - 08-04-PLAN.md
  - 08-05-PLAN.md

# Tech tracking
tech-stack:
  added: [crypto/aes, crypto/cipher, crypto/rand, encoding/hex (stdlib only — no new deps)]
  patterns:
    - AES-256-GCM application-level encryption with base64-encoded nonce||ciphertext blobs
    - Compile-time interface checks (var _ driven.X = (*Impl)(nil)) for all new repos
    - Nil-pointer fields in RepoThreshold for "use global default" semantics
    - INSERT OR IGNORE for idempotent operations (IgnoreRepo.Ignore)
    - INSERT OR REPLACE for credential and threshold upserts

key-files:
  created:
    - internal/domain/model/credential.go
    - internal/domain/model/threshold.go
    - internal/domain/model/attention.go
    - internal/domain/port/driven/credentialstore.go
    - internal/domain/port/driven/thresholdstore.go
    - internal/domain/port/driven/ignorestore.go
    - internal/domain/port/driven/githubwriter.go
    - internal/adapter/driven/sqlite/credentialrepo.go
    - internal/adapter/driven/sqlite/credentialrepo_test.go
    - internal/adapter/driven/sqlite/thresholdrepo.go
    - internal/adapter/driven/sqlite/thresholdrepo_test.go
    - internal/adapter/driven/sqlite/ignorerepo.go
    - internal/adapter/driven/sqlite/ignorerepo_test.go
    - internal/adapter/driven/sqlite/migrations/000010_add_credentials.up.sql
    - internal/adapter/driven/sqlite/migrations/000010_add_credentials.down.sql
    - internal/adapter/driven/sqlite/migrations/000011_add_thresholds.up.sql
    - internal/adapter/driven/sqlite/migrations/000011_add_thresholds.down.sql
    - internal/adapter/driven/sqlite/migrations/000012_add_ignored_prs.up.sql
    - internal/adapter/driven/sqlite/migrations/000012_add_ignored_prs.down.sql
  modified:
    - internal/config/config.go
    - internal/config/config_test.go

key-decisions:
  - "MYGITPANEL_GITHUB_TOKEN demoted from required to optional — app starts with warning, polls disabled until credential set via GUI"
  - "MYGITPANEL_SECRET_KEY optional at startup — credential storage returns ErrEncryptionKeyNotSet when key is nil, app still starts"
  - "IgnoredPR type defined in port/driven package (not model/) — it is a persistence concern not a pure domain entity"
  - "DraftLineComment and ReviewRequest types defined in githubwriter.go — port-layer input types for write operations"
  - "ThresholdRepo uses sql.NullInt64 intermediates to correctly handle NULL columns from repo_thresholds"

patterns-established:
  - "ErrEncryptionKeyNotSet sentinel: nil key returns typed error rather than panic, enabling graceful degradation"
  - "Nil-pointer-as-override pattern: nil field in RepoThreshold means use global default, non-nil means override"
  - "Port-layer input types: DraftLineComment, ReviewRequest, IgnoredPR defined alongside their interface, not in model/"

requirements-completed: [CRED-01, CRED-02, ATT-01, ATT-02, ATT-03, ATT-04]

# Metrics
duration: 22min
completed: 2026-02-19
---

# Phase 08 Plan 01: Data Foundation Summary

**AES-256-GCM credential storage, attention threshold config, and PR ignore list with four new port interfaces and three SQLite migrations**

## Performance

- **Duration:** 22 min
- **Started:** 2026-02-19T20:16:09Z
- **Completed:** 2026-02-19T20:38:00Z
- **Tasks:** 2
- **Files modified:** 21

## Accomplishments

- Established all four hexagonal port interfaces for Phase 8: CredentialStore, ThresholdStore, IgnoreStore, GitHubWriter
- Implemented three SQLite repositories with AES-256-GCM encryption (CredentialRepo), nullable override semantics (ThresholdRepo), and idempotent set operations (IgnoreRepo) — all with compile-time interface checks and real in-memory database test coverage
- Demoted MYGITPANEL_GITHUB_TOKEN from startup-required to optional with graceful warning, enabling credential-via-GUI flow; added MYGITPANEL_SECRET_KEY with 64-hex-char validation

## Task Commits

Each task was committed atomically:

1. **Task 1: Domain models, port interfaces, and config changes** - `4c18c0b` (feat)
2. **Task 2: Migrations 000010-000012 and SQLite repo implementations with tests** - `bb33c50` (feat)

**Plan metadata:** (docs commit to follow)

## Files Created/Modified

- `internal/domain/model/credential.go` - Credential struct (ID, Service, plaintext Value, UpdatedAt)
- `internal/domain/model/threshold.go` - GlobalSettings with defaults, RepoThreshold with nil-pointer overrides
- `internal/domain/model/attention.go` - AttentionSignals (HasAny, Severity methods), EffectiveThresholds
- `internal/domain/port/driven/credentialstore.go` - CredentialStore: Set/Get/List/Delete
- `internal/domain/port/driven/thresholdstore.go` - ThresholdStore: GetGlobalSettings/SetGlobalSettings/GetRepoThreshold/SetRepoThreshold/DeleteRepoThreshold
- `internal/domain/port/driven/ignorestore.go` - IgnoredPR type, IgnoreStore: Ignore/Unignore/IsIgnored/ListIgnored/ListIgnoredIDs
- `internal/domain/port/driven/githubwriter.go` - DraftLineComment, ReviewRequest types; GitHubWriter: SubmitReview/CreateReplyComment/CreateIssueComment/ConvertPullRequestToDraft/MarkPullRequestReadyForReview/ValidateToken
- `internal/adapter/driven/sqlite/credentialrepo.go` - AES-256-GCM encrypt/decrypt, ErrEncryptionKeyNotSet sentinel
- `internal/adapter/driven/sqlite/thresholdrepo.go` - Global settings key-value store + per-repo nullable overrides
- `internal/adapter/driven/sqlite/ignorerepo.go` - INSERT OR IGNORE for idempotent ignore, O(1) ListIgnoredIDs map
- `internal/adapter/driven/sqlite/migrations/000010_add_credentials.up.sql` - credentials table
- `internal/adapter/driven/sqlite/migrations/000011_add_thresholds.up.sql` - global_settings + repo_thresholds tables with seeded defaults
- `internal/adapter/driven/sqlite/migrations/000012_add_ignored_prs.up.sql` - ignored_prs table with FK ON DELETE CASCADE
- `internal/config/config.go` - SecretKey []byte field, optional token/key loading with slog.Warn

## Decisions Made

- MYGITPANEL_GITHUB_TOKEN demoted to optional (warn-not-fail). This is the key architectural enabler for the credential-via-GUI flow in Plans 02-03.
- MYGITPANEL_SECRET_KEY validation requires exactly 64 hex chars (= 32 bytes for AES-256). Present-but-malformed returns error; absent returns nil key with warning and app starts.
- IgnoredPR struct placed in `domain/port/driven` package, not `domain/model`. Rationale: it is a storage/lookup concern used only by the ignore infrastructure, not a pure domain entity referenced across the application.
- DraftLineComment and ReviewRequest defined in `githubwriter.go` alongside the interface (port-layer input types pattern).
- Nil-pointer semantics for RepoThreshold: nil field = "inherit from global". Application layer merges at query time.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed duplicate addTestPR helper from ignorerepo_test.go**
- **Found during:** Task 2 (ignorerepo_test.go compilation)
- **Issue:** reviewrepo_test.go already defines addTestPR; creating another in ignorerepo_test.go caused a redeclaration compile error
- **Fix:** Removed the duplicate; added insertPRForIgnoreTest as a lighter helper for multi-PR list tests that assumes repo already exists
- **Files modified:** internal/adapter/driven/sqlite/ignorerepo_test.go
- **Verification:** go test ./internal/adapter/driven/sqlite/... passes
- **Committed in:** bb33c50 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Minor — test helper naming conflict resolved without affecting functionality or scope.

## Issues Encountered

- Go binary not installed as a Linux native binary on the WSL environment; installed via mise (go@1.25.7) to get a working linux/amd64 binary. Windows Go binary at `/mnt/c/Program Files/Go/bin/go.exe` cannot operate on WSL filesystem paths.

## User Setup Required

None — no external service configuration required for this plan. New env vars are optional with graceful degradation:
- `MYGITPANEL_SECRET_KEY`: Optional; if set must be 64 hex chars (32 bytes). Generate with: `openssl rand -hex 32`
- `MYGITPANEL_GITHUB_TOKEN`: Now optional at startup; credential store used when set via GUI

## Next Phase Readiness

- All four port interfaces are defined and compile-checked; Plans 02-05 can wire them into HTTP handlers and application services
- CredentialStore, ThresholdStore, and IgnoreStore are ready for dependency injection in main.go
- GitHubWriter port is defined; Plan 02 implements the concrete GitHub adapter for write operations

## Self-Check: PASSED

All 15 key files verified present on disk. Both task commits (4c18c0b, bb33c50) verified in git log.

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-19*
