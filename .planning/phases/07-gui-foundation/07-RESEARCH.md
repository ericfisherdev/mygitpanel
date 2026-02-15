# Phase 7: GUI Foundation - Research

**Researched:** 2026-02-14
**Domain:** Server-side rendered web GUI with templ/HTMX/Alpine.js/Tailwind/GSAP
**Confidence:** MEDIUM-HIGH

## Summary

Phase 7 adds a read-only web dashboard to the existing Go API. The stack is templ (Go HTML templating with type safety and code generation), HTMX (HTML-driven AJAX via attributes), Alpine.js (lightweight client-side reactivity), Tailwind CSS (utility-first styling), and GSAP (animation). The alpine-morph HTMX extension is required to prevent Alpine.js state destruction during HTMX content swaps.

The core architectural challenge is cleanly separating the existing JSON API routes (`/api/v1/*`) from the new HTML-serving web routes (`/` and `/app/*`) while sharing the same domain ports and application services. The web handler will be a new driving adapter that renders templ components instead of JSON, consuming the same `driven.PRStore`, `driven.RepoStore`, and application services already wired in `main.go`.

**Primary recommendation:** Create a new `internal/adapter/driving/web/` package as a second driving adapter with its own handler, routes, and templ components. Static assets (CSS, JS libraries) should be embedded into the Go binary via `go:embed` for zero-dependency deployment in the existing scratch-based Docker image. Tailwind CSS should be compiled via the standalone CLI (no Node.js) during the Docker build stage.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| [templ](https://github.com/a-h/templ) | latest (v0.3.x) | Type-safe HTML templating for Go | Compiles to Go code, IDE support, no runtime reflection |
| [HTMX](https://htmx.org/) | 2.0.x | HTML-driven partial page updates | Attribute-based AJAX, no client JS build step |
| [Alpine.js](https://alpinejs.dev/) | 3.x | Client-side reactivity (theme toggle, sidebar collapse, search state) | Lightweight, no build step, x-data/x-show/x-on directives |
| [Alpine.js Morph Plugin](https://alpinejs.dev/plugins/morph) | 3.x | DOM morphing that preserves Alpine state | Required by alpine-morph HTMX extension |
| [Alpine.js Persist Plugin](https://alpinejs.dev/plugins/persist) | 3.x | localStorage persistence for Alpine state | Dark mode preference persistence (GUI-04) |
| [htmx-ext-alpine-morph](https://github.com/bigskysoftware/htmx-extensions/tree/main/src/alpine-morph) | 2.0.x | HTMX swap strategy using Alpine morph | Prevents Alpine state loss on HTMX swaps |
| [Tailwind CSS](https://tailwindcss.com/) | 4.x | Utility-first CSS framework | Standalone CLI (no Node.js), dark mode via `dark:` variant |
| [GSAP](https://gsap.com/) | 3.13.x | Animation library | Free (acquired by Webflow), professional-grade transitions |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Tailwind standalone CLI | 4.x | CSS compilation without Node.js | Build step in Dockerfile and local dev |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| templ | Go html/template | templ has type safety, IDE support, composability; html/template has no code gen step |
| Alpine.js | Vanilla JS | Alpine provides declarative reactivity; vanilla is dependency-free but more verbose |
| Tailwind standalone CLI | npm-based Tailwind | npm approach is better supported in v4 but adds Node.js dependency to Docker image |

**Installation:**

```bash
# templ CLI (code generation)
go install github.com/a-h/templ/cmd/templ@latest
# Or project-local (Go 1.24+):
go get -tool github.com/a-h/templ/cmd/templ@latest

# templ Go module dependency
go get github.com/a-h/templ

# Tailwind standalone CLI (no Node.js)
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64
mv tailwindcss-linux-x64 tailwindcss

# Frontend JS libs: served from vendored files via go:embed (no CDN in production)
# Download once into internal/adapter/driving/web/static/vendor/:
#   htmx.min.js (2.0.x)
#   alpine-morph.js (htmx-ext-alpine-morph 2.0.x)
#   alpinejs-morph CDN min (3.x)
#   alpinejs-persist CDN min (3.x)
#   alpinejs CDN min (3.x)
#   gsap.min.js (3.13.x)
```

## Architecture Patterns

### Recommended Project Structure

```
internal/adapter/driving/
  http/                           # Existing JSON API adapter (unchanged)
    handler.go
    response.go
    middleware.go
  web/                            # NEW: Web GUI driving adapter
    handler.go                    # Web handler (renders templ components)
    routes.go                     # Route registration for / and /app/*
    static/                       # Embedded static assets
      vendor/                     # Vendored JS libs (htmx, alpine, gsap)
        htmx.min.js
        alpine.min.js
        alpine-morph.min.js
        alpine-persist.min.js
        htmx-ext-alpine-morph.js
        gsap.min.js
      css/
        input.css                 # Tailwind input (source)
        output.css                # Tailwind output (generated, git-ignored)
    templates/                    # templ component files
      layout.templ                # Base HTML layout (head, scripts, body shell)
      components/                 # Shared UI components
        pr_card.templ             # PR list item card
        pr_detail.templ           # PR detail panel
        sidebar.templ             # Collapsible sidebar
        search_bar.templ          # Search/filter bar
        repo_manager.templ        # Add/remove repos
        theme_toggle.templ        # Dark/light toggle
      pages/
        dashboard.templ           # Full dashboard page (initial load)
      partials/                   # HTMX partial responses (no layout wrapper)
        pr_list.templ             # PR feed fragment
        pr_detail_content.templ   # PR detail fragment
        repo_list.templ           # Repo list fragment
```

### Pattern 1: Dual Driving Adapter (JSON + Web)

**What:** Two separate driving adapter packages share the same domain ports and application services. The JSON adapter returns `application/json`, the web adapter returns `text/html` via templ components.

**When to use:** When adding a GUI to an existing API without modifying the API layer.

**Example:**

```go
// cmd/mygitpanel/main.go - wire both adapters
mux := http.NewServeMux()

// JSON API routes (existing)
apiHandler := httphandler.NewHandler(prStore, repoStore, ...)
httphandler.RegisterRoutes(mux, apiHandler)

// Web GUI routes (new)
webHandler := webhandler.NewHandler(prStore, repoStore, reviewSvc, healthSvc, pollSvc, username, logger)
webhandler.RegisterRoutes(mux, webHandler)

// Static assets
mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(webhandler.StaticFS)))
```

### Pattern 2: templ Component with HTMX Attributes

**What:** templ components emit HTMX attributes that drive server interactions without JavaScript.

**When to use:** Any interactive element that fetches server data.

**Example:**

```go
// templates/components/pr_card.templ
templ PRCard(pr PRViewModel) {
    <div
        class="p-4 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700"
        hx-get={ fmt.Sprintf("/app/prs/%s/%d", pr.Repository, pr.Number) }
        hx-target="#pr-detail"
        hx-swap="morph"
        hx-ext="alpine-morph"
    >
        <h3 class="font-semibold text-gray-900 dark:text-white">{ pr.Title }</h3>
        <span class="text-sm text-gray-500">{ pr.Repository } #{ strconv.Itoa(pr.Number) }</span>
    </div>
}
```

### Pattern 3: Alpine.js for Client-Only State (Theme, Sidebar)

**What:** Use Alpine.js `x-data` with `$persist` for UI state that is purely client-side and does not need server round-trips.

**When to use:** Theme toggle (GUI-04), sidebar collapse (GUI-05).

**Example:**

```go
// templates/layout.templ
templ Layout(title string, contents templ.Component) {
    <html lang="en" x-data x-bind:class="$store.theme.dark ? 'dark' : ''">
    <head>
        <title>{ title }</title>
        <link rel="stylesheet" href="/static/css/output.css"/>
    </head>
    <body class="bg-white dark:bg-gray-900 text-gray-900 dark:text-white">
        @contents
        // Scripts: order matters - Alpine morph before Alpine core
        <script src="/static/vendor/htmx.min.js"></script>
        <script src="/static/vendor/htmx-ext-alpine-morph.js"></script>
        <script src="/static/vendor/alpine-morph.min.js" defer></script>
        <script src="/static/vendor/alpine-persist.min.js" defer></script>
        <script src="/static/vendor/alpine.min.js" defer></script>
        <script src="/static/vendor/gsap.min.js"></script>
        <script>
            document.addEventListener('alpine:init', () => {
                Alpine.store('theme', {
                    dark: Alpine.$persist(false).as('darkMode')
                });
            });
        </script>
    </body>
    </html>
}
```

### Pattern 4: GSAP Animations on HTMX Swap Events

**What:** Listen for `htmx:afterSwap` events to trigger GSAP animations on newly inserted content.

**When to use:** GUI-06 requirement (animated transitions on PR selection, tab switching, new data arrival).

**Example:**

```javascript
// Animate PR detail content after swap
document.addEventListener('htmx:afterSwap', (event) => {
    if (event.detail.target.id === 'pr-detail') {
        gsap.from('#pr-detail > *', {
            opacity: 0, y: 20, duration: 0.3, stagger: 0.05, ease: 'power2.out'
        });
    }
});
```

### Pattern 5: HTMX Search with Debounce

**What:** Use `hx-trigger="input changed delay:500ms"` for search-as-you-type without JavaScript.

**When to use:** GUI-03 (search and filter PRs).

**Example:**

```html
<input type="text" name="q"
    hx-get="/app/prs/search"
    hx-trigger="input changed delay:500ms"
    hx-target="#pr-list"
    hx-swap="morph"
    hx-ext="alpine-morph"
    placeholder="Search PRs..."/>
```

### Anti-Patterns to Avoid

- **Leaking go-github types into templ components:** Create view model structs in the web adapter, never pass domain models directly to templates.
- **Using innerHTML swap with Alpine components:** Always use `morph` swap via alpine-morph extension when the swapped content contains Alpine directives.
- **CDN script tags in production:** Vendor all JS libraries into `static/vendor/` and embed them. CDN adds external dependency and fails in air-gapped deployments.
- **Tailwind via Node.js in Docker:** Use the standalone CLI binary. Adding Node.js to the Docker build defeats the purpose of the Go-only deployment.
- **Putting templ files in the domain layer:** Templates are a presentation concern and belong in the driving adapter.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Dark mode toggle with persistence | Custom localStorage + class toggling JS | Alpine.js `$persist` + `$store` + Tailwind `dark:` variant | Edge cases: SSR flash, system preference detection, race conditions |
| DOM morphing that preserves state | Custom DOM diffing | alpine-morph HTMX extension | Browser state preservation (focus, scroll, form inputs) is extremely complex |
| CSS utility framework | Custom CSS with BEM/OOCSS | Tailwind CSS | Consistency, dark mode, responsive design all built-in |
| Search debouncing | Custom setTimeout/clearTimeout | HTMX `hx-trigger="input changed delay:500ms"` | Handles edge cases (rapid typing, network timing) |
| Animated transitions | Custom CSS keyframes or requestAnimationFrame | GSAP `gsap.from()`/`gsap.to()` with `htmx:afterSwap` | GSAP handles cross-browser, performance, easing, staggering |

**Key insight:** The templ/HTMX/Alpine.js stack is specifically designed so that each tool handles one concern. Templ owns HTML generation, HTMX owns server communication, Alpine owns client state, Tailwind owns styling, GSAP owns animation. Hand-rolling any of these crosses responsibility boundaries.

## Common Pitfalls

### Pitfall 1: Alpine State Destroyed on HTMX Swap

**What goes wrong:** HTMX default swap (`innerHTML`) replaces DOM elements, destroying Alpine.js reactive state. Theme toggle resets, sidebar collapses, form inputs lose values.
**Why it happens:** HTMX's default swap strategy does a full DOM replacement. Alpine.js binds state to specific DOM elements.
**How to avoid:** Use `hx-swap="morph"` with `hx-ext="alpine-morph"` on ALL elements that contain Alpine directives. Load scripts in correct order: htmx, htmx-ext-alpine-morph, alpine-morph plugin, alpine-persist plugin, alpine core.
**Warning signs:** State resets after clicking any HTMX-driven link. Theme flickers back to light mode.

### Pitfall 2: Script Loading Order Breaks Alpine Morph

**What goes wrong:** Alpine initializes before its morph plugin is loaded, or the HTMX extension loads before HTMX core.
**Why it happens:** Alpine.js must be loaded AFTER its plugins. HTMX extensions must load AFTER htmx.js.
**How to avoid:** Strict script order in layout.templ: (1) htmx.min.js, (2) htmx-ext-alpine-morph.js, (3) alpine-morph plugin (defer), (4) alpine-persist plugin (defer), (5) alpine core (defer). Alpine core MUST be last and use `defer`.
**Warning signs:** Console errors about undefined Alpine plugins. Morph not working despite correct attributes.

### Pitfall 3: Tailwind v4 Standalone CLI Content Scanning

**What goes wrong:** Tailwind v4 standalone CLI does not scan `.templ` files by default. Generated CSS is missing all utility classes used in templates.
**Why it happens:** Tailwind v4 changed from config-file-based content scanning to `@source` directives in the input CSS file.
**How to avoid:** Create `input.css` with explicit `@source` directives pointing to `.templ` files AND generated `_templ.go` files. The generated Go files contain the actual HTML strings that Tailwind needs to scan.
**Warning signs:** Styles missing in production but working in development (if dev uses different build).

```css
/* internal/adapter/driving/web/static/css/input.css */
@import "tailwindcss";
@source "../../templates/**/*.templ";
@source "../../templates/**/*_templ.go";
```

### Pitfall 4: templ Generate Not Run Before Go Build

**What goes wrong:** `go build` fails with missing `*_templ.go` files, or stale generated code serves old HTML.
**Why it happens:** templ files (`.templ`) are not Go files. The `templ generate` command must run first to produce `_templ.go` files that `go build` compiles.
**How to avoid:** Build pipeline must be: (1) `templ generate`, (2) `tailwindcss -i input.css -o output.css --minify`, (3) `go build`. Dockerfile must include templ CLI in the build stage.
**Warning signs:** Build errors referencing templ component functions that "don't exist."

### Pitfall 5: Scratch Docker Image Cannot Serve Static Files from Disk

**What goes wrong:** Static files copied to the scratch container are not accessible because there is no filesystem tooling.
**Why it happens:** The existing Dockerfile uses `FROM scratch` which has no shell, no file utilities.
**How to avoid:** Use `go:embed` to embed all static assets (CSS, JS, images) into the Go binary. Serve via `http.FileServerFS()` with the embedded filesystem. No files need to exist on disk at runtime.
**Warning signs:** 404 errors on all static asset requests in Docker.

### Pitfall 6: HTMX Checkbox/Radio State After Morph Swap

**What goes wrong:** Checkbox and radio button checked states do not update correctly after an alpine-morph swap.
**Why it happens:** The `checked` state is a DOM property, not an attribute. Alpine morph preserves properties from the existing element.
**How to avoid:** Use Alpine.js `x-bind:checked` for checkbox/radio state rather than relying on server-rendered `checked` attributes. Or use `:key` attributes to force element replacement.
**Warning signs:** Filters/checkboxes appear stuck after HTMX updates.

## Code Examples

Verified patterns from official sources:

### templ Component Rendering in HTTP Handler

```go
// Source: https://templ.guide/server-side-rendering/creating-an-http-server-with-templ/
func (h *WebHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
    prs, err := h.prStore.ListAll(r.Context())
    if err != nil {
        h.logger.Error("failed to list PRs", "error", err)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    viewModels := toViewModels(prs)
    component := pages.Dashboard(viewModels)
    component.Render(r.Context(), w)
}
```

### Embedding Static Assets

```go
// Source: https://pkg.go.dev/embed
package webhandler

import "embed"

//go:embed static/*
var StaticFS embed.FS

// In route registration:
// mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(StaticFS)))
```

### Alpine.js Dark Mode Store with Persist

```javascript
// Source: https://alpinejs.dev/plugins/persist
document.addEventListener('alpine:init', () => {
    Alpine.store('theme', {
        dark: Alpine.$persist(false).as('darkMode')
    });
});
```

### Tailwind v4 Input CSS for templ Projects

```css
/* Source: https://github.com/tailwindlabs/tailwindcss/discussions/15815 */
@import "tailwindcss";
@source "../../templates/**/*.templ";
@source "../../templates/**/*_templ.go";
@custom-variant dark (&:where(.dark, .dark *));
```

### HTMX Out-of-Band Swap for Multi-Region Updates

```html
<!-- Source: https://htmx.org/docs/ -->
<!-- When adding a repo, update both the repo list AND the PR feed -->
<form hx-post="/app/repos" hx-target="#repo-list" hx-swap="morph" hx-ext="alpine-morph">
    <input name="full_name" placeholder="owner/repo" />
    <button type="submit">Add</button>
</form>

<!-- Server response includes OOB swap for PR list -->
<!-- <div id="repo-list">...updated repo list...</div> -->
<!-- <div id="pr-list" hx-swap-oob="morph">...updated PR feed...</div> -->
```

### Dockerfile Build Stage with templ + Tailwind

```dockerfile
FROM golang:1.25-alpine AS build

# Install templ CLI
RUN go install github.com/a-h/templ/cmd/templ@latest

# Download Tailwind standalone CLI
RUN wget -O /usr/local/bin/tailwindcss \
    https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64-musl \
    && chmod +x /usr/local/bin/tailwindcss

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .

# Generate templ Go files
RUN templ generate

# Build Tailwind CSS
RUN tailwindcss -i internal/adapter/driving/web/static/css/input.css \
    -o internal/adapter/driving/web/static/css/output.css --minify

# Build Go binary (static assets embedded via go:embed)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/mygitpanel ./cmd/mygitpanel
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Go html/template | templ with code generation | 2023-2024 | Type safety, IDE support, component composition |
| jQuery/fetch for AJAX | HTMX attributes in HTML | 2020-present | No custom JS for server communication |
| Tailwind v3 config file | Tailwind v4 CSS-first config with @source | 2025 (v4.0) | Standalone CLI requires input CSS, not config.js |
| GSAP paid plugins | GSAP fully free (Webflow acquisition) | May 2025 (v3.13) | All plugins including SplitText, MorphSVG now free |
| Alpine.js morph as separate concern | alpine-morph HTMX extension | htmx 2.0 | Seamless Alpine state preservation during HTMX swaps |

**Deprecated/outdated:**
- Tailwind v3 `tailwind.config.js` content array: Replaced by `@source` directives in v4 input CSS
- HTMX 1.x extensions path: Extensions moved to separate `htmx-extensions` repo in HTMX 2.0
- `Alpine.plugin()` registration: Use CDN script tag order instead for browser builds

## Open Questions

1. **Tailwind v4 standalone CLI + templ file scanning reliability**
   - What we know: `@source` directives in input.css can point to `.templ` and `_templ.go` files
   - What's unclear: Whether the standalone CLI v4 reliably scans all class names from templ's Go-like syntax (e.g., conditional classes in `if` statements)
   - Recommendation: Test early in Phase 7. Fallback is scanning only the generated `_templ.go` files which contain pure string literals.

2. **go:embed with generated files (output.css, _templ.go)**
   - What we know: `go:embed` works with any files present at build time
   - What's unclear: Whether the build ordering (templ generate -> tailwind build -> go build with embed) works cleanly in the Dockerfile without race conditions
   - Recommendation: Dockerfile RUN steps are sequential, so ordering is deterministic. Verify with a test build early.

3. **GSAP animation timing with alpine-morph swaps**
   - What we know: `htmx:afterSwap` fires after DOM update; GSAP can animate from that event
   - What's unclear: Whether morph-based swaps (which modify in-place rather than replace) trigger `htmx:afterSwap` the same way as standard swaps
   - Recommendation: Test with a simple morph swap + GSAP animation during the first implementation plan. May need `htmx:afterSettle` instead.

## Sources

### Primary (HIGH confidence)
- `/a-h/templ` (Context7) - component creation, HTTP handler integration, static asset serving
- `/bigskysoftware/htmx` (Context7) - hx-trigger delay, hx-swap, out-of-band swaps, search patterns
- `/alpinejs/alpine` (Context7) - $persist, Alpine.store, x-show, morph plugin
- `/websites/gsap_v3` (Context7) - gsap.to, gsap.from, timeline sequencing
- `/websites/tailwindcss` (Context7) - dark mode, custom variants, CLI usage

### Secondary (MEDIUM confidence)
- [htmx-ext-alpine-morph README](https://github.com/bigskysoftware/htmx-extensions/blob/main/src/alpine-morph/README.md) - extension usage, script order, hx-swap="morph"
- [Tailwind v4 Go project discussion](https://github.com/tailwindlabs/tailwindcss/discussions/15815) - @source directive for templ files
- [templ Docker hosting guide](https://templ.guide/hosting-and-deployment/hosting-using-docker/) - multi-stage Docker build
- [GSAP licensing](https://css-tricks.com/gsap-is-now-completely-free-even-for-commercial-use/) - free for all use as of May 2025

### Tertiary (LOW confidence)
- [Alpine.js state loss discussion](https://github.com/alpinejs/alpine/discussions/4438) - checkbox state after morph (community report, needs validation)
- [Atomic Design with templ/HTMX/Alpine/Tailwind](https://riupress.pl/blog/atomic-design-with-templ-htmx-alpinejs-and-tailwind) - project structure patterns (blog post)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries verified via Context7 and official docs; versions confirmed
- Architecture: MEDIUM-HIGH - Dual driving adapter pattern follows existing hexagonal architecture; templ + HTMX + Alpine integration patterns verified from multiple sources
- Pitfalls: MEDIUM - Alpine state loss and script ordering well-documented; Tailwind v4 standalone CLI with templ is newer territory with fewer production reports

**Research date:** 2026-02-14
**Valid until:** 2026-03-14 (30 days - stack is relatively stable)
