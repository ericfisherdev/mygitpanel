---
phase: 09-jira-integration
plan: 03
subsystem: ui, api
tags: [jira, htmx, alpine, templ, settings-drawer, multi-connection, credential-management]

# Dependency graph
requires:
  - phase: 09-jira-integration/01
    provides: JiraConnectionStore port, JiraClient port, JiraConnection domain model, JiraConnectionRepo SQLite adapter
  - phase: 09-jira-integration/02
    provides: JiraHTTPClient adapter with Ping/GetIssue/AddComment
provides:
  - Multi-connection Jira UI in settings drawer (add/delete/set-default)
  - Per-repo Jira connection assignment in repo popover
  - CreateJiraConnection, DeleteJiraConnection, SetDefaultJiraConnection, SaveJiraRepoMapping HTTP handlers
  - Handler.jiraConnStore and jiraClientFactory fields wired end-to-end
  - JiraConnectionList templ component (HTMX swap target)
  - JiraConnectionViewModel and JiraConnectionStatusViewModel view models
affects: [09-04 jiracard-handler]

# Tech tracking
tech-stack:
  added: []
  patterns: [jiraConnectionByID shared handler for ID-based store operations, jiraConnections threaded through Layout/Sidebar/RepoManager/RepoList/RepoThresholdPopover]

key-files:
  created: []
  modified:
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/templates/components/settings_drawer.templ
    - internal/adapter/driving/web/templates/components/repo_threshold_popover.templ
    - internal/adapter/driving/web/templates/components/repo_manager.templ
    - internal/adapter/driving/web/templates/components/sidebar.templ
    - internal/adapter/driving/web/templates/layout.templ
    - internal/adapter/driving/web/templates/partials/repo_list.templ
    - cmd/mygitpanel/main.go

key-decisions:
  - "Extracted jiraConnectionByID helper to deduplicate Delete and SetDefault handlers (lint duplication rule)"
  - "Converted toRepoViewModels to Handler method to access jiraConnStore for per-repo AssignedJiraConnectionID"
  - "Updated RepoManager to use RepoThresholdPopover for consistent rendering (initial render now matches OOB swap)"
  - "JiraConnections threaded through Layout signature to reach SettingsDrawer without global state"

patterns-established:
  - "jiraConnectionByID: shared handler pattern for parse-ID + CSRF + store-op + render-list"
  - "Alpine x-data collapsible add form with @htmx:after-request reset on success"

# Metrics
duration: 9min
completed: 2026-02-24
---

# Phase 9 Plan 3: Settings UI and Jira Connection Management Summary

**Multi-connection Jira credential management UI replacing Phase 8 stub: CRUD handlers with Ping validation, settings drawer connection list, and per-repo Jira assignment in repo popover**

## Performance

- **Duration:** 9 min
- **Started:** 2026-02-24T23:16:37Z
- **Completed:** 2026-02-24T23:26:02Z
- **Tasks:** 2
- **Files modified:** 19 (10 source + 9 generated _templ.go)

## Accomplishments

- Four new HTTP handlers (CreateJiraConnection with live Ping validation, DeleteJiraConnection, SetDefaultJiraConnection, SaveJiraRepoMapping) wired end-to-end
- Phase 8 single-connection Jira stub completely replaced with multi-connection UI: connection list with default indicator, delete buttons, and collapsible add form
- Per-repo Jira connection assignment dropdown added to repo threshold popover with correct selected state

## Task Commits

Each task was committed atomically:

1. **Task 1: Handler fields, Jira connection CRUD handlers, and routes** - `b3adc26` (feat)
2. **Task 2: Settings drawer Jira UI and repo popover Jira assignment** - `a7ff05e` (feat)

## Files Created/Modified

- `internal/adapter/driving/web/handler.go` - Added jiraConnStore/jiraClientFactory fields, 4 CRUD handlers, jiraConnectionByID helper, renderJiraConnectionList, toJiraConnectionViewModels; replaced SaveJiraCredentials with 410 stub; converted buildDashboardViewModel/toRepoViewModels to methods
- `internal/adapter/driving/web/routes.go` - Replaced POST /app/settings/jira with 4 Jira connection routes
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Added JiraConnectionViewModel, JiraConnectionStatusViewModel, JiraConnections on DashboardViewModel, AssignedJiraConnectionID on RepoViewModel
- `internal/adapter/driving/web/templates/components/settings_drawer.templ` - Replaced stub Jira form with multi-connection UI; added JiraConnectionList component
- `internal/adapter/driving/web/templates/components/repo_threshold_popover.templ` - Added Jira connection assignment select dropdown
- `internal/adapter/driving/web/templates/components/repo_manager.templ` - Updated to use RepoThresholdPopover and accept jiraConnections
- `internal/adapter/driving/web/templates/components/sidebar.templ` - Pass jiraConnections to RepoManager
- `internal/adapter/driving/web/templates/layout.templ` - Accept and pass jiraConnections to SettingsDrawer
- `internal/adapter/driving/web/templates/partials/repo_list.templ` - Accept and pass jiraConnections to RepoThresholdPopover
- `cmd/mygitpanel/main.go` - Wire JiraConnectionRepo and jiraClientFactory into NewHandler

## Decisions Made

- Extracted `jiraConnectionByID` shared helper to deduplicate Delete and SetDefault handlers (golangci-lint dupl rule triggered)
- Converted `toRepoViewModels` from standalone function to Handler method so it can call `jiraConnStore.GetForRepo` per repo
- Updated `RepoManager` to render repos via `RepoThresholdPopover` instead of inline rows, giving consistent initial render and OOB swap behavior
- Threaded `jiraConnections` through the full template chain (Layout -> SettingsDrawer, Sidebar -> RepoManager -> RepoThresholdPopover) rather than introducing global state

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] golangci-lint duplication error for Delete/SetDefault handlers**
- **Found during:** Task 1 (handler CRUD methods)
- **Issue:** DeleteJiraConnection and SetDefaultJiraConnection had identical structure, triggering dupl lint
- **Fix:** Extracted jiraConnectionByID helper accepting a store function parameter
- **Files modified:** internal/adapter/driving/web/handler.go
- **Verification:** golangci-lint passes with 0 issues
- **Committed in:** b3adc26 (Task 1 commit)

**2. [Rule 3 - Blocking] JiraConnectionList templ component needed for handler compilation**
- **Found during:** Task 1 (renderJiraConnectionList calls components.JiraConnectionList)
- **Issue:** Plan placed JiraConnectionList creation in Task 2 but handler in Task 1 references it
- **Fix:** Created JiraConnectionList component in Task 1 alongside handler code
- **Files modified:** internal/adapter/driving/web/templates/components/settings_drawer.templ
- **Committed in:** b3adc26 (Task 1 commit)

**3. [Rule 2 - Missing Critical] RepoManager inconsistent rendering**
- **Found during:** Task 2 (repo popover Jira assignment)
- **Issue:** RepoManager rendered inline repo rows without popovers; OOB swap rendered via RepoThresholdPopover
- **Fix:** Updated RepoManager to use RepoThresholdPopover, matching OOB swap rendering
- **Files modified:** internal/adapter/driving/web/templates/components/repo_manager.templ
- **Committed in:** a7ff05e (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (1 bug, 1 blocking, 1 missing critical)
**Impact on plan:** All auto-fixes necessary for compilation and rendering consistency. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All Jira connection CRUD and per-repo mapping handlers are live
- `JiraConnectionStore.GetForRepo` resolves which connection to use per PR (ready for Plan 04 JiraCard handler)
- Settings drawer and repo popover fully functional for Jira connection management
- Plan 04 (JiraCard handler) can use `jiraClientFactory` with resolved connection to fetch Jira issues

---
*Phase: 09-jira-integration*
*Completed: 2026-02-24*
