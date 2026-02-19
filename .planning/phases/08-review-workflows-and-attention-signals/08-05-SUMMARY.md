---
phase: 08-review-workflows-and-attention-signals
plan: 05
subsystem: ui
tags: [htmx, templ, alpine, attention-signals, threshold, ignore-list]

requires:
  - phase: 08-02
    provides: ThresholdStore and IgnoreStore SQLite adapters, domain model types for GlobalSettings/RepoThreshold/AttentionSignals
  - phase: 08-01
    provides: Web handler foundation, sidebar/PR list partials, settings drawer skeleton

provides:
  - ComputeAttentionSignals pure function in application layer
  - AttentionService with EffectiveThresholdsFor and SignalsForPR methods
  - PR card colored left border (severity 0-4) and attention signal icons
  - Hover-visible ignore button on PR cards; IgnorePR/UnignorePR handlers
  - Collapsible "Show ignored (N)" section in sidebar with Restore buttons
  - Global threshold settings form in settings drawer with OOB PR list refresh
  - Per-repo threshold override popover (gear icon next to each repo)
  - ListIgnoredWithPRData on PRRepo; ListAll/ListNeedingReview filter ignored PRs via LEFT JOIN
affects:
  - Phase 9 (Jira) — attention signals infrastructure available for future signal types

tech-stack:
  added: []
  patterns:
    - "toPRCardViewModelsWithSignals: non-fatal signal computation wraps AttentionService, falls back to zero-value"
    - "OOB PR list refresh pattern reused for ignore/unignore and threshold save handlers"
    - "WithAttentionService post-construction injection avoids circular dependency between Handler and AttentionService"
    - "PRRepo LEFT JOIN ignored_prs transparently filters ignored PRs from main feed without caller changes"

key-files:
  created:
    - internal/application/attentionservice.go
    - internal/application/attentionservice_test.go
    - internal/adapter/driving/web/templates/components/repo_threshold_popover.templ
  modified:
    - internal/adapter/driven/sqlite/prrepo.go
    - internal/domain/port/driven/prstore.go
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/viewmodel.go
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/templates/components/pr_card.templ
    - internal/adapter/driving/web/templates/components/settings_drawer.templ
    - internal/adapter/driving/web/templates/components/sidebar.templ
    - internal/adapter/driving/web/templates/partials/pr_list.templ
    - internal/adapter/driving/web/templates/partials/repo_list.templ
    - internal/adapter/driving/web/templates/layout.templ
    - cmd/mygitpanel/main.go

key-decisions:
  - "PRListOOB passes []model.PullRequest (raw PR data) for ignored section; sidebar passes []vm.PRCardViewModel — same ignore data, different shapes for different code paths"
  - "attentionBorderClass helper placed in pr_card.templ (component file) since it is exclusively a rendering concern for the card"
  - "Layout.templ updated to accept GlobalSettings parameter and pass to SettingsDrawer — only one layout call site (Dashboard handler)"
  - "ignoredSection in pr_list.templ takes model.PullRequest; sidebarIgnoredSection in sidebar.templ takes PRCardViewModel — no conversion needed in OOB path"

requirements-completed: [ATT-01, ATT-02, ATT-03, ATT-04]

duration: 21min
completed: 2026-02-19
---

# Phase 8 Plan 05: Attention Signals, Ignore List, and Threshold UI Summary

**Attention signal computation (pure function + service), PR card colored borders/icons, ignore/restore workflow, and global + per-repo threshold settings forms with OOB PR list refresh**

## Performance

- **Duration:** 21 min
- **Started:** 2026-02-19T21:09:02Z
- **Completed:** 2026-02-19T21:30:04Z
- **Tasks:** 2
- **Files modified:** 17 (3 created, 14 modified)

## Accomplishments

- Pure `ComputeAttentionSignals` function with 17 unit test cases covering NeedsMoreReviews, IsAgeUrgent, HasStaleReview, HasCIFailure, HasAny, and Severity
- PR cards show colored left border (transparent/orange-400/orange-500/red-500) and inline SVG icons for each active signal, only rendered when signals are active
- Ignore button appears on PR card hover; click calls POST /app/prs/{id}/ignore and returns OOB PR list refresh via morph swap
- Collapsible "Show ignored (N)" section at bottom of sidebar PR list with Restore buttons using unignore endpoint
- Settings drawer Thresholds tab replaced with real form (min approvals, age days, stale toggle, CI toggle) that saves and triggers OOB PR list refresh
- Per-repo threshold popover (gear icon) next to each repo name in repo list with Save/Reset-to-global actions
- PRRepo.ListAll and ListNeedingReview transparently exclude ignored PRs via LEFT JOIN ignored_prs

## Task Commits

Each task was committed atomically:

1. **Task 1: AttentionService, prrepo ignore filtering, threshold settings form** - `850e843` (feat)
2. **Task 2: PR card attention visuals and ignore list UI** - `645c5af` (feat)

**Plan metadata:** (docs commit — see state updates below)

## Files Created/Modified

- `internal/application/attentionservice.go` - ComputeAttentionSignals pure function + AttentionService struct
- `internal/application/attentionservice_test.go` - 17 unit test cases for pure function
- `internal/adapter/driven/sqlite/prrepo.go` - ListIgnoredWithPRData method; LEFT JOIN filter in ListAll/ListNeedingReview
- `internal/domain/port/driven/prstore.go` - Added ListIgnoredWithPRData to PRStore interface
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Added ID and Attention fields to PRCardViewModel; IgnoredPRs and GlobalSettings to DashboardViewModel
- `internal/adapter/driving/web/viewmodel.go` - toPRCardViewModel accepts model.AttentionSignals; toPRCardViewModels uses zero-value
- `internal/adapter/driving/web/handler.go` - attentionSvc field, WithAttentionService, new handlers (IgnorePR/UnignorePR/SaveGlobalThresholds/SaveRepoThreshold/DeleteRepoThreshold), renderPRListOOB, toPRCardViewModelsWithSignals, getGlobalSettings
- `internal/adapter/driving/web/routes.go` - 5 new routes for ignore and threshold endpoints
- `internal/adapter/driving/web/templates/components/pr_card.templ` - Colored border, attention icons, hover ignore button
- `internal/adapter/driving/web/templates/components/settings_drawer.templ` - Real threshold form replacing placeholder
- `internal/adapter/driving/web/templates/components/repo_threshold_popover.templ` - New: per-repo threshold override popover
- `internal/adapter/driving/web/templates/components/sidebar.templ` - sidebarIgnoredSection with PRCardViewModel slice
- `internal/adapter/driving/web/templates/partials/pr_list.templ` - PRList/PRListOOB now accept ignoredPRs; ignoredSection with model.PullRequest slice
- `internal/adapter/driving/web/templates/partials/repo_list.templ` - Updated to use RepoThresholdPopover per row
- `internal/adapter/driving/web/templates/layout.templ` - Accepts GlobalSettings parameter
- `cmd/mygitpanel/main.go` - Creates AttentionService and wires via WithAttentionService

## Decisions Made

- PRListOOB passes `[]model.PullRequest` for ignored section while sidebar uses `[]vm.PRCardViewModel` — the OOB path fetches directly from DB, no unnecessary conversion
- `attentionBorderClass` helper placed in pr_card.templ as it is an exclusive rendering concern of the card component
- Layout accepts GlobalSettings to allow the SettingsDrawer to pre-populate threshold inputs with current values (only one Layout call site)
- `WithAttentionService` post-construction injection on Handler avoids circular dependency since AttentionService needs ReviewStore (held by main.go, not Handler)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added ListIgnoredWithPRData to PRStore interface and all mock implementations**
- **Found during:** Task 1 (prrepo.go changes)
- **Issue:** Adding a method to PRRepo without adding it to the PRStore interface would break compile-time satisfaction check; test mocks also needed updating
- **Fix:** Added method to interface and all 3 mock implementations in test files (pollservice_test.go, healthservice_test.go, http/handler_test.go)
- **Files modified:** internal/domain/port/driven/prstore.go, internal/application/pollservice_test.go, internal/application/healthservice_test.go, internal/adapter/driving/http/handler_test.go
- **Verification:** go test ./... passes
- **Committed in:** 850e843 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 2 - missing interface implementation in test mocks)
**Impact on plan:** Required fix for compilation; no scope creep.

## Issues Encountered

None - plan executed cleanly with one expected interface propagation fix.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All Phase 8 success criteria met: CRED-01, CRED-02, REV-01-04, ATT-01-04
- Phase 9 (Jira integration) can build on the existing attention signal infrastructure
- AttentionService extensible with new signal types by adding fields to model.AttentionSignals

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-19*
