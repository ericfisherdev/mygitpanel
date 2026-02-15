---
phase: 07-gui-foundation
plan: 03
subsystem: ui
tags: [htmx, alpine-js, gsap, templ, tailwind, search, filtering, theme-toggle, repo-management]

requires:
  - phase: 07-gui-foundation/07-02
    provides: "PR feed sidebar with cards, detail panel with tabs, HTMX morph swaps"
provides:
  - "Search bar with debounced text search and status/repo filter dropdowns"
  - "In-memory PR filtering by text, status, and repo"
  - "Dark/light theme toggle with Alpine.js persist localStorage"
  - "Repo management GUI (add/remove) with OOB swap pattern"
  - "GSAP animations on PR detail and list swaps"
affects: [08-github-write-ops, 09-jira-integration]

tech-stack:
  added: []
  patterns:
    - "OOB swap pattern: primary target + hx-swap-oob for secondary updates"
    - "In-memory filtering for small datasets instead of new DB queries"
    - "DashboardViewModel aggregating cards, repos, and repo names"
    - "htmx:afterSettle for GSAP animations with morph swaps"

key-files:
  created:
    - "internal/adapter/driving/web/templates/components/search_bar.templ"
    - "internal/adapter/driving/web/templates/components/theme_toggle.templ"
    - "internal/adapter/driving/web/templates/components/repo_manager.templ"
    - "internal/adapter/driving/web/templates/partials/repo_list.templ"
  modified:
    - "internal/adapter/driving/web/handler.go"
    - "internal/adapter/driving/web/routes.go"
    - "internal/adapter/driving/web/viewmodel/viewmodel.go"
    - "internal/adapter/driving/web/templates/components/sidebar.templ"
    - "internal/adapter/driving/web/templates/pages/dashboard.templ"
    - "internal/adapter/driving/web/templates/partials/pr_list.templ"
    - "internal/adapter/driving/web/static/js/animations.js"

key-decisions:
  - "Duplicated isValidRepoName in web handler rather than extracting shared package for a 10-line function"
  - "In-memory PR filtering appropriate for expected scale (dozens to low hundreds of PRs)"
  - "Used htmx:afterSettle instead of htmx:afterSwap for GSAP animations — morph swaps settle after DOM morphing"
  - "RepoManager uses collapsible section to keep sidebar uncluttered"

patterns-established:
  - "OOB swap: repo mutations render primary target + PRListOOB + RepoFilterOptions OOB divs"
  - "DashboardViewModel: aggregate view model passed through dashboard -> sidebar -> child components"
  - "Form-based HTMX: POST with ParseForm for GUI mutations vs JSON for API endpoints"

duration: 4min
completed: 2026-02-14
---

# Phase 7 Plan 3: Interactive Features Summary

**Search/filter with debounced HTMX, dark/light theme toggle via Alpine.js persist, repo management with OOB swaps, and GSAP morph-swap animations**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-15T04:02:19Z
- **Completed:** 2026-02-15T04:06:31Z
- **Tasks:** 2
- **Files modified:** 17

## Accomplishments

- Search bar with 500ms debounced text input, status dropdown (all/open/closed/merged), and repo dropdown filters -- all combinations work via hx-include cross-referencing
- Dark/light theme toggle using Alpine.js $store.theme.dark with persist plugin, wired into layout.templ's existing store
- Repo management GUI: add form with owner/repo validation, remove buttons with hx-confirm, OOB swaps updating PR list and repo filter dropdown simultaneously
- GSAP animations switched to htmx:afterSettle for morph swap compatibility

## Task Commits

Each task was committed atomically:

1. **Task 1: Search bar, filters, search handler, repo management** - `7d79cfa` (feat)
2. **Task 2: GSAP animation refinement for morph swaps** - `1d66d55` (feat)

## Files Created/Modified

- `internal/adapter/driving/web/templates/components/search_bar.templ` - Debounced search input with status/repo filter dropdowns
- `internal/adapter/driving/web/templates/components/theme_toggle.templ` - Dark/light toggle button with Alpine.js persist
- `internal/adapter/driving/web/templates/components/repo_manager.templ` - Collapsible add/remove repo section
- `internal/adapter/driving/web/templates/partials/repo_list.templ` - Repo list partial for OOB swaps
- `internal/adapter/driving/web/templates/partials/pr_list.templ` - Added PRListOOB for out-of-band updates
- `internal/adapter/driving/web/templates/components/sidebar.templ` - Restructured for DashboardViewModel with search, theme toggle, repo manager
- `internal/adapter/driving/web/templates/pages/dashboard.templ` - Takes DashboardViewModel instead of cards slice
- `internal/adapter/driving/web/handler.go` - SearchPRs, AddRepo, RemoveRepo handlers with filterPRs and OOB rendering
- `internal/adapter/driving/web/routes.go` - GET /app/prs/search, POST /app/repos, DELETE /app/repos/{owner}/{repo}
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - RepoViewModel, RepoFilterViewModel, DashboardViewModel
- `internal/adapter/driving/web/static/js/animations.js` - Switched to htmx:afterSettle, extracted animateSwapTarget

## Decisions Made

- Duplicated isValidRepoName validation in web handler (10-line function not worth a shared package)
- Used in-memory filtering for search -- appropriate for expected PR volume, avoids new DB query methods
- Used htmx:afterSettle for GSAP animations since morph swaps settle after DOM morphing completes
- RepoManager uses Alpine.js collapsible section to keep the sidebar compact

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created theme toggle and repo manager in Task 1**
- **Found during:** Task 1 (sidebar restructuring)
- **Issue:** Sidebar references ThemeToggle and RepoManager components that were planned for Task 2
- **Fix:** Created both components in Task 1 to allow templ generate and build to succeed
- **Files modified:** theme_toggle.templ, repo_manager.templ
- **Verification:** templ generate && go build ./... succeeds
- **Committed in:** 7d79cfa (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Task 2 scope reduced since components were created in Task 1. Task 2 focused on animation refinement. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 7 GUI Foundation complete: all 6 GUI requirements delivered (GUI-01 through GUI-07)
- Dashboard serves at / with full PR browsing, search/filter, theme toggle, and repo management
- Ready for Phase 8 (GitHub Write Operations) which adds PR actions through the GUI

---
*Phase: 07-gui-foundation*
*Completed: 2026-02-14*
