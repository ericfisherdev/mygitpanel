# Project Research Summary

**Project:** ReviewHub
**Domain:** GitHub PR Tracking and Review Management API (machine-consumable, AI-agent-oriented)
**Researched:** 2026-02-10
**Confidence:** MEDIUM to HIGH (based on training data for well-established domains)

## Executive Summary

ReviewHub is a localhost API for tracking GitHub pull requests and formatting review feedback for AI agent consumption. The project's core value proposition is transforming raw GitHub review data into AI-consumable context with code snippets and comment threading, enabling AI coding agents to understand and act on review feedback. This is genuinely unserved territory in the PR tooling landscape.

The recommended approach is **hexagonal architecture with Go, stdlib-first dependencies, and pure-Go SQLite**. Use `net/http` (Go 1.22+ offers method routing), `google/go-github` for GitHub API client, and `modernc.org/sqlite` (no CGO) for persistence. Background polling runs in a goroutine with context-based lifecycle management. The HTTP layer is a thin adapter; domain logic stays pure with zero external dependencies. Docker deployment uses Alpine-based multi-stage builds with WAL-mode SQLite on a volume.

The primary risk is **GitHub API rate limit exhaustion from naive polling**. With 10 repos and multiple endpoints per repo, a 60-second interval can burn 3x the 5,000 requests/hour authenticated limit. Mitigation requires conditional requests (ETags), rate-aware budgeting, and staggered polling. The second critical risk is **review comment position mapping complexity** — GitHub's multiple overlapping position fields (`position`, `line`, `side`, `diff_hunk`) require careful handling; use `diff_hunk` as the primary context source to avoid reconstruction errors.

## Key Findings

### Recommended Stack

Go 1.22+ with stdlib-first dependencies minimizes supply chain risk and build complexity. Pure-Go SQLite eliminates CGO, simplifying Docker builds dramatically. The complete direct dependency list is five libraries: `go-github`, `oauth2`, `modernc.org/sqlite`, `golang-migrate`, and `testify` (test only).

**Core technologies:**
- **Go 1.22+**: Method-based routing in stdlib eliminates need for third-party routers like chi or gin
- **`net/http` stdlib**: HTTP server and routing — since Go 1.22, stdlib supports `GET /path/{param}` patterns, making frameworks unnecessary for localhost API
- **`google/go-github/v68`**: Dominant Go client for GitHub REST API — provides typed structs, pagination, rate-limit awareness
- **`modernc.org/sqlite`**: Pure Go SQLite driver (no CGO) — critical for simple Docker builds; performance within 10-20% of CGO version, negligible for single-user workload
- **`log/slog` stdlib**: Structured logging (Go 1.21+) — zero dependencies, standard for new Go projects
- **`golang-migrate/migrate`**: Schema migrations with embedded SQL files via `embed` package
- **`testify`**: Assertion and mocking library — reduces test boilerplate, most widely adopted Go test helper

**Key architectural choices:**
- No framework (gin/echo/fiber) — they couple handlers to framework types, violating hexagonal architecture's port independence
- No ORM (gorm/ent) — explicit SQL keeps persistence transparent and decouples domain from database schema
- No config library (viper/koanf) — five environment variables do not justify dependencies
- Alpine Docker images with WAL-mode SQLite — allows concurrent reads during polling writes

### Expected Features

The feature landscape divides into table stakes (any PR tool must have), differentiators (ReviewHub's niche), and explicit anti-features (common but wrong for this scope).

**Must have (table stakes):**
- PR discovery by author and review-requested status — core use case from PROJECT.md
- PR metadata (title, state, author, timestamps, labels, branch info)
- Review status per reviewer (approved/changes_requested/commented/pending) with deduplication to latest review
- CI/CD combined check status (success/failure/pending) aggregating Status API + Checks API
- Polling with configurable interval and rate-limit awareness
- Repository CRUD — add/remove watched repos without restart
- Draft PR detection and state tracking

**Should have (competitive differentiators):**
- **Review comments with targeted code snippets** — THE core differentiator; AI agent needs comment + code context to generate fixes
- **Multi-line comment context** — surrounding code lines enrich AI understanding
- **Comment-to-file-path mapping** — AI needs exact file and line range to edit
- **Suggested changes extraction** — parse GitHub's ````suggestion```` markdown blocks for direct fix proposals
- **Conversation threading** — reconstruct reply chains so AI sees full discussion context
- **Coderabbit detection** — distinguish AI-generated reviews from human reviews via configurable bot username list
- **Staleness tracking** — days since open/last activity highlights aging PRs
- **Diff stats** — files changed, additions, deletions provide quick complexity signal

**Defer (v2+ or never):**
- Web dashboard/UI — explicitly scoped out; primary consumer is CLI agent
- Webhook receiver — adds deployment complexity; polling is simpler for v1
- PR creation/modification — ReviewHub is read-only tracking
- Notification system (email/Slack) — polling tool for machine consumption, not human alerting
- Multi-user/multi-tenant — single-user, single-token design
- Merge automation — report readiness, don't perform merges
- Historical analytics/trends — track current state only
- Custom review workflows/policies — PullApprove's territory
- GitHub OAuth — PAT via environment variable sufficient for localhost

### Architecture Approach

Hexagonal architecture (ports and adapters) with domain model at center, all infrastructure pushed to boundaries. In Go, this maps to interfaces (ports) defined in domain layer, concrete implementations (adapters) in separate packages.

**Major components:**

1. **Domain Model** (`internal/domain/model`) — Pure Go entities (PullRequest, Repository, Review, ReviewComment, CheckStatus) with zero external dependencies; behavioral methods for staleness, status computation

2. **Domain Ports** (`internal/domain/port`) — Split into driving (primary: PRService, RepoService, PollService) and driven (secondary: PRStore, RepoStore, GitHubClient) interfaces; define contracts without implementation

3. **Application Services** (`internal/application`) — Use case orchestration implementing driving ports; depends on driven ports via interfaces; contains core business logic (CommentEnricher, status computation, change detection)

4. **Adapters** — Concrete implementations live at boundaries:
   - **SQLite adapter** (`internal/adapter/driven/sqlite`) — implements PRStore, RepoStore; maps domain model to/from SQL rows
   - **GitHub adapter** (`internal/adapter/driven/github`) — implements GitHubClient; translates API responses to domain model
   - **HTTP adapter** (`internal/adapter/driving/http`) — thin handlers deserialize requests, call application services, serialize responses

5. **Polling Scheduler** (`internal/application/pollservice.go`) — Background goroutine with `time.Ticker` and `context.Context` for lifecycle; coordinates GitHub fetches, change detection, and persistence

6. **Composition Root** (`cmd/reviewhub/main.go`) — Only place that knows concrete types; wires all dependencies; handles graceful shutdown on SIGTERM

**Key patterns:**
- Constructor-based dependency injection with interfaces
- Context-based cancellation throughout (polling, HTTP, database)
- Adapter-layer mapping (separate structs for serialization in each adapter)
- Domain model as plain structs with methods (no ORM tags, no JSON tags)
- WAL-mode SQLite with `busy_timeout` for concurrent reads during polling writes

**Build order:** Domain model + ports (foundation) → SQLite adapter → GitHub adapter (parallel) → Application services → HTTP adapter → Polling scheduler + wiring

### Critical Pitfalls

The research identified five critical pitfalls that cause rewrites, outages, or fundamental architecture problems:

1. **GitHub API Rate Limit Exhaustion from Naive Polling** — With 10 repos and 4 API calls per repo at 60-second intervals, burns 14,400 requests/hour (3x the 5,000/hr limit). **Prevention:** Use conditional requests (ETags — 304s don't count against limit), budget requests explicitly, stagger polling across repos, implement exponential backoff on 403/429. Must be addressed in Phase 1; not something to add later.

2. **SQLite "database is locked" Errors from Concurrent Access** — Background polling writes while HTTP serves reads; default SQLite file-level locking blocks all readers during writes. **Prevention:** Enable WAL mode (`PRAGMA journal_mode=WAL`), set busy timeout (`PRAGMA busy_timeout=5000`), use single `*sql.DB` instance, set `MaxOpenConns(1)` for writes. Must be addressed in Phase 1 database setup.

3. **Review Comment Position Mapping Complexity** — GitHub has multiple overlapping position fields (`position`, `line`, `side`, `start_line`, `diff_hunk`); naive line-number assumptions break on outdated comments, multi-line comments, and renamed files. **Prevention:** Use `diff_hunk` as primary context source (GitHub already provides relevant context), handle all position field combinations, explicitly handle `null` positions after PR updates, handle file-level comments (no line reference). Core differentiator requiring dedicated phase with explicit test cases.

4. **Polling Interval Tuning Tradeoffs** — Uniform interval is either too aggressive (wastes rate limit) or too lax (stale data during active review). **Prevention:** Implement adaptive polling (increase frequency for recently-active PRs), provide manual refresh endpoint, use conditional requests to make frequent polling cheap, expose `last_polled_at` in API responses. Basic uniform polling in Phase 1; adaptive as Phase 2 enhancement.

5. **GitHub Token Exposure in Logs/Errors/Docker** — PAT leaking in logs, error messages, or Docker image layers gives full repo access to anyone who finds it. **Prevention:** Pass token via environment at runtime (never build-time), scrub authorization headers from errors, never use query parameter tokens, validate token on startup with clear error, use minimal scopes.

## Implications for Roadmap

Based on research findings, dependency analysis, and pitfall avoidance, the recommended phase structure follows the dependency rule (build innermost ring first, work outward) while frontloading critical pitfall mitigation.

### Phase 1: Foundation — Domain, Persistence, Configuration

**Rationale:** Everything depends on clean domain model and working persistence. SQLite configuration (WAL mode, busy timeout) must be correct from day one — cannot be bolted on later. Token handling security is foundational.

**Delivers:**
- Domain model entities (PullRequest, Repository, Review, ReviewComment, CheckStatus) with zero external dependencies
- Domain port interfaces (PRStore, RepoStore, GitHubClient, driving service interfaces)
- SQLite adapter with WAL mode, migrations via `embed`, PRRepo/RepoRepo implementations
- Configuration loading (env vars) with token validation on startup
- Project skeleton with proper directory structure (`internal/domain`, `internal/adapter`, `cmd/`)

**Addresses:**
- Pitfall 2 (SQLite locking) via WAL mode configuration
- Pitfall 5 (token exposure) via secure env-var handling
- Pitfall 13 (schema migrations) via versioned migration framework
- Table stakes: unique PR identification, repository CRUD foundation

**Avoids:**
- No external dependencies in domain layer (hexagonal architecture violation)
- No shared database models (anti-pattern 2)
- No token in Dockerfile ENV or logs

**Research needed:** None — standard patterns, well-documented

---

### Phase 2: GitHub Integration — Polling and Data Ingestion

**Rationale:** Cannot show PR data without fetching it from GitHub. Polling architecture must be rate-aware from the start (Pitfall 1). This phase establishes the data flow backbone.

**Delivers:**
- GitHub adapter implementing GitHubClient port (`google/go-github` integration)
- Response mapping (GitHub API types → domain model)
- Background polling service (goroutine with `time.Ticker`, context-based lifecycle)
- Rate limit tracking (`X-RateLimit-Remaining` header awareness, exponential backoff)
- Conditional requests (ETag caching, `If-None-Match` headers)
- Pagination handling (always use `per_page=100`, follow `Link` headers)
- Change detection (`updated_at` timestamp comparison, only process changed PRs)
- Basic uniform polling interval (configurable via env var)

**Addresses:**
- Pitfall 1 (rate limit exhaustion) via conditional requests, rate budgeting, staggered polling
- Pitfall 6 (pagination ignored) via proper `Link` header parsing
- Pitfall 9 (redundant processing) via change detection
- Pitfall 10 (conflating author/review-requested) via separate API calls
- Table stakes: PR discovery by author and review-requested, polling with rate-limit awareness

**Avoids:**
- Uniform polling for all repos (refined in Phase 4 with adaptive polling)
- No ETag implementation (must be in v1 for rate limit survival)
- No pagination (breaks with >30 open PRs)

**Research needed:**
- Verify current `go-github` version and struct field availability
- Confirm GitHub API pagination behavior hasn't changed
- Validate rate limit header names

---

### Phase 3: Core API — PR Listing and Status

**Rationale:** Data is being polled and stored; now expose it via HTTP. This phase delivers minimum viable API output. Keep handlers thin (no business logic).

**Delivers:**
- HTTP adapter with `net/http` stdlib router (Go 1.22+ method routing)
- Application services implementing driving ports (PRService, RepoService)
- Endpoints:
  - `GET /api/prs` — list PRs (filterable by repo, state, review status)
  - `GET /api/prs/{id}` — single PR with metadata
  - `POST /api/repos` — add watched repository
  - `DELETE /api/repos/{id}` — remove repository
  - `GET /api/repos` — list watched repositories
  - `GET /api/health` — health check
  - `POST /api/repos/{owner}/{repo}/refresh` — manual refresh trigger
- JSON response formatting (domain model → HTTP response DTOs in adapter)
- Graceful shutdown (signal handling, context cancellation, drain HTTP server, close DB)

**Addresses:**
- Table stakes: PR listing, repository CRUD, PR metadata (title, state, author, timestamps, labels)
- Pitfall 4 (polling interval tradeoffs) via manual refresh endpoint
- Pitfall 8 (ungraceful shutdown) via signal handling and context
- Anti-pattern 3 (fat handlers) — handlers are thin, logic in application layer

**Avoids:**
- Business logic in handlers (hexagonal violation)
- Framework coupling (gin/echo would leak into domain)
- No manual refresh option (UX gap)

**Research needed:** None — standard Go HTTP patterns

---

### Phase 4: Review Intelligence — Comment Formatting with Code Context

**Rationale:** This is THE core differentiator. Review comments with targeted code snippets enable AI agents to generate fixes. Most complex feature; deserves dedicated phase with extensive testing.

**Delivers:**
- CommentEnricher in application layer (formats comments with code snippets)
- Review comment fetching via GitHub API (per-PR review comments endpoint)
- Position mapping logic (handles `position`, `line`, `side`, `start_line`, `diff_hunk` fields)
- `diff_hunk` parsing as primary context source (avoids file reconstruction complexity)
- Handling of:
  - Null positions (outdated comments after PR updates)
  - Multi-line comments (`start_line` / `start_side` ranges)
  - File-level comments (no line position)
  - Renamed files (path changes between commits)
- Conversation threading (reconstruct reply chains via `in_reply_to_id`)
- Suggested changes extraction (parse ````suggestion```` markdown blocks)
- Comment-to-file-path mapping for AI consumption
- Review status computation (per-reviewer latest review, overall decision)
- Coderabbit detection (configurable bot username list)

**Addresses:**
- Pitfall 3 (position mapping complexity) — THE critical differentiator risk
- Pitfall 11 (User-Agent header) — set descriptive header in GitHub client
- Pitfall 12 (API error bodies) — parse GitHub error JSON
- Differentiators: Review comments with code snippets, multi-line context, threading, suggested changes
- Table stakes: Review status per reviewer, comment count

**Avoids:**
- File content reconstruction (use `diff_hunk` instead)
- Simple line-number assumptions (breaks on outdated comments)
- Single-line-only comment handling (multi-line comments lose context)

**Research needed:**
- **HIGH PRIORITY:** Verify `go-github` struct fields for review comments (Line, Side, StartLine, DiffHunk, SubjectType)
- Test all position field combinations with real GitHub data
- Validate thread resolution status availability (REST vs GraphQL API)

---

### Phase 5: PR Health Signals — CI Status and Enrichment

**Rationale:** CI/CD status is table stakes for merge readiness. Layered on after comment formatting because it's independent enrichment.

**Delivers:**
- CI/CD check status aggregation (Status API + Checks API)
- Combined status computation (success/failure/pending across both legacy statuses and GitHub Actions checks)
- Individual check names and states
- Diff stats (files changed, additions, deletions) — trivially cheap from PR object
- Draft PR detection — `draft` boolean field
- Staleness tracking — days since open, days since last activity (computed from timestamps)
- Merge conflict detection — `mergeable` and `mergeable_state` fields (with retry logic for null initial values)

**Addresses:**
- Table stakes: CI/CD combined status, individual checks, diff stats, draft detection
- Differentiators: Staleness tracking, merge conflict awareness
- Pitfall 14 (comment thread resolution) — model threads, not individual comments

**Avoids:**
- Required checks identification (needs admin-level token, defer to post-MVP)
- AI review summarization (downstream AI agent's job, not ReviewHub's)

**Research needed:**
- Verify Checks API vs Status API integration (both needed for full coverage)
- Test retry logic for `mergeable` computation (GitHub backend delay)

---

### Phase 6: Docker Deployment and Adaptive Optimizations

**Rationale:** Containerization and performance tuning come after core features work. Adaptive polling refines Phase 2's uniform approach.

**Delivers:**
- Dockerfile with multi-stage Alpine build (Go 1.23-alpine builder, Alpine 3.20 runtime)
- `CGO_ENABLED=0` for static binary (pure-Go SQLite enables this)
- Docker Compose with volume for SQLite persistence
- Adaptive polling (increase frequency for recently-active PRs, decrease for stale)
- Per-repo poll interval overrides (optional)
- Merge readiness composite score (combines reviews + checks + conflicts + staleness)
- Repository health summary (aggregate stats across watched repos)

**Addresses:**
- Pitfall 4 (polling interval tuning) — adaptive polling based on activity
- Pitfall 7 (go-github version) — document pinned version and rationale
- Pitfall 15 (over-engineering) — keep hexagonal structure thin, no layer explosion
- Anti-features: No web UI, no webhooks, no multi-user (explicitly scoped out)

**Avoids:**
- CGO requirement (would need gcc in Docker build)
- Token in Docker image layers (runtime env var only)
- Uniform polling waste (adaptive handles bursty activity)

**Research needed:** None — deployment is standard practice

---

### Phase Ordering Rationale

1. **Domain + Persistence first** — Everything depends on these; SQLite configuration cannot be fixed later
2. **GitHub integration before HTTP** — Cannot serve data without fetching it; polling architecture dictates API structure
3. **Basic API before comment formatting** — Validate polling and storage work with simple data before tackling complex differentiator
4. **Comment formatting isolated** — Most complex feature with most edge cases; deserves dedicated phase with extensive testing
5. **CI status layered on** — Independent enrichment; doesn't block comment formatting progress
6. **Docker and optimizations last** — Deployment after features work; adaptive polling refines uniform approach

This ordering follows hexagonal dependency rule (innermost first), frontloads critical pitfall mitigation (WAL mode, rate limits, token security), and isolates the highest-risk differentiator (position mapping) for focused validation.

### Research Flags

**Phases needing deeper research during planning:**

- **Phase 4 (Review Intelligence):** HIGH PRIORITY — Review comment position mapping is fundamentally complex with multiple overlapping fields. Needs verification of:
  - Current `go-github` library version and struct field availability (Line, Side, StartLine, DiffHunk, SubjectType)
  - REST API vs GraphQL for thread resolution status (`isResolved` not in REST)
  - Edge case behavior (null positions after PR updates, file renames, multi-line ranges)
  - Recommend `/gsd:research-phase "review comment formatting"` before Phase 4 planning

- **Phase 5 (CI Status):** MEDIUM — Checks API vs Status API integration needs validation; GitHub has two separate systems for check reporting. Verify coverage and aggregation approach.

**Phases with standard patterns (skip research-phase):**

- **Phase 1 (Foundation):** Well-documented Go patterns — domain modeling, SQLite WAL mode, configuration loading
- **Phase 2 (Polling):** Standard GitHub API pagination and rate limiting — mature, stable patterns
- **Phase 3 (HTTP API):** Idiomatic Go HTTP handlers — stdlib routing is well-established since Go 1.22
- **Phase 6 (Docker):** Standard multi-stage Alpine builds — no CGO simplifies dramatically

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Go stdlib, SQLite WAL mode, `google/go-github` are mature with stable APIs; specific version numbers need verification at project init |
| Features | MEDIUM to HIGH | GitHub PR data model is stable; table stakes are clear; differentiator complexity (position mapping) is well-known but needs hands-on validation |
| Architecture | HIGH | Hexagonal architecture in Go is well-established; directory structure and patterns are community standard; build order follows clear dependency graph |
| Pitfalls | HIGH | Rate limiting (5,000/hr), SQLite concurrency (WAL mode), token security, and polling patterns are well-documented; position mapping complexity is known domain trap |

**Overall confidence:** MEDIUM to HIGH

The stack, architecture, and major pitfalls are based on well-established, stable domains with extensive training data coverage. GitHub's API surface, SQLite behavior, and Go concurrency patterns have been stable for years. Confidence is HIGH for foundational decisions.

Confidence is MEDIUM for specific details requiring verification:
- Exact `go-github` version and struct field names (library evolves frequently)
- Thread resolution status API availability (REST vs GraphQL question)
- Current GitHub secondary rate limit thresholds (undocumented)

### Gaps to Address

Areas where research was inconclusive or needs validation during implementation:

- **Review comment position fields:** Training data indicates `position` (deprecated), `line`/`side` (modern), `start_line`/`start_side` (multi-line), and `diff_hunk` (context). Exact current state of field availability in `go-github` should be verified during Phase 4 planning. Recommend `/gsd:research-phase` spike before implementation.

- **Thread resolution status:** Whether `is_resolved` is available via REST API or requires GraphQL should be verified. If GraphQL-only, decide whether to add `shurcooL/githubv4` dependency or defer thread resolution to post-MVP.

- **Secondary rate limits:** GitHub's abuse detection has undocumented thresholds. May trigger even below 5,000/hr if requests are too bursty. Monitor in production; adjust staggering if needed.

- **Required checks identification:** Needs branch protection API with admin-level token permissions. Verify token scope requirements before committing to this feature; may need to remain post-MVP.

- **File content at head SHA:** Valuable for richer AI context (full file, not just diff hunk) but significantly increases API calls and storage. Defer until comment formatting with `diff_hunk` is validated; optimize with SHA-based caching if added.

## Sources

### Primary (HIGH confidence)

- **GitHub REST API documentation:** Rate limiting (5,000 req/hr authenticated), pagination (Link header), conditional requests (ETags), PR data model, review comment fields — training data through May 2025, stable API surface for years
- **SQLite documentation:** WAL mode, busy timeout, file-level locking behavior, PRAGMA settings — stable since SQLite 3.7.0 (2010)
- **Go standard library:** `net/http` method routing (Go 1.22+), `context` package, `signal` handling, `embed` package, `log/slog` (Go 1.21+) — well-documented, stable APIs
- **Go hexagonal architecture patterns:** Community conventions (Alistair Cockburn's ports and adapters applied to Go, Three Dots Labs examples, standard project layout)

### Secondary (MEDIUM confidence)

- **`google/go-github` library:** Dominant Go GitHub client, well-maintained by Google — training data confidence HIGH for choice, MEDIUM for exact version numbers (v68 approximate, may be v70+ by implementation time)
- **`modernc.org/sqlite`:** Pure-Go SQLite driver, well-established in community — choice HIGH confidence, API compatibility MEDIUM (verify at project init)
- **`golang-migrate/migrate`:** Widely adopted migration library — stable v4 API, HIGH confidence for choice, MEDIUM for latest version
- **Review comment position field evolution:** `position` → `line`/`side` transition is documented in GitHub API changelog — MEDIUM confidence for current field availability in `go-github` structs

### Tertiary (LOW confidence)

- **GitHub secondary rate limits:** Undocumented abuse detection thresholds — training data provides general guidance (avoid bursts, stagger requests) but exact limits unknown
- **Graphite, PullApprove, Coderabbit features:** Competitive landscape understanding based on training data — features may have evolved since May 2025; core differentiator (AI-agent-oriented comment formatting) remains unserved

**Verification needed at project init:**
1. Run `go get` commands and confirm version numbers resolve
2. Verify `go-github` latest major version and review comment struct fields
3. Confirm `golang:1.23-alpine` Docker tag exists (may need `1.22-alpine` or `1.24-alpine`)
4. Validate GitHub API field names against current docs during Phase 4 planning

---

*Research completed: 2026-02-10*
*Ready for roadmap: yes*
