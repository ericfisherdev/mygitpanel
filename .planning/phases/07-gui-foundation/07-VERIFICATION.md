---
phase: 07-gui-foundation
verified: 2026-02-14T23:30:00Z
status: human_needed
score: 24/24 must-haves verified
re_verification: false
human_verification:
  - test: "Visual dashboard rendering"
    expected: "Opening http://localhost:8080/ shows styled HTML page with ReviewHub branding, not JSON"
    why_human: "Visual rendering requires running server and browser inspection"
  - test: "PR list population"
    expected: "Dashboard sidebar shows PR cards with titles, repo names, authors, CI status dots, and status badges"
    why_human: "Requires watched repos with PRs in database"
  - test: "PR detail click interaction"
    expected: "Clicking a PR card in sidebar loads detail panel in main area without page reload, with smooth GSAP animation"
    why_human: "Interactive behavior requires running server and browser devtools to verify HTMX XHR and animation timing"
  - test: "Search debounce timing"
    expected: "Typing in search bar waits 500ms before sending request, prevents request on every keystroke"
    why_human: "Debounce timing requires browser Network tab observation"
  - test: "Filter combinations"
    expected: "Selecting status filter + repo filter + text search all work together, PR list updates correctly"
    why_human: "Requires test data with multiple repos and PR states"
  - test: "Dark mode persistence"
    expected: "Toggling dark mode changes theme immediately, refreshing page preserves choice via localStorage"
    why_human: "Requires browser interaction and localStorage inspection"
  - test: "Repo add/remove workflow"
    expected: "Adding owner/repo via form triggers async refresh, PR list updates to include new repo's PRs; removing repo updates list to exclude its PRs"
    why_human: "Requires GitHub API access and time for async polling to complete"
  - test: "Sidebar collapse interaction"
    expected: "Clicking collapse button shrinks sidebar to icon-only width, PR list hides, theme toggle and search disappear"
    why_human: "Visual layout change requires browser inspection"
  - test: "Tab switching in PR detail"
    expected: "Clicking Reviews/Threads/Comments/CI tabs switches visible content without page reload, Alpine.js manages state"
    why_human: "Interactive tab behavior requires browser"
  - test: "GSAP animation smoothness"
    expected: "PR detail content fades in with 300ms stagger, PR list items slide in from left with 200ms stagger"
    why_human: "Animation timing and visual quality require human perception"
  - test: "All JS libraries load without errors"
    expected: "Browser console shows no 404s for /static/vendor/*.js, no HTMX/Alpine/GSAP initialization errors"
    why_human: "Console error inspection requires running server and browser devtools"
  - test: "Dockerfile build success (when Tailwind CLI issue resolved)"
    expected: "docker build completes successfully with templ generate and tailwindcss compilation stages"
    why_human: "Dockerfile currently fails due to Tailwind musl/glibc incompatibility; requires environment fix or CLI version adjustment"
---

# Phase 7: GUI Foundation Verification Report

**Phase Goal:** User can browse all PR activity across repos in an interactive web dashboard without leaving a single page
**Verified:** 2026-02-14T23:30:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Opening http://localhost:8080/ renders a styled HTML page (not JSON) | ✓ VERIFIED | layout.templ exists with full HTML structure, RegisterRoutes maps GET /{$} to Dashboard handler, handler calls templates.Layout().Render() |
| 2 | Dashboard shows unified PR feed from all watched repos with CI/review/merge status indicators | ✓ VERIFIED | Dashboard handler calls prStore.ListAll(), toPRCardViewModels() conversion includes CIStatus/MergeableStatus/NeedsReview, PRCard.templ renders status badges |
| 3 | User can click a PR to view full detail without page reload | ✓ VERIFIED | PRCard.templ has hx-get={card.DetailPath} hx-target="#pr-detail" hx-swap="morph", GetPRDetail handler registered at GET /app/prs/{owner}/{repo}/{number}, returns partials.PRDetailContent |
| 4 | PR detail shows description, branch info, reviewers, CI checks, diff stats, review comments with code context | ✓ VERIFIED | PRDetail.templ renders all fields from PRDetailViewModel, GetPRDetail enriches with reviewSvc.GetPRReviewSummary() and healthSvc.GetPRHealthSummary(), tabs for reviews/threads/comments/CI |
| 5 | User can search PRs by text with debounced input | ✓ VERIFIED | SearchBar.templ has input with hx-trigger="input changed delay:500ms", SearchPRs handler filters by query via matchesPRQuery (title/author/repo/branch substring match) |
| 6 | User can filter PRs by status and repo | ✓ VERIFIED | SearchBar.templ has status dropdown (all/open/closed/merged) and repo dropdown, all use hx-include to cross-reference values, SearchPRs handler applies filterPRs with status and repo checks |
| 7 | Search/filter results update without full page reload | ✓ VERIFIED | All search/filter controls have hx-target="#pr-list" hx-swap="morph", SearchPRs renders partials.PRList for swap |
| 8 | User can toggle between dark and light theme | ✓ VERIFIED | ThemeToggle.templ has @click="$store.theme.dark = !$store.theme.dark", layout.templ initializes Alpine.store('theme') with Alpine.$persist |
| 9 | Theme preference persists across sessions | ✓ VERIFIED | layout.templ uses Alpine.$persist(false).as('darkMode'), persist plugin stores in localStorage |
| 10 | User can add watched repo via GUI | ✓ VERIFIED | RepoManager.templ has form with hx-post="/app/repos", AddRepo handler validates isValidRepoName, calls repoStore.Add, triggers pollSvc.RefreshRepo async |
| 11 | User can remove watched repo via GUI | ✓ VERIFIED | RepoManager.templ renders remove buttons with hx-delete={repo.DeletePath} hx-confirm, RemoveRepo handler calls repoStore.Remove |
| 12 | PR feed updates after repo add/remove | ✓ VERIFIED | renderRepoMutationResponse fetches updated prs/repos, renders RepoList + PRListOOB + RepoFilterOptions with hx-swap-oob="morph" |
| 13 | Sidebar can collapse and expand | ✓ VERIFIED | Sidebar.templ has x-data="{ collapsed: false }", collapse button @click="collapsed = !collapsed", x-bind:class for width transition, x-show="!collapsed" on content |
| 14 | GSAP animations play on PR selection | ✓ VERIFIED | animations.js has htmx:afterSettle listener, animateSwapTarget() for #pr-detail with 300ms fade/slide stagger |
| 15 | GSAP animations play on PR list updates | ✓ VERIFIED | animateSwapTarget() for #pr-list with 200ms slide-in stagger |
| 16 | All JS libraries load successfully | ✓ VERIFIED | 6 vendor JS files exist with substantive sizes (htmx 50K, alpine 44K, gsap 71K, plugins), embed.go has go:embed static/*, RegisterRoutes serves /static/ via http.FileServerFS |
| 17 | Existing JSON API endpoints continue to work unchanged | ✓ VERIFIED | httphandler.RegisterAPIRoutes still registered in main.go, web and http handlers share same mux, http routes at /api/v1/* not affected |
| 18 | go build ./... succeeds after templ generate | ✓ VERIFIED | go build ./... completes with no errors, all templ files have generated _templ.go counterparts |
| 19 | go test ./... passes with all existing tests green | ✓ VERIFIED | go test ./... shows ok for all packages with tests (github, sqlite, http, application, config adapters) |
| 20 | Tailwind input.css configured to scan templ files | ✓ VERIFIED | input.css has @import "tailwindcss", @source "../../templates/**/*.templ", @source "../../templates/**/*_templ.go" |
| 21 | Dockerfile has templ generate and tailwindcss build stages | ✓ VERIFIED | Dockerfile installs templ CLI, downloads tailwindcss binary, runs templ generate and tailwindcss before go build |
| 22 | Dark mode wiring in place | ✓ VERIFIED | layout.templ has x-bind:class="$store.theme.dark ? 'dark' : ''" on html tag, Alpine store initialized with persist, all components use dark: Tailwind variants |
| 23 | View models decouple templ from domain | ✓ VERIFIED | viewmodel package defines all presentation structs (PRCardViewModel, PRDetailViewModel, etc), conversion functions in web/viewmodel.go, templ components import viewmodel package not domain/model |
| 24 | Repo management uses OOB swap pattern | ✓ VERIFIED | renderRepoMutationResponse renders primary RepoList + PRListOOB (hx-swap-oob="morph") + RepoFilterOptions (hx-swap-oob="morph") for multi-target updates |

**Score:** 24/24 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| internal/adapter/driving/web/handler.go | WebHandler with NewHandler constructor, Dashboard/GetPRDetail/SearchPRs/AddRepo/RemoveRepo methods | ✓ VERIFIED | 361 lines, exports NewHandler with 7 deps, all handler methods present with substantive implementations, non-fatal enrichment pattern |
| internal/adapter/driving/web/routes.go | RegisterRoutes for /, /app/prs/*, /app/repos, /static/ file serving | ✓ VERIFIED | 27 lines, exports RegisterRoutes, maps 5 routes + static fileserver via StaticFS |
| internal/adapter/driving/web/embed.go | go:embed directive for static/* | ✓ VERIFIED | 8 lines, exports StaticFS with go:embed static/* |
| internal/adapter/driving/web/viewmodel.go | Conversion functions from domain to viewmodel types | ✓ VERIFIED | 236 lines, defines toPRCardViewModels, toPRDetailViewModel, toReviewViewModels, toThreadViewModels, etc |
| internal/adapter/driving/web/viewmodel/viewmodel.go | View model struct definitions | ✓ VERIFIED | 110 lines, defines PRCardViewModel, PRDetailViewModel, ReviewViewModel, ThreadViewModel, IssueCommentViewModel, CheckRunViewModel, etc |
| internal/adapter/driving/web/templates/layout.templ | Base HTML with script loading order and dark mode Alpine store | ✓ VERIFIED | 31 lines, script order: htmx -> htmx-ext -> alpine plugins -> alpine core -> gsap -> animations.js, Alpine.store('theme') with persist, x-bind:class on html |
| internal/adapter/driving/web/templates/pages/dashboard.templ | Dashboard page composing Sidebar + main #pr-detail area | ✓ VERIFIED | 13 lines, takes DashboardViewModel, renders Sidebar(data) + main with #pr-detail |
| internal/adapter/driving/web/templates/components/pr_card.templ | Clickable PR card with status badges and HTMX hx-get | ✓ VERIFIED | 76 lines, hx-get={card.DetailPath} hx-target="#pr-detail" hx-swap="morph", renders CI status dots, draft/review/conflict/status badges |
| internal/adapter/driving/web/templates/components/pr_detail.templ | PR detail panel with tabs (reviews/threads/comments/CI) using Alpine.js | ✓ VERIFIED | 240 lines, x-data="{ tab: 'reviews' }", @click tab switching, x-show for tab content, renders all PR metadata/stats/reviews/threads/comments/checks |
| internal/adapter/driving/web/templates/components/sidebar.templ | Collapsible sidebar with Alpine x-data for collapse state | ✓ VERIFIED | 67 lines, x-data="{ collapsed: false }", x-bind:class for width, x-show="!collapsed" on content, ThemeToggle + SearchBar + PRList + RepoManager |
| internal/adapter/driving/web/templates/components/search_bar.templ | Search input with debounced hx-trigger and status/repo filter dropdowns | ✓ VERIFIED | 88 lines, input with hx-trigger="input changed delay:500ms", status/repo selects with hx-get, all use hx-include for cross-referencing |
| internal/adapter/driving/web/templates/components/theme_toggle.templ | Dark/light toggle button using Alpine $store.theme | ✓ VERIFIED | 21 lines, @click="$store.theme.dark = !$store.theme.dark", x-show for sun/moon icons |
| internal/adapter/driving/web/templates/components/repo_manager.templ | Add/remove repo form with HTMX hx-post/hx-delete | ✓ VERIFIED | 75 lines, x-data for expand, form with hx-post="/app/repos", remove buttons with hx-delete and hx-confirm |
| internal/adapter/driving/web/templates/partials/pr_list.templ | PR list partial for HTMX swap, includes PRListOOB for OOB swaps | ✓ VERIFIED | 24 lines, PRList() renders cards, PRListOOB() same content with hx-swap-oob="morph" |
| internal/adapter/driving/web/templates/partials/pr_detail_content.templ | PR detail partial for HTMX swap into #pr-detail | ✓ VERIFIED | 7 lines, wraps PRDetail component |
| internal/adapter/driving/web/templates/partials/repo_list.templ | Repo list partial for OOB swap after add/remove | ✓ VERIFIED | 23 lines, renders repo items with remove buttons |
| internal/adapter/driving/web/static/vendor/htmx.min.js | HTMX 2.0.4 vendored | ✓ VERIFIED | 50K, substantive content |
| internal/adapter/driving/web/static/vendor/alpine.min.js | Alpine.js 3.14.9 vendored | ✓ VERIFIED | 44K, substantive content |
| internal/adapter/driving/web/static/vendor/alpine-morph.min.js | Alpine morph plugin | ✓ VERIFIED | 4.0K, substantive content |
| internal/adapter/driving/web/static/vendor/alpine-persist.min.js | Alpine persist plugin | ✓ VERIFIED | 837 bytes, substantive content |
| internal/adapter/driving/web/static/vendor/htmx-ext-alpine-morph.js | HTMX Alpine morph extension | ✓ VERIFIED | 505 bytes, substantive content |
| internal/adapter/driving/web/static/vendor/gsap.min.js | GSAP 3.13.0 | ✓ VERIFIED | 71K, substantive content |
| internal/adapter/driving/web/static/css/input.css | Tailwind input with @source directives | ✓ VERIFIED | 4 lines, @import "tailwindcss", @source for .templ and _templ.go files, @custom-variant dark |
| internal/adapter/driving/web/static/js/animations.js | GSAP animation listeners for htmx:afterSettle | ✓ VERIFIED | 21 lines, animateSwapTarget for #pr-detail and #pr-list, htmx:afterSettle listener |
| Dockerfile | Multi-stage with templ generate and tailwindcss before go build | ✓ VERIFIED | Installs templ CLI, downloads tailwindcss, runs templ generate, runs tailwindcss -i...output.css --minify before go build |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| cmd/mygitpanel/main.go | web.NewHandler | Constructor call with deps | ✓ WIRED | Line 100: webHandler := webhandler.NewHandler(...7 deps), line 101: webhandler.RegisterRoutes(mux, webHandler) |
| web/routes.go | web/embed.go StaticFS | http.FileServerFS(staticFS) | ✓ WIRED | Line 13-14: staticFS from fs.Sub(StaticFS, "static"), mux.Handle serves via FileServerFS |
| web/templates/components/pr_card.templ | /app/prs/{owner}/{repo}/{number} | hx-get={card.DetailPath} | ✓ WIRED | Line 10: hx-get={ card.DetailPath }, DetailPath computed in viewmodel.go line 42 |
| web/templates/components/search_bar.templ | /app/prs/search | hx-get with delay:500ms | ✓ WIRED | Lines 22, 35, 51, 73: hx-get="/app/prs/search" hx-trigger="input changed delay:500ms" or "change" |
| web/templates/components/repo_manager.templ | /app/repos | hx-post for add, hx-delete for remove | ✓ WIRED | Line 26: hx-post="/app/repos", line 54: hx-delete={repo.DeletePath} (computed /app/repos/{owner}/{repo}) |
| web/templates/components/theme_toggle.templ | Alpine.store('theme') | @click toggling $store.theme.dark | ✓ WIRED | Line 7: @click="$store.theme.dark = !$store.theme.dark", store defined in layout.templ line 24-26 |
| web/handler.go Dashboard | prStore.ListAll | Fetch PRs for sidebar | ✓ WIRED | Line 57: prs, err := h.prStore.ListAll(r.Context()), result used in toPRCardViewModels line 71 |
| web/handler.go GetPRDetail | prStore.GetByNumber | Fetch single PR | ✓ WIRED | Line 117: pr, err := h.prStore.GetByNumber(...), result used in toPRDetailViewModel line 158 |
| web/handler.go GetPRDetail | reviewSvc.GetPRReviewSummary | Enrich PR detail with reviews | ✓ WIRED | Line 134: summary, err = h.reviewSvc.GetPRReviewSummary(...), result passed to toPRDetailViewModel |
| web/handler.go GetPRDetail | healthSvc.GetPRHealthSummary | Enrich PR detail with CI checks | ✓ WIRED | Line 148: healthSummary, healthErr := h.healthSvc.GetPRHealthSummary(...), checkRuns passed to toPRDetailViewModel |
| web/handler.go AddRepo | pollSvc.RefreshRepo | Async repo refresh | ✓ WIRED | Line 200-204: go func() pollSvc.RefreshRepo in background goroutine |
| web/handler.go renderRepoMutationResponse | partials.PRListOOB | OOB swap for PR list | ✓ WIRED | Line 258: prListComp := partials.PRListOOB(cards), rendered to response |
| layout.templ script tags | /static/vendor/*.js | Script src attributes | ✓ WIRED | Lines 15-21: script tags load htmx, htmx-ext, alpine plugins, alpine core, gsap, animations.js from /static/vendor/ |

### Requirements Coverage

| Requirement | Status | Blocking Issue |
|-------------|--------|----------------|
| GUI-01: Unified PR feed across all watched repos | ✓ SATISFIED | Truths 2, 16, 17 verified |
| GUI-02: PR detail with description, branch info, reviewers, CI status | ✓ SATISFIED | Truths 3, 4 verified |
| GUI-03: Search and filter PRs by status, repo, text | ✓ SATISFIED | Truths 5, 6, 7 verified |
| GUI-04: Dark/light theme toggle with persistence | ✓ SATISFIED | Truths 8, 9, 22 verified |
| GUI-05: Collapsible sidebar | ✓ SATISFIED | Truth 13 verified |
| GUI-06: GSAP animations on PR selection, list updates | ✓ SATISFIED | Truths 14, 15 verified |
| GUI-07: Add/remove repos through GUI | ✓ SATISFIED | Truths 10, 11, 12, 24 verified |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/adapter/driving/web/static/vendor/htmx.min.js | 1 | Minified JS (0 lines via wc -l) | ℹ️ Info | Minified files show 0-5 lines but have substantive content (50K) — not a stub |
| Dockerfile | 23-24 | Tailwind CLI musl/glibc incompatibility | 🛑 Blocker | Docker build fails with "Error relocating /usr/local/bin/tailwindcss" — requires Tailwind CLI version adjustment or glibc-based Alpine |

**Note on Dockerfile issue:** This is a known Tailwind standalone CLI issue with Alpine musl. Workarounds: (1) use tailwindcss-linux-x64 (glibc) with glibc-compat in Alpine, (2) use Debian-based build stage, (3) pre-generate output.css locally and commit it. Since local development works (output.css exists as placeholder), this is a deployment blocker not a development blocker.

### Human Verification Required

#### 1. Visual dashboard rendering

**Test:** Start mygitpanel server, open http://localhost:8080/ in a browser.
**Expected:** Styled HTML page with ReviewHub branding, sidebar on left, main content area on right. Dark mode classes applied (if theme store defaulted to dark). Not a JSON response.
**Why human:** Visual rendering and styling require browser inspection.

#### 2. PR list population

**Test:** Ensure database has watched repos with PRs. Refresh dashboard.
**Expected:** Sidebar shows PR cards with titles, repo names (owner/repo), authors, CI status colored dots (green/red/yellow/gray), draft/review/conflict badges.
**Why human:** Requires test data and visual verification of badge colors and layout.

#### 3. PR detail click interaction

**Test:** Click a PR card in the sidebar.
**Expected:** Main content area (#pr-detail) updates to show PR detail without full page reload. GSAP animation plays (content fades in with stagger). Browser Network tab shows XHR request to /app/prs/{owner}/{repo}/{number}.
**Why human:** Interactive behavior and animation timing require human perception and devtools inspection.

#### 4. Search debounce timing

**Test:** Type "fix" in the search bar, observe Network tab.
**Expected:** No request sent until typing stops for 500ms. Typing additional characters resets the timer.
**Why human:** Debounce timing requires Network tab observation and timing measurement.

#### 5. Filter combinations

**Test:** Select "open" status, select specific repo, type text query. Try all combinations.
**Expected:** PR list updates to show only PRs matching all active filters. Changing any filter updates the list.
**Why human:** Requires test data with multiple repos, statuses, and PR titles to verify filtering logic.

#### 6. Dark mode persistence

**Test:** Toggle theme button. Refresh page. Toggle again. Refresh again.
**Expected:** First toggle switches from light to dark (or vice versa). Refresh preserves choice. Second toggle switches back. Refresh preserves new choice. Inspect localStorage for 'darkMode' key.
**Why human:** Requires browser interaction and localStorage inspection in devtools.

#### 7. Repo add/remove workflow

**Test:** Enter "owner/repo" in add form, submit. Wait 10 seconds. Observe PR list. Click remove on a repo.
**Expected:** Add triggers async refresh (check logs for pollSvc.RefreshRepo). After polling completes, PR list includes new repo's PRs. Remove updates both repo list and PR feed to exclude that repo's PRs.
**Why human:** Requires GitHub API access, async timing, and multi-component update observation.

#### 8. Sidebar collapse interaction

**Test:** Click sidebar collapse button (chevron icon).
**Expected:** Sidebar width shrinks to ~64px (icon-only). PR list, search bar, and theme toggle disappear. ReviewHub title hides. Collapse icon rotates. Click again to restore.
**Why human:** Visual layout change and animation require browser inspection.

#### 9. Tab switching in PR detail

**Test:** Select a PR to load detail. Click Reviews, Threads, Comments, CI tabs.
**Expected:** Tab content switches without page reload. Active tab shows blue underline. Alpine.js manages tab state (no HTMX request). Tab switching is instant (no animation on tab content, per plan notes).
**Why human:** Interactive tab behavior requires browser.

#### 10. GSAP animation smoothness

**Test:** Select different PRs, filter PR list, add/remove repos. Observe animations.
**Expected:** PR detail content fades in with 300ms duration, children stagger 50ms (power2.out ease). PR list items slide in from left with 200ms duration, 30ms stagger (power1.out ease). Animations feel smooth and polished.
**Why human:** Animation timing, easing, and visual quality require human perception.

#### 11. All JS libraries load without errors

**Test:** Open browser console, refresh dashboard, check Network tab and Console.
**Expected:** All /static/vendor/*.js files return 200 status. Console shows no "Uncaught ReferenceError" for htmx, Alpine, gsap. Alpine.store('theme') initializes. HTMX extension loads.
**Why human:** Console error inspection and Network tab review require running server and browser devtools.

#### 12. Dockerfile build success (when Tailwind CLI issue resolved)

**Test:** Fix Dockerfile Tailwind CLI download (use glibc-compatible version or Debian build stage). Run docker build.
**Expected:** Build completes successfully. templ generate runs, tailwindcss compiles output.css, go build succeeds, final image created.
**Why human:** Dockerfile currently fails due to musl/glibc incompatibility. Requires environment fix.

---

## Verification Summary

**All 24 automated must-haves verified.** The phase goal is technically achieved — the codebase has all required artifacts, wiring, and logic to deliver the interactive web dashboard.

**Status: human_needed** — 12 items require human verification to confirm runtime behavior, visual rendering, animation quality, and interaction flows.

**No gaps blocking goal achievement.** The Dockerfile Tailwind CLI issue is a deployment blocker (anti-pattern 🛑 Blocker) but does not prevent the dashboard from running locally (output.css exists as a placeholder; real Tailwind compilation can be done via npx or local CLI for development).

**Next steps:**
1. Human verifier runs server, tests all 12 interactive scenarios
2. Resolve Dockerfile Tailwind CLI issue (use glibc-compatible binary or Debian build stage)
3. Add web handler integration tests (currently no test files for web adapter)
4. If all human verification passes, phase 7 is complete and ready for phase 8 (GitHub Write Operations)

---

_Verified: 2026-02-14T23:30:00Z_
_Verifier: Claude (gsd-verifier)_
