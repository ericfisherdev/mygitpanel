# Stack Research: 2026.2.0 Web GUI + Jira + Review Submission

**Domain:** Web GUI for PR review dashboard with external API integrations
**Researched:** 2026-02-14
**Confidence:** HIGH (versions verified against pkg.go.dev, GitHub releases, CDN registries)

**Scope:** This document covers ONLY the stack additions for milestone 2026.2.0. The existing v1.0 stack (Go 1.25, modernc.org/sqlite, go-github/v82, golang-migrate, etc.) is validated and unchanged.

---

## New Stack Additions

### 1. Templating: a-h/templ

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/a-h/templ` | v0.3.977 | Type-safe HTML templating | Compiles `.templ` files to Go code at build time. Templates are Go functions that return `templ.Component` (implements `io.WriterTo`). Full compile-time type safety means broken templates fail at build, not at runtime. Integrates directly with `net/http` via `templ.Handler()` and `component.Render(ctx, w)`. No reflection, no parsing at runtime. |

**Version confidence:** HIGH -- v0.3.977 verified on pkg.go.dev (published 2025-12-31).

**Integration with existing architecture:**
- templ components satisfy `http.Handler` via `templ.Handler(component)`, so they plug directly into the existing `http.ServeMux`
- A new `internal/adapter/driving/web/` package will hold templ-based handlers alongside the existing `internal/adapter/driving/http/` JSON API
- templ components accept Go types as parameters -- pass domain model structs directly (or thin view-model wrappers) without JSON serialization
- The `templ generate` command runs as a build step, producing `*_templ.go` files that are committed to the repo

**CLI tool required:**
```bash
go install github.com/a-h/templ/cmd/templ@v0.3.977
```

The `templ` CLI is needed for `templ generate` (compiles `.templ` -> `.go`) and `templ fmt` (formats templ files). Add to CI/CD pipeline.

---

### 2. Dynamic Updates: HTMX (CDN)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| HTMX | 2.0.8 | Server-driven dynamic UI updates | Adds `hx-get`, `hx-post`, `hx-swap` attributes to HTML elements, enabling partial page updates without writing JavaScript. The server returns HTML fragments (templ components), not JSON. This keeps all rendering logic server-side in Go, which is consistent with the hexagonal architecture -- the web adapter renders HTML instead of JSON. |

**Delivery:** CDN script tag in the base layout templ component. No npm, no bundler, no node_modules.

```html
<script src="https://unpkg.com/htmx.org@2.0.8" integrity="sha384-..." crossorigin="anonymous"></script>
```

**Why HTMX 2.x, not 4.x:** HTMX 4.0 is expected early-to-mid 2026 but will not be marked "latest" until early 2027. Stick with the stable 2.0.x line. The upgrade path from 2.x to 4.x is straightforward (response code handling changes, improved event model).

**Go helper library:**

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/angelofallars/htmx-go` | v0.5.0 | Type-safe HTMX response headers | Use for setting HTMX response headers (`HX-Retarget`, `HX-Reswap`, `HX-Trigger`) in Go handlers. Provides `htmx.IsHTMX(r)` to detect HTMX requests vs. full page loads. Has built-in `RenderTempl()` for combining header setting with templ component rendering. Uses standard `net/http` types -- no framework coupling. |

**Version confidence:** MEDIUM -- v0.5.0 was the latest on pkg.go.dev (published 2024-02-05). May have newer versions; verify with `go get github.com/angelofallars/htmx-go@latest`.

---

### 3. Client-Side Interactivity: Alpine.js (CDN)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Alpine.js | 3.15.x | Lightweight client-side state and interactions | Handles UI state that does not warrant a server round-trip: dropdown toggles, tab switching, modal open/close, form validation feedback, optimistic UI updates. Declared inline with `x-data`, `x-show`, `x-on` attributes -- no build step, no component files, no virtual DOM. Complements HTMX: Alpine handles local UI state, HTMX handles server communication. |

**Delivery:** CDN script tag.

```html
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.15/dist/cdn.min.js"></script>
```

**Version confidence:** HIGH -- 3.15.8 confirmed as latest on npm (published ~2026-02-02).

**Scope boundary:** Alpine.js should ONLY manage ephemeral UI state (is-this-dropdown-open, which-tab-is-active). Any state that needs to persist or be shared across page sections should go through HTMX to the server. This prevents the classic SPA trap of duplicating server state on the client.

---

### 4. Styling: Tailwind CSS v4 (Standalone CLI)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Tailwind CSS | v4.1.x | Utility-first CSS framework | Tailwind v4 uses a CSS-first configuration model (no `tailwind.config.js`). The standalone CLI (`@tailwindcss/standalone`) is a single binary -- no Node.js, no npm, no package.json. This keeps the Go project free of JavaScript toolchain dependencies. Scans `.templ` files for class usage and produces optimized CSS. |

**Delivery:** Standalone CLI binary downloaded to project tooling.

```bash
# Download standalone CLI for Linux x64
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64
mv tailwindcss-linux-x64 ./tools/tailwindcss
```

**Configuration (v4 style -- CSS-only, no JS config):**

```css
/* internal/adapter/driving/web/static/input.css */
@import "tailwindcss";
@source "../**/*.templ";
```

**Build command:**
```bash
./tools/tailwindcss -i internal/adapter/driving/web/static/input.css -o internal/adapter/driving/web/static/dist/styles.css --minify
```

**Version confidence:** HIGH -- v4.1.18 confirmed on GitHub releases (published ~2025-12-xx). The standalone CLI bundles popular plugins (@tailwindcss/forms, @tailwindcss/typography) automatically.

**Docker integration:** The standalone CLI binary runs during the Docker build stage. The output CSS file is embedded into the Go binary via `//go:embed` for serving from the scratch runtime image.

---

### 5. Animations: GSAP (CDN)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| GSAP | 3.14.x | Performant UI animations | Handles smooth transitions for PR card state changes, attention signal pulses, and review status updates. GSAP became 100% free (including all plugins) in April 2025 after Webflow's acquisition. The standard license covers commercial use. Framework-agnostic -- works with any DOM elements, pairs naturally with HTMX's `htmx:afterSwap` events for animating newly inserted content. |

**Delivery:** CDN script tag.

```html
<script src="https://cdn.jsdelivr.net/npm/gsap@3.14/dist/gsap.min.js"></script>
```

**Version confidence:** HIGH -- 3.14.2 confirmed on npm/jsDelivr.

**License note:** Free for all uses EXCEPT building no-code visual animation tools that compete with Webflow. MyGitPanel is a PR dashboard -- no licensing concern.

**Integration pattern:** Listen for HTMX lifecycle events to trigger animations:
```javascript
document.addEventListener('htmx:afterSwap', (event) => {
    gsap.from(event.detail.target, { opacity: 0, y: 20, duration: 0.3 });
});
```

---

### 6. Jira Integration: ctreminiom/go-atlassian

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/ctreminiom/go-atlassian/v2` | v2.10.0 | Jira Cloud REST API client | Supports Jira v2 and v3 REST APIs with typed structs, pagination helpers, and multiple auth methods (Basic, OAuth 2.0, PAT). The API design mirrors go-github's patterns (service-based client, typed request/response structs), which is familiar to this codebase. Covers the endpoints we need: issue search (JQL), issue details, add comment. MIT licensed, actively maintained (published 2026-01-26). |

**Version confidence:** HIGH -- v2.10.0 verified on pkg.go.dev (published 2026-01-26).

**Integration with hexagonal architecture:**
- Define a new `driven.JiraClient` port interface in the domain layer
- Implement with go-atlassian in `internal/adapter/driven/jira/`
- The domain never imports go-atlassian types -- the adapter translates to domain models
- Same pattern as the existing GitHub adapter

**Required endpoints:**

| Jira API Operation | go-atlassian Service | Domain Use Case |
|---|---|---|
| Search issues (JQL) | `jira.Issue.Search.Post()` | Find Jira tickets linked to PRs (by branch name, PR title, commit messages) |
| Get issue details | `jira.Issue.Get()` | Display ticket summary, status, priority alongside PR |
| Add comment | `jira.Issue.Comment.Add()` | Post PR status updates to linked Jira tickets |

**Why NOT other Jira clients:**

| Alternative | Why Not |
|-------------|---------|
| `andygrunwald/go-jira` | v1 is stable but aging; v2 is in development with breaking changes on main branch. go-atlassian is more actively maintained and covers Jira v3 API. |
| Raw HTTP calls | Jira's REST API has complex pagination, authentication, and error handling. go-atlassian provides this for free with strong typing. |
| `essentialkaos/go-jira` | Smaller community, less comprehensive API coverage. |

---

### 7. GitHub Review Submission (Existing Library -- New Port Methods)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `google/go-github/v82` | v82.0.0 (existing) | Submit PR reviews via GitHub REST API | **Already in go.mod.** The `PullRequestsService.CreateReview()` method accepts a `PullRequestReviewRequest` with Event field values: `"APPROVE"`, `"REQUEST_CHANGES"`, `"COMMENT"`. No new dependency needed -- just extend the existing `driven.GitHubClient` port interface with write methods. |

**New port methods to add to `driven.GitHubClient`:**

```go
// SubmitReview creates a pull request review (approve, request changes, or comment).
SubmitReview(ctx context.Context, repoFullName string, prNumber int, event string, body string) error

// CreateReviewComment posts an inline code comment on a pull request.
CreateReviewComment(ctx context.Context, repoFullName string, prNumber int, body string, path string, line int) error
```

**Implementation uses existing go-github types:**

```go
func (c *Client) SubmitReview(ctx context.Context, repoFullName string, prNumber int, event string, body string) error {
    owner, repo, err := splitRepo(repoFullName)
    if err != nil {
        return err
    }
    _, _, err = c.gh.PullRequests.CreateReview(ctx, owner, repo, prNumber, &gh.PullRequestReviewRequest{
        Event: gh.Ptr(event),
        Body:  gh.Ptr(body),
    })
    return err
}
```

---

## Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `templ` CLI (v0.3.977) | Compile `.templ` to `.go`, format templ files | `go install github.com/a-h/templ/cmd/templ@v0.3.977`. Run `templ generate` before `go build`. |
| Tailwind CSS standalone CLI (v4.1.x) | Compile utility CSS from templ class usage | Download binary to `./tools/`. Run before `go build` to produce output CSS. |
| `air` (cosmtrek/air) | Hot reload during development | Watches `.templ` and `.go` files, re-runs `templ generate` and `go build`. Optional dev convenience, not a production dependency. |

---

## Installation

```bash
# New Go dependencies
go get github.com/a-h/templ@v0.3.977
go get github.com/angelofallars/htmx-go@latest
go get github.com/ctreminiom/go-atlassian/v2@v2.10.0

# templ CLI (build tool)
go install github.com/a-h/templ/cmd/templ@v0.3.977

# Tailwind standalone CLI (build tool -- no npm)
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64
mkdir -p tools && mv tailwindcss-linux-x64 tools/tailwindcss

# Frontend libraries -- served via CDN, no installation needed
# HTMX 2.0.8:     https://unpkg.com/htmx.org@2.0.8
# Alpine.js 3.15:  https://cdn.jsdelivr.net/npm/alpinejs@3.15/dist/cdn.min.js
# GSAP 3.14:       https://cdn.jsdelivr.net/npm/gsap@3.14/dist/gsap.min.js
```

---

## What NOT to Add

These are explicitly excluded from the stack.

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Node.js / npm / package.json | Adds an entire JavaScript toolchain to a Go project. Tailwind standalone CLI and CDN-served libraries eliminate the need. | Tailwind standalone CLI + CDN links |
| React / Vue / Svelte (SPA frameworks) | Massive complexity for a dashboard app. Requires bundler, API serialization layer, client state management, hydration. Templ + HTMX achieves the same UX with server-side rendering. | templ + HTMX + Alpine.js |
| Webpack / Vite / esbuild (bundlers) | No JavaScript to bundle. All JS is served via CDN. CSS is compiled by Tailwind standalone. | CDN + Tailwind CLI |
| `html/template` (stdlib) | No type safety. Template errors are runtime panics. String-based template references are fragile. templ provides compile-time safety with better DX. | a-h/templ |
| `andygrunwald/go-jira` | v1 is aging, v2 has unstable main branch. go-atlassian is more actively maintained with broader Atlassian API coverage. | ctreminiom/go-atlassian/v2 |
| WebSocket libraries | HTMX's `hx-trigger="every 30s"` provides polling-based live updates without WebSocket complexity. The existing 5-minute GitHub poll interval means data changes slowly -- WebSocket push adds complexity without proportional benefit. | HTMX polling triggers |
| Gorilla WebSocket | See above. If real-time becomes necessary later, HTMX has SSE extension support built in. | HTMX SSE extension (future) |

---

## Stack Patterns by Variant

**For full-page navigation (dashboard views, settings pages):**
- templ renders complete HTML page with layout wrapper
- HTMX handles partial updates within the page
- Alpine.js manages local UI state (dropdowns, tabs)

**For HTMX partial updates (PR list refresh, review actions):**
- Handler checks `htmx.IsHTMX(r)` to return HTML fragment vs. full page
- templ component renders JUST the changed section
- GSAP animates the swap via `htmx:afterSwap` event

**For API consumers (existing CLI integration):**
- Existing `/api/v1/*` JSON endpoints remain unchanged
- Web GUI routes live under `/` and `/app/*`
- Both share the same domain ports and application services

---

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| templ v0.3.x | Go 1.21+ | Uses generics internally; our Go 1.25 is well above minimum |
| go-atlassian v2.10.0 | Go 1.20+ | No compatibility concerns with Go 1.25 |
| htmx-go v0.5.0 | HTMX 2.x | Supports HTMX 2.0 header conventions |
| HTMX 2.0.8 | Alpine.js 3.x | No conflicts -- they operate in different scopes (server comm vs. local state) |
| Tailwind CSS v4.1.x | templ files | Standalone CLI scans `.templ` files for class names via `@source` directive |

---

## Updated Dependency Summary

### Direct Dependencies After 2026.2.0 (go.mod)

```
# Existing (unchanged)
github.com/google/go-github/v82         -- GitHub REST API client
github.com/gofri/go-github-ratelimit/v2 -- Rate limit middleware
github.com/gregjones/httpcache           -- ETag caching
github.com/golang-migrate/migrate/v4     -- Database migrations
modernc.org/sqlite                       -- Pure Go SQLite
github.com/stretchr/testify              -- Test assertions

# NEW for 2026.2.0
github.com/a-h/templ                     -- Type-safe HTML templates
github.com/angelofallars/htmx-go         -- HTMX header helpers
github.com/ctreminiom/go-atlassian/v2    -- Jira Cloud API client
```

**Dependency count goes from 6 to 9 direct dependencies.** Three additions, each providing substantial value:
- templ: entire templating engine with compile-time safety
- htmx-go: type-safe HTMX headers (small but prevents header string typos)
- go-atlassian: full Jira API client (would be hundreds of lines to hand-write)

---

## Sources

- [a-h/templ pkg.go.dev](https://pkg.go.dev/github.com/a-h/templ) -- v0.3.977 verified (HIGH confidence)
- [a-h/templ GitHub releases](https://github.com/a-h/templ/releases) -- release history (HIGH confidence)
- [HTMX releases](https://github.com/bigskysoftware/htmx/releases) -- v2.0.8 confirmed (HIGH confidence)
- [Alpine.js npm](https://www.npmjs.com/package/alpinejs) -- v3.15.8 confirmed (HIGH confidence)
- [Tailwind CSS releases](https://github.com/tailwindlabs/tailwindcss/releases) -- v4.1.18 confirmed (HIGH confidence)
- [Tailwind standalone CLI docs](https://tailwindcss.com/docs/installation/tailwind-cli) -- standalone binary approach (HIGH confidence)
- [GSAP cdnjs](https://cdnjs.com/libraries/gsap) -- v3.14.2 confirmed (HIGH confidence)
- [GSAP licensing](https://gsap.com/licensing/) -- free for commercial use confirmed (HIGH confidence)
- [go-atlassian pkg.go.dev](https://pkg.go.dev/github.com/ctreminiom/go-atlassian/v2) -- v2.10.0 verified (HIGH confidence)
- [go-atlassian GitHub](https://github.com/ctreminiom/go-atlassian) -- Jira v2/v3 API support (HIGH confidence)
- [go-github pulls_reviews.go](https://github.com/google/go-github/blob/master/github/pulls_reviews.go) -- CreateReview API (HIGH confidence)
- [angelofallars/htmx-go pkg.go.dev](https://pkg.go.dev/github.com/angelofallars/htmx-go) -- v0.5.0 (MEDIUM confidence -- may have newer)
- [templ + HTMX integration patterns](https://tailbits.com/blog/setting-up-htmx-and-templ-for-go) -- community reference (MEDIUM confidence)

---

*Stack research for: MyGitPanel 2026.2.0 Web GUI + Jira + Review Submission*
*Researched: 2026-02-14*
