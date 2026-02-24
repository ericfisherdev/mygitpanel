# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-14)

**Core value:** A single dashboard where a developer can see all PRs needing attention, review and comment on them, and link to Jira context
**Current focus:** Phase 9 — Jira Integration

## Current Position

Milestone: 2026.2.0 Web GUI
Phase: 9 of 9 (Jira Integration)
Plan: 3 of 4
Status: In progress
Last activity: 2026-02-24 — Completed 09-03-PLAN.md (Settings UI and Jira connection management)

Progress: [===============.....] 75% (3/4 plans)

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
| 08-review-workflows-and-attention-signals | 5/5 | 54min | 10.8min |
| 09-jira-integration | 3/4 | 16min | 5.3min |

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
- [Phase 08-review-workflows-and-attention-signals]: PRListOOB passes []model.PullRequest for ignored section; sidebar uses []vm.PRCardViewModel — no conversion in OOB path
- [Phase 08-review-workflows-and-attention-signals]: WithAttentionService post-construction injection avoids circular dependency between Handler and AttentionService
- [Phase 08-review-workflows-and-attention-signals]: Layout.templ accepts GlobalSettings to pre-populate threshold form in SettingsDrawer
- [Phase 09-jira-integration]: Duplicated encrypt/decrypt in JiraConnectionRepo (not shared with CredentialRepo) per SRP
- [Phase 09-jira-integration]: GetByID/GetForRepo return zero-value JiraConnection + nil error when not found (project convention)
- [Phase 09-jira-integration]: repo_jira_mapping uses ON DELETE SET NULL so mapping rows persist after connection deletion
- [Phase 09-jira-integration]: parseJiraTime fallback for Jira's non-standard timezone offset format (+0000 vs +00:00)
- [Phase 09-jira-integration]: Separated extractADFDocText (top-level doc) from extractADFText (recursive node) for clean ADF handling
- [Phase 09-jira-integration]: jiraConnectionByID shared handler pattern for ID-based store operations (deduplicates Delete/SetDefault)
- [Phase 09-jira-integration]: jiraConnections threaded through Layout/Sidebar/RepoManager/RepoList/RepoThresholdPopover (no global state)
- [Phase 09-jira-integration]: RepoManager updated to use RepoThresholdPopover for consistent initial render and OOB swap behavior

### Pending Todos

None.

### Blockers/Concerns

- Phase 9: Jira rate limiting is opaque — plan for research-phase during Phase 9 planning

## Session Continuity

Last session: 2026-02-24
Stopped at: Completed 09-03-PLAN.md (Settings UI and Jira connection management) — Phase 9 in progress
Resume file: .planning/phases/09-jira-integration/09-04-PLAN.md
