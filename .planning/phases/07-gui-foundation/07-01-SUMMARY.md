---
phase: 07-gui-foundation
plan: 01
subsystem: ui
tags: [templ, htmx, alpine.js, tailwindcss, gsap, go-embed]

# Dependency graph
requires:
  - phase: 05-pr-health-signals
    provides: "HTTP handler with JSON API routes, middleware stack"
provides:
  - "Web GUI driving adapter with templ-based HTML rendering at /"
  - "Vendored JS libraries (htmx, Alpine.js, GSAP) embedded via go:embed"
  - "Tailwind CSS pipeline with @source scanning for templ files"
  - "RegisterAPIRoutes and ApplyMiddleware extracted for dual-adapter wiring"
  - "Dockerfile with templ generate -> tailwindcss -> go build pipeline"
affects: [07-02, 07-03, 08-github-write-ops]

# Tech tracking
tech-stack:
  added: [github.com/a-h/templ, htmx 2.0.4, Alpine.js 3.14.9, GSAP 3.13.0, htmx-ext-alpine-morph 2.0.1]
  patterns: [templ components for HTML rendering, dual driving adapter pattern, vendored JS via go:embed]

key-files:
  created:
    - internal/adapter/driving/web/handler.go
    - internal/adapter/driving/web/routes.go
    - internal/adapter/driving/web/embed.go
    - internal/adapter/driving/web/viewmodel.go
    - internal/adapter/driving/web/templates/layout.templ
    - internal/adapter/driving/web/templates/pages/dashboard.templ
    - internal/adapter/driving/web/static/css/input.css
    - internal/adapter/driving/web/static/js/animations.js
  modified:
    - cmd/mygitpanel/main.go
    - internal/adapter/driving/http/handler.go
    - Dockerfile
    - .gitignore
    - go.mod

key-decisions:
  - "Removed explicit templ import from layout.templ to avoid duplicate import in generated code"
  - "Extracted RegisterAPIRoutes and ApplyMiddleware from NewServeMux for dual-adapter composition"

patterns-established:
  - "Dual driving adapter: web package serves HTML at /, httphandler serves JSON at /api/v1/*"
  - "Templ components: layout.templ wraps page components, pages/ subpackage for route-specific templates"
  - "Vendored JS: no npm/node, curl-downloaded minified libs in static/vendor/"
  - "Build pipeline: templ generate -> tailwindcss -> go build (Dockerfile codifies this)"

# Metrics
duration: 3min
completed: 2026-02-14
---

# Phase 7 Plan 01: Web GUI Scaffold Summary

**Dual driving adapter with templ layout, vendored htmx/Alpine/GSAP, Tailwind CSS pipeline, and Docker build stages**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-15T03:45:56Z
- **Completed:** 2026-02-15T03:49:24Z
- **Tasks:** 2
- **Files modified:** 22

## Accomplishments

- Vendored 6 JS libraries (htmx, Alpine.js core + morph + persist, GSAP, htmx-ext-alpine-morph) with no npm dependency
- Created web GUI adapter scaffold with templ layout, dark mode wiring, and dashboard placeholder
- Refactored HTTP handler to support dual driving adapters (HTML + JSON) on single mux
- Updated Dockerfile with templ generate and tailwindcss compilation pipeline

## Task Commits

Each task was committed atomically:

1. **Task 1: Vendor JS libraries and create Tailwind input CSS** - `a9eec7e` (feat)
2. **Task 2: Create web adapter scaffold with templ layout, handler, routes, embed, and Dockerfile update** - `797f8cc` (feat)

## Files Created/Modified

- `internal/adapter/driving/web/handler.go` - Web GUI handler with Dashboard method rendering templ components
- `internal/adapter/driving/web/routes.go` - Route registration for GET /{$} and /static/ file serving
- `internal/adapter/driving/web/embed.go` - go:embed directive for static assets filesystem
- `internal/adapter/driving/web/viewmodel.go` - PageData struct for layout template data
- `internal/adapter/driving/web/templates/layout.templ` - Base HTML layout with script loading order and dark mode store
- `internal/adapter/driving/web/templates/pages/dashboard.templ` - Placeholder dashboard with sidebar and main content areas
- `internal/adapter/driving/web/static/css/input.css` - Tailwind input with @source directives for templ scanning
- `internal/adapter/driving/web/static/js/animations.js` - GSAP animation listeners for htmx:afterSwap events
- `internal/adapter/driving/web/static/vendor/*.js` - 6 vendored JS libraries
- `cmd/mygitpanel/main.go` - Wired dual driving adapters (web + HTTP)
- `internal/adapter/driving/http/handler.go` - Extracted RegisterAPIRoutes and ApplyMiddleware
- `Dockerfile` - Added templ generate and tailwindcss build stages
- `.gitignore` - Added output.css exclusion
- `go.mod` / `go.sum` - Added github.com/a-h/templ dependency

## Decisions Made

- Removed explicit `import "github.com/a-h/templ"` from layout.templ because the templ generator automatically adds it, causing duplicate import errors in the generated Go code
- Extracted `RegisterAPIRoutes` and `ApplyMiddleware` from `NewServeMux` to allow both web and HTTP adapters to register on the same mux while sharing middleware

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed duplicate templ import from layout.templ**
- **Found during:** Task 2 (templ generate + go build)
- **Issue:** The layout.templ had `import "github.com/a-h/templ"` but templ generator also adds this import automatically, causing "templ redeclared in this block" compile error
- **Fix:** Removed the explicit import from the .templ file; templ generator handles it
- **Files modified:** internal/adapter/driving/web/templates/layout.templ
- **Verification:** `templ generate && go build ./...` succeeds
- **Committed in:** 797f8cc (Task 2 commit)

**2. [Rule 1 - Bug] Added export comment to StaticFS variable**
- **Found during:** Task 2 (pre-commit lint)
- **Issue:** golangci-lint/revive flagged exported var `StaticFS` missing doc comment
- **Fix:** Added descriptive comment to the exported variable
- **Files modified:** internal/adapter/driving/web/embed.go
- **Verification:** golangci-lint passes
- **Committed in:** 797f8cc (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both auto-fixes necessary for compilation and lint compliance. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Web scaffold complete, ready for Plan 02 (PR list feed and detail panel)
- Dashboard placeholder has `#pr-list` and `#pr-detail` targets ready for htmx partial swaps
- Dark mode Alpine store initialized, toggle UI comes in Plan 03

---
*Phase: 07-gui-foundation*
*Completed: 2026-02-14*
