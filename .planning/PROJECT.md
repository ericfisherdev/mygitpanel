# ReviewHub

## What This Is

A Dockerized Go application with a web GUI that tracks GitHub pull requests across multiple repositories — both authored by the user and those awaiting review. It provides a unified PR feed with threaded review workflows, Jira issue context, and configurable attention signals. The backend polls GitHub with adaptive frequency and stores state in SQLite; the frontend uses templ/HTMX/Alpine.js/Tailwind/GSAP for a responsive, animated interface.

## Core Value

A single dashboard where a developer can see all PRs needing attention, review and comment on them, and link to Jira context — without switching between GitHub tabs and Jira.

## Current State

**Version:** 2026.2.0 Web GUI (completed 2026-02-24, pending merge)
**Codebase:** 18,782 lines Go, dual driving adapters (JSON API + web GUI)
**Tech stack:** Go 1.25, modernc.org/sqlite, google/go-github v82, templ, HTMX, Alpine.js, Tailwind CSS, GSAP, Docker scratch
**Architecture:** Hexagonal (ports & adapters), 17 routes (10 API + 7 GUI app), adaptive polling
**Known tech debt:** Dockerfile Tailwind CLI musl/glibc incompatibility (local dev works, Docker build needs fix)

## Requirements

### Validated

- ✓ Poll GitHub API for PRs authored by configured user — v1.0
- ✓ Poll GitHub API for PRs needing review from configured repositories — v1.0
- ✓ CRUD endpoints to add/remove/list watched repositories at runtime — v1.0
- ✓ Configure GitHub token and username via environment variables — v1.0
- ✓ Store PR data in SQLite between polls — v1.0
- ✓ Status endpoint returning PRs with statuses: merged, closed, ready to merge, changes requested — v1.0
- ✓ Boolean flags per PR: reviewed/needs review, Coderabbit reviewed/awaiting Coderabbit — v1.0
- ✓ Detect Coderabbit status by checking for reviews from @coderabbitai user — v1.0
- ✓ Format review comments with targeted code snippets (relevant hunk/lines around the comment) — v1.0
- ✓ Track resolved vs open comment threads per PR — v1.0
- ✓ CI/CD check status flag (GitHub Actions / checks passing or failing) — v1.0
- ✓ Staleness tracking: days since opened, days since last activity — v1.0
- ✓ Diff stats per PR: files changed, lines added, lines removed — v1.0
- ✓ Poll GitHub at recommended intervals with adaptive scheduling — v1.0
- ✓ Run in Docker container — v1.0
- ✓ Localhost-only access, no authentication required — v1.0
- ✓ Web GUI with unified PR feed across repos — 2026.2.0
- ✓ GitHub username/token configuration via GUI, persisted in SQLite — 2026.2.0
- ✓ Add/remove/manage watched repos through the GUI — 2026.2.0
- ✓ Read PR descriptions and comments — 2026.2.0
- ✓ Customizable review thresholds per repo (required reviewer count) — 2026.2.0
- ✓ Configurable urgency flagging by PR age per repo — 2026.2.0
- ✓ PR ignore list with re-add capability — 2026.2.0
- ✓ View and reply to PR comments/change requests — 2026.2.0
- ✓ Submit full PR reviews (approve, request changes) on others' PRs — 2026.2.0
- ✓ Toggle PR between active and draft — 2026.2.0
- ✓ Jira API connection configuration (URL, email, token) via GUI — 2026.2.0
- ✓ View linked Jira issue details (description, comments, priority, status) — 2026.2.0
- ✓ Post comments to linked Jira issues from the GUI — 2026.2.0

### Active

- Implementing 2026.2.0 features (PR `#8` open, pending merge)

### Out of Scope

- GitHub webhook receiver — polling only, webhooks deferred
- Multi-user support — single user, single GitHub token
- OAuth login flows — token-based configuration only
- Review summary/digest generation — the AI agent handles summarization
- Push notifications — no alerting, pull-based only
- Jira issue creation/status changes — read + comment only
- React/Vue/Svelte or heavy frontend frameworks — HTMX/Alpine.js stack only

## Context

- Primary consumers: (1) Web GUI for human developer, (2) JSON API for Claude Code CLI agent
- User wants a single pane of glass for all PR activity across repos
- Coderabbit is an AI code review bot that posts reviews as the `@coderabbitai` GitHub user
- Go chosen for performance, single binary, and Docker-friendly deployment
- SQLite chosen for zero-infra persistence that fits single-container deployment
- v1.0 shipped in 5 days with 16 plans across 6 phases (API-only)
- 2026.2.0 shipped in 10 days with 12 plans across 3 phases (web GUI + workflows + Jira)
- UI design reference: JSX mockup with dark/light theme, sidebar PR list, detail panel, Jira panel, GSAP animations
- Versioning switched from SemVer to CalVer (YYYY.MM.MICRO) starting 2026.2.0

## Constraints

- **Language**: Go — user preference, good Docker fit
- **Frontend**: templ (a-h/templ) + HTMX + Alpine.js + Tailwind CSS + GSAP — no React/Vue/Svelte
- **Storage**: SQLite — file-based, no additional containers
- **Deployment**: Docker — single container, localhost only
- **GitHub API**: Must respect rate limits (adaptive polling + ETag caching)
- **Jira API**: REST API v3 via URL/email/token — read + comment only
- **Architecture**: Hexagonal architecture per user's DDD preferences
- **Code Style**: Clean code, SOLID principles, domain-driven design

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over PHP/Python | Docker-native, single binary, fast | ✓ Good — 17MB scratch image, ~1s startup |
| SQLite over Postgres | Single container, no infra overhead | ✓ Good — WAL mode, dual reader/writer works well |
| Polling over webhooks | Simpler v1, no public endpoint needed | ✓ Good — adaptive polling reduces API calls 50-70% |
| Targeted snippets over full diffs | Focused context for AI, smaller payloads | ✓ Good — diff hunks + line numbers sufficient for fixes |
| Localhost-only, no auth | Simplicity, secured by network isolation | ✓ Good — Docker port binding enforces localhost |
| CRUD API for repo config | Runtime flexibility over static config files | ✓ Good — add/remove repos without restart |
| Pure Go SQLite (modernc.org) | Zero CGO, cross-platform, scratch-compatible | ✓ Good — enables scratch Docker image |
| Hexagonal architecture | Clean separation, testable, extensible | ✓ Good — 76% coverage, clean dependency graph |
| GraphQL for thread resolution | REST API lacks isResolved field | ✓ Good — graceful degradation on failure |
| 4-tier adaptive polling | Reduce rate limit consumption | ✓ Good — Hot(2m)/Active(5m)/Warm(15m)/Stale(30m) |
| templ over html/template | Type-safe components, better DX with HTMX | ✓ Good — compile-time safety, clean component model |
| HTMX/Alpine.js over React | User preference, no build step, lightweight | ✓ Good — alpine-morph prevents state destruction on swaps |
| CalVer over SemVer | Calendar-based versioning matches release cadence | ✓ Good — YYYY.MM.MICRO readable and unambiguous |
| Jira read+comment scope | Minimal viable integration, avoid Jira write complexity | ✓ Good — ADF handling contained in adapter, no domain leakage |
| Dual driving adapter composition | Separate JSON API and web GUI handlers | ✓ Good — RegisterAPIRoutes + RegisterRoutes compose cleanly |
| Non-fatal enrichment pattern | Review/health/Jira failures don't break PR display | ✓ Good — graceful degradation on all enrichment paths |
| AES-256-GCM for credentials | Secure at-rest storage with nonce-prepend | ✓ Good — same pattern for GitHub token and Jira connections |
| tokenProvider closure for hot-swap | Avoid app→adapter import cycle for PollService | ✓ Good — zero-restart token updates work cleanly |
| Optimistic draft flip | UI updates immediately, poll brings consistency | ✓ Good — perceived performance with no race condition |

---
*Last updated: 2026-02-24 after 2026.2.0 milestone completion (pending PR `#8` merge)*
