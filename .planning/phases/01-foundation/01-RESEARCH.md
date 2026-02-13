# Phase 1: Foundation - Research

**Researched:** 2026-02-10
**Domain:** Go project skeleton, hexagonal architecture, SQLite persistence, configuration, graceful shutdown
**Confidence:** HIGH

## Summary

Phase 1 establishes the innermost rings of the hexagonal architecture: domain entities (pure Go structs with zero dependencies), port interfaces (driven ports for persistence and GitHub client), SQLite persistence with WAL mode, environment-variable configuration with fail-fast validation, and graceful shutdown handling. This is a greenfield Go project with no existing code.

The standard approach is stdlib-first with minimal dependencies: `modernc.org/sqlite` (pure Go, no CGO) for persistence via `database/sql`, `golang-migrate/migrate` v4 with the `iofs` source and `sqlite` database driver for embedded migrations, `net/http` with Go 1.22+ enhanced routing for the HTTP skeleton, `log/slog` for structured logging, and `signal.NotifyContext` for shutdown orchestration. The domain layer has zero external dependencies.

Key findings: (1) `modernc.org/sqlite` registers as driver name `"sqlite"` with `database/sql`, and pragmas can be applied via DSN `_pragma` parameters or via `RegisterConnectionHook` (hook preferred for clarity); (2) golang-migrate has a dedicated `database/sqlite` driver that uses `modernc.org/sqlite` (NOT the `database/sqlite3` driver which requires CGO); (3) the dual sql.DB pattern (writer with `MaxOpenConns(1)`, reader with higher pool) prevents `SQLITE_BUSY` errors; (4) Go 1.22+ enhanced routing requires the `go` directive in `go.mod` to be set to `1.22` or higher, otherwise patterns like `"GET /path"` silently fail.

**Primary recommendation:** Build the domain model and ports first (zero dependencies), then the SQLite adapter with embedded migrations and WAL mode, then the config loader with fail-fast, then the graceful shutdown skeleton in `main.go` -- following the hexagonal dependency rule strictly inward-out.

## Standard Stack

The established libraries/tools for this phase:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.23+ | Language runtime | Latest stable; 1.22+ required for enhanced `net/http` routing |
| `modernc.org/sqlite` | v1.45+ | Pure Go SQLite driver for `database/sql` | No CGO, no `gcc` in Docker, cross-compilation works; registers as driver `"sqlite"` |
| `golang-migrate/migrate/v4` | v4.x (latest) | Schema migrations | Embedded SQL migrations via `iofs` source driver; has dedicated `database/sqlite` driver for `modernc.org/sqlite` |
| `database/sql` (stdlib) | Go stdlib | Database abstraction | Standard interface; `modernc.org/sqlite` registers as a driver |
| `net/http` (stdlib) | Go 1.22+ | HTTP server skeleton | Method-based routing (`"GET /path/{id}"`) since 1.22; no framework needed |
| `log/slog` (stdlib) | Go 1.21+ | Structured logging | Built-in, zero dependencies, JSON and text handlers |
| `os/signal` + `context` (stdlib) | Go stdlib | Graceful shutdown | `signal.NotifyContext` for SIGTERM/SIGINT handling |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` (stdlib) | Go stdlib | JSON serialization | HTTP response serialization in adapter layer |
| `net/http/httptest` (stdlib) | Go stdlib | HTTP handler testing | Testing the HTTP adapter |
| `github.com/stretchr/testify` | v1.9+ | Test assertions and mocks | `assert`/`require` for assertions, `mock` for port interface mocks |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `modernc.org/sqlite` | `mattn/go-sqlite3` | Requires CGO, needs `gcc` in Docker build, better raw performance but irrelevant at this scale |
| `golang-migrate/migrate` | `pressly/goose` | Also good; golang-migrate has a dedicated pure-Go SQLite driver and broader adoption |
| `golang-migrate/migrate` | Hand-rolled migration runner | Feasible for 5-10 tables but loses version tracking safety for negligible simplicity gain |
| `net/http` (stdlib) | `go-chi/chi` | Chi adds middleware chaining and route groups; stdlib is sufficient since Go 1.22; chi would add a dependency |
| `os.LookupEnv` (stdlib) | `koanf` or `envconfig` | Overkill for 5 env vars; stdlib is sufficient |

**Installation:**
```bash
go mod init github.com/efisher/reviewhub

# Core dependencies
go get modernc.org/sqlite
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/sqlite
go get github.com/golang-migrate/migrate/v4/source/iofs

# Test dependencies
go get github.com/stretchr/testify
```

## Architecture Patterns

### Recommended Project Structure
```
reviewhub/
|-- cmd/
|   +-- reviewhub/
|       +-- main.go                 # Composition root: wiring, config, shutdown
|
|-- internal/
|   |-- domain/
|   |   |-- model/
|   |   |   |-- pullrequest.go      # PullRequest entity
|   |   |   |-- repository.go       # Repository entity (watched repo)
|   |   |   |-- review.go           # Review entity
|   |   |   |-- reviewcomment.go    # ReviewComment entity
|   |   |   |-- checkstatus.go      # CheckStatus value object
|   |   |   +-- enums.go            # PRStatus, ReviewStatus, CIStatus enums
|   |   |
|   |   +-- port/
|   |       +-- driven/
|   |           |-- prstore.go      # PRStore interface
|   |           |-- repostore.go    # RepoStore interface
|   |           +-- githubclient.go # GitHubClient interface (defined now, implemented in Phase 2)
|   |
|   |-- adapter/
|   |   +-- driven/
|   |       +-- sqlite/
|   |           |-- db.go           # Connection setup, pragma config, dual reader/writer
|   |           |-- migrate.go      # Migration runner (iofs + embedded SQL)
|   |           |-- prrepo.go       # SQLite PRStore adapter
|   |           +-- reporepo.go     # SQLite RepoStore adapter
|   |
|   +-- config/
|       +-- config.go               # Env var loading, validation, fail-fast
|
|-- migrations/
|   |-- 000001_initial_schema.up.sql
|   +-- 000001_initial_schema.down.sql
|
|-- go.mod
+-- go.sum
```

**Key structural decisions:**
- `internal/` enforces Go's encapsulation boundary -- nothing outside the module can import these packages
- `domain/model/` has ZERO imports from any external library or any other internal package (except stdlib `time`)
- `domain/port/driven/` defines interfaces consumed by application services and implemented by adapters
- No `domain/port/driving/` in Phase 1 -- driving ports (use case interfaces) are deferred to Phase 3 when the HTTP adapter needs them
- No `internal/application/` in Phase 1 -- application services come in Phase 2+
- `migrations/` is at the project root, embedded into the binary via `//go:embed`

### Pattern 1: Domain Entities as Pure Structs

**What:** Domain model types are plain Go structs with behavior methods. No ORM tags, no JSON tags, no database annotations.
**When to use:** Always for domain layer entities.
**Example:**
```go
// internal/domain/model/pullrequest.go
package model

import "time"

type PRStatus string

const (
    PRStatusOpen   PRStatus = "open"
    PRStatusClosed PRStatus = "closed"
    PRStatusMerged PRStatus = "merged"
)

type PullRequest struct {
    ID             int64
    Number         int
    RepoFullName   string
    Title          string
    Author         string
    Status         PRStatus
    IsDraft        bool
    URL            string
    Branch         string
    BaseBranch     string
    OpenedAt       time.Time
    UpdatedAt      time.Time
    LastActivityAt time.Time
}

func (pr *PullRequest) DaysSinceOpened() int {
    return int(time.Since(pr.OpenedAt).Hours() / 24)
}

func (pr *PullRequest) IsStale(thresholdDays int) bool {
    return int(time.Since(pr.LastActivityAt).Hours()/24) > thresholdDays
}
```

### Pattern 2: Port Interfaces Defined Where Consumed

**What:** Interfaces are defined in the domain/port package. Adapters satisfy them implicitly (Go duck typing). Application services depend on these interfaces, never on concrete adapters.
**When to use:** All inter-layer dependencies.
**Example:**
```go
// internal/domain/port/driven/prstore.go
package driven

import (
    "context"
    "reviewhub/internal/domain/model"
)

type PRStore interface {
    Upsert(ctx context.Context, pr model.PullRequest) error
    GetByRepository(ctx context.Context, repoFullName string) ([]model.PullRequest, error)
    GetByStatus(ctx context.Context, status model.PRStatus) ([]model.PullRequest, error)
    GetByNumber(ctx context.Context, repoFullName string, number int) (*model.PullRequest, error)
    Delete(ctx context.Context, repoFullName string, number int) error
}
```

### Pattern 3: SQLite Dual Connection (Reader + Writer)

**What:** Create two `*sql.DB` instances from the same database file. The writer has `MaxOpenConns(1)` (SQLite allows only one writer). The reader has a larger pool for concurrent reads.
**When to use:** Always with SQLite + WAL mode when reads and writes can be concurrent.
**Example:**
```go
// internal/adapter/driven/sqlite/db.go
package sqlite

import (
    "context"
    "database/sql"
    "fmt"

    "modernc.org/sqlite"
)

type DB struct {
    Writer *sql.DB
    Reader *sql.DB
    path   string
}

func NewDB(dbPath string) (*DB, error) {
    dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-64000)", dbPath)

    writer, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open writer: %w", err)
    }
    writer.SetMaxOpenConns(1)

    reader, err := sql.Open("sqlite", dsn)
    if err != nil {
        writer.Close()
        return nil, fmt.Errorf("open reader: %w", err)
    }
    reader.SetMaxOpenConns(4) // Concurrent readers

    return &DB{Writer: writer, Reader: reader, path: dbPath}, nil
}

func (db *DB) Close() error {
    rErr := db.Reader.Close()
    wErr := db.Writer.Close()
    if wErr != nil {
        return wErr
    }
    return rErr
}
```

**Alternative approach (RegisterConnectionHook):** Instead of DSN pragmas, use `sqlite.RegisterConnectionHook()` to run pragma SQL on each new connection. This is clearer for many pragmas but the hook is global (affects all connections opened by the driver). DSN pragmas are per-connection-string and more explicit.

### Pattern 4: Embedded Migrations with golang-migrate

**What:** SQL migration files are embedded into the binary using `//go:embed` and run at startup via `golang-migrate` with `iofs` source driver and `sqlite` database driver.
**When to use:** Always for schema management.
**Example:**
```go
// internal/adapter/driven/sqlite/migrate.go
package sqlite

import (
    "database/sql"
    "embed"
    "errors"
    "fmt"

    "github.com/golang-migrate/migrate/v4"
    migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
    "github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(db *sql.DB) error {
    sourceDriver, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return fmt.Errorf("create migration source: %w", err)
    }

    dbDriver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
    if err != nil {
        return fmt.Errorf("create migration db driver: %w", err)
    }

    m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
    if err != nil {
        return fmt.Errorf("create migrator: %w", err)
    }

    if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return fmt.Errorf("run migrations: %w", err)
    }

    return nil
}
```

**Important note on embed location:** The `//go:embed` directive embeds files relative to the Go source file containing it. If migration SQL files live at `migrations/` in the project root, the embed directive must be in a Go file that can reference that relative path. Two common approaches: (a) put the embed directive and `RunMigrations` function in a file at the project root (e.g., `migrations/embed.go`), or (b) put migration SQL files inside `internal/adapter/driven/sqlite/migrations/` alongside the Go code. Approach (b) keeps migrations co-located with the adapter.

### Pattern 5: Configuration Loading with Fail-Fast

**What:** Load all required config from environment variables at startup. If any required variable is missing, print a clear error and exit immediately.
**When to use:** At the very start of `main()`, before any other initialization.
**Example:**
```go
// internal/config/config.go
package config

import (
    "fmt"
    "os"
    "time"
)

type Config struct {
    GitHubToken    string
    GitHubUsername string
    PollInterval   time.Duration
    ListenAddr     string
    DBPath         string
}

func Load() (*Config, error) {
    token, ok := os.LookupEnv("REVIEWHUB_GITHUB_TOKEN")
    if !ok || token == "" {
        return nil, fmt.Errorf("REVIEWHUB_GITHUB_TOKEN is required but not set")
    }

    username, ok := os.LookupEnv("REVIEWHUB_GITHUB_USERNAME")
    if !ok || username == "" {
        return nil, fmt.Errorf("REVIEWHUB_GITHUB_USERNAME is required but not set")
    }

    pollStr := os.Getenv("REVIEWHUB_POLL_INTERVAL")
    pollInterval := 5 * time.Minute
    if pollStr != "" {
        d, err := time.ParseDuration(pollStr)
        if err != nil {
            return nil, fmt.Errorf("REVIEWHUB_POLL_INTERVAL invalid duration %q: %w", pollStr, err)
        }
        pollInterval = d
    }

    listenAddr := os.Getenv("REVIEWHUB_LISTEN_ADDR")
    if listenAddr == "" {
        listenAddr = "127.0.0.1:8080"
    }

    dbPath := os.Getenv("REVIEWHUB_DB_PATH")
    if dbPath == "" {
        dbPath = "reviewhub.db"
    }

    return &Config{
        GitHubToken:    token,
        GitHubUsername: username,
        PollInterval:   pollInterval,
        ListenAddr:     listenAddr,
        DBPath:         dbPath,
    }, nil
}
```

### Pattern 6: Graceful Shutdown Orchestration

**What:** Use `signal.NotifyContext` to create a context that cancels on SIGINT/SIGTERM. Pass this context to all long-running components. On cancellation, shut down HTTP server, drain in-flight work, close database, then exit.
**When to use:** In `main.go`, as the lifecycle orchestrator.
**Example:**
```go
// cmd/reviewhub/main.go
package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "reviewhub/internal/config"
    dbpkg "reviewhub/internal/adapter/driven/sqlite"
)

func main() {
    if err := run(); err != nil {
        slog.Error("fatal error", "error", err)
        os.Exit(1)
    }
}

func run() error {
    // 1. Load configuration (fail fast)
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("config: %w", err)
    }
    slog.Info("config loaded", "listen", cfg.ListenAddr, "db", cfg.DBPath)

    // 2. Setup signal-based context
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    // 3. Open database
    db, err := dbpkg.NewDB(cfg.DBPath)
    if err != nil {
        return fmt.Errorf("database: %w", err)
    }
    defer db.Close()

    // 4. Run migrations (on writer connection)
    if err := dbpkg.RunMigrations(db.Writer); err != nil {
        return fmt.Errorf("migrations: %w", err)
    }
    slog.Info("migrations complete")

    // 5. Wire adapters and services (future phases add more here)
    // prStore := dbpkg.NewPRRepo(db)
    // repoStore := dbpkg.NewRepoRepo(db)

    // 6. Start HTTP server (future phases)
    // go server.ListenAndServe(...)

    // 7. Wait for shutdown signal
    <-ctx.Done()
    slog.Info("shutting down...")

    // 8. Graceful shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // server.Shutdown(shutdownCtx) -- future phases
    _ = shutdownCtx // suppress unused warning until HTTP server is added

    slog.Info("shutdown complete")
    return nil
}
```

### Anti-Patterns to Avoid

- **Leaking external types into domain:** Never import `modernc.org/sqlite` or `github.com/google/go-github` in `domain/model/` or `domain/port/`. Ports use only domain types and stdlib types.
- **Global database connection:** Never use `var db *sql.DB` at package level. Pass `*sql.DB` (or the `DB` wrapper) to adapter constructors.
- **Shared database models:** Never add `db:"column"` or `json:"field"` tags to domain structs. Each adapter defines its own data structures and maps to/from domain types.
- **Business logic in adapters:** SQLite adapter does SQL and mapping. No PR staleness calculation or status derivation in the persistence layer.
- **Enormous port interfaces:** Keep interfaces small and focused. `PRStore` has ~5 methods. `RepoStore` has ~4 methods. Do not combine them.
- **Over-engineering the domain layer:** Phase 1 domain entities are simple structs. Do not add complex domain services or use case orchestration yet. That comes in Phase 2+.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Schema migration tracking | Custom `schema_version` table + migration runner | `golang-migrate/migrate` v4 with `iofs` source + `sqlite` database driver | Migration ordering, dirty state handling, up/down support, version tracking |
| SQLite driver (pure Go) | Raw syscalls or CGO wrapper | `modernc.org/sqlite` via `database/sql` | Pure Go, well-tested, handles all SQLite edge cases |
| Signal handling | Manual `os.Signal` channel management | `signal.NotifyContext` (stdlib since Go 1.16) | Integrates cleanly with `context.Context` propagation |
| Structured logging | `fmt.Printf` or `log.Printf` | `log/slog` (stdlib since Go 1.21) | Structured, leveled, JSON output, zero dependencies |
| HTTP method routing | Custom dispatcher switch on `r.Method` | `net/http.ServeMux` with Go 1.22+ patterns | Built-in `"GET /path/{id}"` syntax, path parameters via `r.PathValue()` |
| Test assertions | `if got != want { t.Errorf(...) }` everywhere | `testify/assert` and `testify/require` | Dramatically reduces boilerplate, better error messages |

**Key insight:** Phase 1 is entirely stdlib + 3 external libraries (`modernc.org/sqlite`, `golang-migrate/migrate`, `testify`). Resist adding more.

## Common Pitfalls

### Pitfall 1: Go Module Version Directive Breaks Routing
**What goes wrong:** Enhanced `net/http` routing patterns like `"GET /health"` silently fail (404) if the `go` directive in `go.mod` is not set to `1.22` or higher. Go treats modules without an explicit version as Go 1.16, which uses the old routing behavior.
**Why it happens:** `go mod init` creates `go.mod` with whatever Go version is installed, but if edited manually or the directive is missing, the fallback is 1.16.
**How to avoid:** Ensure `go.mod` has `go 1.23` (or whatever version is installed). Verify with `go version` and check the `go` directive in `go.mod` matches.
**Warning signs:** Routes return 404 that should match. `"GET /path"` patterns treated as literal string paths.
**Confidence:** HIGH (verified via Go issue #69686)

### Pitfall 2: SQLite "database is locked" from Single sql.DB Pool
**What goes wrong:** A single `*sql.DB` instance has multiple connections in its pool. Multiple goroutines acquire different connections. One writes while another reads. Without WAL mode, readers block writers. Even with WAL mode, if two goroutines try to write simultaneously through different pool connections, one gets `SQLITE_BUSY`.
**Why it happens:** `database/sql` is designed for connection-pooling servers (Postgres, MySQL) where multiple connections to the same database are normal. SQLite is file-based and allows only one writer at a time.
**How to avoid:** Use the dual `sql.DB` pattern: writer with `MaxOpenConns(1)`, reader with a small pool (4-8). Set `PRAGMA busy_timeout=5000` so SQLite retries instead of failing immediately. Always enable WAL mode.
**Warning signs:** Intermittent "database is locked" errors; errors that only appear under concurrent access (polling + API reads).
**Confidence:** HIGH

### Pitfall 3: golang-migrate sqlite3 vs sqlite Driver Confusion
**What goes wrong:** Developer imports `github.com/golang-migrate/migrate/v4/database/sqlite3` (which requires `mattn/go-sqlite3` with CGO) instead of `github.com/golang-migrate/migrate/v4/database/sqlite` (which uses `modernc.org/sqlite`, pure Go). Build fails with CGO errors in Docker.
**Why it happens:** Older tutorials and Stack Overflow answers reference `sqlite3`. The `sqlite` driver (pure Go) was added later.
**How to avoid:** Use import path `github.com/golang-migrate/migrate/v4/database/sqlite` (no "3"). The driver name is `"sqlite"`. Verify by checking that `modernc.org/sqlite` is the only SQLite dependency in `go.sum`.
**Warning signs:** CGO compile errors, `gcc` required in Docker build, `mattn/go-sqlite3` appearing in `go.sum`.
**Confidence:** HIGH (verified via golang-migrate GitHub docs)

### Pitfall 4: Migrations Not Embedded -- Binary Missing SQL Files
**What goes wrong:** Migration SQL files exist on disk but are not embedded into the binary. The app runs fine in development (files are on disk) but fails in Docker (only the binary is copied to the runtime image).
**Why it happens:** Developer forgets the `//go:embed` directive, or the embed path does not match the actual file location.
**How to avoid:** Use `//go:embed migrations/*.sql` in the same package as (or a parent of) the migration SQL files. Verify by building the binary and running it from a different directory -- migrations should still work because they are inside the binary.
**Warning signs:** "no migration found" errors only in Docker or CI; works locally but not after `go build`.
**Confidence:** HIGH

### Pitfall 5: Pragmas Set After Connections Are Already Open
**What goes wrong:** Developer calls `db.Exec("PRAGMA journal_mode=WAL")` after `sql.Open()`, but `sql.Open` may have already opened a connection (or the pool reuses connections without the pragma). New connections from the pool do not inherit the pragma settings.
**Why it happens:** `PRAGMA` statements in SQLite are connection-scoped. `database/sql` opens new connections transparently. A pragma set on one connection does not apply to others.
**How to avoid:** Set pragmas via DSN parameters (`_pragma=journal_mode(WAL)`) so they apply to every connection. Alternatively, use `sqlite.RegisterConnectionHook()` to execute pragmas on every new connection. Do NOT rely on `db.Exec("PRAGMA ...")` alone.
**Warning signs:** WAL mode intermittently not active; `PRAGMA journal_mode` returns `delete` on some connections; foreign key constraints not enforced sporadically.
**Confidence:** HIGH

### Pitfall 6: Missing db.Close() on Shutdown Corrupts WAL
**What goes wrong:** Application exits without calling `db.Close()`. The WAL file is not checkpointed. On next startup, SQLite may need recovery, and in worst cases recent writes are lost.
**Why it happens:** SIGTERM kills the process before the deferred `db.Close()` runs, or the developer does not handle signals at all.
**How to avoid:** Use `signal.NotifyContext` to catch SIGTERM/SIGINT. Ensure `db.Close()` is called in the shutdown path (via `defer` in `run()` function that is controlled by the signal context). Set Docker `stop_grace_period` longer than the app's shutdown timeout.
**Warning signs:** WAL file growing unbounded; "database disk image is malformed" after container restart; container taking exactly 10 seconds to stop (Docker SIGKILL timeout).
**Confidence:** HIGH

### Pitfall 7: SQLite Migrations Cannot ALTER COLUMN
**What goes wrong:** Developer writes a migration with `ALTER TABLE ... ALTER COLUMN` or `ALTER TABLE ... DROP COLUMN` (on older SQLite versions). SQLite's `ALTER TABLE` is limited.
**Why it happens:** SQLite is not Postgres. It has minimal `ALTER TABLE` support.
**How to avoid:** For column changes, use the "create new table, copy data, drop old table, rename" pattern. Plan schema carefully upfront. `ALTER TABLE ... ADD COLUMN` is supported and safe.
**Warning signs:** Migration fails with "near ALTER: syntax error" or "no such column".
**Confidence:** HIGH

## Code Examples

Verified patterns from official sources:

### SQLite DSN with Pragmas
```go
// Source: modernc.org/sqlite documentation (pkg.go.dev)
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

dsn := "file:reviewhub.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
db, err := sql.Open("sqlite", dsn)
```

### Alternative: Connection Hook for Pragmas
```go
// Source: theitsolutions.io/blog/modernc.org-sqlite-with-go
import "modernc.org/sqlite"

const initSQL = `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA cache_size = -64000;
`

sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
    _, err := conn.ExecContext(context.Background(), initSQL, nil)
    return err
})
```

### Embedded Migration with iofs + sqlite
```go
// Source: golang-migrate/migrate GitHub docs + pkg.go.dev
import (
    "database/sql"
    "embed"
    "errors"
    "github.com/golang-migrate/migrate/v4"
    migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
    "github.com/golang-migrate/migrate/v4/source/iofs"
    _ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(db *sql.DB) error {
    srcDriver, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return err
    }
    dbDriver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
    if err != nil {
        return err
    }
    m, err := migrate.NewWithInstance("iofs", srcDriver, "sqlite", dbDriver)
    if err != nil {
        return err
    }
    if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return err
    }
    return nil
}
```

### Go 1.22+ HTTP Routing with Method Patterns
```go
// Source: go.dev/blog/routing-enhancements
mux := http.NewServeMux()

// Method-specific routes
mux.HandleFunc("GET /health", healthHandler)
mux.HandleFunc("GET /api/prs", listPRsHandler)
mux.HandleFunc("GET /api/prs/{repo}/{number}", getPRHandler)

// Path parameters accessed via r.PathValue()
func getPRHandler(w http.ResponseWriter, r *http.Request) {
    repo := r.PathValue("repo")
    number := r.PathValue("number")
    // ...
}
```

### Signal-Based Graceful Shutdown
```go
// Source: go.dev stdlib docs + victoriametrics.com/blog/go-graceful-shutdown
ctx, stop := signal.NotifyContext(context.Background(),
    os.Interrupt, syscall.SIGTERM)
defer stop()

// Pass ctx to all long-running goroutines
go pollService.Start(ctx)

// HTTP server with graceful shutdown
server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}
go func() {
    if err := server.ListenAndServe(); err != http.ErrServerClosed {
        slog.Error("http server error", "error", err)
    }
}()

<-ctx.Done()
slog.Info("shutting down")

shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
db.Close()
```

### Localhost-Only HTTP Binding (INFR-07)
```go
// Source: Go net/http docs
// Bind to 127.0.0.1 to ensure API is localhost only
server := &http.Server{
    Addr:    "127.0.0.1:8080",
    Handler: mux,
}
```

### Initial Schema Migration SQL
```sql
-- migrations/000001_initial_schema.up.sql
CREATE TABLE IF NOT EXISTS repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT NOT NULL UNIQUE,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pull_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    number INTEGER NOT NULL,
    repo_full_name TEXT NOT NULL,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    is_draft INTEGER NOT NULL DEFAULT 0,
    url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    base_branch TEXT NOT NULL DEFAULT '',
    opened_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    last_activity_at DATETIME NOT NULL,
    created_in_db_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repo_full_name) REFERENCES repositories(full_name) ON DELETE CASCADE,
    UNIQUE(repo_full_name, number)
);

CREATE INDEX idx_pull_requests_repo ON pull_requests(repo_full_name);
CREATE INDEX idx_pull_requests_status ON pull_requests(status);
CREATE INDEX idx_pull_requests_author ON pull_requests(author);
```

```sql
-- migrations/000001_initial_schema.down.sql
DROP TABLE IF EXISTS pull_requests;
DROP TABLE IF EXISTS repositories;
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `mattn/go-sqlite3` (CGO) | `modernc.org/sqlite` (pure Go) | Mature since ~2022 | No CGO, simpler Docker builds |
| `net/http` custom routing | `net/http.ServeMux` with method patterns | Go 1.22 (Feb 2024) | `"GET /path/{id}"` syntax, no third-party router needed |
| `log` (stdlib) | `log/slog` (stdlib) | Go 1.21 (Aug 2023) | Structured, leveled logging built-in |
| `golang-migrate` `sqlite3` driver (CGO) | `golang-migrate` `sqlite` driver (pure Go) | Available in golang-migrate | Import `database/sqlite` not `database/sqlite3` |
| `signal.Notify` + channel | `signal.NotifyContext` | Go 1.16 (Feb 2021) | Directly integrates with `context.Context` |
| `db.Exec("PRAGMA ...")` after Open | DSN `_pragma=` parameters or `RegisterConnectionHook` | modernc.org/sqlite feature | Pragmas apply to every connection in the pool |

**Deprecated/outdated:**
- `mattn/go-sqlite3`: Still works but requires CGO; `modernc.org/sqlite` is preferred for new projects
- `gorilla/mux`: Archived/maintenance-only since 2022; use stdlib `net/http` instead
- `sirupsen/logrus`: Legacy; use `log/slog` instead
- `golang-migrate` `database/sqlite3` driver: Requires CGO; use `database/sqlite` driver instead

## Open Questions

Things that could not be fully resolved:

1. **Exact go-github latest major version**
   - What we know: Was around v60-v68 in early-mid 2025. Increments frequently.
   - What's unclear: The exact current version as of Feb 2026.
   - Recommendation: Run `go list -m -versions github.com/google/go-github/v68` at project init. If v68 does not resolve, try higher numbers. This does NOT block Phase 1 since the GitHubClient port is just an interface definition -- no concrete implementation needed until Phase 2.

2. **modernc.org/sqlite RegisterConnectionHook vs DSN pragmas**
   - What we know: Both approaches work. DSN uses `_pragma=journal_mode(WAL)` syntax. Hook uses `sqlite.RegisterConnectionHook()`.
   - What's unclear: Whether the hook is scoped per-driver-instance or truly global (affecting all `sql.Open("sqlite", ...)` calls).
   - Recommendation: Use DSN `_pragma` parameters for simplicity and per-connection-string control. Fall back to hook only if DSN approach proves insufficient. LOW risk either way.

3. **golang-migrate sqlite driver transaction wrapping**
   - What we know: The sqlite driver wraps each migration in an implicit transaction. Migrations must NOT contain explicit `BEGIN`/`COMMIT`.
   - What's unclear: Behavior with multi-statement migrations (e.g., `CREATE TABLE` + `CREATE INDEX` in one file).
   - Recommendation: Test with the initial migration. If issues arise, set `NoTxWrap: true` in the driver config and wrap manually. LOW risk.

## Sources

### Primary (HIGH confidence)
- [modernc.org/sqlite - pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite) -- Driver name `"sqlite"`, DSN `_pragma` syntax, version v1.45.0
- [golang-migrate/migrate database/sqlite](https://github.com/golang-migrate/migrate/tree/master/database/sqlite) -- Pure Go SQLite driver, import path, config
- [golang-migrate/migrate source/iofs](https://pkg.go.dev/github.com/golang-migrate/migrate/v4/source/iofs) -- Embedded filesystem migration source
- [Go 1.22 Routing Enhancements](https://go.dev/blog/routing-enhancements) -- Method-based routing, path parameters
- [Go issue #69686](https://github.com/golang/go/issues/69686) -- go.mod version directive required for enhanced routing

### Secondary (MEDIUM confidence)
- [modernc.org/sqlite with Go - The IT Solutions](https://theitsolutions.io/blog/modernc.org-sqlite-with-go) -- Dual reader/writer pattern, RegisterConnectionHook, pragma configuration
- [Graceful Shutdown in Go: Practical Patterns - VictoriaMetrics](https://victoriametrics.com/blog/go-graceful-shutdown/) -- signal.NotifyContext, shutdown sequence
- [Graceful Shutdowns with signal.NotifyContext - millhouse.dev](https://millhouse.dev/posts/graceful-shutdowns-in-golang-with-signal-notify-context) -- Context-based shutdown

### Tertiary (LOW confidence)
- Training data for Go hexagonal architecture package layout conventions -- well-established patterns but not live-verified

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries verified via pkg.go.dev and official documentation
- Architecture: HIGH -- hexagonal architecture in Go is a well-established pattern; directory structure follows Go conventions
- Pitfalls: HIGH -- SQLite concurrency, WAL mode, and Go module version issues are well-documented and verified
- Code examples: HIGH -- all examples verified against official documentation or authoritative blog posts

**Research date:** 2026-02-10
**Valid until:** 2026-03-10 (30 days -- all technologies are stable)
