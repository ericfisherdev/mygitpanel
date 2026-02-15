# Project Research Summary

**Project:** MyGitPanel 2026.2.0 Milestone - Web GUI + Jira Integration + Review Submission
**Domain:** Web-based PR review dashboard with external API integrations
**Researched:** 2026-02-14
**Confidence:** HIGH

## Executive Summary

MyGitPanel v2.0 adds a web GUI to the existing v1.0 Go API using a modern stack that aligns perfectly with the project's hexagonal architecture: templ for type-safe HTML templating, HTMX for server-driven partial updates, Alpine.js for client-side UI state, and Tailwind CSS for styling. This approach keeps all business logic server-side in Go, avoiding the complexity of SPA frameworks while delivering a rich, interactive dashboard experience. The existing v1.0 foundation (Go 1.25, modernc.org/sqlite, go-github v82, hexagonal architecture) requires no changes—the web GUI is a new driving adapter that shares domain ports with the existing JSON API.

The recommended path forward is to build the web GUI in three distinct phases: (1) read-only dashboard foundation with templ/HTMX/Alpine.js integration, (2) write operations (GitHub review submission) and credential management, (3) Jira integration with issue linking and commenting. This phasing isolates the critical architectural decisions (route separation, Alpine+HTMX state management, static asset embedding) in Phase 1, where mistakes would require the most rework. The research surfaced several critical pitfalls—particularly around Alpine.js state preservation during HTMX swaps, templ code generation in the build pipeline, and credential encryption—that must be addressed in foundational work before feature development begins.

The stack research confirms that no heavyweight dependencies are needed: templ and go-atlassian are the only new Go modules, while HTMX, Alpine.js, and GSAP are served via CDN or embedded as static files. The hexagonal architecture's strict port boundaries make adding Jira integration straightforward: define a JiraClient port, implement with go-atlassian, never let Jira types leak into the domain. The biggest risk is not technical complexity but rather the "two frameworks fighting over the DOM" problem where HTMX, Alpine.js, and GSAP all manipulate the DOM with different models—this requires clear boundaries and the alpine-morph HTMX extension to prevent state destruction.

## Key Findings

### Recommended Stack

The web GUI stack adds type-safe templating, server-driven dynamic updates, and minimal client-side JavaScript to the existing Go foundation. All additions align with the project's zero-npm, minimal-dependency philosophy.

**Core technologies:**
- **templ (v0.3.977)**: Type-safe HTML templating that compiles to Go code at build time. Templates are Go functions returning components, with full compile-time type checking. No reflection, no runtime parsing. Integrates directly with net/http.
- **HTMX (2.0.8, vendored)**: Server-driven partial page updates via HTML attributes. The server returns HTML fragments (templ components), not JSON. Keeps rendering logic server-side in Go, consistent with hexagonal architecture. Vendored under `static/vendor/` and embedded via `//go:embed`.
- **Alpine.js (3.15.x, vendored)**: Lightweight client-side state for UI interactions that don't need server round-trips (dropdowns, tabs, modals). Declared inline with x-data attributes—no build step, no components, no virtual DOM. Vendored under `static/vendor/` and embedded via `//go:embed`.
- **Tailwind CSS (v4.1.x standalone CLI)**: Utility-first CSS compiled via standalone binary. Scans .templ files for class usage and produces optimized CSS. Uses `@tailwindcss/typography` plugin (installed via npm in Docker build stage).
- **GSAP (3.14.x, vendored)**: Performant animations for PR card transitions, attention signals, and status updates. Free for all uses (Webflow acquisition, April 2025). Works with HTMX lifecycle events. Vendored under `static/vendor/` and embedded via `//go:embed`.
- **go-atlassian (v2.10.0)**: Jira Cloud REST API client. Supports Jira v2/v3 APIs with typed structs, pagination, and multiple auth methods. Mirrors go-github's service-based design (familiar pattern).
- **go-github v82 (existing)**: Already includes PullRequests.CreateReview() for review submission. No new dependency needed—just extend the existing GitHubClient port with write methods.

**Build tools (not runtime dependencies):**
- templ CLI (v0.3.977): Compiles .templ → .go before go build
- Tailwind standalone CLI: Compiles utility CSS from templ class usage
- air (optional): Hot reload during development

**Critical version notes:**
- HTMX 2.x (not 4.x): v4.0 expected mid-2026 but won't be marked "latest" until 2027. Stick with stable 2.0.x line.
- Tailwind v4: Uses CSS-first configuration (no tailwind.config.js). Standalone CLI bundles popular plugins automatically.

### Expected Features

Research identified clear tiers of features based on competitive analysis (Graphite, GitHub native, DevDynamics) and existing v1.0 API capabilities.

**Must have (table stakes):**
- **Unified PR feed** across all watched repos with filter/search/sort—every PR dashboard has this; Graphite sets the bar with pre-made sections
- **PR detail view** with full metadata, review threads with code context, CI check status, reviewer status list—the existing v1.0 API already provides rich data for this
- **Repo and bot management UI**—currently API-only; GUI must surface these to be self-contained
- **Dark mode (default) with light mode toggle**—developers overwhelmingly prefer dark mode; accessibility requires toggle
- **Visual status indicators** (CI, merge, review) at-a-glance—GitHub's green check/red X pattern is universal

**Should have (competitive differentiators):**
- **Review workflow actions from dashboard**: Approve, request changes, comment-only, reply to review comments—eliminates context switch to GitHub, core value prop
- **Draft PR toggle**: Convert to draft (REST API), mark ready for review (GraphQL-only, requires new adapter)
- **PR ignore list**: Hide specific PRs to reduce noise (dependabot, long-lived branches) with undo/re-add capability
- **Configurable urgency thresholds**: Per-repo review count requirements, age-based urgency levels (color-coded), attention score composite
- **Jira integration**: Auto-extract Jira keys from PR title/branch, view linked issue details (status, assignee, priority), post comments on Jira issues from PR view
- **GitHub credential management**: Store PAT in SQLite (encrypted), token validation on save, scope display to warn if missing write permissions

**Defer (v2+):**
- **Attention score (composite)**—needs UX iteration to get the formula right; ship simple urgency colors first
- **Bulk ignore by label/author**—power user feature; ship single-PR ignore first
- **Jira status badge in PR list**—requires caching strategy; ship detail-view integration first
- **Full diff viewer in-app**—massive complexity (syntax highlighting, side-by-side); GitHub does this perfectly, just link to it
- **Historical analytics/trends**—LinearB, Sleuth own this space; storage and charting complexity is high

**Anti-features (explicitly do NOT build):**
- Merge button from dashboard—GitHub's merge safety rails are hard to replicate
- PR creation—out of scope, users create from CLI/IDE
- Multi-user/multi-tenant—single-user tool, adding auth is massive scope creep
- Notification system (email/Slack)—dashboard is the notification
- AI review summarization—Claude Code handles this, MyGitPanel provides data
- Real-time WebSocket updates—polling works fine for single user

### Architecture Approach

The existing v1.0 hexagonal architecture requires no changes—the web GUI is a new driving adapter that coexists with the JSON API. Both adapters share the same domain ports and application services.

**Major components:**

1. **WebHandler (new driving adapter)** — `internal/adapter/driving/web/` contains templ-based handlers that render HTML partials. Depends on the same ports (PRStore, RepoStore, GitHubClient) as the existing Handler. Checks `HX-Request` header to decide between full page (initial load) or HTMX partial (subsequent interactions).

2. **templ components/layouts/pages** — `internal/adapter/driving/web/components/`, `/layouts/`, `/pages/`. Reusable UI components (pr_card.templ, review_thread.templ), base layout with head/scripts, and full page compositions. templ components accept view models (presentation-ready structs), not domain models directly.

3. **Jira port and adapter (new driven components)** — `internal/domain/port/driven/jiraclient.go` defines the interface (GetIssue, SearchIssuesByPR, AddComment). `internal/adapter/driven/jira/` implements with go-atlassian. Domain model `internal/domain/model/jira.go` has JiraIssue entity. No Jira types leak into domain.

4. **GitHubClient port extension** — Add write methods to existing driven.GitHubClient interface: `SubmitReview`, `CreateReviewComment`, `ReplyToComment`. Implemented using go-github v82's existing CreateReview/CreateComment methods. No new dependency.

5. **New application services** — `ReviewSubmitService` orchestrates review submission with validation and triggers refresh. `JiraService` orchestrates Jira lookups with branch-name key extraction. Both depend only on ports, never on concrete adapters.

6. **Route separation** — `/api/v1/*` routes hit the JSON adapter (existing, stable, untouched). `/` and `/app/*` routes hit the web adapter. Both mount on the same http.ServeMux with Go 1.22+ method+path routing. No content negotiation, no shared handlers.

7. **Static asset embedding** — Use `//go:embed static` to embed Tailwind CSS output, HTMX/Alpine.js/GSAP scripts, and any vendored assets. Serves from the binary in the Docker scratch image. Tailwind standalone CLI runs in Docker build stage before go build.

**Key architectural patterns:**
- **View models decouple templ from domain**: WebHandler maps domain models to presentation-ready structs before passing to templ components. templ files never import domain/model/.
- **HTMX partial vs full page**: Every GET handler checks `HX-Request: true` header. If present, render just the content partial. If absent, render full page with layout.
- **HTMX response headers for cascading updates**: After mutations (submit review, add repo), set `HX-Trigger` headers to tell other HTMX elements on the page to refresh.
- **Alpine.js state scoping**: Keep `x-data` on elements outside HTMX swap targets. Use alpine-morph HTMX extension when swapping content that contains Alpine state.
- **Jira branch name extraction**: Extract Jira issue keys from PR branch names using regex pattern `[A-Z]+-\d+` when loading Jira sidebar.

### Critical Pitfalls

Research identified five critical pitfalls that require architectural decisions in foundational work.

1. **Content negotiation trap (sharing handlers between JSON and HTML)** — Attempting to serve both JSON and HTML from the same handler by checking Accept or HX-Request headers creates tightly coupled handlers that violate SRP. HTMX creator explicitly warns against this. Prevention: Separate route namespaces entirely (/api/v1/* for JSON, /app/* for HTML), create dedicated WebHandler struct in new package, share domain logic through ports not handlers.

2. **templ code generation not in build pipeline** — Developer writes .templ files but Docker/CI runs go build without first running templ generate, producing images with stale/missing templates. Prevention: Add templ generate to Dockerfile build stage, do NOT commit generated *_templ.go files (add to .gitignore), pin templ version in Docker/CI to avoid version mismatch.

3. **Alpine.js state destruction on HTMX DOM swaps** — HTMX replaces DOM elements, destroying Alpine's reactive state (dropdowns close, accordions collapse, form values lost). Prevention: Use `hx-swap="morph:outerHTML"` with alpine-morph extension, scope HTMX swap targets carefully (do not swap x-data containers), remove id attributes from Alpine-managed elements inside swap targets.

4. **Storing Jira/GitHub credentials in SQLite without encryption** — Plaintext tokens in the database expose credentials to anyone with read access to the database file (Docker volume, backup, container escape). Prevention: Encrypt credentials at rest using AES-256-GCM with key from env var (MYGITPANEL_CREDENTIAL_KEY), or keep credentials as env vars only (no GUI storage). Never log credential values.

5. **Static assets missing from Docker scratch image** — Developer serves static assets from filesystem during development, but Docker scratch image has no filesystem—only the binary. Production container serves 404 for all CSS/JS files. Prevention: Use `//go:embed` to embed all static assets into the binary, build Tailwind CSS in Docker build stage, vendor HTMX/Alpine.js/GSAP or use CDN links (acceptable for single-user tool).

**Moderate pitfalls to address during feature development:**
- **GSAP animations breaking on HTMX swaps** — Re-initialize GSAP on htmx:afterSettle events, kill existing instances on htmx:beforeSwap
- **Jira REST API rate limiting is opaque** — Use webhooks instead of polling, implement exponential backoff with jitter on 429, cache Jira responses aggressively
- **Jira auth model mismatch (Cloud vs Data Center)** — Pick Jira Cloud-only (email + API token), validate credentials on save, provide clear configuration guidance
- **GitHub review submission triggering secondary rate limits** — Separate read/write tokens, rate-limit write operations client-side, queue write operations with controlled pacing
- **Tailwind CSS build complexity** — Use Tailwind standalone CLI (no Node.js), configure to scan .templ files, order Dockerfile stages correctly (Tailwind after templ files copied, before go build)

## Implications for Roadmap

Based on research, the 2026.2.0 milestone should be structured into three phases that isolate architectural risks early and defer complexity until foundational patterns are proven.

### Phase 1: Read-Only Dashboard Foundation

**Rationale:** Establishes the critical architectural patterns (route separation, templ code generation, Alpine+HTMX integration, static asset embedding) that all subsequent work depends on. Mistakes here require the most rework. Building the foundation with read-only features uses existing v1.0 API data (zero new backend work), allowing focus on the new presentation layer.

**Delivers:** Functional web GUI that displays all existing v1.0 data with modern, interactive UX. Users can browse PRs, filter/search/sort, view PR details with review threads and CI status, manage watched repos and bots—all without leaving the dashboard.

**Addresses features:**
- Unified PR feed with search/filter/sort (table stakes)
- PR detail view with review threads and CI status (table stakes, leverages existing rich API)
- Repo and bot management UI (currently CLI/API only)
- Dark/light theme toggle (table stakes)
- Visual urgency indicators (color-coded age/status badges using existing data)
- GSAP entrance animations (polish, low effort)

**Avoids pitfalls:**
- Content negotiation trap: Separate /api/v1/* and / route namespaces from day one
- templ build pipeline: Dockerfile runs templ generate before go build, *_templ.go in .gitignore
- Alpine+HTMX state: alpine-morph extension, swap targets scoped below x-data containers
- Static assets: //go:embed pattern, Tailwind standalone CLI in Docker build stage

**Research flag:** Standard patterns—templ/HTMX/Alpine.js integration is well-documented. No additional research needed during planning.

### Phase 2: Review Workflows + Credential Management

**Rationale:** Adds write operations after the presentation layer is stable. Requires token scope upgrade (breaking config change for existing users) and credential storage (encryption decision affects both GitHub and Jira). Isolating this in Phase 2 means Phase 1 can ship to users without breaking existing read-only deployments.

**Delivers:** Dashboard becomes a full review tool—users can approve PRs, request changes, post comments, reply to review threads, and toggle draft status without leaving the UI. Credentials stored securely in encrypted form (or env vars only, TBD during planning).

**Uses stack elements:**
- go-github v82 existing methods (PullRequests.CreateReview, CreateComment, CreateCommentInReplyTo)
- Extends GitHubClient port with write methods (SubmitReview, CreateReviewComment, ReplyToComment)
- New ReviewSubmitService orchestrates submission with validation

**Implements architecture components:**
- GitHubClient port extension with write methods
- ReviewSubmitService in application layer
- CredentialStore port and SQLite adapter with AES-256-GCM encryption (or env-var-only pattern)
- Review submission handlers in WebHandler (POST endpoints)

**Addresses features:**
- Approve/Request Changes/Comment submission (differentiator)
- Reply to review comments inline (differentiator)
- Post general PR comment (differentiator)
- Draft toggle: convert to draft (REST), mark ready for review (GraphQL—requires shurcooL/githubv4 or gh CLI wrapper)
- GitHub credential management (store PAT with write scope, encrypted)
- PR ignore list with undo (differentiator)

**Avoids pitfalls:**
- Credential encryption: AES-256-GCM with env-var key or env-vars only (no plaintext in DB)
- Secondary rate limits from writes: Separate read/write tokens, client-side rate limiting on write operations
- GSAP animations breaking on swaps: Re-initialize on htmx:afterSettle, kill on htmx:beforeSwap (PR list animations)

**Research flag:** Draft-to-ready is GraphQL-only—needs spike to decide: add shurcooL/githubv4, shell out to gh CLI, or defer feature. Plan this decision during phase planning.

### Phase 3: Jira Integration

**Rationale:** Jira is fully independent of GitHub workflows and GUI foundation. Deferring to Phase 3 means Phases 1-2 deliver a complete PR review dashboard without Jira complexity. Jira integration follows the same hexagonal pattern as GitHub (port → adapter → service), so architectural risk is low.

**Delivers:** Jira issue context displayed alongside PRs. Users see linked Jira tickets (auto-extracted from branch names), view issue status/assignee/priority, and post PR status updates to Jira—all from the PR detail view.

**Uses stack elements:**
- go-atlassian v2.10.0 (Jira Cloud REST API client)
- New JiraClient port interface (GetIssue, SearchIssuesByPR, AddComment)
- Jira adapter in internal/adapter/driven/jira/

**Implements architecture components:**
- JiraClient port in domain/port/driven/
- Jira adapter using go-atlassian
- JiraIssue domain model in domain/model/jira.go
- JiraService in application layer
- Jira sidebar handlers in WebHandler (HTMX lazy-loaded partials)

**Addresses features:**
- Jira credential management (URL, email, API token—encrypted or env vars)
- Jira key extraction from PR title/branch (regex [A-Z]+-\d+)
- Jira issue detail display (status, assignee, priority)
- Post Jira comments from PR detail view
- Configurable urgency thresholds per repo (new config table)
- Configurable review count requirements per repo (new column on repositories table)

**Avoids pitfalls:**
- Jira auth model mismatch: Pick Cloud-only (email + API token), validate creds on save, provide clear UI guidance
- Jira rate limiting opaque: Use JQL with `updated >= -5m` filter, exponential backoff on 429, cache responses aggressively, consider webhooks instead of polling
- Credential storage: Same encryption pattern as Phase 2 GitHub credentials

**Research flag:** Jira integration has multiple unknowns—webhook setup, JQL query optimization, ADF (Atlassian Document Format) for comments, rate limit behavior post-March 2026 quota changes. Plan for research-phase during Phase 3 planning.

### Phase Ordering Rationale

- **Phase 1 first** because the architectural patterns (route separation, templ/HTMX/Alpine.js integration, asset embedding) are foundational. Getting these wrong means rewriting all downstream features. Building Phase 1 with read-only features uses existing v1.0 data, allowing pure focus on the presentation layer.

- **Phase 2 before Phase 3** because credential management affects both GitHub write operations and Jira integration. Solving encryption in Phase 2 means Phase 3 reuses the pattern. Also, review submission (GitHub write) is higher value than Jira integration for the core use case (PR review dashboard).

- **Jira deferred to Phase 3** because it is fully independent of the dashboard foundation and GitHub workflows. Phases 1-2 deliver a complete PR review dashboard without Jira. Jira adds project management context but is not essential for the core review workflow.

- **Pitfall avoidance drives ordering**: Content negotiation trap, templ build pipeline, Alpine+HTMX state management, and static asset embedding must all be resolved in Phase 1. Deferring these to later phases would mean rebuilding earlier features.

### Research Flags

**Phases needing deeper research during planning:**
- **Phase 2 (Review Workflows)**: Draft-to-ready is GraphQL-only—spike needed to decide between adding shurcooL/githubv4, shelling out to gh CLI, or deferring the feature entirely.
- **Phase 3 (Jira Integration)**: Multiple unknowns—webhook setup, JQL query optimization for rate limit efficiency, ADF (Atlassian Document Format) for comment formatting, rate limit behavior post-March 2026 quota changes. Plan for /gsd:research-phase during Phase 3 planning.

**Phases with standard patterns (skip research-phase):**
- **Phase 1 (GUI Foundation)**: templ/HTMX/Alpine.js integration is well-documented with community examples. Asset embedding via //go:embed is standard Go. Tailwind standalone CLI is documented by Tailwind Labs.
- **Phase 2 (Review Workflows, partial)**: go-github v82 review submission methods (CreateReview, CreateComment) are well-documented in GitHub's REST API docs. Credential encryption with AES-256-GCM is standard Go crypto.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified against pkg.go.dev, GitHub releases, CDN registries. templ v0.3.977 confirmed, go-atlassian v2.10.0 confirmed, HTMX 2.0.8 confirmed, Alpine.js 3.15.8 confirmed, Tailwind v4.1.18 confirmed, GSAP 3.14.2 confirmed. htmx-go v0.5.0 is MEDIUM (published 2024-02-05, may have newer). |
| Features | HIGH | Verified against GitHub REST API docs, Jira REST API v3 docs, Graphite/ReviewStack competitive analysis, and existing v1.0 codebase. Draft-to-ready GraphQL-only confirmed in GitHub community discussions. |
| Architecture | HIGH | Existing hexagonal architecture is well-suited. templ/HTMX/Alpine.js integration patterns documented in community examples (hexaGO, full-stack Go guides). Port/adapter pattern for Jira mirrors existing GitHub adapter. |
| Pitfalls | MEDIUM-HIGH | Critical pitfalls verified via official docs (HTMX content negotiation essay, templ Docker hosting guide, GitHub rate limits). Alpine+HTMX state destruction documented in GitHub issues with confirmed alpine-morph solution. GSAP+HTMX integration is community-sourced (MEDIUM). Jira rate limit behavior has high-level docs but per-endpoint costs not published. |

**Overall confidence:** HIGH

The stack is well-established, the architecture fits the existing hexagonal pattern, and the critical pitfalls have documented solutions. The main areas of uncertainty are: (1) htmx-go may have a newer version than v0.5.0, (2) GSAP+HTMX integration patterns are community-sourced not officially documented, (3) Jira rate limit per-endpoint costs are not published (requires trial and error).

### Gaps to Address

**During Phase 1 planning:**
- **Verify htmx-go latest version**: Run `go get github.com/angelofallars/htmx-go@latest` to check if v0.5.0 is still latest. If newer version exists, verify API compatibility.
- **Spike alpine-morph extension**: Confirm alpine-morph HTMX extension works with HTMX 2.0.8 and Alpine.js 3.15.x. Build a minimal test case before committing to it.
- **Decide on static asset delivery**: CDN links vs vendoring. For single-user tool, CDN links are acceptable (no build complexity), but requires internet access from browser. Vendoring + //go:embed keeps deployment offline-capable but adds build steps.

**During Phase 2 planning:**
- **Spike draft-to-ready GraphQL requirement**: Test whether adding shurcooL/githubv4 is worth it for this single feature. Alternative: shell out to `gh pr ready`, which uses GitHub's CLI. Or defer the feature and only support draft conversion (REST API covers this).
- **Credential encryption vs env vars only**: Decide whether the complexity of AES-256-GCM encryption is justified. For a single-user tool, env-var-only config (MYGITPANEL_GITHUB_PAT_WRITE, MYGITPANEL_JIRA_TOKEN) may be simpler. GUI can display "configured via environment" without needing storage. Validate this decision during planning.

**During Phase 3 planning:**
- **Jira webhook feasibility**: Research whether Jira Cloud webhooks can POST to localhost (likely not without ngrok/tunneling). If webhooks require public endpoint, polling is mandatory. This affects the Jira adapter design.
- **JQL query optimization for rate limits**: Research which JQL patterns consume fewer rate limit points. Test with `updated >= -5m` filter vs broader queries. Document findings for the Jira adapter implementation.
- **ADF (Atlassian Document Format) for comments**: Research minimum viable ADF structure for plain text comments. Jira's comment API requires ADF JSON, not plain markdown. go-atlassian may have helpers for this.

## Sources

### Primary (HIGH confidence)

**Stack research:**
- [a-h/templ pkg.go.dev](https://pkg.go.dev/github.com/a-h/templ) — v0.3.977 verified
- [a-h/templ GitHub releases](https://github.com/a-h/templ/releases) — release history
- [HTMX releases](https://github.com/bigskysoftware/htmx/releases) — v2.0.8 confirmed
- [Alpine.js npm](https://www.npmjs.com/package/alpinejs) — v3.15.8 confirmed
- [Tailwind CSS releases](https://github.com/tailwindlabs/tailwindcss/releases) — v4.1.18 confirmed
- [Tailwind standalone CLI docs](https://tailwindcss.com/docs/installation/tailwind-cli) — standalone binary approach
- [GSAP cdnjs](https://cdnjs.com/libraries/gsap) — v3.14.2 confirmed
- [GSAP licensing](https://gsap.com/licensing/) — free for commercial use confirmed
- [go-atlassian pkg.go.dev](https://pkg.go.dev/github.com/ctreminiom/go-atlassian/v2) — v2.10.0 verified
- [go-atlassian GitHub](https://github.com/ctreminiom/go-atlassian) — Jira v2/v3 API support
- [go-github pulls_reviews.go](https://github.com/google/go-github/blob/master/github/pulls_reviews.go) — CreateReview API

**Feature research:**
- [GitHub REST API - Pull Request Reviews](https://docs.github.com/en/rest/pulls/reviews) — verified endpoints
- [GitHub REST API - Review Comments](https://docs.github.com/en/rest/pulls/comments) — verified reply endpoint
- [GitHub Community - Convert PR to Draft](https://github.com/orgs/community/discussions/45174) — REST PATCH works for draft=true
- [GitHub Community - Ready for Review](https://github.com/orgs/community/discussions/70061) — GraphQL-only for markReadyForReview
- [Jira Basic Auth](https://developer.atlassian.com/cloud/jira/software/basic-auth-for-rest-apis/) — email + API token authentication
- MyGitPanel v1.0 codebase — internal/adapter/driving/http/handler.go, response.go, domain models (direct inspection)

**Architecture research:**
- [templ Project Structure](https://templ.guide/project-structure/project-structure/) — official component organization
- [templ Template Composition](https://templ.guide/syntax-and-usage/template-composition/) — component passing patterns
- [GitHub REST API: Pull Request Reviews](https://docs.github.com/en/rest/pulls/reviews) — official API reference

**Pitfall research:**
- [HTMX: Why I Tend Not To Use Content Negotiation](https://htmx.org/essays/why-tend-not-to-use-content-negotiation/) — official essay
- [templ: Template Generation](https://templ.guide/core-concepts/template-generation/) — official build docs
- [templ: Hosting Using Docker](https://templ.guide/hosting-and-deployment/hosting-using-docker/) — official deployment guide
- [Jira Cloud: Rate Limiting](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/) — official docs
- [GitHub: Rate Limits for the REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) — official docs

### Secondary (MEDIUM confidence)

**Stack research:**
- [angelofallars/htmx-go pkg.go.dev](https://pkg.go.dev/github.com/angelofallars/htmx-go) — v0.5.0 (may have newer)
- [templ + HTMX integration patterns](https://tailbits.com/blog/setting-up-htmx-and-templ-for-go) — community reference

**Feature research:**
- [Jira REST API v3 - Issue Search](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/) — JQL search endpoint (did not verify exact response schema)
- [Jira REST API - Add Comment](https://developer.atlassian.com/server/jira/platform/jira-rest-api-example-add-comment-8946422/) — comment endpoint structure
- [Graphite Features](https://graphite.dev/features) — inbox sections and dashboard patterns
- [GitHub Community - Draft PR Visibility](https://github.com/orgs/community/discussions/165497) — draft label UX problem
- [DevDynamics - Open PR Age](https://docs.devdynamics.ai/features/metrics/git-dashboard/open-pr-age) — age threshold tiers
- [HTMX + Alpine.js + Go patterns](https://ntorga.com/full-stack-go-app-with-htmx-and-alpinejs/) — integration patterns

**Architecture research:**
- [hexaGO](https://github.com/edlingao/hexaGO) — hexagonal architecture template with Go/templ/HTMX/SQLite
- [HTMX and Alpine.js Integration](https://www.infoworld.com/article/3856520/htmx-and-alpine-js-how-to-combine-two-great-lean-front-ends.html) — combining patterns
- [Full-Stack Go App with HTMX and Alpine.js](https://ntorga.com/full-stack-go-app-with-htmx-and-alpinejs/) — Go-specific integration guide
- [Using Alpine.js In HTMX](https://www.bennadel.com/blog/4787-using-alpine-js-in-htmx.htm) — Alpine state preservation with HTMX swaps

**Pitfall research:**
- [Alpine.js x-bind not applying after HTMX swap](https://github.com/alpinejs/alpine/discussions/3985) — GitHub discussion
- [Alpine does not see x-show on elements swapped by htmx](https://github.com/alpinejs/alpine/discussions/3809) — GitHub discussion
- [HTMX + Alpine.js back button issues](https://github.com/bigskysoftware/htmx/discussions/2931) — GitHub discussion
- [How to Secure API Tokens in Your Database](https://hoop.dev/blog/how-to-secure-api-tokens-in-your-database-before-they-leak/) — credential encryption patterns
- [SQLite Encryption and Secure Storage](https://www.sqliteforum.com/p/sqlite-encryption-and-secure-storage) — SQLite encryption guidance
- [Setting up Go templ with Tailwind, HTMX and Docker](https://mbaraa.com/blog/setting-up-go-templ-with-tailwind-htmx-docker) — Docker integration
- [GSAP: Update animation after DOM change](https://gsap.com/community/forums/topic/35696-update-the-animation-after-the-change-dom/) — GSAP forum
- [HTMX: Animations](https://htmx.org/examples/animations/) — official animation examples
- [Deep-Dive Guide to Building a Jira API Integration](https://www.getknit.dev/blog/deep-dive-developer-guide-to-building-a-jira-api-integration) — Jira integration guide
- [How to Secure Jira REST API Calls in Data Center](https://success.atlassian.com/solution-resources/agile-and-devops-ado/platform-administration/how-to-secure-jira-and-confluence-rest-api-calls-in-data-center) — Jira auth
- [Top 5 REST API Authentication Challenges in Jira](https://www.miniorange.com/blog/rest-api-authentication-problems-solved/) — Jira auth challenges
- [HTMX history cache and Alpine template tags](https://github.com/alpinejs/alpine/discussions/2924) — GitHub discussion

---
*Research completed: 2026-02-14*
*Ready for roadmap: yes*
