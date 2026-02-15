# ReviewHub

## What This Is

A Dockerized Go API that tracks GitHub pull requests — both authored by the user and those awaiting review from configured repositories. It polls GitHub with adaptive frequency, stores PR state in SQLite, and serves structured endpoints designed for consumption by Claude Code. The core differentiator is formatting review comments with targeted code snippets so an AI agent can read a comment and generate a fix.

## Core Value

Review comments are formatted with enough code context that an AI agent can understand the request and produce a working fix — without the user manually copying context.

## Current State

**Version:** v1.0 MVP (shipped 2026-02-14)
**Codebase:** 9,052 lines Go, 76% test coverage
**Tech stack:** Go 1.25, modernc.org/sqlite, google/go-github v82, Docker scratch
**Architecture:** Hexagonal (ports & adapters), 10 REST endpoints, adaptive polling

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

### Active

(None — v1.0 shipped, next milestone requirements TBD)

### Out of Scope

- Web dashboard / frontend UI — API-only, consumed by CLI agents
- GitHub webhook receiver — polling only for v1, webhooks deferred to v2
- Multi-user support — single user, single GitHub token
- OAuth login flows — token-based configuration only
- Review summary/digest generation — the AI agent handles summarization
- Push notifications — no alerting, pull-based only

## Context

- Primary consumer is Claude Code CLI agent
- User wants to pipe review comments directly into an AI coding workflow: fetch comment → understand request → generate fix
- Coderabbit is an AI code review bot that posts reviews as the `@coderabbitai` GitHub user
- The API needs to be structured for machine consumption, not human readability
- Go chosen for performance, single binary, and Docker-friendly deployment
- SQLite chosen for zero-infra persistence that fits single-container deployment
- v1.0 shipped in 5 days with 16 plans across 6 phases

## Constraints

- **Language**: Go — user preference, good Docker fit
- **Storage**: SQLite — file-based, no additional containers
- **Deployment**: Docker — single container, localhost only
- **GitHub API**: Must respect rate limits (adaptive polling + ETag caching)
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

---
*Last updated: 2026-02-14 after v1.0 milestone*
