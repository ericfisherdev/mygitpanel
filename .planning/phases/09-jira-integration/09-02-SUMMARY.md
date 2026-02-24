---
phase: 09-jira-integration
plan: 02
subsystem: api
tags: [jira, rest-api-v3, adf, basic-auth, net-http, hexagonal]

# Dependency graph
requires:
  - phase: 09-jira-integration
    provides: JiraClient port interface, JiraIssue/JiraComment domain models, sentinel errors
provides:
  - JiraHTTPClient implementing driven.JiraClient via net/http
  - ADF (Atlassian Document Format) recursive text extraction
  - ADF plain-text-to-doc envelope for comment posting
  - Jira credential validation via Ping (GET /rest/api/3/myself)
affects: [09-03 settings-ui, 09-04 jiracard-handler]

# Tech tracking
tech-stack:
  added: []
  patterns: [net/http Jira adapter with manual JSON structs, recursive ADF text extraction via json.RawMessage, Jira timestamp fallback parsing]

key-files:
  created:
    - internal/adapter/driven/jira/client.go
    - internal/adapter/driven/jira/client_test.go
  modified: []

key-decisions:
  - "parseJiraTime fallback for Jira's non-standard timezone offset format (+0000 vs +00:00)"
  - "extractADFDocText separates top-level blocks with double newlines; extractADFText handles recursive node tree"

patterns-established:
  - "ADF text extraction: recursive json.RawMessage unmarshalling handles arbitrary nesting depth"
  - "Jira error mapping: HTTP status codes to domain sentinel errors via mapStatusCode helper"

# Metrics
duration: 3min
completed: 2026-02-24
---

# Phase 9 Plan 2: Jira HTTP Client Adapter Summary

**net/http-based Jira Cloud REST API v3 client with recursive ADF text extraction, Basic auth, and HTTP error-to-domain sentinel mapping**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-24T23:11:20Z
- **Completed:** 2026-02-24T23:14:14Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- JiraHTTPClient satisfies driven.JiraClient at compile time with GetIssue, AddComment, and Ping
- Recursive ADF text extraction handles paragraph, bulletList, orderedList, listItem, codeBlock, hardBreak node types
- All three sentinel errors (ErrJiraNotFound, ErrJiraUnauthorized, ErrJiraUnavailable) mapped from HTTP status codes
- 18 tests covering success paths, error codes, ADF extraction, and Basic auth header generation

## Task Commits

Each task was committed atomically:

1. **Task 1: JiraHTTPClient -- Basic auth, ADF handling, error mapping** - `3166880` (feat)

## Files Created/Modified

- `internal/adapter/driven/jira/client.go` - JiraHTTPClient with GetIssue, AddComment, Ping; ADF types and extraction; Basic auth; error mapping
- `internal/adapter/driven/jira/client_test.go` - 18 tests using httptest.NewServer for all methods, error codes, and ADF extraction

## Decisions Made

- Added parseJiraTime fallback parser for Jira's non-standard timezone format (+0000 without colon) alongside RFC3339
- Separated extractADFDocText (top-level doc with paragraph separation) from extractADFText (recursive node extraction) for clean separation of concerns

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added Jira timestamp fallback parser**
- **Found during:** Task 1 (client implementation)
- **Issue:** Jira Cloud returns timestamps with non-standard timezone offset format (e.g., "+0000" instead of RFC3339 "+00:00")
- **Fix:** Added parseJiraTime helper that tries RFC3339 first, then falls back to "2006-01-02T15:04:05.000-0700" format
- **Files modified:** internal/adapter/driven/jira/client.go
- **Verification:** TestGetIssue_Success passes with comment CreatedAt correctly parsed
- **Committed in:** 3166880 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Essential for correctly parsing real Jira API timestamps. No scope creep.

## Issues Encountered

- golangci-lint flagged "marshalling" as misspelling (American English prefers "marshaling") -- fixed before commit

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- JiraHTTPClient ready for Plan 03 (settings UI credential validation via Ping)
- JiraHTTPClient ready for Plan 04 (JiraCard handler wiring via GetIssue and AddComment)
- No new Go dependencies introduced (go.mod unchanged)

---
*Phase: 09-jira-integration*
*Completed: 2026-02-24*
