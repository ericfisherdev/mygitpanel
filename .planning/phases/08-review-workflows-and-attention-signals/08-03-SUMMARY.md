---
phase: 08-review-workflows-and-attention-signals
plan: 03
subsystem: api
tags: [htmx, templ, credential-management, review-workflow, draft-toggle, csrf, hot-swap]

# Dependency graph
requires:
  - phase: 08-01
    provides: "CredentialStore, RepoSettingsStore, IgnoreStore adapters and NodeID on PullRequest"
  - phase: 08-02
    provides: "GitHubClient write methods, GitHubClientProvider, optional config credentials"
provides:
  - "Composition root wired with GitHubClientProvider and all new stores"
  - "Credential GUI form with SQLite persistence and hot-swap"
  - "PollService using provider.Get() per cycle (nil-safe)"
  - "Review submission form (approve/request-changes/comment)"
  - "Inline comment reply targeting root comment ID"
  - "Draft toggle button using GraphQL mutation with NodeID"
  - "4 new POST write endpoints for PR interactions"
affects: [08-04, composition-root, web-handlers]

# Tech tracking
tech-stack:
  added: []
  patterns: [credential-hot-swap-gui, write-handler-pattern, pr-detail-refresh-after-write]

key-files:
  created:
    - internal/adapter/driving/web/templates/components/credential_form.templ
    - internal/adapter/driving/web/templates/components/credential_status.templ
    - internal/adapter/driving/web/templates/components/review_form.templ
    - internal/adapter/driving/web/templates/components/comment_reply.templ
    - internal/adapter/driving/web/templates/components/draft_toggle.templ
  modified:
    - cmd/mygitpanel/main.go
    - internal/adapter/driven/github/client.go
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/viewmodel.go
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/templates/components/pr_detail.templ
    - internal/adapter/driving/web/templates/components/sidebar.templ
    - internal/application/pollservice.go
    - internal/application/pollservice_test.go

key-decisions:
  - "CredentialStatus component in components/ (not partials/) to avoid import cycle with sidebar"
  - "Write handlers use requireGitHubClient helper returning 403 if provider has no client"
  - "renderPRDetailRefresh shared method for re-rendering after all write operations"
  - "ThreadCardWithReply variant passes PR context for reply form URLs"
  - "toPRDetailViewModelWithWriteCaps replaces toPRDetailViewModel with credential-aware flags"

patterns-established:
  - "Write handler pattern: ParseForm, validateCSRF, requireGitHubClient, execute, renderPRDetailRefresh"
  - "Credential hot-swap GUI: form POST saves to SQLite + calls provider.Replace(newClient)"
  - "PR detail refresh after write: shared method enriches with review+health data before re-render"

# Metrics
duration: 11min
completed: 2026-02-15
---

# Phase 8 Plan 3: End-to-End Wiring Summary

**Credential GUI with hot-swap, review submission form, comment reply, and draft toggle -- all wired through composition root to GitHub API via provider**

## Performance

- **Duration:** 11 min
- **Started:** 2026-02-16T04:19:39Z
- **Completed:** 2026-02-16T04:31:11Z
- **Tasks:** 2
- **Files modified:** 16

## Accomplishments

- Rewired composition root with GitHubClientProvider, credential resolution (stored > env vars), and all new stores
- PollService now uses provider.Get() per cycle -- safely skips polling when no credentials configured
- Credential form saves to SQLite and hot-swaps GitHub client without restart
- Review submission form with 3 event types (approve, request changes, comment) on PR detail
- Inline comment reply forms on thread root comments targeting correct root ID
- Draft toggle button with Mark as Ready / Convert to Draft labels using GraphQL NodeID
- NodeID mapped from go-github PullRequest in adapter

## Task Commits

Each task was committed atomically:

1. **Task 1: Composition root rewire and credential management** - `036ddf3` (feat)
2. **Task 2: Review submission, comment reply, and draft toggle UI** - `a89ddaa` (feat)

## Files Created/Modified

- `cmd/mygitpanel/main.go` - Rewired with GitHubClientProvider, credential stores, credential resolution
- `internal/adapter/driven/github/client.go` - Added NodeID mapping in mapPullRequest
- `internal/adapter/driving/web/handler.go` - Added credential, review, comment, reply, draft toggle handlers
- `internal/adapter/driving/web/routes.go` - Registered 6 new routes (2 credential, 4 PR write)
- `internal/adapter/driving/web/viewmodel.go` - Added write-capable PR detail VM builder
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Added CredentialStatusViewModel, write fields on PRDetailViewModel
- `internal/adapter/driving/web/templates/components/credential_form.templ` - HTMX credential input form
- `internal/adapter/driving/web/templates/components/credential_status.templ` - Status indicator with edit button
- `internal/adapter/driving/web/templates/components/review_form.templ` - Review submission with radio buttons
- `internal/adapter/driving/web/templates/components/comment_reply.templ` - Inline reply form for threads
- `internal/adapter/driving/web/templates/components/draft_toggle.templ` - Draft/ready toggle button
- `internal/adapter/driving/web/templates/components/pr_detail.templ` - Integrated review form, reply buttons, draft toggle
- `internal/adapter/driving/web/templates/components/sidebar.templ` - Added credential status indicator
- `internal/application/pollservice.go` - Uses GitHubClientProvider, nil-safe polling
- `internal/application/pollservice_test.go` - Updated for provider-based constructor

## Decisions Made

- CredentialStatus component placed in components/ (not partials/) to avoid import cycle between components/sidebar and partials/pr_detail_content
- Write handlers share a requireGitHubClient helper that returns 403 "GitHub credentials not configured" when provider has no client
- renderPRDetailRefresh is a shared method used by all 4 write handlers to re-render the PR detail after mutations
- ThreadCardWithReply is a new variant of ThreadCard that receives PR context for generating reply form action URLs
- Removed original toPRDetailViewModel in favor of toPRDetailViewModelWithWriteCaps which always computes write capability flags

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Import cycle between components and partials packages**
- **Found during:** Task 1 (credential status)
- **Issue:** Placing CredentialStatus in partials/ created an import cycle: components/sidebar imports partials, partials/pr_detail_content imports components
- **Fix:** Moved CredentialStatus to components/ package where sidebar already lives
- **Files modified:** Created components/credential_status.templ instead of partials/credential_status.templ
- **Verification:** templ generate and go build succeed with no import cycles
- **Committed in:** 036ddf3 (Task 1 commit)

**2. [Rule 1 - Bug] Unused function flagged by golangci-lint**
- **Found during:** Task 2 (commit attempt)
- **Issue:** Original toPRDetailViewModel became unused after introducing toPRDetailViewModelWithWriteCaps
- **Fix:** Removed the unused function, consolidated into single toPRDetailViewModelWithWriteCaps
- **Files modified:** internal/adapter/driving/web/viewmodel.go
- **Verification:** golangci-lint passes with 0 issues
- **Committed in:** a89ddaa (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both auto-fixes necessary for compilation/linting. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Credentials can now be provided via the GUI.

## Next Phase Readiness

- All write operations wired end-to-end: credential form, review submission, comment reply, draft toggle
- PR detail view shows interactive forms when credentials are configured
- App starts cleanly with or without MYGITPANEL_GITHUB_TOKEN env var
- Ready for Plan 04 (attention signals, repo settings, ignore list)

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-15*
