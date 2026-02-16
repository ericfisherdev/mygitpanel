# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-14)

**Core value:** A single dashboard where a developer can see all PRs needing attention, review and comment on them, and link to Jira context
**Current focus:** Phase 8 — Review Workflows and Attention Signals

## Current Position

Milestone: 2026.2.0 Web GUI
Phase: 8 of 9 (Review Workflows and Attention Signals)
Plan: 2 of 4
Status: In progress
Last activity: 2026-02-15 — Completed 08-02-PLAN.md (GitHub write operations, credential hot-swap, flexible config)

Progress: [==========..........] 50% (2/4 plans)

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
| 08-review-workflows | 2/4 | 6min | 3min |

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
- CredentialStore.Get returns empty string for missing keys (not error) — consistent with nil-nil pattern
- IgnoreStore.Ignore uses ON CONFLICT DO NOTHING for idempotency
- RepoSettings foreign key to repositories with ON DELETE CASCADE
- Raw GraphQL mutations for draft toggle (no shurcooL/githubv4 dependency needed)
- GitHubClientProvider with RWMutex for runtime credential hot-swap
- Config.Load() optional GitHub credentials — backward compatible, GUI can provide at runtime

### Pending Todos

None.

### Blockers/Concerns

- Phase 8: Draft-to-ready resolved — raw GraphQL mutations (markReadyMutation/convertToDraftMutation) following existing FetchThreadResolution pattern
- Phase 9: Jira rate limiting is opaque — plan for research-phase during Phase 9 planning

## Session Continuity

Last session: 2026-02-15
Stopped at: Completed 08-02-PLAN.md (GitHub write operations and credential hot-swap)
Resume file: .planning/phases/08-review-workflows-and-attention-signals/08-03-PLAN.md
