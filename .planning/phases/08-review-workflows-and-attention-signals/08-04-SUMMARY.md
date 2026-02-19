---
phase: 08-review-workflows-and-attention-signals
plan: "04"
subsystem: ui
tags: [graphql, htmx, alpine, templ, draft-toggle, github-api]

requires:
  - phase: 08-02
    provides: GitHubWriter port, credStore, ConvertPullRequestToDraft/MarkPullRequestReadyForReview stubs

provides:
  - GraphQL draft mutation implementation (convertToDraftMutation, markReadyMutation) in graphql.go
  - executeDraftMutation private method on Client (follows FetchThreadResolution pattern)
  - Real ConvertPullRequestToDraft and MarkPullRequestReadyForReview replacing stubs in writer.go
  - IsOwnPR field on PRDetailViewModel computed from pr.Author == authenticatedUser
  - authenticatedUsername helper on Handler (checks credStore.github_username then falls back to config)
  - PRDetailHeader templ component extracted from PRDetail with id=pr-detail-header (morph target)
  - Draft toggle button: visible on own open PRs, Alpine loading state, SVG spinner, morph swap
  - ToggleDraftStatus handler: auth gate, author check, GraphQL call, optimistic draft flip
  - POST /app/prs/{owner}/{repo}/{number}/draft-toggle route

affects: [08-05, future UI development]

tech-stack:
  added: []
  patterns:
    - "executeDraftMutation follows same pattern as FetchThreadResolution: build request, POST, decode minimal response"
    - "Optimistic UI update: flip IsDraft in viewmodel immediately, fire background poll for eventual consistency"
    - "PRDetailHeader as named morph target: extracted sub-component wrapping id=pr-detail-header"
    - "authenticatedUsername: credential store takes precedence over static config username for write operations"

key-files:
  created: []
  modified:
    - internal/adapter/driven/github/graphql.go
    - internal/adapter/driven/github/writer.go
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/viewmodel.go
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/templates/components/pr_detail.templ
    - internal/adapter/driving/web/templates/components/pr_detail_templ.go

key-decisions:
  - "executeDraftMutation reuses graphqlRequest struct and http.DefaultClient from FetchThreadResolution — no new dependencies"
  - "Node ID fetched on-demand via REST PullRequests.Get at toggle time (not stored in DB)"
  - "Optimistic draft flip in ToggleDraftStatus: UI updates immediately while background poll catches DB up"
  - "PRDetailHeader extracted to named component so ToggleDraftStatus can render only the header as morph response"
  - "authenticatedUsername checks credStore first (dynamic GUI credentials) then falls back to static config username"
  - "toPRDetailViewModel signature extended with authenticatedUser param rather than adding it to Handler state"

patterns-established:
  - "Morph target extraction: create named sub-component with stable id for partial re-renders on write operations"
  - "Optimistic updates: flip in-memory state immediately, fire-and-forget background poll for DB consistency"

requirements-completed: [REV-04]

duration: 4min
completed: 2026-02-19
---

# Phase 8 Plan 04: Draft Status Toggle Summary

**GraphQL draft mutations via hand-rolled client with optimistic UI: toggle button on own PRs morphs the header badge in one click**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-19T21:02:02Z
- **Completed:** 2026-02-19T21:06:00Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- Replaced two "not yet implemented" stubs in `writer.go` with real GraphQL mutations that fetch the PR node ID on-demand via REST and execute the draft toggle
- Added `PRDetailHeader` templ component with `id="pr-detail-header"` as the morph swap target for ToggleDraftStatus responses
- Draft toggle button with Alpine.js loading state (spinner + disabled) visible only on the authenticated user's own open PRs

## Task Commits

Each task was committed atomically:

1. **Task 1: GraphQL draft mutations and writer.go implementation** - `e536363` (feat)
2. **Task 2: Draft toggle handler, button, and IsOwnPR viewmodel field** - `58e150d` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `internal/adapter/driven/github/graphql.go` - Added `convertToDraftMutation`, `markReadyMutation` constants and `executeDraftMutation` private method
- `internal/adapter/driven/github/writer.go` - Replaced both draft stubs; node_id fetched via `PullRequests.Get` then passed to `executeDraftMutation`
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Added `IsOwnPR bool` field to `PRDetailViewModel`
- `internal/adapter/driving/web/viewmodel.go` - Extended `toPRDetailViewModel` signature with `authenticatedUser string`, sets `IsOwnPR`
- `internal/adapter/driving/web/handler.go` - Added `authenticatedUsername` helper, updated three `toPRDetailViewModel` call sites, added `ToggleDraftStatus` handler
- `internal/adapter/driving/web/routes.go` - Registered `POST /app/prs/{owner}/{repo}/{number}/draft-toggle`
- `internal/adapter/driving/web/templates/components/pr_detail.templ` - Extracted `PRDetailHeader` component with draft toggle button
- `internal/adapter/driving/web/templates/components/pr_detail_templ.go` - Regenerated by templ

## Decisions Made

- Used `pullRequestId` as the GraphQL variable name (matches GitHub GraphQL API spec for `convertPullRequestToDraft` and `markPullRequestReadyForReview` input objects)
- Optimistic UI flip: `ToggleDraftStatus` flips `IsDraft` in the view model before rendering the morph response, so the badge updates immediately without waiting for the async background poll
- `authenticatedUsername` checks `credStore.Get("github_username")` first to support credentials set via the GUI settings drawer, then falls back to the static `h.username` from config
- `toPRDetailViewModel` signature extended with `authenticatedUser` rather than storing it on `Handler` — keeps view model construction pure and testable

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- REV-04 complete: draft toggle live for own open PRs
- Phase 8 Plan 05 (attention signal improvements) can proceed
- No remaining stubs in GitHubWriter interface

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-19*
