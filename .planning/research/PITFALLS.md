# Domain Pitfalls

**Domain:** GitHub PR Tracking API (Go, SQLite, Docker, Polling)
**Project:** ReviewHub
**Researched:** 2026-02-10
**Overall confidence:** MEDIUM (training data corroborated by well-established, stable domain knowledge; web verification tools were unavailable)

---

## Critical Pitfalls

Mistakes that cause rewrites, outages, or fundamental architecture problems.

---

### Pitfall 1: GitHub API Rate Limit Exhaustion from Naive Polling

**What goes wrong:** The application polls all endpoints for all repositories on every interval tick. With 10 repos and 4 API calls per repo (list PRs, reviews, review comments, check runs), a 60-second interval burns 240 requests/minute = 14,400/hour -- nearly 3x the authenticated limit of 5,000 requests/hour. The app hits 403s, stops getting data, and the user thinks it is broken.

**Why it happens:** Developers calculate rate limits for a single repo, then forget to multiply by the number of repos, endpoints per repo, and pagination. GitHub's rate limit is global across all API calls with a given token -- not per-endpoint.

**Consequences:**
- Complete API blackout for up to an hour when the limit resets
- If the same token is used for other tools (gh CLI, other integrations), ALL of them stop working
- Polling threads may spin on 403 errors, logging noise without recovering
- The secondary rate limit (anti-abuse, undocumented exact thresholds) can trigger even below 5,000/hr if requests are too bursty

**Prevention:**
1. **Use conditional requests (ETags/If-Modified-Since).** GitHub returns `304 Not Modified` for unchanged resources and does NOT count these against the rate limit. This is the single most impactful optimization. Cache the `ETag` header from each response and send `If-None-Match` on subsequent requests.
2. **Budget requests explicitly.** Calculate: `(repos * endpoints_per_repo * avg_pages) / poll_interval_seconds * 3600` must be well under 5,000. Build a rate budget tracker that knows how many requests remain (from the `X-RateLimit-Remaining` header) and backs off proactively.
3. **Stagger polling across repos.** Do not poll all repos simultaneously. Spread requests across the interval window. If polling every 5 minutes, distribute repo polls across that 5-minute window.
4. **Implement exponential backoff on 403/429.** When rate-limited, respect the `X-RateLimit-Reset` or `Retry-After` header. Do not retry immediately.
5. **Use the `since` parameter** on endpoints that support it (e.g., list comments since a timestamp) to reduce response size and avoid pagination on unchanged data.

**Detection (warning signs):**
- `X-RateLimit-Remaining` drops below 1000 during normal operation
- 403 responses with `X-RateLimit-Remaining: 0`
- 403 responses with a message about secondary rate limits (abuse detection)

**Phase mapping:** Must be addressed in Phase 1 (core polling infrastructure). This is not something to add later -- the polling architecture must be designed around rate awareness from day one.

**Confidence:** HIGH -- GitHub's rate limit behavior is well-documented and stable for years. The 5,000/hr authenticated limit and ETag/conditional request mechanism are long-established.

---

### Pitfall 2: SQLite "database is locked" Errors from Concurrent Access

**What goes wrong:** Background polling goroutines write PR data to SQLite while the HTTP API serves read requests. Under default SQLite settings, a write lock blocks all readers, and concurrent writes fail with "database is locked" (SQLITE_BUSY). The API returns 500 errors during polling cycles.

**Why it happens:** SQLite uses file-level locking by default (journal mode DELETE). Go's `database/sql` opens a connection pool, and multiple goroutines try to use different connections to the same database file simultaneously. Without WAL mode, even readers block on writers.

**Consequences:**
- Intermittent 500 errors on API reads during polling writes
- Data corruption risk if the application retries writes without proper transaction handling
- Under load, cascading lock contention as goroutines queue up waiting for the lock

**Prevention:**
1. **Enable WAL mode immediately on database open:** `PRAGMA journal_mode=WAL;` This allows concurrent readers with a single writer. This is non-negotiable for any SQLite application with concurrent access.
2. **Set a busy timeout:** `PRAGMA busy_timeout=5000;` (5 seconds). Without this, SQLite returns SQLITE_BUSY immediately instead of waiting. With it, SQLite retries internally for the specified duration.
3. **Use a single `*sql.DB` instance** shared across the entire application. Do not open multiple database connections from different goroutines. Go's `database/sql` pool handles this correctly when there is one `*sql.DB`.
4. **Set `MaxOpenConns(1)` for writes** or use a write-serializing pattern. WAL mode allows one writer at a time. Channeling all writes through a single goroutine or using a mutex around write transactions prevents write contention entirely.
5. **Configure connection pragmas via DSN:** When using `mattn/go-sqlite3` or `modernc.org/sqlite`, set pragmas in the connection string: `file:reviewhub.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON`

**Detection (warning signs):**
- "database is locked" errors in logs
- API latency spikes correlating with polling intervals
- Intermittent test failures in CI

**Phase mapping:** Must be addressed in Phase 1 (database layer setup). The SQLite connection configuration is foundational -- every subsequent feature depends on it.

**Confidence:** HIGH -- SQLite concurrency behavior is extremely well-documented. WAL mode has been the recommended approach for concurrent-access scenarios since SQLite 3.7.0 (2010).

---

### Pitfall 3: Review Comment Position Mapping is Fundamentally Complex

**What goes wrong:** The developer assumes GitHub review comment positions are simple line numbers. They are not. The `position` field on a review comment refers to a line within the diff hunk, not a line in the file. When trying to extract "the code this comment refers to," the app either shows wrong code, crashes on edge cases, or produces garbage context for the AI consumer.

**Why it happens:** GitHub's review comment API has evolved over time and has multiple overlapping position fields:
- `position` (deprecated-ish): The line index in the diff (1-indexed from the start of the diff). Only valid for the original diff; becomes `null` if the PR is updated after the comment.
- `original_position`: The position in the original diff when the comment was created.
- `line`: The line number in the file (added later). Can be `null` for older comments.
- `original_line`: The original line number when the comment was created.
- `side`: `LEFT` (deletion) or `RIGHT` (addition) -- critical for understanding which version of the file the comment targets.
- `start_line` / `start_side`: For multi-line comments (comment spans a range).
- `diff_hunk`: The actual diff context GitHub provides -- a snippet of the unified diff around the comment. This is often the most reliable source of context.
- `subject_type`: `line` or `file` -- file-level comments have no line position at all.
- `path`: The file path, which may have changed between commits if the file was renamed.

**Consequences:**
- AI agent receives wrong code context, generates fixes for the wrong lines
- Comments on outdated diffs have `null` positions, and the app either crashes or silently drops them
- Multi-line comments are treated as single-line, losing context
- File-level comments (no position) break the position-parsing logic
- Renamed files cause path mismatches

**Prevention:**
1. **Use `diff_hunk` as the primary context source.** GitHub already provides the relevant diff context in the `diff_hunk` field of every review comment. For an AI consumer, this is often sufficient and avoids the entire position-mapping problem. Parse the `diff_hunk` to extract the code snippet rather than trying to reconstruct it from file contents + position.
2. **Handle all position field combinations.** Build a position resolver that tries fields in order: `line`/`side` (modern) -> `diff_hunk` parsing (fallback) -> `position` (legacy fallback) -> graceful degradation (show comment text without code context).
3. **Handle `null` positions explicitly.** When a PR is updated after a comment, positions can become `null`. The `diff_hunk` is still available. Do not assume positions are always present.
4. **Handle multi-line comments.** Check for `start_line` / `start_side` and expand the code context window accordingly.
5. **Handle file-level comments.** When `subject_type` is `file`, there is no line reference. Present these differently.
6. **Do not reconstruct diffs yourself.** Fetching the file contents and trying to map positions back is error-prone and fragile. Use what GitHub gives you (`diff_hunk`, `line`, `path`).

**Detection (warning signs):**
- Comments appearing with no code context
- AI agent responses referencing wrong code sections
- Null pointer panics in position handling
- Multi-line review comments showing only the last line

**Phase mapping:** This is the core differentiator of ReviewHub and must be designed carefully. Recommend a dedicated phase (likely Phase 2 or 3) focused entirely on comment extraction and formatting, with explicit test cases for every position field combination. Do not combine this with basic PR listing.

**Confidence:** MEDIUM -- The field names and general behavior are well-established in the GitHub API. The exact edge cases around null positions after PR updates and the evolution of `position` vs `line` fields are based on extensive community knowledge, but the specific current state of the API should be verified against the latest GitHub docs.

---

### Pitfall 4: Polling Interval Tuning and Stale Data Tradeoffs

**What goes wrong:** The developer picks a single polling interval (e.g., 60 seconds) and applies it uniformly. This is either too aggressive (wastes rate limit budget on inactive repos) or too lax (misses rapid changes during active review cycles). Users either burn through rate limits or see stale data.

**Why it happens:** Uniform polling seems simpler. Developers do not consider that PR activity is bursty -- a PR might get 20 comments in 10 minutes during active review, then nothing for days.

**Consequences:**
- Rate limit exhaustion on repos with no activity
- Stale data frustration when a PR is actively being reviewed
- No way for the user to trigger an immediate refresh when they know something changed

**Prevention:**
1. **Implement adaptive polling.** Increase poll frequency for recently-active PRs and decrease it for stale ones. Track `updated_at` timestamps; if a PR was updated in the last hour, poll every 2-3 minutes. If unchanged for 24 hours, poll every 30 minutes.
2. **Provide a manual refresh endpoint.** `POST /api/repos/{owner}/{repo}/refresh` or `POST /api/prs/{id}/refresh` to allow the consumer to trigger an immediate poll. This is cheap to implement and covers the "I just pushed, refresh now" use case.
3. **Use conditional requests to make frequent polling cheap.** With ETags, polling every 60 seconds costs almost nothing for unchanged resources (304s are not rate-limited). This makes aggressive intervals viable.
4. **Separate polling schedules by endpoint type.** PR list changes (new PRs, state changes) are less frequent than comment activity. Poll the PR list every 5 minutes but comments on active PRs every 2 minutes.
5. **Expose polling status in the API.** Return `last_polled_at` and `next_poll_at` in API responses so the consumer knows data freshness.

**Detection (warning signs):**
- User complaints about stale data
- Rate limit exhaustion despite few repos configured
- All repos polled at the same frequency regardless of activity

**Phase mapping:** Basic uniform polling in Phase 1, adaptive polling as a Phase 2 enhancement. The manual refresh endpoint should be in Phase 1 as it is cheap and critical for UX.

**Confidence:** HIGH -- polling pattern design is a well-established domain with stable best practices.

---

### Pitfall 5: GitHub Token Exposure in Logs, Errors, and Docker Configuration

**What goes wrong:** The GitHub personal access token (PAT) appears in error messages, debug logs, HTTP client traces, or is baked into the Docker image. A leaked PAT gives full repository access to anyone who finds it.

**Why it happens:** Go's `http.Client` can log request headers (including Authorization) in debug mode. Error messages from the GitHub client library may include the URL with token query parameters. Docker layers preserve environment variables set during build.

**Consequences:**
- Token leaked in application logs -> anyone with log access has repository access
- Token baked into Docker image layers -> anyone who pulls the image has the token
- Token in error messages returned via API -> consumer-facing exposure
- GitHub detects leaked tokens and revokes them, breaking the app with no clear error message

**Prevention:**
1. **Pass token via environment variable at runtime, never at build time.** Use `docker run -e GITHUB_TOKEN=...` or Docker secrets, not `ENV` in Dockerfile or `.env` files committed to git.
2. **Scrub authorization headers from error messages.** Wrap the HTTP client to redact `Authorization` headers before logging. Never log raw HTTP requests/responses containing the token.
3. **Never include the token in URLs as a query parameter.** Always use the `Authorization: Bearer <token>` header. Some older GitHub examples used `?access_token=` which appears in logs and referrer headers.
4. **Add `.env` and any secrets files to `.gitignore` from the very start.** The project already has `.planning/` in `.gitignore`; ensure `.env`, `*.pem`, and `*.key` are also excluded.
5. **Validate token on startup.** Call `GET /user` to verify the token works and has the needed scopes. Fail fast with a clear error ("Token invalid or missing required scopes") rather than mysterious 401s later during polling.
6. **Use minimal token scopes.** The app only needs `repo` scope (for private repos) or no scope at all (for public repos only). Do not use tokens with `admin`, `delete`, or `write` scopes.

**Detection (warning signs):**
- Token visible in `docker inspect` output
- Authorization header appearing in log output
- GitHub sending "token has been revoked" emails
- `.env` file appearing in `git status`

**Phase mapping:** Phase 1 (project setup and configuration). Token handling is the first thing configured and must be secure from the start.

**Confidence:** HIGH -- token security practices are well-established and stable.

---

## Moderate Pitfalls

Mistakes that cause delays, technical debt, or degraded experience.

---

### Pitfall 6: GitHub API Pagination Ignored or Incorrectly Implemented

**What goes wrong:** The developer fetches only the first page of results (default 30 items) from endpoints like "list pull requests" or "list review comments." For active repos with many PRs, this silently drops data. The app appears to work during development (small test repos) but fails in production (large repos).

**Why it happens:** GitHub's default page size is 30 items. During development, test repos have fewer than 30 open PRs, so pagination is never triggered. The issue only surfaces with real-world repos.

**Prevention:**
1. **Always paginate every list endpoint.** Use the `Link` header from GitHub's response to detect additional pages. The `rel="next"` link indicates more pages exist.
2. **Set `per_page=100`** (GitHub's maximum) to minimize the number of requests needed.
3. **Use the `state` parameter to filter.** For PR listing, use `state=open` to only fetch open PRs (you rarely need thousands of closed PRs for a tracking tool).
4. **Implement a pagination helper** that wraps list endpoints and handles the `Link` header parsing automatically. The `google/go-github` library has built-in pagination support via `ListOptions` and checking `Response.NextPage`.
5. **Cap maximum pages** as a safety measure. If a repo has 10,000 open PRs, you probably have a configuration problem, not a pagination problem. Log a warning if page count exceeds a threshold.

**Detection (warning signs):**
- App shows exactly 30 PRs for a repo you know has more
- Missing PRs that exist in the GitHub web UI
- No `Link` header parsing in the HTTP client code

**Phase mapping:** Phase 1 (core polling). Pagination must be correct from the first implementation.

**Confidence:** HIGH -- GitHub pagination behavior is well-documented and has been stable for many years.

---

### Pitfall 7: go-github Library Version and API Compatibility

**What goes wrong:** The developer uses an outdated version of `google/go-github` that does not support newer API fields (like `line` and `side` on review comments, multi-line comment support, or GraphQL-backed fields). Alternatively, the developer uses `go-github/v60+` which requires Go 1.21+ and the developer is targeting an older Go version.

**Why it happens:** `go-github` follows GitHub API changes closely and has frequent major version bumps (v50, v55, v60+). It is easy to follow an old tutorial that imports `go-github/v45` and miss newer fields. The library versions are tightly coupled to Go versions.

**Prevention:**
1. **Use the latest stable `go-github` version** at project start. Check the GitHub releases page for the current version.
2. **Verify the review comment struct fields.** Ensure the version you are using has `Line`, `Side`, `StartLine`, `StartSide`, `SubjectType`, and `DiffHunk` fields on `PullRequestComment`.
3. **Pin the version in `go.mod`** and document why a specific version was chosen.
4. **Consider whether `go-github` is even necessary.** For a focused application like ReviewHub that only uses a handful of endpoints, a thin custom HTTP client with typed response structs may be simpler and more maintainable than a massive generated library. You avoid version churn and only model the fields you actually need.

**Detection (warning signs):**
- Import path does not include a major version suffix (`go-github` without `/vXX`)
- Review comment struct does not have `Line` field
- Compile errors after Go version update

**Phase mapping:** Phase 1 (project setup). The GitHub client choice is foundational.

**Confidence:** MEDIUM -- The general advice is stable, but specific version numbers and struct field availability should be verified against current `go-github` releases.

---

### Pitfall 8: Graceless Docker Container Shutdown Corrupting SQLite

**What goes wrong:** Docker sends SIGTERM, but the application does not handle it. Goroutines are killed mid-transaction. SQLite WAL file is left in an inconsistent state. On restart, the database may be corrupted or missing recent writes.

**Why it happens:** Go applications without signal handling exit immediately on SIGTERM (Docker's default stop signal). If a SQLite write transaction is in progress, the WAL checkpoint does not complete. Docker waits 10 seconds then sends SIGKILL if the process has not stopped.

**Consequences:**
- Lost writes from the last polling cycle
- Potential SQLite database corruption requiring recovery or rebuild
- WAL file grows unbounded if checkpoints never complete cleanly

**Prevention:**
1. **Handle SIGTERM and SIGINT** using `signal.NotifyContext` or `signal.Notify`. On signal, stop accepting new polls, wait for in-flight transactions to complete, close the database, then exit.
2. **Use `context.Context` throughout.** Pass a cancellable context to all polling goroutines and database operations. When shutdown is triggered, cancel the context and wait for goroutines to finish via a `sync.WaitGroup`.
3. **Set a reasonable shutdown timeout.** Wait up to 30 seconds for in-flight operations, then force-exit. This must be less than Docker's stop timeout (default 10s, configurable via `stop_grace_period`).
4. **Configure Docker `stop_grace_period`** to be longer than the application's shutdown timeout. If the app needs 15 seconds to drain, set `stop_grace_period: 30s`.
5. **Call `db.Close()` explicitly** in the shutdown path. This ensures the WAL is checkpointed and the database file is consistent.
6. **Use a Docker HEALTHCHECK** to detect if the app is stuck. If shutdown hangs, the health check fails, and Docker can force-restart.

**Detection (warning signs):**
- "database disk image is malformed" errors after container restart
- Missing data after container restart
- WAL file growing very large (never checkpointed)
- Container taking exactly 10 seconds to stop (SIGKILL after timeout)

**Phase mapping:** Phase 1 (Docker and infrastructure setup). Graceful shutdown must be in the initial skeleton.

**Confidence:** HIGH -- Go signal handling and SQLite shutdown behavior are well-documented.

---

### Pitfall 9: Not Tracking PR Update Timestamps Leading to Redundant Processing

**What goes wrong:** Every polling cycle fully re-fetches and re-processes every PR, even if nothing has changed. This wastes rate limit budget, creates unnecessary database churn, and makes it impossible to determine what actually changed between polls.

**Why it happens:** The developer stores the current PR state but does not compare it to the previous state. Without change tracking, the app cannot distinguish "PR was updated" from "PR is the same as last poll."

**Prevention:**
1. **Store `updated_at` from GitHub's PR response.** This is the definitive "something changed" signal. Only re-fetch details (comments, reviews, checks) for PRs whose `updated_at` is newer than the stored value.
2. **Use ETags per resource.** Store the ETag for each API response (per-repo PR list, per-PR comments). Only process the response body when the ETag has changed (i.e., GitHub returns 200, not 304).
3. **Implement change detection in the database layer.** When upserting PR data, compare the new state to the stored state and only trigger downstream processing (comment re-fetch, status update) if something actually changed.
4. **Use `sort=updated&direction=desc`** when listing PRs. This puts recently-changed PRs first. Combined with `since` parameter awareness, you can stop paginating once you reach PRs older than your last poll.
5. **Log what changed.** When a PR is updated, log which fields changed. This is invaluable for debugging and confirms the change detection is working.

**Detection (warning signs):**
- Rate limit consumption does not decrease after initial data load
- Database row update timestamps change every poll even when nothing changed on GitHub
- Polling a single repo uses 20+ requests per cycle regardless of activity

**Phase mapping:** Phase 1 (polling design). Change tracking should be built into the initial polling loop, not bolted on later.

**Confidence:** HIGH -- standard polling optimization patterns.

---

### Pitfall 10: Conflating "PR Author" and "Review Requested" Queries

**What goes wrong:** The developer uses a single API call or search query to get both "PRs I authored" and "PRs where my review is requested." The two concepts require different API calls with different parameters, and conflating them produces incorrect results or misses PRs entirely.

**Why it happens:** It seems like a single search query could handle both. But GitHub's `author:` and `review-requested:` search qualifiers cannot be ORed -- they are ANDed. A search for `author:me review-requested:me` returns PRs where BOTH conditions are true (which is almost never).

**Prevention:**
1. **Make two separate API calls:** One for `GET /search/issues?q=author:{username}+type:pr+state:open` and one for `GET /search/issues?q=review-requested:{username}+type:pr+state:open`. Alternatively, use the repository PR list endpoint with appropriate parameters.
2. **Use the repos-based approach for review requests.** For "PRs needing my review" from configured repos, iterate through each configured repo with `GET /repos/{owner}/{repo}/pulls?state=open` and check the `requested_reviewers` field. This is more reliable than search and respects the configured repo list.
3. **Deduplicate results.** A PR the user authored might also have a review request from them (self-review). Ensure the data model handles this -- a PR can be both "authored" and "review-requested."
4. **Handle team review requests.** Review requests can be assigned to teams, not just individuals. If the user is a member of a requested team, the PR should show as needing review. This requires an additional call to check team memberships or using the `review-requested` search qualifier which handles teams.

**Detection (warning signs):**
- "PRs needing review" list is empty when the user knows they have pending reviews
- Search API queries using AND where OR is intended
- Missing PRs from repos not in the configured list

**Phase mapping:** Phase 1 (core polling queries). The query design is fundamental to what data the app shows.

**Confidence:** HIGH -- GitHub Search API qualifier behavior is well-documented.

---

## Minor Pitfalls

Mistakes that cause annoyance but are fixable without major rework.

---

### Pitfall 11: Ignoring GitHub's Requested `User-Agent` Header

**What goes wrong:** GitHub requires a `User-Agent` header on all API requests. Requests without one may be rejected with a 403. Go's default `http.Client` sets `User-Agent` to `Go-http-client/1.1`, which works but is not informative. Some rate limiting or blocking decisions by GitHub consider the User-Agent.

**Prevention:** Set a descriptive `User-Agent` header: `ReviewHub/1.0 (github.com/username/reviewhub)`. If using `go-github`, it sets its own User-Agent, but you can override it.

**Phase mapping:** Phase 1 (HTTP client setup). Trivial to add upfront, annoying to debug later.

**Confidence:** HIGH.

---

### Pitfall 12: Not Handling GitHub API Error Response Bodies

**What goes wrong:** The developer checks only HTTP status codes but ignores the JSON error body. GitHub returns detailed error messages (including which field failed validation, whether rate limit is primary or secondary, and documentation URLs) in the response body. Without parsing these, debugging failures is guesswork.

**Prevention:**
1. Parse all non-2xx responses as GitHub error objects: `{"message": "...", "documentation_url": "...", "errors": [...]}`.
2. Log the full error body, not just the status code.
3. Distinguish primary rate limit (status 403, `X-RateLimit-Remaining: 0`) from secondary/abuse rate limit (status 403 or 429, message about abuse detection) as they have different backoff strategies.

**Phase mapping:** Phase 1 (HTTP client/error handling). Build this into the GitHub client wrapper from the start.

**Confidence:** HIGH.

---

### Pitfall 13: SQLite Schema Migrations in Docker

**What goes wrong:** The database file is persisted via a Docker volume. When the application updates and the schema changes, there is no migration mechanism. The app crashes on startup with schema mismatch errors, or worse, silently writes to wrong columns.

**Prevention:**
1. **Implement versioned migrations from the start.** Use a simple migration table (`schema_version`) and numbered migration files. Libraries like `golang-migrate/migrate` or `pressly/goose` work well, but even a hand-rolled version table is sufficient for a small project.
2. **Run migrations on startup** before the app begins serving or polling.
3. **Never alter columns in SQLite.** SQLite's `ALTER TABLE` is limited. To change a column, create a new table, copy data, drop the old table, rename. Plan for this in your migration strategy.
4. **Back up the database file before migrations** in the Docker entrypoint. A simple `cp reviewhub.db reviewhub.db.bak` before starting the app.

**Phase mapping:** Phase 1 (database setup). The migration mechanism must exist before the first schema is created.

**Confidence:** HIGH.

---

### Pitfall 14: Resolved vs. Unresolved Comment Threads are Not a Simple Boolean

**What goes wrong:** The developer models comment resolution as a boolean on each comment. But GitHub's resolution model is at the conversation/thread level, not the individual comment level. A thread (started by one comment with replies) is resolved or not, and any participant can resolve or unresolve it. Additionally, "outdated" (comment on code that has since been changed) is a separate state from "resolved."

**Why it happens:** The PR review comments API returns individual comments, not threads. Thread structure must be reconstructed from the `in_reply_to_id` field. Resolution status is on the review thread, accessible via GraphQL or the `pulls/comments` endpoint's `is_resolved` field (added later, may not be on all comment types consistently).

**Prevention:**
1. **Model threads, not individual comments.** Group comments by their root comment (follow `in_reply_to_id` chains). The thread is the unit of resolution, not individual replies.
2. **Check for `is_resolved` field availability** in your `go-github` version. If unavailable, consider the GraphQL API for thread resolution status, or fetch the pull request review threads endpoint.
3. **Distinguish "resolved" from "outdated."** A comment can be outdated (code has changed) but not resolved (the concern has not been addressed). Both states matter for the AI consumer.
4. **Present thread context to the AI consumer.** When formatting a comment for AI consumption, include the full thread (original comment + all replies) so the AI understands the full conversation, not just the last message.

**Phase mapping:** Phase 2-3 (comment formatting). This is the differentiator feature and needs careful modeling.

**Confidence:** MEDIUM -- The general threading model is well-known, but the specific API fields for resolution status should be verified against current GitHub API docs and `go-github` struct definitions.

---

### Pitfall 15: Hexagonal Architecture Over-Engineering in a Small Go Service

**What goes wrong:** The developer creates deeply nested port/adapter/domain layers for a service with one domain concept (PRs), one external dependency (GitHub), and one storage mechanism (SQLite). The result is dozens of interfaces and adapter files for what could be three packages. Every change requires touching 5 files. New contributors (or the AI agent maintaining it) struggle with the indirection.

**Why it happens:** Hexagonal architecture is prescribed by project conventions (and it IS the right pattern for DDD). But Go's idiom is "a little copying is better than a little dependency." Over-abstraction is a Go anti-pattern even when hexagonal architecture is correct.

**Prevention:**
1. **Start with clear boundaries but thin layers.** Three core boundaries are sufficient: `domain` (PR models, business rules), `github` (adapter for GitHub API), `sqlite` (adapter for storage), `http` (adapter for serving the API). Each is a Go package.
2. **Define ports as interfaces in the domain package** where they are consumed, not where they are implemented. This is idiomatic Go (`io.Reader` is defined in `io`, not in `os`).
3. **Do not create interfaces until you have two implementations** or a clear testing need. A `GitHubClient` interface is justified (you will mock it in tests). A `PRFormatter` interface for one implementation is premature.
4. **Keep the adapter layer thin.** Adapters should translate between external formats and domain types. They should not contain business logic. But they also should not be split into sub-layers.
5. **Avoid the "Clean Architecture" file explosion.** You do not need `usecase`, `interactor`, `presenter`, `gateway`, `controller` as separate concepts. In Go, a handler calls a service which calls a repository. Three layers, not seven.

**Phase mapping:** Phase 1 (project structure). Get the package layout right from the start. Refactoring package structure in Go is painful because of import cycles.

**Confidence:** HIGH -- this is standard Go architecture guidance combined with the project's hexagonal architecture requirement.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation | Severity |
|-------------|---------------|------------|----------|
| Project setup / skeleton | Token insecurity, no migration framework, wrong SQLite pragmas | Configure WAL+busy_timeout, migrations, env-var token on day one | Critical |
| Core polling loop | Rate limit exhaustion, no pagination, no change detection | Budget-aware polling with ETags, always paginate, track `updated_at` | Critical |
| PR data model | Conflating author vs reviewer queries, wrong thread model | Separate queries, model threads not individual comments | Moderate |
| Comment formatting | Position mapping complexity, missing context | Use `diff_hunk` as primary source, handle all null cases | Critical |
| Docker deployment | Ungraceful shutdown, schema migration on volume | Signal handling + context cancellation, versioned migrations | Moderate |
| Adaptive features | Uniform polling waste | Adaptive intervals based on activity, conditional requests | Minor (Phase 2) |

---

## Domain-Specific Insight: The "Works in Development, Fails in Production" Gap

Many of these pitfalls share a common thread: they are invisible during development with small test repos and light usage, but surface immediately with real-world repositories.

**The gap manifests as:**
- Pagination: never triggered with < 30 PRs
- Rate limiting: never hit with 1-2 repos
- Comment positions: never null with fresh PRs (only after pushes to existing PRs)
- SQLite locking: never contended with manual API testing (no concurrent polling)
- Stale data: never noticed with frequent manual refreshes

**Mitigation:** From Phase 1, test with a realistic scenario: 5+ repos, at least one with 50+ open PRs, PRs that have been updated multiple times (stale comments with null positions), and concurrent API + polling load. Do not wait until deployment to encounter scale issues.

---

## Sources and Confidence Notes

All findings in this document are based on training data (cutoff May 2025) covering well-established, stable domains:

- **GitHub REST API:** Rate limiting (5,000/hr authenticated), pagination (Link header), conditional requests (ETags), and review comment fields have been stable for years. HIGH confidence these fundamentals have not changed.
- **SQLite concurrency:** WAL mode, busy_timeout, and file-level locking behavior have been stable since SQLite 3.7.0. HIGH confidence.
- **Go concurrency patterns:** Signal handling, context cancellation, sync.WaitGroup are core Go patterns unchanged since Go 1.7+. HIGH confidence.
- **Review comment position fields:** The evolution from `position` to `line`/`side` is well-documented but the exact current state of field availability should be verified against current GitHub API docs and `go-github` library. MEDIUM confidence.
- **`go-github` library versions:** Specific version numbers and struct field availability change frequently. MEDIUM confidence; verify current version at project start.

**Verification recommended for:**
1. Current `go-github` latest version and its review comment struct fields
2. GitHub's secondary rate limit thresholds (undocumented, may have changed)
3. Whether `is_resolved` is available on review comments via REST API or requires GraphQL
4. Current GitHub best practices documentation for any new recommendations
