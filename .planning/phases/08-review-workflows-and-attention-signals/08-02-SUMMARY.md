---
phase: 08-review-workflows-and-attention-signals
plan: 02
subsystem: ui
tags: [alpine, htmx, templ, credentials, settings-drawer, pollservice, github-auth]

requires:
  - phase: 08-01
    provides: CredentialStore, ThresholdStore, IgnoreStore SQLite implementations and domain ports

provides:
  - Slide-in settings drawer (Alpine store + templ component) outside HTMX swap targets
  - SaveGitHubCredentials handler: ValidateToken before store, inline success/error fragments
  - SaveJiraCredentials handler: stores url/email/token without validation
  - PollService hot-swaps GitHub client each cycle via tokenProvider/clientFactory closures
  - GitHubWriter interface fully satisfied by Client (ValidateToken real; other 5 stubbed)
  - Gear icon in sidebar header opens drawer; backdrop click closes it
affects: [08-03, 08-04, 08-05]

tech-stack:
  added: []
  patterns:
    - "Closure injection for adapter hot-swap without import cycle (tokenProvider/clientFactory)"
    - "Always-in-DOM drawer rendered in layout outside swap targets to preserve Alpine state"
    - "HTMX hx-target inline fragment pattern for credential status feedback"
    - "Stub-then-implement pattern for GitHubWriter interface satisfaction across plans"

key-files:
  created:
    - internal/adapter/driven/github/writer.go
    - internal/adapter/driving/web/templates/components/settings_drawer.templ
  modified:
    - internal/application/pollservice.go
    - cmd/mygitpanel/main.go
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/templates/layout.templ
    - internal/adapter/driving/web/templates/components/sidebar.templ
    - internal/adapter/driving/web/static/js/stores.js

key-decisions:
  - "Closure pattern (tokenProvider + clientFactory funcs) for PollService hot-swap — avoids application-to-adapter import cycle while keeping application layer decoupled from concrete github adapter"
  - "GitHubWriter stubs in writer.go return fmt.Errorf('not yet implemented') — satisfies compile-time check unblocking handler wiring while Plans 03/04 add real implementations"
  - "SaveGitHubCredentials validates token before storing — prevents storing invalid tokens that would silently break polling"
  - "Drawer rendered in layout.templ outside @contents — Alpine state survives HTMX morph swaps across PR detail loads"

patterns-established:
  - "Alpine drawer store pattern: show(section)/hide() methods, always-in-DOM, x-show transitions"
  - "HTMX credential form pattern: hx-post to /app/settings/{service}, hx-target to inline status div, hx-indicator for spinner"

requirements-completed: [CRED-01, CRED-02]

duration: 7min
completed: 2026-02-19
---

# Phase 8 Plan 02: Settings Drawer and Credential Management Summary

**Slide-in settings drawer with GitHub token validation, Jira credential storage, and PollService hot-swap via closure injection — CRED-01 and CRED-02 fully delivered**

## Performance

- **Duration:** 7 min
- **Started:** 2026-02-19T20:42:04Z
- **Completed:** 2026-02-19T20:48:26Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- Settings drawer renders in layout (outside HTMX swap targets) with Alpine store-controlled open/close state that survives morph swaps
- GitHub token validated against GitHub API via ValidateToken before encryption and storage; inline success/error feedback in drawer
- Jira credential fields (URL, email, token) stored without validation — Phase 9 adds Jira-specific validation
- PollService reads GitHub token from CredentialStore each poll cycle via closure injection, falling back to startup env-var token
- GitHubWriter interface satisfied by Client (ValidateToken real; SubmitReview, CreateReplyComment, CreateIssueComment, ConvertToDraft, MarkReady stubbed for Plans 03/04)
- Gear icon added to sidebar header opens drawer to Credentials section

## Task Commits

Each task was committed atomically:

1. **Task 1: GitHubWriter ValidateToken, PollService hot-swap, composition root wiring** - `9d6fa3b` (feat)
2. **Task 2: Settings drawer UI — Alpine store, templ component, handlers, routes** - `7595bef` (feat)

**Plan metadata:** (committed with SUMMARY.md)

## Files Created/Modified
- `internal/adapter/driven/github/writer.go` - GitHubWriter implementation on Client; ValidateToken one-shot client; 5 stubs
- `internal/application/pollservice.go` - tokenProvider/clientFactory closures + maybeRefreshToken(); NewPollService new params
- `internal/application/pollservice_test.go` - Updated NewPollService callsites with nil,nil for optional params
- `cmd/mygitpanel/main.go` - credStore/thresholdStore/ignoreStore wired; tokenProvider/clientFactory closures; NewHandler extended
- `internal/adapter/driving/web/handler.go` - Added credStore/thresholdStore/ignoreStore/ghWriter fields; SaveGitHubCredentials/SaveJiraCredentials handlers
- `internal/adapter/driving/web/routes.go` - POST /app/settings/github and POST /app/settings/jira registered
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Added CredentialStatusViewModel
- `internal/adapter/driving/web/templates/layout.templ` - Added @components.SettingsDrawer() call before scripts
- `internal/adapter/driving/web/templates/components/settings_drawer.templ` - Full slide-in drawer with backdrop, tabs, credential forms
- `internal/adapter/driving/web/templates/components/sidebar.templ` - Gear icon button in header triggering drawer
- `internal/adapter/driving/web/static/js/stores.js` - Alpine.store('drawer') with show/hide methods

## Decisions Made
- Closure injection pattern (tokenProvider + clientFactory) for PollService avoids creating an application-to-adapter import cycle while keeping the hot-swap feature clean
- GitHubWriter stubs satisfy the compile-time interface check immediately, unblocking the web handler wiring; real implementations follow in Plans 03 and 04
- Token validated before storing to prevent storing invalid tokens that would silently break polling
- Drawer placed in layout outside @contents so Alpine state survives HTMX morph swaps on PR detail loads

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Handler struct fields updated in Task 1 commit**
- **Found during:** Task 1 (composition root wiring)
- **Issue:** main.go passed new stores to NewHandler, but Handler struct hadn't been updated yet — compiler would reject the call
- **Fix:** Updated Handler struct and NewHandler signature as part of Task 1 to keep the build passing between commits
- **Files modified:** internal/adapter/driving/web/handler.go
- **Verification:** go build ./... succeeds after Task 1 commit
- **Committed in:** 9d6fa3b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 2 - missing critical for build correctness)
**Impact on plan:** Necessary for compilability between task commits. No scope creep.

## Issues Encountered
- templ binary not in PATH; installed via `go install github.com/a-h/templ/cmd/templ@v0.3.977` — templ generate ran successfully after installation

## User Setup Required
None - no external service configuration required beyond existing MYGITPANEL_SECRET_KEY env var (optional, documented).

## Next Phase Readiness
- Settings drawer and credential management complete; Plans 03 and 04 can implement real SubmitReview, CreateReplyComment, CreateIssueComment, ConvertToDraft, and MarkReady methods on the GitHubWriter stubs
- credStore, thresholdStore, ignoreStore all wired in Handler ready for Plan 05 (threshold configuration UI)
- PollService hot-swap ready — configuring a token via GUI will take effect on the next poll cycle without restart

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-19*

## Self-Check: PASSED

All key files present. All commits verified. Build passes. Key patterns confirmed in generated files.
