# Roadmap: ReviewHub

## Overview

ReviewHub delivers a Dockerized Go API that tracks GitHub pull requests and formats review comments with code context for AI agent consumption. The roadmap progresses from inside out following hexagonal architecture: domain model and persistence first, then GitHub data ingestion, then HTTP API exposure, then the core differentiator (AI-ready comment formatting), then enrichment signals (CI/CD, staleness), and finally containerized deployment with polling optimizations. Each phase delivers a coherent, verifiable capability that the next phase builds upon.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation** - Domain model, SQLite persistence, configuration, and project skeleton
- [ ] **Phase 2: GitHub Integration** - Polling engine, GitHub API adapter, and PR discovery
- [ ] **Phase 3: Core API** - HTTP endpoints for PR listing, repository management, and health check
- [ ] **Phase 4: Review Intelligence** - Comment formatting with code context, threading, and bot detection
- [ ] **Phase 5: PR Health Signals** - CI/CD status, staleness tracking, diff stats, and merge conflict detection
- [ ] **Phase 6: Docker Deployment** - Containerization, adaptive polling, and production readiness

## Phase Details

### Phase 1: Foundation
**Goal**: A clean hexagonal project skeleton exists with domain entities, port interfaces, working SQLite persistence (WAL mode), and secure configuration loading -- the inner rings that everything else depends on
**Depends on**: Nothing (first phase)
**Requirements**: INFR-01, INFR-02, INFR-05, INFR-06, INFR-07
**Success Criteria** (what must be TRUE):
  1. Application starts, loads GitHub token and username from environment variables, and fails fast with a clear error if either is missing
  2. SQLite database is created on first run with WAL mode enabled, and schema migrations run automatically on startup
  3. Application shuts down gracefully on SIGTERM/SIGINT -- drains in-flight work, closes database connection, and exits cleanly
  4. Domain model entities (PullRequest, Repository, Review, ReviewComment, CheckStatus) exist as pure Go structs with zero external dependencies
  5. Port interfaces (PRStore, RepoStore, GitHubClient) are defined and the SQLite adapter implements the store ports with passing tests
**Plans**: TBD

Plans:
- [ ] 01-01: TBD
- [ ] 01-02: TBD
- [ ] 01-03: TBD

### Phase 2: GitHub Integration
**Goal**: The system fetches PR data from GitHub for configured repositories, respects rate limits, handles pagination, and persists discovered PRs to SQLite
**Depends on**: Phase 1
**Requirements**: DISC-01, DISC-02, DISC-03, DISC-04, DISC-05, POLL-01, POLL-02, POLL-04, POLL-05, POLL-06, POLL-07
**Success Criteria** (what must be TRUE):
  1. System polls GitHub at a configurable interval (default 5 minutes) and discovers all open PRs authored by the configured user across watched repositories
  2. System discovers PRs where the user (or the user's team) is requested as a reviewer, and deduplicates PRs that appear in both authored and review-requested queries
  3. System correctly distinguishes draft PRs from ready PRs
  4. System tracks GitHub API rate limit budget, uses conditional requests (ETags) to avoid consuming limits on unchanged data, uses `updated_at` timestamps to skip re-processing, and handles pagination for all list endpoints
  5. A manual refresh can be triggered for a specific repository or PR, bypassing the polling interval
**Plans**: TBD

Plans:
- [ ] 02-01: TBD
- [ ] 02-02: TBD
- [ ] 02-03: TBD

### Phase 3: Core API
**Goal**: PR data and repository configuration are accessible via structured HTTP endpoints that a CLI agent can consume, with basic PR metadata on every response
**Depends on**: Phase 2
**Requirements**: API-01, API-02, API-03, API-04, REPO-01, REPO-02, REPO-03, STAT-01, STAT-07
**Success Criteria** (what must be TRUE):
  1. GET endpoint returns all tracked PRs with current status (open/merged/closed), title, author, branch, base branch, URL, and labels
  2. GET endpoint returns a single PR with its full metadata
  3. GET endpoint returns only PRs needing attention (changes requested or needs review)
  4. POST/DELETE/GET endpoints allow adding, removing, and listing watched repositories at runtime without restart
  5. Health check endpoint returns application status and the API is accessible on localhost only
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD
- [ ] 03-03: TBD

### Phase 4: Review Intelligence
**Goal**: Review comments are formatted with targeted code context, threaded into conversations, and enriched with bot detection -- enabling an AI agent to read a comment and generate a working fix
**Depends on**: Phase 3
**Requirements**: REVW-01, REVW-02, REVW-03, REVW-04, REVW-05, CFMT-01, CFMT-02, CFMT-03, CFMT-04, CFMT-05, CFMT-06, CFMT-07, REPO-04, STAT-02, STAT-06
**Success Criteria** (what must be TRUE):
  1. Each review comment returned by the API includes the targeted diff hunk with surrounding code lines, the file path, and line number(s) -- sufficient for an AI agent to locate and edit the file
  2. Comments are grouped into conversation threads (original + replies), and resolved vs open threads are tracked per PR
  3. GitHub suggestion blocks are extracted and presented as structured proposed changes distinct from regular comment text
  4. Inline (line-specific) comments are distinguished from general PR-level comments, and each comment includes reviewer name, timestamp, and review action
  5. Coderabbit reviews are detected by @coderabbitai author, nitpick comments are flagged separately, outdated reviews are marked, and bot usernames are configurable via API endpoint
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD
- [ ] 04-03: TBD
- [ ] 04-04: TBD

### Phase 5: PR Health Signals
**Goal**: Each PR shows CI/CD check status, staleness, diff stats, and merge conflict status -- giving the consumer a complete picture of PR health beyond review comments
**Depends on**: Phase 3 (parallel to Phase 4)
**Requirements**: CICD-01, CICD-02, CICD-03, STAT-03, STAT-04, STAT-05
**Success Criteria** (what must be TRUE):
  1. Each PR shows combined CI/CD check status (passing/failing/pending) aggregated from both Status API and Checks API
  2. Each PR lists individual check runs with name, status, and conclusion, and identifies required vs optional checks when token permissions allow
  3. Each PR shows staleness metrics: days since opened and days since last activity
  4. Each PR shows diff stats (files changed, lines added, lines removed) and merge conflict status (mergeable/conflicted/unknown)
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD
- [ ] 05-03: TBD

### Phase 6: Docker Deployment
**Goal**: The application runs in a Docker container with persistent storage, adaptive polling optimizes rate limit usage, and the system is production-ready for daily use
**Depends on**: Phases 4 and 5
**Requirements**: INFR-03, INFR-04, POLL-03
**Success Criteria** (what must be TRUE):
  1. Application runs in a Docker container built via multi-stage Alpine build with no CGO dependency, and SQLite data persists across container restarts via Docker volume
  2. Adaptive polling adjusts frequency based on PR activity -- recently active PRs are polled more frequently, stale ones less -- reducing rate limit consumption
  3. `docker compose up` starts the full application with a single command, and the API is accessible on localhost
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6
Note: Phases 4 and 5 are independent of each other (both depend on Phase 3) but are sequenced 4 before 5 because Phase 4 is the core differentiator.

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 0/3 | Not started | - |
| 2. GitHub Integration | 0/3 | Not started | - |
| 3. Core API | 0/3 | Not started | - |
| 4. Review Intelligence | 0/4 | Not started | - |
| 5. PR Health Signals | 0/3 | Not started | - |
| 6. Docker Deployment | 0/2 | Not started | - |
