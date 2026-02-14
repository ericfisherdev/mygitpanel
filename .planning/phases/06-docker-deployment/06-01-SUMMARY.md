---
phase: 06-docker-deployment
plan: 01
subsystem: infra
tags: [docker, scratch, healthcheck, x509, sqlite]

requires:
  - phase: 05-pr-health-signals
    provides: Complete application with all features ready for containerization
provides:
  - Multi-stage scratch Dockerfile with zero CGO
  - Docker Compose one-command startup with SQLite volume persistence
  - Healthcheck binary for scratch containers (no shell/curl)
  - CA cert embedding via x509roots/fallback
  - .env.example template for user configuration
affects: [06-02-adaptive-polling]

tech-stack:
  added: [golang.org/x/crypto/x509roots/fallback]
  patterns: [multi-stage-scratch-build, healthcheck-binary, env-file-config]

key-files:
  created:
    - Dockerfile
    - docker-compose.yml
    - .dockerignore
    - .env.example
    - cmd/healthcheck/main.go
  modified:
    - cmd/mygitpanel/main.go
    - go.mod
    - go.sum
    - .gitignore
    - internal/adapter/driven/github/client.go

key-decisions:
  - "x509roots/fallback blank import embeds CA certs in binary — no COPY from build stage needed"
  - "Healthcheck is a separate Go binary (not shell script) since scratch has no shell"
  - "Container listens on 0.0.0.0:8080 inside, host restricts via 127.0.0.1:8080 port binding"
  - "Named Docker volume mygitpanel-data:/data for SQLite persistence across restarts"

patterns-established:
  - "Scratch container pattern: build stage creates empty dirs (/data, /tmp) copied to scratch"
  - "Healthcheck binary pattern: minimal Go binary hitting health endpoint, exit 0/1"

duration: 5min
completed: 2026-02-14
---

# Plan 06-01: Docker Containerization Summary

**Multi-stage scratch Dockerfile (17MB image), Docker Compose with SQLite volume persistence, healthcheck binary, and CA cert embedding**

## Performance

- **Duration:** 5 min
- **Tasks:** 3 (2 auto + 1 human-verify checkpoint)
- **Files created:** 5
- **Files modified:** 4

## Accomplishments
- Scratch-based Docker image at 17.2MB with zero OS dependencies
- `docker compose up` starts the full application with one command
- SQLite data persists across container restarts via named volume
- Container health check reports healthy via `docker ps`
- CA certs embedded in binary for TLS in scratch containers

## Task Commits

1. **Task 1: Healthcheck binary and CA cert embedding** - `6f0ddf3` (feat)
2. **Task 2: Dockerfile, Docker Compose, and supporting config** - `8e42c25` (feat)
3. **Task 3: Human verification** - approved by user after runtime fix

## Files Created/Modified
- `Dockerfile` - Multi-stage build: golang:1.25-alpine → scratch
- `docker-compose.yml` - Service with named volume, env_file, localhost port binding
- `.dockerignore` - Excludes .git, .planning, .env, *.db from build context
- `.env.example` - Template with required and optional env vars
- `cmd/healthcheck/main.go` - Minimal binary hitting /api/v1/health for Docker HEALTHCHECK
- `cmd/mygitpanel/main.go` - Added x509roots/fallback blank import
- `.gitignore` - Added .env to prevent secret leakage

## Decisions Made
- x509roots/fallback embeds Mozilla CA bundle in the binary — no need to copy certs from build stage
- Healthcheck binary uses http.NewRequestWithContext (not client.Get) per linter requirements
- Container binds 0.0.0.0 inside (Docker forwarding needs it), host restricts via 127.0.0.1 port mapping

## Deviations from Plan

### Auto-fixed Issues

**1. [Bug] go-github-ratelimit v2 NewClient panic on nil option**
- **Found during:** Human verification (container crash loop)
- **Issue:** `github_ratelimit.NewClient(cacheTransport, nil)` panics — v2 variadic opts doesn't accept nil
- **Fix:** Changed to `github_ratelimit.NewClient(cacheTransport)` (no args)
- **Files modified:** internal/adapter/driven/github/client.go
- **Verification:** Container starts and serves requests successfully
- **Committed in:** `10cdd3a`

---

**Total deviations:** 1 auto-fixed (1 runtime bug)
**Impact on plan:** Pre-existing bug only visible at runtime. Fix is minimal and correct.

## Issues Encountered
- Container crash loop due to nil option panic in go-github-ratelimit — fixed by removing explicit nil from variadic call

## User Setup Required
Users need to create `.env` from `.env.example` with their GitHub token and username before running `docker compose up`.

## Next Phase Readiness
- Docker containerization complete, ready for adaptive polling (06-02)
- No blockers

---
*Phase: 06-docker-deployment*
*Completed: 2026-02-14*
