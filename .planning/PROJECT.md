# ReviewHub

## What This Is

A Dockerized Go API that tracks GitHub pull requests — both authored by the user and those awaiting review from configured repositories. It polls GitHub at regular intervals, stores PR state in SQLite, and serves structured endpoints designed for consumption by Claude Code. The core differentiator is formatting review comments with targeted code snippets so an AI agent can read a comment and generate a fix.

## Core Value

Review comments are formatted with enough code context that an AI agent can understand the request and produce a working fix — without the user manually copying context.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Poll GitHub API for PRs authored by configured user
- [ ] Poll GitHub API for PRs needing review from configured repositories
- [ ] CRUD endpoints to add/remove/list watched repositories at runtime
- [ ] Configure GitHub token and username via environment variables or API
- [ ] Store PR data in SQLite between polls
- [ ] Status endpoint returning PRs with statuses: merged, closed, ready to merge, changes requested
- [ ] Boolean flags per PR: reviewed/needs review, Coderabbit reviewed/awaiting Coderabbit
- [ ] Detect Coderabbit status by checking for reviews from @coderabbitai user
- [ ] Format review comments with targeted code snippets (relevant hunk/lines around the comment)
- [ ] Track resolved vs open comment threads per PR
- [ ] CI/CD check status flag (GitHub Actions / checks passing or failing)
- [ ] Staleness tracking: days since opened, days since last activity
- [ ] Diff stats per PR: files changed, lines added, lines removed
- [ ] Poll GitHub at recommended intervals (respect rate limits)
- [ ] Run in Docker container
- [ ] Localhost-only access, no authentication required

### Out of Scope

- Web dashboard / frontend UI — API-only, consumed by CLI agents
- GitHub webhook receiver — polling only for v1, webhooks deferred
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

## Constraints

- **Language**: Go — user preference, good Docker fit
- **Storage**: SQLite — file-based, no additional containers
- **Deployment**: Docker — single container, localhost only
- **GitHub API**: Must respect rate limits (polling interval tuning)
- **Architecture**: Hexagonal architecture per user's DDD preferences
- **Code Style**: Clean code, SOLID principles, domain-driven design

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over PHP/Python | Docker-native, single binary, fast | — Pending |
| SQLite over Postgres | Single container, no infra overhead | — Pending |
| Polling over webhooks | Simpler v1, no public endpoint needed | — Pending |
| Targeted snippets over full diffs | Focused context for AI, smaller payloads | — Pending |
| Localhost-only, no auth | Simplicity, secured by network isolation | — Pending |
| CRUD API for repo config | Runtime flexibility over static config files | — Pending |

---
*Last updated: 2026-02-10 after initialization*
