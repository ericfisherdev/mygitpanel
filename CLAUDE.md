# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MyGitPanel is a Go API that tracks GitHub pull requests and formats review comments with code context for AI agent consumption (Claude Code CLI). It polls GitHub, stores PR state in SQLite, and serves structured JSON endpoints.

**Module:** `github.com/ericfisherdev/mygitpanel`
**Go version:** 1.25.4
**Status:** Phase 3/6 complete (Foundation, GitHub Integration, Core API done; Review Intelligence, PR Health Signals, Docker Deployment pending)

## Build & Test Commands

```bash
go build ./...                                    # Build all packages
go test ./...                                     # Run all tests
go test ./internal/adapter/driven/sqlite/...      # Run tests for a specific package
go test -run TestListPRs ./internal/adapter/driving/http/...  # Run a single test
go test -v -cover ./...                           # Verbose with coverage
go vet ./...                                      # Static analysis
```

No Makefile or Dockerfile exists yet (Docker deployment is Phase 6).

## Architecture: Hexagonal (Ports & Adapters)

Dependencies flow inward — domain has zero external dependencies. The composition root (`cmd/mygitpanel/main.go`) wires everything together via constructor injection.

```
cmd/mygitpanel/main.go             ← Composition root
internal/
  domain/model/                    ← Pure entities (PullRequest, Repository, Review, ReviewComment, enums)
  domain/port/driven/              ← Secondary port interfaces (GitHubClient, PRStore, RepoStore)
  application/                     ← Use cases (PollService: polling orchestration, deduplication)
  adapter/driven/github/           ← GitHub API adapter (go-github v82, ETag cache, rate limit)
  adapter/driven/sqlite/           ← SQLite adapter (modernc.org/sqlite, no CGO)
  adapter/driving/http/            ← HTTP REST adapter (stdlib net/http with Go 1.22+ routing)
  config/                          ← Env var loading with fail-fast validation
```

### Key Architectural Rules

- **Domain model structs have no external dependencies** — no ORM tags, no framework imports
- **All go-github types are translated to domain types in the GitHub adapter** — never leak API types
- **Ports are minimal interfaces** following Interface Segregation (e.g., PRStore has 8 methods, not a god interface)
- **HTTP handlers are thin** — they parse requests, call stores/services, return JSON DTOs
- **Application layer depends only on ports**, never on concrete adapters

## Database

SQLite with dual reader/writer connections (WAL mode). Writer pool: 1 connection; reader pool: 4 connections.

- Migrations in `internal/adapter/driven/sqlite/migrations/` using golang-migrate with embedded SQL files
- Labels stored as JSON text column, not a join table
- Upsert via `ON CONFLICT` to preserve auto-increment IDs
- Composite unique constraint: `(repo_full_name, number)` on pull_requests

## HTTP API (7 Endpoints)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/prs` | All tracked PRs |
| GET | `/api/v1/prs/attention` | PRs needing review |
| GET | `/api/v1/repos/{owner}/{repo}/prs/{number}` | Single PR detail |
| GET | `/api/v1/repos` | All watched repos |
| POST | `/api/v1/repos` | Add repo to watch list (triggers async refresh) |
| DELETE | `/api/v1/repos/{owner}/{repo}` | Remove repo |
| GET | `/api/v1/health` | Health check |

## Testing Patterns

- **testify** for assertions (`require`, `assert`)
- Mock implementations of port interfaces defined in test files (e.g., `mockPRStore`)
- `httptest` for HTTP handler tests
- SQLite adapter tests use real in-memory databases via `testhelper_test.go`
- Table-driven tests for handler endpoint coverage

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MYGITPANEL_GITHUB_TOKEN` | Yes | — | GitHub personal access token |
| `MYGITPANEL_GITHUB_USERNAME` | Yes | — | GitHub username to track |
| `MYGITPANEL_GITHUB_TEAMS` | No | — | Comma-separated team slugs for review detection |
| `MYGITPANEL_POLL_INTERVAL` | No | `5m` | Polling frequency |
| `MYGITPANEL_LISTEN_ADDR` | No | `127.0.0.1:8080` | HTTP listen address |
| `MYGITPANEL_DB_PATH` | No | `mygitpanel.db` | SQLite database file path |

## Key Dependencies

- `google/go-github/v82` — GitHub REST API client
- `gregjones/httpcache` — ETag-based HTTP caching (applied before rate limiting in transport stack)
- `gofri/go-github-ratelimit/v2` — Secondary rate limit middleware
- `golang-migrate/migrate/v4` — Database migrations with embedded SQL
- `modernc.org/sqlite` — Pure Go SQLite (no CGO required)
- `stretchr/testify` — Test assertions

## Planning & Phase Docs

Phase planning documents live in `.planning/` with per-phase subdirectories. `STATE.md` tracks current project velocity and accumulated decisions. `ROADMAP.md` has the 6-phase delivery plan.

## Stubs for Future Phases

`FetchReviews()` and `FetchReviewComments()` in the GitHub adapter are implemented as part of Phase 4 (Review Intelligence).
