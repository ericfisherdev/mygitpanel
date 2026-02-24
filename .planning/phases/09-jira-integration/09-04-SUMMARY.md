---
phase: 09-jira-integration
plan: 04
subsystem: ui, api
tags: [jira, templ, htmx, alpine, collapsible-card, comment-posting, jira-key-extraction]

# Dependency graph
requires:
  - phase: 09-jira-integration/01
    provides: JiraConnection domain model, JiraConnectionStore port, PRRepo jira_key persistence, ExtractJiraKey helper
  - phase: 09-jira-integration/02
    provides: JiraHTTPClient with GetIssue, AddComment, Ping
  - phase: 09-jira-integration/03
    provides: Handler.jiraConnStore and jiraClientFactory fields, Jira connection CRUD handlers
provides:
  - JiraCard templ component with four display states (no creds, no key, load error, issue loaded)
  - JiraCardViewModel, JiraIssueVM, JiraCommentVM view model structs
  - Server-side Jira issue fetch in GetPRDetail (non-fatal enrichment pattern)
  - CreateJiraComment handler with Ping-before-POST connectivity validation
  - PollService jira_key extraction on each poll cycle
  - POST /app/prs/{owner}/{repo}/{number}/jira-comment route
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [non-fatal Jira enrichment in GetPRDetail, friendlyJiraError sentinel-to-message mapping, Ping-before-POST connectivity validation]

key-files:
  created:
    - internal/adapter/driving/web/templates/components/jira_card.templ
  modified:
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/templates/components/pr_detail.templ
    - internal/application/pollservice.go

key-decisions:
  - "friendlyJiraError maps sentinel errors to user-facing messages consistently across buildJiraCardVM and CreateJiraComment"
  - "JiraCard injected after PRDetailHeader and before info section in pr_detail.templ"

patterns-established:
  - "Non-fatal Jira enrichment: buildJiraCardVM captures all errors in LoadError field, never prevents PR detail rendering"
  - "Connectivity validation: CreateJiraComment calls Ping before AddComment to fail fast on unreachable Jira"

# Metrics
duration: 4min
completed: 2026-02-24
---

# Phase 9 Plan 4: JiraCard Component and Handler Wiring Summary

**Collapsible JiraCard templ component with server-side issue fetch, HTMX comment posting with Ping validation, and PollService jira_key extraction per poll cycle**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-24T23:29:18Z
- **Completed:** 2026-02-24T23:33:16Z
- **Tasks:** 2
- **Files modified:** 6 (5 source + 1 new templ)

## Accomplishments

- JiraCard templ component renders four states: no credentials (link to settings), no linked issue (muted placeholder), load error (expandable with retry), issue loaded (collapsible with summary, metadata, comments, and comment form)
- GetPRDetail enriches PRDetailViewModel with Jira issue data server-side using non-fatal enrichment pattern (errors populate LoadError, never block PR rendering)
- CreateJiraComment handler validates connectivity via Ping before posting, re-renders JiraCard with updated comment list on success
- PollService extracts jira_key from branch name (priority) then title on each poll cycle, persisting it via prStore.Upsert

## Task Commits

Each task was committed atomically:

1. **Task 1: JiraCard view models, component, and PR detail integration** - `025ab36` (feat)
2. **Task 2: GetPRDetail Jira fetch, CreateJiraComment handler, PollService jira_key extraction** - `646ef44` (feat)

## Files Created/Modified

- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Added JiraCardViewModel, JiraIssueVM, JiraCommentVM structs; JiraCard field on PRDetailViewModel
- `internal/adapter/driving/web/templates/components/jira_card.templ` - JiraCard component with four states, Alpine collapsible, HTMX comment form
- `internal/adapter/driving/web/templates/components/pr_detail.templ` - Injected JiraCard after PRDetailHeader
- `internal/adapter/driving/web/handler.go` - buildJiraCardVM, friendlyJiraError, CreateJiraComment handler; Jira enrichment call in GetPRDetail
- `internal/adapter/driving/web/routes.go` - POST /app/prs/{owner}/{repo}/{number}/jira-comment route
- `internal/application/pollservice.go` - ExtractJiraKey call in pollRepo before prStore.Upsert

## Decisions Made

- friendlyJiraError maps all three sentinel errors (ErrJiraUnauthorized, ErrJiraNotFound, ErrJiraUnavailable) to user-friendly messages, shared by both buildJiraCardVM and CreateJiraComment
- JiraCard placed after PRDetailHeader and before the info section grid, matching the locked decision "Collapsible card at the top of the PR detail panel (above description/reviews)"

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 9 (Jira Integration) is now complete: all 4 plans delivered
- End-to-end flow: Jira connections configured in settings, per-repo mapping, jira_key auto-extracted on poll, issue fetched and displayed in PR detail, comments posted from dashboard

---
*Phase: 09-jira-integration*
*Completed: 2026-02-24*
