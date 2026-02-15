# Domain Pitfalls

**Domain:** Adding Web GUI (templ/HTMX/Alpine.js/GSAP) + Jira Integration to Existing Go API
**Project:** MyGitPanel v2.0 Milestone
**Researched:** 2026-02-14
**Overall confidence:** MEDIUM-HIGH (verified via official docs, community discussions, and codebase analysis)

---

## Critical Pitfalls

Mistakes that cause rewrites, architectural conflicts, or security breaches.

---

### Pitfall 1: Content Negotiation Trap -- Sharing Handlers Between JSON API and HTML Views

**What goes wrong:** The developer tries to serve both JSON and HTML from the same handler by checking the `Accept` header or an `HX-Request` header. The existing JSON API endpoints (`/api/v1/prs`, `/api/v1/repos`) get `if htmx { renderTemplate } else { writeJSON }` logic bolted onto them. This creates tightly coupled handlers that serve two masters with conflicting requirements.

**Why it happens:** It seems DRY to reuse the same handler and data-fetching logic. The developer sees `HX-Request: true` header from HTMX and thinks "easy, just branch on this." Carson Gross (HTMX creator) explicitly warns against this approach: "Your JSON API needs to be a stable set of endpoints that client code can rely on. Your hypermedia API can change dramatically based on user interface needs. These two things don't mix well."

**Consequences:**
- JSON API changes break the HTML views and vice versa
- Handler functions balloon with branching logic, violating SRP
- Testing becomes combinatorial (every handler x every content type)
- The JSON API loses its stability guarantee -- the existing Claude Code CLI consumer breaks when HTML-driven changes alter response shapes
- HTMX responses need HTML fragments, not full pages, but JSON consumers need complete data objects -- these are fundamentally different response shapes

**Prevention:**
1. **Separate route namespaces entirely.** Keep `/api/v1/*` for JSON (existing, stable, untouched). Add `/app/*` or `/ui/*` for HTML/HTMX views. Each gets its own handler struct.
2. **Create a dedicated `WebHandler` struct** in a new package (`internal/adapter/driving/web/`) that depends on the same domain ports as the existing `Handler` but renders templ components instead of JSON.
3. **Share domain logic through ports, not handlers.** Both `Handler` (JSON) and `WebHandler` (HTML) inject the same `driven.PRStore`, `driven.RepoStore`, etc. The domain layer is shared; the presentation layer is separate.
4. **Register routes on the same `http.ServeMux`.** Go 1.22+ routing with method+path patterns handles this cleanly -- `/api/v1/prs` and `/app/prs` coexist on the same mux without conflict.

**Detection (warning signs):**
- `if r.Header.Get("HX-Request") == "true"` appearing inside existing JSON handlers
- `Accept` header parsing in handler functions
- JSON response struct changes breaking existing CLI consumer
- Handler functions exceeding 40 lines due to content-type branching

**Phase mapping:** Must be decided in Phase 1 (GUI foundation). The route architecture determines everything downstream.

**Confidence:** HIGH -- HTMX's own documentation explicitly recommends against content negotiation. The existing codebase's `NewServeMux` in `handler.go` already uses Go 1.22+ routing which cleanly supports parallel route namespaces.

**Sources:**
- [HTMX: Why I Tend Not To Use Content Negotiation](https://htmx.org/essays/why-tend-not-to-use-content-negotiation/)

---

### Pitfall 2: templ Code Generation Not Integrated into Build Pipeline

**What goes wrong:** The developer writes `.templ` files but forgets that templ requires a code generation step (`templ generate`) before `go build`. The Dockerfile runs `go build` without first running `templ generate`. The CI pipeline compiles stale generated code. Locally, the developer runs `templ generate` manually but the Docker build and CI do not, producing images with outdated or missing templates.

**Why it happens:** Unlike standard Go where `go build` is self-sufficient, templ adds a pre-build step. Generated `*_templ.go` files may or may not be committed to git. If committed, version drift between the `.templ` source and generated `.go` file causes silent bugs (rendered HTML does not match the template source). If not committed, every build environment must have `templ` installed.

**Consequences:**
- Docker images serve stale HTML that does not match the template source
- CI builds pass with outdated generated code, hiding template errors
- "Works on my machine" syndrome -- developer runs `templ generate` locally, CI does not
- Version mismatch between local templ CLI and CI templ CLI produces different generated code
- The current Dockerfile (`FROM golang:1.25-alpine AS build`) does not install `templ`

**Prevention:**
1. **Add `templ generate` to the Dockerfile build stage.** Install templ in the build stage: `RUN go install github.com/a-h/templ/cmd/templ@latest && templ generate` before `go build`.
2. **Do NOT commit generated `*_templ.go` files.** Add `*_templ.go` to `.gitignore`. Generate fresh in CI and Docker builds. This eliminates version drift entirely.
3. **Add a `go generate` directive.** In a root file: `//go:generate templ generate ./...` so `go generate ./...` runs templ. This is idiomatic Go.
4. **Pin the templ version.** Use `go install github.com/a-h/templ/cmd/templ@v0.3.x` (pin to specific version) in Dockerfile and CI. Version mismatch between environments causes subtle rendering differences.
5. **Add a CI check for template formatting.** `templ fmt --check` returns non-zero if templates need formatting, catching style drift.

**Detection (warning signs):**
- HTML output does not match what the `.templ` file shows
- `*_templ.go` files in git with timestamps older than their `.templ` sources
- Docker build succeeds but serves wrong HTML
- `templ generate` produces diffs on freshly cloned repo

**Phase mapping:** Phase 1 (GUI foundation). The build pipeline must be updated before any templates are written.

**Confidence:** HIGH -- templ's official docs explicitly document this requirement. The current Dockerfile needs modification.

**Sources:**
- [templ: Template Generation](https://templ.guide/core-concepts/template-generation/)
- [templ: Hosting Using Docker](https://templ.guide/hosting-and-deployment/hosting-using-docker/)

---

### Pitfall 3: Alpine.js State Destruction on HTMX DOM Swaps

**What goes wrong:** HTMX swaps new HTML into the DOM, destroying Alpine.js reactive state. Components initialized with `x-data` lose their state (open/closed toggles, form values, filter selections). The UI "resets" on every HTMX interaction -- dropdowns close, accordions collapse, tab selections revert.

**Why it happens:** HTMX's default swap strategy (`innerHTML`) replaces the target element's children entirely. Alpine.js binds reactive state to DOM elements. When those elements are replaced, Alpine's reactive proxies are garbage-collected. The new HTML has fresh `x-data` attributes but no connection to the previous state. Even with `id` preservation, HTMX's settle algorithm can conflict with Alpine's reactivity tracking.

**Consequences:**
- UI feels broken -- users click a dropdown, trigger an HTMX request, and the dropdown snaps closed
- Form state is lost mid-interaction
- Complex Alpine components (multi-step forms, accordion panels) become unusable
- Developer starts fighting the frameworks instead of building features

**Prevention:**
1. **Use `hx-swap="morph:outerHTML"` with the Alpine Morph extension.** The `alpine-morph` HTMX extension uses Alpine's morph algorithm which preserves Alpine state during DOM updates. This is the officially recommended solution when combining HTMX and Alpine.
2. **Scope HTMX swap targets carefully.** Do not swap the entire `x-data` container. Swap only the data-display portion inside it. Keep Alpine state containers as parents of HTMX swap targets, not the targets themselves.
3. **Use `hx-select` to extract fragments.** When the server returns a full component, use `hx-select` to extract only the changing portion, leaving Alpine's state container untouched.
4. **Remove `id` attributes from Alpine-managed elements** when they are inside HTMX swap targets. HTMX's settle algorithm matches elements by `id` and can mutate attributes in ways that break Alpine's reactivity tracking.
5. **Prefer Alpine for client-only state, HTMX for server state.** Dropdown open/closed is Alpine territory (never goes to server). PR list content is HTMX territory (fetched from server). Draw clear boundaries.

**Detection (warning signs):**
- UI elements "resetting" after HTMX requests
- `x-show` or `x-bind` not applying after swap
- Alpine console warnings about missing reactive data
- Developer adding `setTimeout` hacks to re-initialize Alpine after swaps

**Phase mapping:** Phase 1 (GUI foundation). The Alpine+HTMX integration pattern must be established before building any interactive components. Building multiple components with the wrong pattern means rewriting all of them.

**Confidence:** HIGH -- Multiple GitHub issues document this exact problem, and the alpine-morph solution is well-established.

**Sources:**
- [Alpine.js x-bind not applying after HTMX swap](https://github.com/alpinejs/alpine/discussions/3985)
- [Alpine does not see x-show on elements swapped by htmx](https://github.com/alpinejs/alpine/discussions/3809)
- [HTMX + Alpine.js back button issues](https://github.com/bigskysoftware/htmx/discussions/2931)
- [Using Alpine.js in HTMX (Ben Nadel)](https://www.bennadel.com/blog/4787-using-alpine-js-in-htmx.htm)

---

### Pitfall 4: Storing Jira/GitHub Credentials in SQLite Without Encryption

**What goes wrong:** The developer stores Jira API tokens and GitHub PATs as plaintext in the SQLite database. Anyone with read access to the database file (Docker volume mount, backup, container escape) gets full API access to the user's Jira and GitHub accounts.

**Why it happens:** SQLite has no built-in column-level encryption. The developer thinks "it is a local tool, who would access the database?" But the database file is on a Docker volume, potentially backed up, and accessible to any process in the container. The existing `MYGITPANEL_GITHUB_TOKEN` is an environment variable (good), but adding Jira credentials via the web GUI means they must be persisted somewhere -- and the GUI naturally wants to store them in the database.

**Consequences:**
- Credential theft from database backup or volume access
- Jira tokens grant access to all projects the user can see -- potentially company-wide sensitive data
- GitHub PATs with `repo` scope give full read/write access to private repositories
- If the database is accidentally committed or shared, credentials are exposed
- Compliance violation for any organization with credential storage policies

**Prevention:**
1. **Encrypt credentials at rest using AES-256-GCM.** Use Go's `crypto/aes` and `crypto/cipher` to encrypt tokens before storing and decrypt on read. Store the ciphertext + nonce in the database, not the plaintext.
2. **Derive the encryption key from an environment variable.** `MYGITPANEL_CREDENTIAL_KEY` passed at runtime. Never hardcode the key or store it in the database. This follows the principle: the database alone is not sufficient to recover credentials.
3. **Create a `CredentialStore` port/adapter.** Domain port defines `StoreCredential(ctx, name, value)` and `GetCredential(ctx, name)`. The SQLite adapter handles encryption/decryption transparently. This keeps encryption concerns out of application logic.
4. **Consider keeping credentials as env vars only.** For a single-user tool, environment variables (`MYGITPANEL_JIRA_TOKEN`, `MYGITPANEL_JIRA_URL`) may be sufficient. The GUI can display "configured via environment" without needing database storage. This avoids the encryption complexity entirely.
5. **Never log credential values.** Ensure `slog` calls never include token values. Use `slog.String("jira_configured", "true")` not `slog.String("jira_token", token)`.
6. **Rotate tokens independently.** Store credentials with metadata (created_at, last_used_at) so users can identify and rotate stale tokens.

**Detection (warning signs):**
- Plaintext tokens visible in SQLite CLI: `sqlite3 mygitpanel.db "SELECT * FROM credentials;"`
- Token values appearing in application logs
- No `MYGITPANEL_CREDENTIAL_KEY` environment variable in Docker configuration
- Database backup containing readable API tokens

**Phase mapping:** Must be resolved before Jira integration phase. The credential storage pattern affects both Jira and any future GitHub write operations (submitting reviews).

**Confidence:** HIGH -- Encryption at rest for credentials is an industry-standard requirement. The specific Go crypto primitives are stable and well-documented.

**Sources:**
- [How to Secure API Tokens in Your Database](https://hoop.dev/blog/how-to-secure-api-tokens-in-your-database-before-they-leak/)
- [SQLite Encryption and Secure Storage](https://www.sqliteforum.com/p/sqlite-encryption-and-secure-storage)

---

### Pitfall 5: Static Assets (Tailwind CSS, HTMX JS, Alpine JS, GSAP) Missing from Docker Scratch Image

**What goes wrong:** The developer serves static assets from the filesystem during development. The Docker scratch image has no filesystem -- only the binary. The production container starts but serves 404 for all CSS/JS files. The app renders unstyled, non-interactive HTML.

**Why it happens:** The current Dockerfile copies only `/bin/mygitpanel` and `/bin/healthcheck` into the scratch image. There is no mechanism to include CSS, JavaScript, or other static files. During development, `http.FileServer(http.Dir("assets/"))` works because the assets directory exists. In scratch, it does not.

**Consequences:**
- Production deployment serves broken, unstyled pages
- HTMX, Alpine.js, and GSAP JavaScript does not load -- the entire GUI is non-functional
- Tailwind CSS does not load -- raw unstyled HTML
- The developer adds a filesystem to the scratch image (defeating its purpose) or switches to a larger base image unnecessarily

**Prevention:**
1. **Use Go's `//go:embed` to embed all static assets into the binary.** Create an `internal/assets/` package with `//go:embed static/*` that embeds CSS, JS, and any other static files. Serve via `http.FileServer(http.FS(assets.Static))`. This keeps the single-binary deployment model.
2. **Build Tailwind CSS in the Docker build stage.** Add a Node.js step or use the Tailwind standalone CLI in the Dockerfile build stage: download the tailwind binary, run `tailwindcss -i input.css -o static/styles.css --minify`, then embed the output.
3. **Use CDN links for HTMX, Alpine.js, and GSAP during development**, but vendor them for production. Download specific versions into the `static/` directory and embed them. This avoids CDN dependency in production and ensures version pinning.
4. **Alternative: Use CDN in production too.** For a single-user tool, CDN links for HTMX (14KB), Alpine.js (15KB), and GSAP are acceptable. But this requires internet access from the browser, which may not be available in all deployment scenarios.
5. **Add an integration test** that starts the compiled binary and requests `/static/styles.css` to verify embedding works. This catches the "assets not embedded" bug before deployment.

**Detection (warning signs):**
- Browser console showing 404 for `/static/*.css` and `/static/*.js`
- HTML renders but with no styling or interactivity
- Docker image size does not increase after adding static assets (they were not embedded)
- `http.Dir("assets/")` in production code (filesystem-dependent, breaks in scratch)

**Phase mapping:** Phase 1 (GUI foundation). The asset pipeline must work before any templates reference CSS or JS.

**Confidence:** HIGH -- The current Dockerfile is a scratch image. Go's `embed` package is the standard solution for this exact problem. The Dockerfile will need modification.

**Sources:**
- [Setting up Go templ with Tailwind, HTMX and Docker](https://mbaraa.com/blog/setting-up-go-templ-with-tailwind-htmx-docker)
- [templ: Hosting Using Docker](https://templ.guide/hosting-and-deployment/hosting-using-docker/)

---

## Moderate Pitfalls

Mistakes that cause delays, technical debt, or degraded experience.

---

### Pitfall 6: GSAP Animations Breaking on HTMX DOM Swaps

**What goes wrong:** GSAP animations are applied to elements on page load. When HTMX swaps new content into the page, the animated elements are destroyed and replaced with new, unanimated elements. Animations stop working after the first HTMX interaction. Worse, GSAP timelines and ScrollTrigger instances referencing destroyed elements leak memory and throw errors.

**Why it happens:** GSAP attaches animation state to specific DOM element references. When HTMX replaces those elements, GSAP's references become stale (pointing to removed nodes). Unlike CSS transitions which are declarative and automatically apply to new elements, GSAP animations are imperative -- they must be explicitly re-initialized on new DOM elements.

**Consequences:**
- Animations play once (on initial page load) then never again
- Memory leaks from orphaned GSAP instances referencing removed DOM nodes
- Console errors from GSAP trying to animate `null` targets
- ScrollTrigger instances accumulate, causing performance degradation
- Developer disables HTMX for animated sections, losing the partial-update benefit

**Prevention:**
1. **Re-initialize GSAP on `htmx:afterSettle` events.** Listen for HTMX lifecycle events and re-run GSAP animations on the newly swapped content:

   ```javascript
   document.addEventListener('htmx:afterSettle', function(event) {
     gsap.from(event.detail.target.querySelectorAll('.animate-in'), {
       opacity: 0, y: 20, stagger: 0.1
     });
   });
   ```

2. **Kill existing GSAP instances before swap.** Listen to `htmx:beforeSwap` to kill animations on elements about to be removed:

   ```javascript
   document.addEventListener('htmx:beforeSwap', function(event) {
     gsap.killTweensOf(event.detail.target.querySelectorAll('*'));
     ScrollTrigger.getAll().forEach(st => {
       if (event.detail.target.contains(st.trigger)) st.kill();
     });
   });
   ```

3. **Use CSS-based animations for simple transitions.** HTMX natively supports CSS transitions via `htmx-added`, `htmx-settling`, and `htmx-swapping` classes. Reserve GSAP for complex sequences (staggered lists, physics-based motion, timeline choreography) and use CSS for simple fade/slide transitions.
4. **Scope GSAP to swap targets.** Do not apply GSAP to the entire page. Scope animations to the specific elements being swapped, making cleanup targeted and predictable.
5. **Create a reusable animation initializer.** A function like `initAnimations(container)` that can be called on any DOM subtree, used both on page load and after HTMX swaps.

**Detection (warning signs):**
- Animations work on first page load but not after clicking HTMX-powered links
- Browser DevTools showing increasing memory usage over time
- Console warnings: "GSAP target not found"
- `ScrollTrigger.getAll().length` growing without bound

**Phase mapping:** Phase 2 or later (after basic GUI is working). GSAP animations are polish, not foundation. Get HTMX+templ+Alpine working first, add GSAP animations last.

**Confidence:** MEDIUM -- GSAP's DOM-reference model is well-understood, but specific HTMX integration patterns are community-sourced, not officially documented by either project.

**Sources:**
- [GSAP: Update animation after DOM change](https://gsap.com/community/forums/topic/35696-update-the-animation-after-the-change-dom/)
- [HTMX: Animations](https://htmx.org/examples/animations/)

---

### Pitfall 7: Jira REST API Rate Limiting is Opaque and Punishing

**What goes wrong:** The developer polls Jira for issue updates using the same aggressive polling pattern used for GitHub. Jira's rate limits are less transparent than GitHub's -- Jira Cloud uses a points-based system where complex queries consume more points than simple ones. The app hits 429s with no clear indication of when to retry, and gets temporarily blocked.

**Why it happens:** Unlike GitHub which publishes clear rate limit headers (`X-RateLimit-Remaining`), Jira Cloud's rate limiting is points-based and the point cost per request is not documented per endpoint. The developer assumes "5 requests per minute is fine" without understanding that a JQL search with many results costs more points than a simple issue fetch.

**Consequences:**
- 429 responses with `Retry-After` headers that may be minutes long
- Temporary IP or token blocking for sustained abuse
- Starting March 2, 2026, new tiered quota rate limits apply to all OAuth 2.0 apps (though API token traffic is governed by existing burst limits)
- No way to predict remaining budget without trial and error

**Prevention:**
1. **Use webhooks instead of polling for Jira.** Jira supports webhooks for issue updates. Configure a webhook to POST to your app when issues change state. This eliminates polling entirely and is Jira's recommended approach.
2. **If polling is necessary, use JQL with `updated >= -5m`.** Filter to recently-changed issues only. This reduces response size and API point cost.
3. **Implement exponential backoff with jitter on 429.** Respect the `Retry-After` header exactly. Do not retry before the specified time.
4. **Cache Jira responses aggressively.** Issue metadata (project, type, priority) changes rarely. Cache it for hours, not minutes. Only poll for status changes.
5. **Separate Jira polling from GitHub polling.** Use independent polling loops with independent rate budgets. A Jira rate limit should not affect GitHub data freshness.
6. **Use Jira's `fields` parameter** to request only the fields you need. `GET /rest/api/3/issue/KEY?fields=status,summary,assignee` costs fewer points than fetching all fields.

**Detection (warning signs):**
- 429 responses from Jira with long `Retry-After` values
- Jira data going stale for minutes at a time
- Application logs showing repeated Jira request failures

**Phase mapping:** Jira integration phase. Design the Jira adapter with rate limiting awareness from the start.

**Confidence:** MEDIUM -- Jira's rate limiting model is documented at a high level, but per-endpoint point costs are not published. The March 2026 tiered quota changes add uncertainty.

**Sources:**
- [Jira Cloud: Rate Limiting](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/)
- [Deep-Dive Guide to Building a Jira API Integration](https://www.getknit.dev/blog/deep-dive-developer-guide-to-building-a-jira-api-integration)

---

### Pitfall 8: Jira Authentication Model Mismatch (Cloud vs Data Center)

**What goes wrong:** The developer implements Jira authentication for Cloud (email + API token via Basic Auth) but the user runs Jira Data Center (PAT or OAuth). Or vice versa. The app fails to authenticate, and the error message is unhelpful ("401 Unauthorized" with no context on which auth method was expected).

**Why it happens:** Jira Cloud and Jira Data Center/Server have different authentication mechanisms:
- **Jira Cloud:** Email + API token sent as Basic Auth (`email:token` base64-encoded), or OAuth 2.0
- **Jira Data Center:** Personal Access Token (Bearer token), or username + password (deprecated), or OAuth 1.0a
- Atlassian is retiring basic auth with username/password pairs, but API tokens still use Basic Auth format (confusingly)

**Consequences:**
- App works for Cloud users but fails silently for Data Center users (or vice versa)
- Users provide the wrong credential format (PAT where API token is expected)
- OAuth vs Basic Auth requires completely different flows and token storage
- Error messages like "401" do not explain what the user needs to change

**Prevention:**
1. **Pick one Jira deployment model and document it.** For a personal tool, Jira Cloud with API token is the simplest. Explicitly state "Jira Cloud only" in documentation and configuration UI.
2. **Validate credentials on save.** When the user enters Jira credentials via the GUI, immediately test them with a `GET /rest/api/3/myself` call. Show success/failure before saving.
3. **Provide clear configuration guidance.** The GUI should explain: "Create a Jira API token at https://id.atlassian.com/manage-profile/security/api-tokens. Enter your email and the token."
4. **If supporting both Cloud and Data Center,** add a `jira_deployment_type` configuration field that switches the authentication adapter. Use the Strategy pattern -- the Jira port interface is the same, but `JiraCloudAdapter` and `JiraDataCenterAdapter` handle auth differently.

**Detection (warning signs):**
- "401 Unauthorized" from Jira with no additional context
- Users confused about which credentials to enter
- Auth working for some users but not others (different Jira deployments)

**Phase mapping:** Jira integration phase, first task. Authentication must work before any Jira features can be built.

**Confidence:** HIGH -- Jira Cloud vs Data Center auth differences are well-documented by Atlassian.

**Sources:**
- [How to Secure Jira REST API Calls in Data Center](https://success.atlassian.com/solution-resources/agile-and-devops-ado/platform-administration/how-to-secure-jira-and-confluence-rest-api-calls-in-data-center)
- [Top 5 REST API Authentication Challenges in Jira](https://www.miniorange.com/blog/rest-api-authentication-problems-solved/)

---

### Pitfall 9: GitHub Review Submission API Triggering Secondary Rate Limits

**What goes wrong:** The GUI lets users submit reviews (approve, request changes, comment) via the GitHub API. The developer uses `POST /repos/{owner}/{repo}/pulls/{number}/reviews` which is a write endpoint. Creating content quickly triggers GitHub's secondary (abuse) rate limits, which have lower thresholds than the primary 5,000/hr limit and are not well-documented.

**Why it happens:** GitHub's secondary rate limits specifically target "creating content too quickly." Review submissions, comments, and status updates are write operations that GitHub monitors more aggressively. The existing polling-only app only does reads; adding write operations changes the rate limit risk profile.

**Consequences:**
- 403 or 429 with abuse detection message
- Temporary block that affects ALL API operations (reads and writes) for the token
- The existing polling loop stops working because the shared token is blocked
- Users cannot submit reviews during the block period

**Prevention:**
1. **Separate read and write tokens.** Use one PAT for polling (read-only, `repo:read` scope) and another for write operations (review submission). If one is blocked, the other continues working.
2. **Rate-limit write operations client-side.** Add a minimum delay between write operations (e.g., 1 second between review submissions). Users are unlikely to submit reviews faster than this.
3. **Queue write operations.** Do not fire API calls directly from the HTTP handler. Enqueue write operations and process them with controlled pacing.
4. **Show clear feedback on rate limit errors.** If a review submission is rate-limited, show "GitHub is temporarily limiting requests. Your review will be submitted shortly." and retry automatically.
5. **The existing `gofri/go-github-ratelimit/v2` middleware handles secondary rate limits.** Ensure it is applied to the write client as well, not just the polling client.

**Detection (warning signs):**
- "You have exceeded a secondary rate limit" error messages
- Review submissions failing intermittently
- Polling data going stale after a burst of review submissions
- 403 responses on read operations that were previously working

**Phase mapping:** Review submission feature phase. Must be designed before the "submit review from GUI" feature.

**Confidence:** HIGH -- GitHub's secondary rate limit behavior for content creation is well-documented.

**Sources:**
- [GitHub: Rate Limits for the REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
- [GitHub: REST API for Pull Request Reviews](https://docs.github.com/en/rest/pulls/reviews)

---

### Pitfall 10: Tailwind CSS Build Complexity in Multi-Stage Docker Build

**What goes wrong:** Tailwind CSS requires a build step that scans template files for class usage and generates a purged CSS file. In the existing Dockerfile, there is no Node.js runtime and no Tailwind CLI. The developer either includes the entire Tailwind CDN (300KB+ unpurged) or tries to add a Node.js build stage to the lean Docker pipeline.

**Why it happens:** Tailwind CSS v3+ uses JIT compilation that requires scanning source files. The `.templ` files contain Tailwind classes, but the Tailwind CLI does not know about `.templ` file format by default. Without proper configuration, Tailwind either misses classes (purging too aggressively) or includes everything (CDN mode, bloated CSS).

**Consequences:**
- Production CSS missing classes used in templ templates (broken styling)
- Bloated CSS file if purging is disabled (300KB+ instead of ~10KB)
- Docker build complexity increases significantly with Node.js stage
- Build times increase with every Tailwind version upgrade

**Prevention:**
1. **Use the Tailwind standalone CLI** (no Node.js required). Download the platform-specific binary in the Dockerfile build stage:
   ```dockerfile
   RUN curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
       && chmod +x tailwindcss-linux-x64 \
       && ./tailwindcss-linux-x64 -i input.css -o static/styles.css --minify
   ```
2. **Configure Tailwind to scan `.templ` files.** In `tailwind.config.js`, add `"./internal/**/*.templ"` to the `content` array. Tailwind's JIT scanner treats them as text files and finds class names.
3. **Embed the built CSS via `//go:embed`.** After Tailwind builds the CSS in the Docker build stage, copy it to the embed directory before `go build`.
4. **Alternative: Use Tailwind CDN for simplicity.** For a single-user tool, the CDN play-mode (`<script src="https://cdn.tailwindcss.com">`) avoids the entire build pipeline. Accept the larger payload (~300KB) in exchange for zero build complexity. This is a valid tradeoff for a personal tool.
5. **Order the Dockerfile stages correctly.** Tailwind build must happen AFTER templ files are copied but BEFORE `go build` (so the CSS can be embedded).

**Detection (warning signs):**
- Missing CSS classes in production (elements unstyled)
- CSS file size > 100KB in production (unpurged)
- Dockerfile adding a `node:alpine` build stage
- `tailwind.config.js` not listing `.templ` in content paths

**Phase mapping:** Phase 1 (GUI foundation). The CSS build pipeline must work before any styled components are created.

**Confidence:** HIGH -- Tailwind's standalone CLI and content scanning are well-documented. The `.templ` file scanning is a known configuration step.

---

## Minor Pitfalls

Mistakes that cause annoyance but are fixable without major rework.

---

### Pitfall 11: HTMX History Cache Conflicts with Alpine.js Template State

**What goes wrong:** HTMX's history cache saves and restores HTML snapshots for back/forward navigation. When a cached page is restored, Alpine.js components re-initialize from their `x-data` attributes, but any state that was modified client-side (open dropdowns, selected tabs, expanded accordions) reverts to the initial state. If `x-if` or `<template>` tags were used, their content may be missing from the cached HTML entirely.

**Why it happens:** HTMX serializes the DOM to HTML for history caching. Alpine's `<template>` elements and conditionally-rendered content (`x-if`) exist in a state that HTMX cannot capture. When the HTML is restored, Alpine sees fresh `x-data` attributes and re-initializes, losing any runtime state.

**Prevention:**
1. **Use `hx-push-url="false"` on requests that should not be cached.** Modal opens, tab switches, and accordion toggles should not create history entries.
2. **Avoid `x-if` with `<template>` in HTMX-cached pages.** Use `x-show` instead, which toggles `display:none` rather than removing/adding DOM elements. HTMX can cache `x-show` state correctly.
3. **If history caching is needed, disable it selectively.** Use `hx-history="false"` on the body or specific containers where Alpine state is complex.

**Phase mapping:** Phase 2+ (after basic navigation works). History integration is polish, not foundation.

**Confidence:** MEDIUM -- Based on GitHub issues and community discussions.

**Sources:**
- [HTMX history cache and Alpine template tags](https://github.com/alpinejs/alpine/discussions/2924)

---

### Pitfall 12: WebHandler Growing into a God Struct

**What goes wrong:** Following the existing `Handler` pattern, the developer creates a `WebHandler` with every store and service injected. As features grow (PR views, repo management, Jira views, settings, review submission), the constructor takes 12+ parameters. The struct becomes a catch-all that violates SRP.

**Why it happens:** The existing `Handler` struct already takes 8 parameters. Adding Jira-related ports, credential stores, and review submission services doubles this. The developer follows the established pattern without questioning whether it scales.

**Prevention:**
1. **Group web handlers by feature domain.** `PRWebHandler`, `RepoWebHandler`, `JiraWebHandler`, `SettingsWebHandler`. Each has only the dependencies it needs.
2. **Use a handler registry pattern.** A `WebRouter` function takes all handlers and registers routes, similar to the existing `NewServeMux` but for web routes.
3. **Apply ISP aggressively.** If `JiraWebHandler` only needs `JiraIssueReader`, do not inject the full `JiraStore` interface.

**Phase mapping:** Phase 1 (GUI foundation). Set the handler grouping pattern before building features.

**Confidence:** HIGH -- This is a standard Go architecture concern, and the existing codebase already shows the early signs with 8 constructor parameters.

---

### Pitfall 13: templ Component Prop Explosion

**What goes wrong:** Templ components start with simple props but grow to accept 10+ parameters as the UI becomes richer. A `PRCard` component that starts as `PRCard(pr model.PullRequest)` grows to `PRCard(pr model.PullRequest, reviews []model.Review, threads []ReviewThread, isExpanded bool, showActions bool, jiraIssue *JiraIssue, ciStatus string, ...)`. Templates become unreadable.

**Prevention:**
1. **Use view models.** Create dedicated structs for template data: `type PRCardViewModel struct { ... }`. The handler builds the view model from domain data. The template receives one struct.
2. **Compose components.** `PRCard` renders the card frame. `PRReviewSection` renders reviews inside it. Each component has a focused prop set.
3. **Avoid passing domain models directly to templates.** Templates should receive presentation-ready data, not raw domain objects. This also prevents the templ package from importing domain types unnecessarily.

**Phase mapping:** Phase 1 (GUI foundation). Establish the view model pattern with the first template.

**Confidence:** HIGH -- Standard MVC/MVVM pattern, applicable to any template system.

---

### Pitfall 14: CORS Issues When GUI and API Share the Same Origin

**What goes wrong:** The developer adds CORS middleware for the JSON API (needed by external consumers like Claude Code CLI) and accidentally applies it to the HTML GUI routes. Or conversely, forgets that the GUI making HTMX requests to its own server does not need CORS at all, and wastes time debugging "why does CORS work in development but not production."

**Prevention:**
1. **HTMX requests to the same origin do not need CORS.** If the GUI is served from the same Go server, HTMX requests are same-origin and CORS is irrelevant.
2. **Apply CORS middleware only to `/api/v1/*` routes,** not to `/app/*` routes. Use route-scoped middleware, not global middleware.
3. **The existing middleware stack** (`loggingMiddleware`, `recoveryMiddleware`) is global. Add CORS as route-scoped, not another global wrapper.

**Phase mapping:** Phase 1 (GUI foundation). Middleware scoping should be decided when adding web routes.

**Confidence:** HIGH.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation | Severity |
|-------------|---------------|------------|----------|
| GUI foundation (templ + routing) | Content negotiation trap; templ build pipeline missing | Separate route namespaces; add `templ generate` to Dockerfile | Critical |
| Asset pipeline (Tailwind + JS) | Static assets missing from scratch image | `//go:embed` all static files; Tailwind standalone CLI in Docker | Critical |
| HTMX + Alpine.js integration | Alpine state destroyed on swaps | Alpine Morph extension; scope swap targets below `x-data` containers | Critical |
| GSAP animations | Animations break after first swap | Re-initialize on `htmx:afterSettle`; kill on `htmx:beforeSwap` | Moderate |
| Jira integration | Auth model mismatch; rate limits opaque | Pick Cloud-only; validate creds on save; webhook instead of polling | Moderate |
| Credential storage | Plaintext tokens in SQLite | AES-256-GCM encryption with env-var key, or env-vars only | Critical |
| GitHub review submission | Secondary rate limits from writes | Separate read/write tokens; client-side rate limiting | Moderate |
| Handler architecture | God struct growth | Group handlers by feature domain; use view models | Minor |

---

## Domain-Specific Insight: The "Two Frameworks Fighting Over the DOM" Problem

The core architectural tension in this stack is that HTMX and Alpine.js both manipulate the DOM, but with conflicting models:

- **HTMX** replaces DOM subtrees entirely (server-rendered HTML swapped in)
- **Alpine.js** binds reactive state to existing DOM elements (client-side reactivity)
- **GSAP** attaches animation state to DOM element references (imperative animation)

When HTMX swaps content, it invalidates Alpine's reactive bindings AND GSAP's animation targets. The solution is clear boundaries:

1. **HTMX owns data content.** PR lists, review threads, Jira issue status -- content that comes from the server.
2. **Alpine.js owns UI state.** Dropdown visibility, tab selection, filter toggles -- client-only state that never goes to the server.
3. **GSAP owns transitions.** Entry animations, list reordering, attention-drawing effects -- visual polish tied to HTMX lifecycle events.

The alpine-morph extension is the glue that makes HTMX and Alpine coexist. Without it, every HTMX swap breaks Alpine. This is not optional -- it is a hard requirement for this stack.

---

## Sources and Confidence Notes

| Source | Type | Confidence Impact |
|--------|------|-------------------|
| [HTMX: Content Negotiation Essay](https://htmx.org/essays/why-tend-not-to-use-content-negotiation/) | Official docs | HIGH |
| [templ: Docker Hosting](https://templ.guide/hosting-and-deployment/hosting-using-docker/) | Official docs | HIGH |
| [templ: Template Generation](https://templ.guide/core-concepts/template-generation/) | Official docs | HIGH |
| [Alpine/HTMX GitHub Discussions](https://github.com/alpinejs/alpine/discussions/3985) | Community verified | HIGH |
| [Jira Cloud: Rate Limiting](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/) | Official docs | HIGH |
| [GitHub: Rate Limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) | Official docs | HIGH |
| GSAP + HTMX integration patterns | Community forums | MEDIUM |
| Credential encryption patterns | Industry standard | HIGH |

**Verification recommended for:**
1. Alpine Morph extension compatibility with current Alpine.js and HTMX versions
2. Tailwind standalone CLI support for `.templ` file scanning (may need custom extractor)
3. Jira Cloud tiered quota rate limits post-March 2026 enforcement
4. GSAP licensing for commercial use (GSAP has a specific license model)
