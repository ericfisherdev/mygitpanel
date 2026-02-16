---
phase: 08-review-workflows-and-attention-signals
plan: 04
subsystem: web-gui
tags: [htmx, templ, attention-signals, repo-settings, ignore-list, tailwind]

# Dependency graph
requires:
  - phase: 08-01
    provides: "RepoSettingsStore, IgnoreStore adapters, RepoSettings and IgnoredPR domain models"
  - phase: 08-03
    provides: "Composition root with all stores wired, credential GUI, write handler pattern"
provides:
  - "Per-repo settings form for review count and urgency thresholds"
  - "Threshold-based visual flags on PR cards (needs reviews, stale)"
  - "PR ignore/unignore with sidebar filtering and dedicated ignore list page"
  - "CountApprovals method on ReviewStore port and SQLite adapter"
affects: [composition-root, web-handlers, sidebar]

# Tech tracking
tech-stack:
  added: []
  patterns: [attention-signal-enrichment, ignored-pr-filtering, settings-cache-per-request]

key-files:
  created:
    - internal/adapter/driving/web/templates/components/repo_settings.templ
    - internal/adapter/driving/web/templates/components/ignore_button.templ
    - internal/adapter/driving/web/templates/pages/ignored_prs.templ
  modified:
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/viewmodel.go
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/templates/components/pr_card.templ
    - internal/adapter/driving/web/templates/components/pr_detail.templ
    - internal/adapter/driving/web/templates/components/repo_manager.templ
    - internal/adapter/driving/web/templates/components/sidebar.templ
    - internal/adapter/driven/sqlite/reviewrepo.go
    - internal/domain/port/driven/reviewstore.go
    - cmd/mygitpanel/main.go

key-decisions:
  - "Settings cache per request: map[string]*RepoSettings avoids N+1 queries when enriching PR cards"
  - "Ignored PR filtering at display layer: ignoreStore.ListIgnored once, build set, filter O(n+m)"
  - "CountApprovals added to ReviewStore port (lightweight query vs full review fetch for list view)"
  - "Repo settings gear icon uses hx-trigger='click once' to lazy-load form on first click"

patterns-established:
  - "Attention signal enrichment: enrichPRCardsWithAttentionSignals caches settings per repo, adds flags"
  - "Display-layer filtering: filterIgnoredPRs removes ignored PRs from all feed endpoints"
  - "Inline expandable settings: Alpine x-data per repo row with HTMX lazy-loaded form panel"

# Metrics
duration: 9min
completed: 2026-02-15
---

# Phase 8 Plan 4: Attention Signals and Ignore List Summary

**Per-repo review/urgency thresholds with visual PR card badges, and PR ignore list with sidebar filtering and dedicated restore page**

## Performance

- **Duration:** 9 min
- **Started:** 2026-02-16T04:33:50Z
- **Completed:** 2026-02-16T04:42:46Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments

- Per-repo settings form (required review count, urgency days) accessible via gear icon on each repo
- PR cards show "N/M approvals" amber badge and "Xd inactive" red badge based on configured thresholds
- Ignore button on PR detail, ignored PRs filtered from all feed endpoints, dedicated ignore list page with restore
- CountApprovals lightweight query on ReviewStore for efficient approval count in list view

## Task Commits

Each task was committed atomically:

1. **Task 1: Repo settings form and threshold-based visual flags** - `a707daa` (feat)
2. **Task 2: PR ignore list and dashboard filtering** - `5c8a463` (feat)

## Files Created/Modified

- `internal/adapter/driving/web/templates/components/repo_settings.templ` - Per-repo settings form with HTMX POST
- `internal/adapter/driving/web/templates/components/ignore_button.templ` - Ignore/restore toggle button
- `internal/adapter/driving/web/templates/pages/ignored_prs.templ` - Ignored PRs list page with restore buttons
- `internal/adapter/driving/web/handler.go` - GetRepoSettings, SaveRepoSettings, IgnorePR, UnignorePR, ListIgnoredPRs handlers
- `internal/adapter/driving/web/routes.go` - 5 new routes (settings GET/POST, ignore POST/DELETE, ignored list GET)
- `internal/adapter/driving/web/viewmodel.go` - IgnoreURL on PR detail, enrichPRCardsWithAttentionSignals
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Attention signal fields, RepoSettingsViewModel, IgnoredPRViewModel
- `internal/adapter/driving/web/templates/components/pr_card.templ` - Attention signal badges (needs reviews, stale)
- `internal/adapter/driving/web/templates/components/pr_detail.templ` - Ignore button in header
- `internal/adapter/driving/web/templates/components/repo_manager.templ` - Settings gear icon per repo
- `internal/adapter/driving/web/templates/components/sidebar.templ` - Ignored PRs count link
- `internal/adapter/driven/sqlite/reviewrepo.go` - CountApprovals method
- `internal/domain/port/driven/reviewstore.go` - CountApprovals on ReviewStore interface
- `cmd/mygitpanel/main.go` - Pass reviewStore to web handler

## Decisions Made

- Settings cache per request (map[string]*RepoSettings) to avoid N+1 queries when enriching PR card list
- Ignored PR filtering at display layer using set-based O(n+m) approach, not per-PR IsIgnored queries
- CountApprovals added as a dedicated lightweight query rather than fetching full review data for list view
- Repo settings gear icon lazy-loads form via hx-trigger="click once" to avoid unnecessary requests
- csrfToken return value removed (was never used, flagged by unparam linter)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] csrfToken return value unused**
- **Found during:** Task 2 (commit attempt)
- **Issue:** golangci-lint unparam flagged csrfToken's string return value as never used
- **Fix:** Changed csrfToken signature from `string` return to void, removed return statements
- **Files modified:** internal/adapter/driving/web/csrf.go
- **Verification:** golangci-lint passes with 0 issues
- **Committed in:** 5c8a463 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Pre-existing lint issue that surfaced during commit. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 8 complete: all 4 plans delivered
- Review workflows: credential GUI, review submission, comment reply, draft toggle
- Attention signals: per-repo thresholds, visual flags, ignore list
- Ready for Phase 9 (Jira integration or whatever comes next per ROADMAP)

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-15*
