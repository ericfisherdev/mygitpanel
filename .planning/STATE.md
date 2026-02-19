# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-14)

**Core value:** A single dashboard where a developer can see all PRs needing attention, review and comment on them, and link to Jira context
**Current focus:** Phase 8 — Review Workflows and Attention Signals

## Current Position

Milestone: 2026.2.0 Web GUI
Phase: 8 of 9 (Review Workflows and Attention Signals) — IN PROGRESS
Plan: 3 of 5
Status: In progress
Last activity: 2026-02-19 — Completed 08-02-PLAN.md (settings drawer, credential management, PollService hot-swap)

Progress: [========            ] 40% (2/5 plans)

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
| 08-review-workflows-and-attention-signals | 2/5 | 29min | 14.5min |

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

### Pending Todos

None.

### Blockers/Concerns

- Phase 8: Draft-to-ready is GraphQL-only — extend hand-rolled graphql.go client (research confirmed this is the right approach)
- Phase 9: Jira rate limiting is opaque — plan for research-phase during Phase 9 planning

## Session Continuity

Last session: 2026-02-19
Stopped at: Completed 08-02-PLAN.md (settings drawer, credential management, PollService hot-swap)
Resume file: Ready for 08-03-PLAN.md
