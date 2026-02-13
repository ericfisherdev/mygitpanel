# Technology Stack

**Project:** ReviewHub
**Researched:** 2026-02-10
**Overall Confidence:** MEDIUM -- All recommendations based on training data (cutoff May 2025). Exact versions should be verified with `go get` and `go list -m -versions` at project init time. Library choices and architectural rationale are HIGH confidence; pinned version numbers are MEDIUM.

---

## Recommended Stack

### Go Version

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.23+ | Language runtime | Latest stable as of early 2026. Use whatever `go version` ships in the golang Docker image tagged `1.23-alpine`. Go 1.23 brought iterator support, improved `net/http` routing (1.22+), and continued performance improvements. | MEDIUM |

**Note:** Go 1.22 introduced method-based routing in `net/http.ServeMux` (`mux.HandleFunc("GET /api/prs", handler)`), which significantly narrows the gap with third-party routers. Go 1.23 is the recommended minimum.

---

### HTTP Router / Web Framework

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `net/http` (stdlib) | Go 1.22+ | HTTP server and routing | **Recommended.** Since Go 1.22, the stdlib router supports method-based routing (`GET /path/{param}`), middleware chaining is trivial with `http.Handler`, and there are zero external dependencies. For a localhost-only API consumed by a single CLI agent, a full framework is unnecessary overhead. | HIGH |

#### Why NOT Other Routers

| Alternative | Why Not |
|-------------|---------|
| `chi` (go-chi/chi) | Excellent library, but since Go 1.22 the stdlib covers chi's core value prop (method routing, path params, middleware). Chi adds dependency weight without proportional benefit for this project's scope. |
| `gin` (gin-gonic/gin) | Framework-level abstraction with its own context type (`gin.Context`). Couples handlers to gin, violates hexagonal architecture's port independence. Also ships reflection-based binding magic that fights clean code principles. |
| `echo` (labstack/echo) | Same coupling problem as gin. Own context type, framework lock-in. Inappropriate for hexagonal architecture where HTTP is an adapter, not the core. |
| `fiber` (gofiber/fiber) | Built on fasthttp, not net/http. Incompatible with most middleware ecosystem. Optimized for throughput this project does not need. |
| `gorilla/mux` | Archived/maintenance-only since 2022. Not recommended for new projects. |

**Architecture rationale:** In hexagonal architecture, the HTTP layer is an *adapter*. Using `net/http` directly means handlers are thin adapters that translate HTTP requests into domain calls and domain responses into HTTP responses. No framework coupling leaks into the domain.

#### Supplementary HTTP Libraries

| Library | Purpose | Why | Confidence |
|---------|---------|-----|------------|
| `encoding/json` (stdlib) | JSON serialization | Standard, sufficient for this scope. No need for `json-iterator` or `sonic` -- the API serves a single local consumer. | HIGH |
| `net/http/httptest` (stdlib) | HTTP handler testing | Built-in test server for adapter-layer integration tests. | HIGH |

---

### GitHub API Client

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `google/go-github/v68` | v68.x | GitHub REST API client | The dominant Go client for the GitHub API, maintained by Google. Covers all endpoints this project needs: pull requests, reviews, review comments, check runs, and diffs. Well-typed structs, pagination helpers, and rate-limit awareness built in. | MEDIUM (version) / HIGH (choice) |

**Version note:** The `go-github` library increments its major version frequently (it was around v60-v68 in early-mid 2025). Use whatever the latest `v6x` or `v7x` is when you run `go get github.com/google/go-github/v68`. The API is stable across major bumps; they bump for GitHub API additions.

| Supporting Library | Version | Purpose | Why | Confidence |
|---|---|---|---|---|
| `golang.org/x/oauth2` | latest | GitHub token transport | Required by go-github to create an authenticated `*http.Client`. Even though we are using a static PAT (not OAuth), go-github expects the token via `oauth2.StaticTokenSource`. | HIGH |

#### Why NOT Other GitHub Clients

| Alternative | Why Not |
|-------------|---------|
| Raw `net/http` calls to GitHub API | Reinventing pagination, rate limiting, response parsing, and struct definitions. go-github gives this for free with strong typing. |
| `shurcooL/githubv4` (GraphQL) | GraphQL is more efficient for complex nested queries, but adds query complexity. The REST API is simpler for our polling use case and go-github's REST coverage is excellent. Can migrate specific hot paths to GraphQL later if rate limits become an issue. |
| `cli/go-gh` | GitHub CLI's internal library. Not designed as a general-purpose API client. Tightly coupled to `gh` CLI concerns. |

---

### Database / SQLite

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `modernc.org/sqlite` | latest | SQLite driver (pure Go) | **Recommended.** Pure Go implementation -- no CGO required. This is critical for Docker: no need for `gcc` in the build stage, simpler multi-stage builds, and cross-compilation works out of the box. Performance is within 10-20% of CGO `mattn/go-sqlite3` for this project's workload (single-user, small dataset). | HIGH |
| `database/sql` (stdlib) | Go stdlib | Database abstraction | Standard Go database interface. `modernc.org/sqlite` registers as a `database/sql` driver. Using the stdlib interface keeps the persistence adapter portable and testable. | HIGH |

#### Why NOT Other SQLite Options

| Alternative | Why Not |
|-------------|---------|
| `mattn/go-sqlite3` | Requires CGO. This means the Docker build needs `gcc` and `musl-dev` (or `build-base`) in the Alpine build stage, bloating build time and image layer complexity. The performance advantage is negligible for our workload (hundreds of PRs, not millions of rows). |
| `crawshaw.io/sqlite` | Lower-level API, less mature ecosystem. `modernc.org/sqlite` is the community standard for pure-Go SQLite. |
| `jmoiron/sqlx` | Adds struct scanning and named parameters on top of `database/sql`. Nice convenience, but for a small project with clean architecture, raw `database/sql` with explicit scanning keeps the code transparent and testable. If boilerplate becomes painful, add it later. |
| `gorm` / `ent` / ORMs | ORMs fight hexagonal architecture. The domain defines its own repository interfaces; the persistence adapter implements them with direct SQL. An ORM inserts itself as the domain model, coupling persistence concerns into the core. Explicit SQL is clearer, more debuggable, and produces better query plans for SQLite. |

#### SQLite Configuration

Key PRAGMA settings for this project:

```sql
PRAGMA journal_mode=WAL;          -- Write-ahead logging: allows concurrent reads during writes
PRAGMA busy_timeout=5000;         -- Wait up to 5s for locks instead of failing immediately
PRAGMA synchronous=NORMAL;        -- Good durability without FULL's performance cost
PRAGMA foreign_keys=ON;           -- Enforce referential integrity
PRAGMA cache_size=-64000;         -- 64MB page cache (negative = KB)
```

**WAL mode** is essential because the poller writes while HTTP handlers read. Without WAL, readers block writers and vice versa.

---

### Configuration

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `os.Getenv` / `os.LookupEnv` (stdlib) | Go stdlib | Environment variable reading | For a single-container, single-user app, environment variables are the right configuration mechanism. No need for config file parsing libraries. Docker `--env` and `docker-compose.yml` environment sections handle injection. | HIGH |

#### Why NOT Config Libraries

| Alternative | Why Not |
|-------------|---------|
| `viper` | Massively over-engineered for env-var-only config. Pulls in dozens of transitive dependencies (YAML, TOML, HCL, etcd, consul). ReviewHub reads ~5 env vars. |
| `envconfig` (kelseyhightower) | Nice but archived. The pattern it implements (struct tags for env vars) is trivial to hand-write for 5 fields. |
| `koanf` | Reasonable viper alternative but still more complexity than needed. Worth considering only if config sources multiply (files, remote, etc.). |

#### Configuration Shape

```go
type Config struct {
    GitHubToken    string        // REVIEWHUB_GITHUB_TOKEN
    GitHubUsername string        // REVIEWHUB_GITHUB_USERNAME
    PollInterval   time.Duration // REVIEWHUB_POLL_INTERVAL (default: "5m")
    ListenAddr     string        // REVIEWHUB_LISTEN_ADDR (default: ":8080")
    DBPath         string        // REVIEWHUB_DB_PATH (default: "/data/reviewhub.db")
}
```

---

### Logging

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `log/slog` (stdlib) | Go 1.21+ | Structured logging | Built into Go since 1.21. Structured, leveled, with JSON and text handlers. Zero dependencies. The standard for new Go projects. | HIGH |

#### Why NOT Other Loggers

| Alternative | Why Not |
|-------------|---------|
| `uber-go/zap` | Faster than slog in benchmarks, but slog is fast enough. Zap adds a dependency and its own API. slog is the standard now. |
| `sirupsen/logrus` | Effectively legacy. The Go community has moved to slog. Logrus is in maintenance mode. |
| `zerolog` | Same story as zap: marginal performance gain, unnecessary dependency when slog exists. |
| `log` (stdlib old) | Unstructured, no levels. Use slog instead. |

---

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `testing` (stdlib) | Go stdlib | Test framework | Go's built-in test framework. No reason to use anything else for the test runner. | HIGH |
| `testify/assert` + `testify/require` | v1.9+ | Test assertions | Dramatically reduces assertion boilerplate vs. bare `if got != want { t.Errorf(...) }`. `require` stops on failure (good for setup), `assert` continues (good for multiple checks). The most widely used Go test helper. | HIGH |
| `testify/mock` | v1.9+ | Interface mocking | Generates mock implementations of domain port interfaces. Essential for testing adapters in isolation and domain services without real infrastructure. | HIGH |
| `net/http/httptest` (stdlib) | Go stdlib | HTTP handler testing | Test HTTP adapters without starting a real server. | HIGH |

#### Why NOT Other Test Libraries

| Alternative | Why Not |
|-------------|---------|
| `gocheck` | Older, less community adoption than testify. |
| `gomock` (uber or Google) | More ceremony than testify/mock. Code generation step with `mockgen`. Testify mock is simpler for this project's scale. |
| `ginkgo` / `gomega` | BDD-style frameworks. Add significant complexity and non-Go-idiomatic patterns. Overkill for a focused API project. |

---

### Scheduling / Polling

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `time.Ticker` + goroutine (stdlib) | Go stdlib | Periodic GitHub polling | A goroutine with `time.NewTicker` is the idiomatic Go pattern for periodic work. Combine with `context.Context` for graceful shutdown. No library needed. | HIGH |

#### Why NOT Cron Libraries

| Alternative | Why Not |
|-------------|---------|
| `robfig/cron` | Designed for cron-expression scheduling. ReviewHub polls at a fixed interval, not on a cron schedule. `time.Ticker` is simpler and more appropriate. |
| `go-co-op/gocron` | Same over-engineering issue. Fixed-interval polling is a 10-line goroutine, not a scheduler problem. |

---

### Database Migrations

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `golang-migrate/migrate` | v4.x | Schema migrations | Embedded SQL migration files, run at application startup. Supports SQLite. File-based migrations (`.sql` files) keep schema changes versioned and reviewable. Embed migration files with Go 1.16+ `embed` package. | HIGH |

#### Why NOT Other Migration Tools

| Alternative | Why Not |
|-------------|---------|
| `pressly/goose` | Also good, but golang-migrate has broader adoption and cleaner programmatic API for running migrations at startup. |
| `atlas` (ariga.io) | Declarative schema management. More powerful but more complex. Overkill for a project with ~5-10 tables. |
| Hand-rolled migrations | Works for tiny projects, but golang-migrate is lightweight enough that the safety of proper migration tracking is worth it from day one. |
| `gorm` auto-migrate | Couples to gorm. Generates unpredictable DDL. Never use auto-migrate for production. |

---

### Docker

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `golang:1.23-alpine` | Build stage | Compile Go binary | Alpine-based image minimizes build layer size. Since we use `modernc.org/sqlite` (pure Go, no CGO), we do NOT need `build-base` or `gcc`. | HIGH |
| `alpine:3.20` | Runtime stage | Run compiled binary | Minimal runtime image (~7MB base). Include `ca-certificates` for HTTPS to GitHub API. SQLite data stored on a Docker volume. | MEDIUM (version) |

#### Dockerfile Pattern

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /reviewhub ./cmd/reviewhub

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /reviewhub /usr/local/bin/reviewhub
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["reviewhub"]
```

**Key decisions:**
- `CGO_ENABLED=0` because `modernc.org/sqlite` is pure Go. This produces a fully static binary.
- Alpine runtime (not scratch/distroless) because `ca-certificates` and `tzdata` are easier to install via apk, and having a shell aids debugging.
- `/data` volume for SQLite persistence across container restarts.

#### Why NOT Other Base Images

| Alternative | Why Not |
|-------------|---------|
| `scratch` | No shell, no CA certs, no timezone data. Debugging is painful. The ~7MB Alpine overhead is worth it. |
| `distroless` | Google's minimal images. No shell for debugging, more complex CA cert handling. Good for production microservices, overkill for a localhost dev tool. |
| `ubuntu` / `debian` | 80-120MB base images. Unnecessary bloat for a single static binary. |

---

### Graceful Shutdown

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `os/signal` + `context` (stdlib) | Go stdlib | Clean shutdown on SIGTERM/SIGINT | Catch shutdown signals, cancel context, drain HTTP server, stop poller, close DB. Standard Go pattern. Critical for SQLite data integrity (WAL checkpoint on close). | HIGH |

---

## Complete Dependency List

### Direct Dependencies (go.mod)

```
github.com/google/go-github/v68    -- GitHub API client
golang.org/x/oauth2                -- Token authentication for GitHub
modernc.org/sqlite                 -- Pure Go SQLite driver
github.com/golang-migrate/migrate/v4 -- Database migrations
github.com/stretchr/testify        -- Test assertions and mocks (test only)
```

**That is five direct dependencies.** This is intentional. A small dependency tree means fewer supply chain risks, faster builds, and less to audit. Every dependency above earns its place by providing substantial value that would require significant code to replicate.

### Installation

```bash
# Initialize module
go mod init github.com/efisher/reviewhub

# Core dependencies
go get github.com/google/go-github/v68
go get golang.org/x/oauth2
go get modernc.org/sqlite
go get github.com/golang-migrate/migrate/v4

# Test dependencies
go get github.com/stretchr/testify
```

**Important:** The `go-github` major version (v68) is approximate. Run `go get github.com/google/go-github/v68@latest` and if it fails, check the latest version with `go list -m -versions github.com/google/go-github/v68` or browse the GitHub releases page.

---

## Alternatives Considered (Summary Matrix)

| Category | Recommended | Runner-Up | Why Runner-Up Lost |
|----------|-------------|-----------|-------------------|
| HTTP Router | `net/http` (stdlib) | `go-chi/chi` | Stdlib routing sufficient since Go 1.22; chi adds dependency without proportional benefit |
| GitHub Client | `google/go-github` | Raw HTTP + `shurcooL/githubv4` | go-github provides typed structs, pagination, rate limiting for free |
| SQLite Driver | `modernc.org/sqlite` | `mattn/go-sqlite3` | Pure Go = no CGO = simpler Docker builds and cross-compilation |
| ORM/Query | `database/sql` (stdlib) | `jmoiron/sqlx` | Explicit SQL is clearer for hexagonal architecture; sqlx can be added later if boilerplate hurts |
| Config | `os.Getenv` (stdlib) | `koanf` | Five env vars do not justify a config library |
| Logging | `log/slog` (stdlib) | `uber-go/zap` | slog is the standard since Go 1.21; zap adds dependency for marginal perf gain |
| Testing | `testify` | `gomock` | Testify is simpler, more idiomatic, widely adopted |
| Migrations | `golang-migrate` | `pressly/goose` | Better programmatic API, broader adoption |
| Scheduling | `time.Ticker` (stdlib) | `robfig/cron` | Fixed-interval polling is a ticker, not a cron job |
| Docker Base | Alpine | Distroless | Shell access for debugging; trivial size difference |

---

## Technology Decisions for Hexagonal Architecture

The stack choices above are specifically tailored for hexagonal (ports & adapters) architecture:

**Domain Layer (zero dependencies on external libraries):**
- Pure Go types and interfaces
- No framework annotations, no ORM decorators, no struct tags from external libs
- Domain defines port interfaces (e.g., `PRRepository`, `GitHubClient`, `Poller`)

**Adapter Layer (where external libraries live):**
- HTTP adapter: `net/http` handlers implement the inbound port
- SQLite adapter: `database/sql` + `modernc.org/sqlite` implements the repository port
- GitHub adapter: `google/go-github` implements the GitHub client port
- Config adapter: `os.Getenv` reads environment and produces domain `Config`

**Why this matters:**
- The domain layer compiles and tests with ZERO external dependencies
- Adapters can be swapped (e.g., swap SQLite for Postgres) by writing a new adapter that satisfies the same port interface
- Tests mock ports, not implementations -- testify/mock generates mocks from port interfaces

---

## Sources and Confidence

| Recommendation | Source | Confidence | Notes |
|---|---|---|---|
| Go 1.22+ stdlib routing | Training data (Go 1.22 release notes, confirmed feature) | HIGH | Method-based routing in ServeMux is well-documented |
| `modernc.org/sqlite` pure Go | Training data (well-established library) | HIGH | Pure Go / no CGO is its defining feature |
| `google/go-github` | Training data (dominant GitHub client for Go) | HIGH for choice, MEDIUM for version | Version number increments frequently; verify latest |
| `log/slog` | Training data (added in Go 1.21, standard) | HIGH | Part of stdlib |
| `golang-migrate/migrate` | Training data (widely adopted) | HIGH | Stable v4 API |
| Alpine Docker images | Training data (standard practice) | HIGH | Long-standing best practice |
| SQLite WAL mode / PRAGMAs | Training data (SQLite documentation) | HIGH | Well-documented SQLite configuration |
| testify v1.9+ | Training data | MEDIUM | Version may have incremented; use latest |

**Verification needed at project init:**
1. Run `go get` commands and check that version numbers resolve
2. Confirm `go-github` latest major version
3. Confirm `golang:1.23-alpine` Docker tag exists (may need `1.22-alpine` or `1.24-alpine` depending on release timing)

---

*This stack prioritizes stdlib-first, minimal dependencies, and clean hexagonal architecture boundaries. Every external library earns its place by providing substantial value that would require hundreds of lines to replicate.*
