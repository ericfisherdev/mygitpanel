---
phase: 07-gui-foundation
plan: 02
subsystem: ui
tags: [templ, htmx, alpine.js, tailwindcss, viewmodel, sidebar, pr-detail]

# Dependency graph
requires:
  - phase: 07-gui-foundation
    provides: "Web GUI scaffold with templ layout, vendored JS, dual driving adapter"
  - phase: 04-review-intelligence
    provides: "ReviewService with PR review summary, threading, suggestions"
  - phase: 05-pr-health-signals
    provides: "HealthService with check runs and CI status"
provides:
  - "PR feed sidebar with clickable cards showing CI/review/merge status"
  - "PR detail panel with tabbed reviews, threads, comments, and CI checks"
  - "Collapsible sidebar with Alpine.js state management"
  - "HTMX partial routes for PR list and PR detail content swaps"
  - "View model package decoupling templ components from domain models"
affects: [07-03, 08-github-write-ops]

# Tech tracking
tech-stack:
  added: []
  patterns: [viewmodel package for templ/domain decoupling, HTMX partial endpoints, Alpine.js tabbed UI]

key-files:
  created:
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/templates/components/pr_card.templ
    - internal/adapter/driving/web/templates/components/pr_detail.templ
    - internal/adapter/driving/web/templates/components/sidebar.templ
    - internal/adapter/driving/web/templates/partials/pr_list.templ
    - internal/adapter/driving/web/templates/partials/pr_detail_content.templ
  modified:
    - internal/adapter/driving/web/viewmodel.go
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/templates/pages/dashboard.templ

key-decisions:
  - "Extracted viewmodel package to break import cycle between web handler and templ components"
  - "Non-fatal enrichment pattern: review and health data failures don't block PR detail rendering"

patterns-established:
  - "Viewmodel package: templ components import viewmodel, web handler imports viewmodel — no import cycle"
  - "HTMX partials: /app/prs/{owner}/{repo}/{number} returns HTML fragment swapped into #pr-detail"
  - "Alpine.js tabs: x-data on container, @click toggles tab state, x-show controls visibility"
  - "Sidebar collapse: Alpine x-data with collapsed boolean, x-bind:class for width transition"

# Metrics
duration: 7min
completed: 2026-02-14
---

# Phase 7 Plan 02: PR Feed Sidebar and Detail Panel Summary

**Clickable PR feed sidebar with CI/review/merge badges and tabbed PR detail panel with reviews, threads, comments, and CI checks via HTMX partial swaps**

## Performance

- **Duration:** 7 min
- **Started:** 2026-02-15T03:52:18Z
- **Completed:** 2026-02-15T03:59:41Z
- **Tasks:** 2
- **Files modified:** 16

## Accomplishments

- Built viewmodel layer decoupling templ components from domain models via separate package
- Created PR card component with CI status dots, review/draft/conflict badges, and HTMX click-to-load
- Built collapsible sidebar with Alpine.js collapse toggle and smooth width transition
- Implemented PR detail panel with 4 tabbed sections (reviews, threads, comments, CI) using Alpine.js
- Added GetPRDetail handler with non-fatal review and health enrichment (mirrors JSON API pattern)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create view models and domain-to-viewmodel conversion functions** - `5abe763` (feat)
2. **Task 2: Create PR card, PR detail, sidebar templ components and wire handler routes** - `15255b8` (feat)

## Files Created/Modified

- `internal/adapter/driving/web/viewmodel/viewmodel.go` - View model struct definitions (PRCardViewModel, PRDetailViewModel, ReviewViewModel, etc.)
- `internal/adapter/driving/web/viewmodel.go` - Conversion functions from domain models to view models
- `internal/adapter/driving/web/templates/components/pr_card.templ` - Clickable PR card with status badges and HTMX hx-get
- `internal/adapter/driving/web/templates/components/pr_detail.templ` - Full PR detail with tabbed reviews/threads/comments/CI
- `internal/adapter/driving/web/templates/components/sidebar.templ` - Collapsible sidebar with Alpine.js x-data toggle
- `internal/adapter/driving/web/templates/partials/pr_list.templ` - PR list partial for HTMX swap
- `internal/adapter/driving/web/templates/partials/pr_detail_content.templ` - PR detail partial for HTMX swap
- `internal/adapter/driving/web/templates/pages/dashboard.templ` - Updated to compose sidebar + detail area
- `internal/adapter/driving/web/handler.go` - Dashboard fetches PRs; GetPRDetail with enrichment
- `internal/adapter/driving/web/routes.go` - Added GET /app/prs/{owner}/{repo}/{number} route

## Decisions Made

- Extracted view model structs into `internal/adapter/driving/web/viewmodel/` package to break import cycle (templ components in sub-packages cannot import parent `web` package without cycle)
- Kept conversion functions in `web` package since they need access to both `viewmodel` and `domain/model` packages
- Followed non-fatal enrichment pattern from HTTP handler: review and health failures log errors but still render basic PR data

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Extracted viewmodel package to break import cycle**
- **Found during:** Task 2 (templ generate + go build)
- **Issue:** Plan specified view models in `web` package, but templ components in sub-packages (pages, components, partials) import the parent `web` package for view model types, creating `web -> pages -> web` import cycle
- **Fix:** Extracted view model struct definitions to `internal/adapter/driving/web/viewmodel/` package; conversion functions stay in `web` package
- **Files modified:** internal/adapter/driving/web/viewmodel/viewmodel.go (new), internal/adapter/driving/web/viewmodel.go (conversion functions only)
- **Verification:** `go build ./...` succeeds with no import cycles
- **Committed in:** 5abe763 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Package extraction necessary to resolve Go import cycle. Clean separation — structs in viewmodel package, conversion functions in web package. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Dashboard renders PR list and detail with full review/CI data
- Ready for Plan 03 (dark mode toggle, settings panel, action buttons)
- All HTMX swap targets in place (#pr-list, #pr-detail)
- Alpine.js stores initialized (theme store from Plan 01, tab/collapse state from Plan 02)

---
*Phase: 07-gui-foundation*
*Completed: 2026-02-14*
