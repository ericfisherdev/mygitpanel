---
phase: 08-review-workflows-and-attention-signals
plan: 03
subsystem: ui
tags: [htmx, alpinejs, templ, github-api, review-workflow, morph-swap]

# Dependency graph
requires:
  - phase: 08-review-workflows-and-attention-signals/08-02
    provides: GitHubWriter interface with stubs, credStore for token lookup, ghWriter injected into handler
  - phase: 08-review-workflows-and-attention-signals/08-01
    provides: ThreadViewModel, ReviewCommentViewModel with FilePath and CommitID fields
provides:
  - ReviewThread templ component with Alpine collapse/expand inline reply box
  - PRReviewsSection templ component with threads, issue comments, and staged review submit form
  - SubmitReview, CreateReplyComment, CreateIssueComment fully implemented in GitHub adapter
  - Three new POST handlers with auth check and morph swap re-render
  - Three new POST routes registered in routes.go
affects: [08-04, 08-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Alpine inline state (x-data/x-model/x-show) for collapse/expand per-thread reply boxes
    - HTMX morph swap targeting element IDs after write operations (reply -> #thread-{id}, review -> #pr-reviews-section)
    - Alpine pending comment accumulation (pendingComments[]) with @submit.prevent JSON serialization
    - auth check in write handlers via credStore.Get before calling ghWriter
    - re-fetch-then-re-render pattern for morph swap responses

key-files:
  created:
    - internal/adapter/driving/web/templates/components/review_thread.templ
    - internal/adapter/driving/web/templates/components/pr_reviews_section.templ
  modified:
    - internal/adapter/driven/github/writer.go
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/viewmodel.go
    - internal/adapter/driving/web/viewmodel/viewmodel.go
    - internal/adapter/driving/web/templates/components/pr_detail.templ

key-decisions:
  - "PRReviewsSection placed in components package (not partials) to avoid components<->partials import cycle"
  - "Owner and RepoName added to PRDetailViewModel for URL construction in templates (no string splitting in templ)"
  - "ReviewThread component receives owner/repo/prNumber as separate args to keep view model clean"
  - "CreateReplyComment renders only the affected thread on success (morph #thread-{rootID}); SubmitReview and CreateIssueComment render the full PRReviewsSection (morph #pr-reviews-section)"
  - "Missing GitHub token returns 422 with actionable HTML fragment, not 500"
  - "SubmitReview re-fetches PR head SHA if CommitID is empty; 422 from GitHub surfaced as user-friendly stale-review message"

patterns-established:
  - "Write handler pattern: auth check -> call ghWriter -> re-fetch -> re-render for morph swap"
  - "422 responses from GitHub API mapped to user-friendly error messages before returning HTML fragment"

requirements-completed: [REV-01, REV-02, REV-03]

# Metrics
duration: 7min
completed: 2026-02-19
---

# Phase 8 Plan 03: Review Workflows Summary

**Threaded review display with Alpine inline reply boxes, HTMX morph-swap write handlers, and fully implemented GitHub API write operations (SubmitReview, CreateReplyComment, CreateIssueComment)**

## Performance

- **Duration:** 7 min
- **Started:** 2026-02-19T20:51:42Z
- **Completed:** 2026-02-19T20:58:43Z
- **Tasks:** 2
- **Files modified:** 9 (2 created, 7 modified)

## Accomplishments

- Replaced stub implementations in writer.go with real GitHub API calls for SubmitReview, CreateReplyComment, and CreateIssueComment
- Created ReviewThread component with Alpine-managed inline reply box (collapses/expands per thread, hx-post on submit, morph swap)
- Created PRReviewsSection component combining threads + issue comments + staged review form with event selector (APPROVE/REQUEST_CHANGES/COMMENT)
- Added three POST handlers with auth gate (credStore token check) and morph-swap re-render on success
- Updated PRDetail threads tab to use the new interactive PRReviewsSection instead of static ThreadCard

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement SubmitReview, CreateReplyComment, CreateIssueComment in GitHub adapter** - `4e9f530` (feat)
2. **Task 2: Review thread component, review submit form, reply and review handlers** - `7ccce66` (feat)

**Plan metadata:** (included in final docs commit)

## Files Created/Modified

- `internal/adapter/driven/github/writer.go` - Replaced three stubs with real go-github API calls; errors.As for 422 detection
- `internal/adapter/driving/web/templates/components/review_thread.templ` - Thread component: root comment, indented replies, Alpine collapse/expand reply box
- `internal/adapter/driving/web/templates/components/pr_reviews_section.templ` - Reviews section: threads, comments, staged review form
- `internal/adapter/driving/web/templates/components/pr_detail.templ` - Updated Threads tab to use PRReviewsSection
- `internal/adapter/driving/web/handler.go` - Added CreateReplyComment, SubmitReview, CreateIssueComment handlers + helpers
- `internal/adapter/driving/web/routes.go` - Registered three new POST routes
- `internal/adapter/driving/web/viewmodel.go` - Added Owner/RepoName population from RepoFullName split
- `internal/adapter/driving/web/viewmodel/viewmodel.go` - Added Owner and RepoName fields to PRDetailViewModel

## Decisions Made

- **PRReviewsSection in components package**: The natural home for it is partials (a page section rendered via HTMX), but placing it there created an import cycle: components imports partials (pr_detail.templ) and partials imports components (PRReviewsSection calls ReviewThread). Moved to components to break the cycle.
- **Owner/RepoName on PRDetailViewModel**: Templ has no string manipulation, so the URL segments needed to be pre-computed. Added two fields to the view model rather than splitting in the template.
- **Per-thread render vs. full section render**: Reply submission morphs just the affected thread (smaller payload, preserves Alpine state of other threads). Review and issue comment submission morphs the full section (all threads may shift).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Moved PRReviewsSection from partials to components to break import cycle**
- **Found during:** Task 2 (review thread component and partial creation)
- **Issue:** Plan placed PRReviewsSection in `partials` package, but `components/pr_detail.templ` imports `partials`, and `partials/pr_reviews_section.templ` would import `components` for ReviewThread — circular dependency.
- **Fix:** Created `pr_reviews_section.templ` in `components` package instead of `partials`. No functional change.
- **Files modified:** components/pr_reviews_section.templ (new location)
- **Verification:** `go build ./...` succeeds with no import cycle errors
- **Committed in:** 7ccce66 (Task 2 commit)

**2. [Rule 2 - Missing Critical] Added Owner and RepoName fields to PRDetailViewModel**
- **Found during:** Task 2 (template URL construction)
- **Issue:** Plan said "pass owner, repo, prNumber to the partial" but PRDetail component only received `viewmodel.PRDetailViewModel`. Templ cannot split strings, so URL construction would be impossible without pre-computed fields.
- **Fix:** Added `Owner string` and `RepoName string` to PRDetailViewModel, populated in `toPRDetailViewModel` via `strings.SplitN(pr.RepoFullName, "/", 2)`.
- **Files modified:** viewmodel/viewmodel.go, viewmodel.go
- **Verification:** Build succeeds; all existing tests pass
- **Committed in:** 7ccce66 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug/structural, 1 missing critical field)
**Impact on plan:** Both required for the implementation to compile and function correctly. No scope creep.

## Issues Encountered

None — both deviations were detected and resolved inline.

## Next Phase Readiness

- REV-01, REV-02, REV-03 delivered: threaded display, inline reply, staged review submit
- Plan 04 (ConvertToDraft/MarkReady stubs in writer.go) builds on the same GitHubWriter interface established here
- Plan 05 (attention signals) can leverage the same handler/viewmodel patterns

## Self-Check: PASSED

- FOUND: internal/adapter/driven/github/writer.go (SubmitReview, CreateReplyComment, CreateIssueComment implemented)
- FOUND: internal/adapter/driving/web/templates/components/review_thread.templ (ReviewThread component)
- FOUND: internal/adapter/driving/web/templates/components/pr_reviews_section.templ (PRReviewsSection component)
- FOUND: internal/adapter/driving/web/handler.go (SubmitReview, CreateReplyComment, CreateIssueComment handlers)
- FOUND: .planning/phases/08-review-workflows-and-attention-signals/08-03-SUMMARY.md
- Commits verified: 4e9f530 (Task 1), 7ccce66 (Task 2)
- `go build ./...` passes
- `go test ./...` passes

---
*Phase: 08-review-workflows-and-attention-signals*
*Completed: 2026-02-19*
