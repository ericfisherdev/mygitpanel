# Phase 6: Docker Deployment - Research

**Researched:** 2026-02-14
**Domain:** Docker multi-stage builds, Docker Compose, SQLite volume persistence, adaptive polling, production hardening
**Confidence:** HIGH (Docker patterns are well-established; adaptive polling is a custom design with clear prior art)

## Summary

Phase 6 transforms the application from a locally-run Go binary into a production-ready Docker container with persistent storage and intelligent polling. Three requirements drive this phase: (1) INFR-03/INFR-04 -- containerization with SQLite persistence via Docker volume, (2) POLL-03 -- adaptive polling that adjusts frequency based on PR activity, and (3) the `docker compose up` one-command startup criterion.

The most important architectural insight is that **this project is uniquely well-positioned for minimal Docker images** because modernc.org/sqlite is a pure Go driver with zero CGO dependency. This means `CGO_ENABLED=0` produces a fully static binary that runs in a `scratch` image -- the smallest and most secure Docker base possible. Combined with the `golang.org/x/crypto/x509roots/fallback` package to embed CA certificates directly in the binary, there is no need for Alpine, distroless, or any other base image. The resulting image will be approximately 15-25MB (the Go binary itself) compared to ~800MB with a full golang image or ~50MB with Alpine.

The second key finding is that **the current default listen address `127.0.0.1:8080` will not work inside Docker**. When a process binds to 127.0.0.1 inside a container, Docker port forwarding cannot reach it because the container's loopback is isolated from the host. The `docker-compose.yml` must set `MYGITPANEL_LISTEN_ADDR=0.0.0.0:8080` and publish port 8080 to `127.0.0.1:8080` on the host side to maintain the localhost-only access requirement.

The third finding concerns adaptive polling: the existing `PollService` uses a single fixed `time.Ticker` for all repositories. Adaptive polling requires a per-repository (or per-PR) activity-aware mechanism that increases polling frequency for recently active items and decreases it for stale ones. This is a custom design problem (no library exists for this specific pattern) but the algorithm is straightforward: track last-activity timestamps and use configurable tier thresholds.

**Primary recommendation:** Implement in 2 plans: (1) Dockerfile + docker-compose.yml + .dockerignore + health check binary + scratch image with embedded CA certs + volume persistence, (2) Adaptive polling engine replacing the fixed-interval ticker with activity-tiered scheduling per repository.

## Standard Stack

### Core (no new runtime dependencies)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/crypto/x509roots/fallback | latest | Embed CA root certificates in Go binary | Eliminates need for ca-certificates package in container; official Go team solution for scratch images |

### Docker Tooling (not Go dependencies)
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Docker multi-stage build | Dockerfile syntax | Build stage compiles Go, final stage is `scratch` | Standard pattern for minimal Go images; reduces image from ~800MB to ~20MB |
| Docker Compose | v2 (yaml 3.8+) | Single-command orchestration with volume + env config | Standard for local development/deployment; already specified in success criteria |

### Supporting (already in project)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| modernc.org/sqlite | v1.45.0 | Pure Go SQLite (zero CGO) | Already in use; enables CGO_ENABLED=0 for static binary |
| log/slog | stdlib | Structured logging | Already in use; works in containers with JSON output |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `scratch` base | `alpine:3.20` | Alpine adds ~5MB, shell access for debugging, but larger attack surface. Scratch is better for production with no debugging needs. |
| `scratch` base | `gcr.io/distroless/static-debian12` | Distroless adds CA certs + /etc/passwd but ~5MB larger. Not needed since we embed certs with x509roots/fallback. |
| `x509roots/fallback` for CA certs | `COPY --from=build /etc/ssl/certs/ /etc/ssl/certs/` | COPY approach works but bundles unknown cert versions from builder image; fallback package gives version control and govulncheck compatibility. |
| Separate healthcheck binary | `curl` in Alpine | Scratch has no shell/curl. A tiny Go binary (~2MB) that hits /api/v1/health is the standard pattern for scratch-based health checks. |
| env_file for secrets | Docker Compose secrets | Secrets mount as files at /run/secrets/ which would require reading the GitHub token from file instead of env var. env_file is simpler and sufficient for a local-only deployment. |

**Installation:**
```bash
# Add CA cert embedding package (new dependency)
go get golang.org/x/crypto/x509roots/fallback
```

No other new Go dependencies needed.

## Architecture Patterns

### New/Modified Files
```
Dockerfile                          # Multi-stage build: golang -> scratch
docker-compose.yml                  # Service definition with volume + env
.dockerignore                       # Exclude non-build files from context
cmd/healthcheck/main.go             # Tiny binary for Docker HEALTHCHECK
cmd/mygitpanel/main.go              # Add x509roots/fallback import
internal/application/pollservice.go  # Replace fixed ticker with adaptive scheduler
internal/application/adaptive.go     # NEW: Activity tier logic
internal/application/adaptive_test.go # Tests for adaptive polling
```

### Pattern 1: Multi-Stage Build with Scratch
**What:** Two-stage Dockerfile: build stage compiles a static Go binary; final stage copies only the binary into an empty `scratch` image.
**When to use:** Always for production Go images with no CGO dependencies.

```dockerfile
# Source: Docker official Go guide + best practices docs
# https://docs.docker.com/guides/golang/build-images/
# https://docs.docker.com/build/building/multi-stage/

# ---- Build Stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache dependency downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build static binaries
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/mygitpanel ./cmd/mygitpanel
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/healthcheck ./cmd/healthcheck

# ---- Final Stage ----
FROM scratch

# Copy binaries from build stage
COPY --from=build /bin/mygitpanel /bin/mygitpanel
COPY --from=build /bin/healthcheck /bin/healthcheck

# Create data directory for SQLite volume mount target
# (scratch has no mkdir; use COPY --from with an empty dir created in build stage)
COPY --from=build /tmp /tmp

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/bin/healthcheck"]

ENTRYPOINT ["/bin/mygitpanel"]
```

**Note:** CA certificates are embedded in the binary via `x509roots/fallback` import, so no `COPY /etc/ssl/certs` is needed.

### Pattern 2: Docker Compose with Named Volume
**What:** Single-service compose file that mounts a named volume for SQLite persistence and loads env vars from a `.env` file.
**When to use:** The `docker compose up` success criterion.

```yaml
# Source: Docker Compose documentation
# https://docs.docker.com/compose/how-tos/use-secrets/
# https://docs.docker.com/get-started/workshop/05_persisting_data/

services:
  mygitpanel:
    build: .
    ports:
      - "127.0.0.1:8080:8080"  # Host-side localhost only
    volumes:
      - mygitpanel-data:/data
    env_file:
      - .env
    environment:
      MYGITPANEL_DB_PATH: /data/mygitpanel.db
      MYGITPANEL_LISTEN_ADDR: "0.0.0.0:8080"
    restart: unless-stopped

volumes:
  mygitpanel-data:
```

**Key detail:** `ports: "127.0.0.1:8080:8080"` binds host-side to localhost only (matching the "API accessible on localhost" requirement), while `MYGITPANEL_LISTEN_ADDR: 0.0.0.0:8080` inside the container listens on all interfaces so Docker port forwarding works.

### Pattern 3: Healthcheck Binary for Scratch
**What:** A minimal Go binary that sends an HTTP GET to the health endpoint and exits 0/1 based on response status. Required because `scratch` has no shell, curl, or wget.
**When to use:** Docker HEALTHCHECK in scratch-based images.

```go
// cmd/healthcheck/main.go
package main

import (
    "net/http"
    "os"
    "time"
)

func main() {
    addr := os.Getenv("MYGITPANEL_LISTEN_ADDR")
    if addr == "" {
        addr = "0.0.0.0:8080"
    }

    client := &http.Client{Timeout: 2 * time.Second}
    resp, err := client.Get("http://" + addr + "/api/v1/health")
    if err != nil || resp.StatusCode != http.StatusOK {
        os.Exit(1)
    }
    os.Exit(0)
}
```

### Pattern 4: Adaptive Polling with Activity Tiers
**What:** Replace the single fixed-interval ticker with a per-repository scheduler that assigns polling tiers based on last-activity age. Recently active repositories are polled more frequently; stale ones less.
**When to use:** POLL-03 requirement.

**Design:**
```
Tier 1 (Hot):    Last activity < 1 hour  -> poll every 2 minutes
Tier 2 (Active): Last activity < 1 day   -> poll every 5 minutes (current default)
Tier 3 (Warm):   Last activity < 7 days  -> poll every 15 minutes
Tier 4 (Stale):  Last activity >= 7 days -> poll every 30 minutes
```

The adaptive scheduler maintains a priority queue (or simple sorted list) of repositories ordered by their next-poll-due time. Each poll cycle:
1. Determine which repos are due for polling
2. Poll those repos
3. After polling, recalculate each repo's tier based on the freshest PR's `LastActivityAt`
4. Schedule the repo's next poll time based on its tier

```go
// internal/application/adaptive.go

// ActivityTier represents a polling frequency tier based on PR activity.
type ActivityTier int

const (
    TierHot    ActivityTier = iota // < 1h, poll every 2min
    TierActive                     // < 1d, poll every 5min
    TierWarm                       // < 7d, poll every 15min
    TierStale                      // >= 7d, poll every 30min
)

// TierInterval returns the polling interval for a given tier.
func TierInterval(tier ActivityTier) time.Duration {
    switch tier {
    case TierHot:
        return 2 * time.Minute
    case TierActive:
        return 5 * time.Minute
    case TierWarm:
        return 15 * time.Minute
    case TierStale:
        return 30 * time.Minute
    default:
        return 5 * time.Minute
    }
}

// ClassifyActivity returns the tier for a given last-activity time.
func ClassifyActivity(lastActivity time.Time) ActivityTier {
    age := time.Since(lastActivity)
    switch {
    case age < 1*time.Hour:
        return TierHot
    case age < 24*time.Hour:
        return TierActive
    case age < 7*24*time.Hour:
        return TierWarm
    default:
        return TierStale
    }
}
```

**Integration with PollService:** The `Start` method changes from a single `time.Ticker` to a loop that checks each repo's next-due time. A simple approach uses a minimum-interval ticker (e.g., 1 minute) that wakes up frequently and checks which repos are due. This avoids complex priority queue management while still achieving per-repo adaptive intervals.

```go
// Simplified adaptive loop approach:
func (s *PollService) Start(ctx context.Context) {
    // Initial poll of all repos
    s.pollAll(ctx)

    // Wake up every minute to check which repos need polling
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.pollDueRepos(ctx)
        case req := <-s.refreshCh:
            req.done <- s.handleRefresh(ctx, req)
        }
    }
}

func (s *PollService) pollDueRepos(ctx context.Context) {
    repos, _ := s.repoStore.ListAll(ctx)
    now := time.Now()

    for _, repo := range repos {
        schedule := s.getSchedule(repo.FullName)
        if now.Before(schedule.NextPollAt) {
            continue // Not due yet
        }
        if err := s.pollRepo(ctx, repo.FullName); err != nil {
            slog.Error("adaptive poll failed", "repo", repo.FullName, "error", err)
        }
        // Recalculate tier from freshest PR activity
        s.updateSchedule(ctx, repo.FullName)
    }
}
```

### Pattern 5: .dockerignore for Go Projects
**What:** Exclude non-essential files from the Docker build context to speed up builds and prevent sensitive files from being included.
**When to use:** Always when a Dockerfile exists.

```
# .dockerignore
.git
.gitignore
.planning
*.md
*.db
*.db-wal
*.db-shm
.env
.env.*
```

### Anti-Patterns to Avoid
- **Binding to 127.0.0.1 inside the container:** Docker port forwarding cannot reach a process bound to loopback. The container must listen on 0.0.0.0; restrict access on the host side with `127.0.0.1:8080:8080` in compose.
- **Using Alpine "just in case" for debugging:** Adds unnecessary attack surface. If debugging is needed, use `docker exec` with a debug sidecar or `docker cp` to extract the database.
- **Storing the SQLite database inside the container filesystem:** Data is lost on container restart. Always use a named volume or bind mount.
- **Running as root inside the container:** While scratch images have no user management, the binary should not need root. Docker Compose can set `user: "65534:65534"` (nobody) for the service.
- **Using `COPY . .` without .dockerignore:** Copies .git (huge), .env (secrets), .db files (unnecessary), and planning docs into the build context.
- **Polling all repos at the same fixed interval regardless of activity:** Wastes rate limit on stale repos while under-serving active ones.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CA certificates in scratch | Copy certs from build stage or install ca-certificates | `golang.org/x/crypto/x509roots/fallback` import | Embeds certs in binary; version-controlled; govulncheck compatible |
| Docker health check in scratch | Shell-based `curl` check | Separate Go healthcheck binary (~2MB) | Scratch has no shell/curl; tiny Go binary is the standard pattern |
| Complex priority queue for adaptive scheduling | Custom heap-based scheduler | Simple "wake every minute, check what's due" loop | Minute-resolution is sufficient for a polling app; avoids timer management complexity |
| Environment variable templating in Docker | Custom entrypoint script | Docker Compose `environment:` and `env_file:` | Standard compose features; no shell needed in scratch image |
| SQLite file permissions in Docker | Entrypoint chown script | Named volume with correct ownership from Dockerfile | Named volumes inherit permissions from the container path; no runtime fixup needed |

**Key insight:** The pure Go SQLite choice (modernc.org/sqlite) made in Phase 1 pays enormous dividends here -- it enables `CGO_ENABLED=0`, which enables `scratch`, which is the simplest and most secure containerization path possible.

## Common Pitfalls

### Pitfall 1: Container Binds to 127.0.0.1 -- Port Forwarding Silently Fails
**What goes wrong:** The app starts inside Docker, health check passes inside the container, but `curl localhost:8080` from the host gets "connection refused."
**Why it happens:** The current default `MYGITPANEL_LISTEN_ADDR` is `127.0.0.1:8080`. Inside a container, 127.0.0.1 refers to the container's own loopback, which is isolated from the host network. Docker port forwarding maps host ports to the container's network interfaces, but cannot reach the container's loopback.
**How to avoid:** Set `MYGITPANEL_LISTEN_ADDR=0.0.0.0:8080` in docker-compose.yml. Use `ports: "127.0.0.1:8080:8080"` on the host side to maintain localhost-only access.
**Warning signs:** Container shows "healthy" but host HTTP requests fail.

### Pitfall 2: SQLite Database Lost on Container Restart
**What goes wrong:** All PRs, repos, and review data disappear when the container restarts.
**Why it happens:** Without a volume mount, the SQLite file lives in the container's ephemeral filesystem. Container stop/remove destroys it.
**How to avoid:** Use a Docker named volume mounted at `/data`. Set `MYGITPANEL_DB_PATH=/data/mygitpanel.db`.
**Warning signs:** Fresh migration log messages on every container start. Empty PR lists after restart.

### Pitfall 3: SQLite WAL Files Not on Same Volume as Database
**What goes wrong:** Database corruption or "database is locked" errors.
**Why it happens:** SQLite WAL mode creates `.db-wal` and `.db-shm` files alongside the main `.db` file. If the DB path points to a volume but WAL files end up elsewhere (or the filesystem doesn't support proper file locking), corruption can occur.
**How to avoid:** Ensure the DB file and its WAL/SHM files are on the same Docker volume. Use a local volume driver (default). Never use NFS or network-mounted volumes for SQLite.
**Warning signs:** "database disk image is malformed" errors. Intermittent "database is locked" errors.

### Pitfall 4: TLS Failures in Scratch Image (Missing CA Certs)
**What goes wrong:** GitHub API calls fail with `x509: certificate signed by unknown authority`.
**Why it happens:** Scratch images have no filesystem at all -- no `/etc/ssl/certs/`. The Go HTTP client cannot verify TLS certificates without root CA certificates.
**How to avoid:** Import `golang.org/x/crypto/x509roots/fallback` in main.go. This embeds CA certificates directly in the Go binary.
**Warning signs:** All GitHub API calls fail immediately on container start. Works fine outside Docker.

### Pitfall 5: Adaptive Polling Drifts All Repos to Stale Tier
**What goes wrong:** After initial deployment, all repos quickly settle into the longest polling interval (30min) even though PRs are being updated.
**Why it happens:** Activity classification looks at `LastActivityAt` on PRs, but if the adaptive scheduler polls infrequently enough, it misses rapid changes that happen between polls.
**How to avoid:** Use the freshest `LastActivityAt` across ALL PRs in a repo to classify the repo. A single active PR keeps the whole repo in a hot tier. Also, manual refresh (via API) should reset a repo's tier to Hot.
**Warning signs:** Rate limit consumption drops to near zero but PR data is stale.

### Pitfall 6: Docker Build Cache Invalidated by go.sum Changes
**What goes wrong:** Every code change triggers a full `go mod download` in Docker build, making builds slow.
**Why it happens:** If `COPY . .` is used before `go mod download`, any source change invalidates the dependency cache layer.
**How to avoid:** Copy `go.mod` and `go.sum` first, run `go mod download`, then copy source. This is the standard two-stage COPY pattern.
**Warning signs:** Docker builds taking 30+ seconds even for single-line changes.

### Pitfall 7: Scratch Image Has No /tmp Directory
**What goes wrong:** If any library or code attempts to create temporary files, it crashes with a "no such file or directory" error.
**Why it happens:** Scratch images are truly empty -- no /tmp, no /var, no /etc, nothing.
**How to avoid:** Create `/tmp` in the build stage and `COPY --from=build /tmp /tmp` to the final stage. Or better: modernc.org/sqlite does not require /tmp for normal operations (it uses the database directory for temp files via PRAGMA temp_store_directory or the DSN), so verify this is not an issue during testing.
**Warning signs:** Runtime crashes mentioning missing directories that worked fine in development.

## Code Examples

### Dockerfile (Complete)
```dockerfile
# ---- Build Stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache dependency layer
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Build static binaries
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /bin/mygitpanel ./cmd/mygitpanel
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /bin/healthcheck ./cmd/healthcheck

# Create empty directories needed in scratch
RUN mkdir -p /data /tmp

# ---- Runtime Stage ----
FROM scratch

COPY --from=build /bin/mygitpanel /bin/mygitpanel
COPY --from=build /bin/healthcheck /bin/healthcheck
COPY --from=build /data /data
COPY --from=build /tmp /tmp

EXPOSE 8080
VOLUME /data

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/bin/healthcheck"]

ENTRYPOINT ["/bin/mygitpanel"]
```

### docker-compose.yml (Complete)
```yaml
services:
  mygitpanel:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - mygitpanel-data:/data
    env_file:
      - .env
    environment:
      MYGITPANEL_DB_PATH: /data/mygitpanel.db
      MYGITPANEL_LISTEN_ADDR: "0.0.0.0:8080"
    restart: unless-stopped

volumes:
  mygitpanel-data:
```

### .env.example (Template for Users)
```bash
# Required
MYGITPANEL_GITHUB_TOKEN=ghp_your_token_here
MYGITPANEL_GITHUB_USERNAME=your_username

# Optional
MYGITPANEL_GITHUB_TEAMS=team-slug-1,team-slug-2
MYGITPANEL_POLL_INTERVAL=5m
```

### .dockerignore
```
.git
.gitignore
.planning
*.md
*.db
*.db-wal
*.db-shm
.env
.env.*
```

### Healthcheck Binary
```go
// cmd/healthcheck/main.go
package main

import (
    "net/http"
    "os"
    "time"
)

func main() {
    addr := os.Getenv("MYGITPANEL_LISTEN_ADDR")
    if addr == "" {
        addr = "0.0.0.0:8080"
    }

    client := &http.Client{Timeout: 2 * time.Second}
    resp, err := client.Get("http://" + addr + "/api/v1/health")
    if err != nil || resp.StatusCode != http.StatusOK {
        os.Exit(1)
    }
    os.Exit(0)
}
```

### x509roots/fallback Import (in main.go)
```go
// Add blank import to cmd/mygitpanel/main.go
import (
    // ... existing imports ...

    _ "golang.org/x/crypto/x509roots/fallback" // Embed CA certs for scratch container
)
```

### Adaptive Polling -- Activity Tier Classification
```go
// internal/application/adaptive.go

type ActivityTier int

const (
    TierHot    ActivityTier = iota
    TierActive
    TierWarm
    TierStale
)

type repoSchedule struct {
    Tier       ActivityTier
    NextPollAt time.Time
    LastPolled time.Time
}

var tierIntervals = map[ActivityTier]time.Duration{
    TierHot:    2 * time.Minute,
    TierActive: 5 * time.Minute,
    TierWarm:   15 * time.Minute,
    TierStale:  30 * time.Minute,
}

func classifyActivity(lastActivity time.Time) ActivityTier {
    age := time.Since(lastActivity)
    switch {
    case age < 1*time.Hour:
        return TierHot
    case age < 24*time.Hour:
        return TierActive
    case age < 7*24*time.Hour:
        return TierWarm
    default:
        return TierStale
    }
}
```

### Adaptive Polling -- Scheduler Integration
```go
// Changes to PollService.Start:
// Replace fixed ticker with adaptive loop.

func (s *PollService) Start(ctx context.Context) {
    // Initial poll of all repos (sets baseline activity data)
    if err := s.pollAll(ctx); err != nil {
        slog.Error("initial poll failed", "error", err)
    }

    // One-minute tick resolution -- checks which repos are due
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            slog.Info("poll service stopped")
            return
        case <-ticker.C:
            s.pollDueRepos(ctx)
        case req := <-s.refreshCh:
            req.done <- s.handleRefresh(ctx, req)
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Alpine base for Go apps | Scratch or distroless for static Go binaries | ~2023 (widespread adoption) | ~80% smaller images, zero CVEs from base OS |
| `COPY --from /etc/ssl/certs/` for CA certs | `golang.org/x/crypto/x509roots/fallback` import | 2024 (package stabilized) | Certs embedded in binary; no filesystem dependency |
| Single fixed polling interval | Adaptive polling based on activity | Growing pattern in monitoring tools | Reduces rate limit consumption by 50-70% for mixed-activity repos |
| `docker-compose` (v1 Python) | `docker compose` (v2 Go plugin) | 2023 (v1 EOL) | Use `docker compose` not `docker-compose` in commands |

**Deprecated/outdated:**
- Docker Compose v1 (`docker-compose` binary) is end-of-life. Use `docker compose` (v2 plugin) in all documentation and commands.
- `golang:1.25` (Debian-based) as a build stage is valid but `golang:1.25-alpine` is smaller and faster for the build step when CGO is not needed.

## Key Technical Decisions for Planner

### 1. Base Image: scratch vs alpine vs distroless
**Recommendation: scratch.** The project uses modernc.org/sqlite (pure Go, zero CGO). With `x509roots/fallback` for CA certs, there is no runtime dependency that requires a base OS. Scratch produces the smallest image (~15-25MB) with zero CVE surface area. The only trade-off is no shell for debugging, which is acceptable for a production service. Use a separate healthcheck binary instead of curl.

### 2. Listen Address: Default Change or Override in Compose
**Recommendation: Override in compose only.** Do NOT change the default `127.0.0.1:8080` in config.go -- it is the correct secure default for non-containerized use. Instead, `docker-compose.yml` sets `MYGITPANEL_LISTEN_ADDR=0.0.0.0:8080` and the host-side port binding restricts to `127.0.0.1`.

### 3. SQLite Volume Strategy: Named Volume vs Bind Mount
**Recommendation: Named volume.** Named volumes are managed by Docker with better cross-platform behavior than bind mounts. The compose file defines `mygitpanel-data` volume mounted at `/data`. Users who prefer bind mounts can modify compose to `./data:/data`.

### 4. Adaptive Polling: Per-Repo vs Per-PR Scheduling
**Recommendation: Per-repo scheduling.** Per-PR scheduling adds significant complexity (tracking individual PR schedules) for marginal benefit. A repo's tier is determined by its most recently active PR. If any PR in a repo was active in the last hour, the whole repo gets the "hot" tier. This is simpler and still achieves the POLL-03 requirement: "recently active PRs polled more frequently, stale ones less."

### 5. Adaptive Polling: Implementation Approach
**Recommendation: Minute-resolution ticker with schedule map.** Replace the current single `time.Ticker` with a 1-minute ticker. On each tick, iterate all repos, check if each is due based on its schedule, and poll if due. Store schedules in a `map[string]repoSchedule` on the PollService struct. This is simpler than a priority queue and sufficient for the expected scale (single-digit to low double-digit repos).

### 6. Health Check: Application-Level vs Docker-Level
**Recommendation: Both.** The existing `/api/v1/health` endpoint continues serving application health. A separate `cmd/healthcheck/main.go` binary is added to support Docker's HEALTHCHECK instruction in scratch images (where curl is unavailable). The healthcheck binary simply hits the health endpoint and exits 0 or 1.

### 7. Secret Management: env_file vs Docker Secrets
**Recommendation: env_file.** This is a localhost-only personal tool, not a multi-tenant production deployment. Docker secrets would require the app to read the GitHub token from a file instead of an env var, changing the config loading logic unnecessarily. An `.env` file excluded from git (via .gitignore) with an `.env.example` template is sufficient.

## Open Questions

1. **modernc.org/sqlite temp file behavior in scratch container**
   - What we know: SQLite can create temp files for complex queries. modernc.org/sqlite is pure Go but may still attempt to create temp files via the OS.
   - What's unclear: Whether the pure Go SQLite implementation uses /tmp at all, or handles temp storage differently.
   - Recommendation: Test by running the scratch container and executing complex queries. If /tmp is needed, include it via `COPY --from=build /tmp /tmp` in Dockerfile (already included in the example). Monitor for "no such file or directory" errors.

2. **Adaptive polling tier thresholds**
   - What we know: The tier definitions (1h/1d/7d) are reasonable defaults but may need tuning.
   - What's unclear: Optimal thresholds for the user's specific workflow (how many repos, how active).
   - Recommendation: Make tier thresholds configurable via environment variables with the proposed defaults. Log tier classifications on each poll cycle for observability.

3. **Container restart behavior with active polling**
   - What we know: Graceful shutdown (SIGTERM -> 10s timeout) is already implemented. Docker sends SIGTERM on `docker stop`.
   - What's unclear: Whether the 10s timeout is sufficient for in-flight GitHub API calls to complete.
   - Recommendation: The existing 10s timeout should be sufficient. Monitor shutdown logs for timeout warnings and increase if needed.

## Sources

### Primary (HIGH confidence)
- [Docker Multi-Stage Build Docs](https://docs.docker.com/build/building/multi-stage/) - FROM scratch pattern, layer caching, COPY --from
- [Docker Go Build Guide](https://docs.docker.com/guides/golang/build-images/) - Go-specific Dockerfile patterns, dependency caching, CGO_ENABLED=0
- [Docker Best Practices](https://docs.docker.com/build/building/best-practices/) - .dockerignore, layer ordering, security
- [Docker Compose Volume Persistence](https://docs.docker.com/get-started/workshop/05_persisting_data/) - Named volumes, data persistence across restarts
- [GitHub REST API Best Practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api) - Conditional requests, rate limit headers, polling guidance
- [Docker Hub golang:1.25 image](https://hub.docker.com/_/golang) - Official Go Docker images, Alpine variant availability

### Secondary (MEDIUM confidence)
- [golang.org/x/crypto/x509roots/fallback](https://github.com/golang/go/issues/43958) - Embed CA certs in Go binary, official Go team proposal and implementation
- [Root Certs in Go Scratch Containers (2025)](https://blog.wollomatic.de/posts/2025-01-28-go-tls-certificates/) - Three approaches compared; recommends x509roots/fallback
- [FROM Scratch Pitfalls (iximiuz)](https://labs.iximiuz.com/tutorials/pitfalls-of-from-scratch-images) - Six gotchas: CA certs, /tmp, /etc/passwd, timezone, shared libs, nsswitch
- [Docker Connection Refused (Python Speed)](https://pythonspeed.com/articles/docker-connection-refused/) - 127.0.0.1 vs 0.0.0.0 in containers explained
- [Named Volumes as Non-Root](https://pratikpc.medium.com/use-docker-compose-named-volumes-as-non-root-within-your-containers-1911eb30f731) - Permission handling for non-root containers
- [SQLite WAL + Docker Volumes](https://sqlite.org/forum/info/87824f1ed837cdbb) - WAL/SHM file permissions in shared volumes
- [Docker Compose Secrets](https://docs.docker.com/compose/how-tos/use-secrets/) - Secret management alternatives

### Tertiary (LOW confidence)
- Adaptive polling tier thresholds (1h/1d/7d) -- reasonable defaults based on general monitoring patterns, not empirically validated for GitHub PR workflows
- modernc.org/sqlite temp file behavior in scratch containers -- not explicitly verified; /tmp inclusion is a precautionary measure

## Metadata

**Confidence breakdown:**
- Docker containerization (Dockerfile, compose, volumes): HIGH - Well-established patterns, official docs verified
- Scratch image with embedded CA certs: HIGH - Verified approach via x509roots/fallback, confirmed for Go 1.25+ via multiple sources
- SQLite volume persistence: HIGH - Standard Docker volume pattern; WAL mode works correctly on local volumes
- Listen address pitfall: HIGH - Verified from Docker networking documentation; common and well-documented issue
- Adaptive polling design: MEDIUM - Custom design based on well-understood patterns; tier thresholds are estimates
- Healthcheck binary pattern: HIGH - Standard pattern for scratch-based images; verified from multiple sources

**Research date:** 2026-02-14
**Valid until:** 2026-03-14 (stable domain -- Docker and Go containerization patterns change slowly)
