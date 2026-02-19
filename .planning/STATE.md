# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-14)

**Core value:** A single dashboard where a developer can see all PRs needing attention, review and comment on them, and link to Jira context
**Current focus:** Phase 8 — Review Workflows and Attention Signals

## Current Position

Milestone: 2026.2.0 Web GUI
Phase: 8 of 9 (Review Workflows and Attention Signals) — IN PROGRESS
Plan: 5 of 5
Status: In progress
Last activity: 2026-02-19 — Completed 08-04-PLAN.md (draft status toggle: GraphQL mutations, toggle button, IsOwnPR)

Progress: [================    ] 80% (4/5 plans)

## Performance Metrics

**v1.0 Velocity:**

- Total plans completed: 16
- Average duration: 6min
- Total execution time: ~1.5 hours
- Timeline: 5 days (2026-02-10 to 2026-02-14)

**2026.2.0 Velocity:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 07-gui-foundation | 3/3 | 14min | 5min |
| 08-review-workflows-and-attention-signals | 4/5 | 33min | 8.25min |

## Accumulated Context

### Decisions

Full v1.0 decision log archived in .planning/milestones/v1.0-ROADMAP.md.

Recent decisions (2026.2.0):
- CalVer YYYY.MM.MICRO versioning going forward
- templ/HTMX/Alpine.js/Tailwind/GSAP frontend stack (no React/Vue/Svelte)
- Separate route namespaces: /api/v1/* for JSON, / and /app/* for web GUI
- alpine-morph HTMX extension to prevent Alpine state destruction during swaps
- GitHub creds in Phase 8 (needed for write ops), Jira creds in Phase 9
- Removed explicit templ import from .templ files — generator adds it automatically
- Extracted RegisterAPIRoutes and ApplyMiddleware from NewServeMux for dual-adapter composition
- Extracted viewmodel package to break import cycle between web handler and templ sub-packages
- Non-fatal enrichment pattern: review/health failures log errors but still render basic PR data
- Duplicated isValidRepoName in web handler (10-line function, not worth shared package)
- In-memory PR filtering for search — appropriate for expected scale, no new DB queries needed
- htmx:afterSettle for GSAP animations — morph swaps settle after DOM morphing completes
- OOB swap pattern: repo mutations render primary target + PRListOOB + RepoFilterOptions
- MYGITPANEL_GITHUB_TOKEN demoted from required to optional (warn-not-fail) to enable credential-via-GUI flow
- MYGITPANEL_SECRET_KEY: optional at startup (nil = credential storage disabled); present-but-malformed = error
- IgnoredPR type defined in port/driven package (not model/) — persistence concern, not pure domain entity
- DraftLineComment and ReviewRequest defined in githubwriter.go alongside the interface (port-layer input types)
- Nil-pointer semantics for RepoThreshold: nil field = inherit global default
- Closure injection (tokenProvider/clientFactory) for PollService hot-swap avoids application-to-adapter import cycle
- GitHubWriter stubs satisfy compile-time check immediately; real implementations in Plans 03 and 04
- Token validated before storing to prevent silently-broken polling from invalid tokens
- Drawer rendered in layout.templ outside @contents — Alpine state survives HTMX morph swaps
- PRReviewsSection placed in components package (not partials) to avoid components<->partials import cycle
- Owner and RepoName added to PRDetailViewModel for URL construction in templates (templ has no string splitting)
- ReviewThread component receives owner/repo/prNumber as separate args to keep view model clean
- CreateReplyComment morphs only the affected #thread-{rootID}; SubmitReview/CreateIssueComment morph #pr-reviews-section
- Write handlers auth-gate via credStore.Get before calling ghWriter; missing token returns 422 with actionable HTML fragment
- Draft toggle GraphQL mutations use pullRequestId variable (not id) per GitHub API spec; node_id fetched on-demand via REST
- Optimistic draft flip in ToggleDraftStatus handler: UI updates immediately, background poll brings DB to consistency
- PRDetailHeader extracted as named templ component with id=pr-detail-header for morph swap on toggle response
- authenticatedUsername helper: checks credStore github_username first, falls back to static config username

### Pending Todos

None.

### Blockers/Concerns

- Phase 9: Jira rate limiting is opaque — plan for research-phase during Phase 9 planning

## Session Continuity

Last session: 2026-02-19
Stopped at: Completed 08-04-PLAN.md (draft status toggle: GraphQL mutations, toggle button, IsOwnPR)
Resume file: Ready for 08-05-PLAN.md
