# Phase 9: Jira Integration - Research

**Researched:** 2026-02-24
**Domain:** Jira Cloud REST API v3, multi-connection credential storage, HTMX collapsible card, hexagonal adapter pattern
**Confidence:** HIGH (codebase patterns), MEDIUM (Jira API specifics via official docs + community)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Jira issue display**
- Collapsible card at the **top** of the PR detail panel (above description/reviews)
- Card auto-fetches Jira issue data on PR detail load (not deferred to expand)
- Collapsed by default; shows issue key + summary in the header
- Expanded view shows: summary, description, status, priority, assignee, and existing comments
- When no Jira key is detected: show a subtle "No linked issue" placeholder (muted, non-intrusive) — do NOT hide the card entirely
- When Jira credentials are not configured: card shows collapsed with "Configure Jira in Settings" and a link to the Settings drawer

**Issue key detection**
- Scan branch name first, then PR title (branch takes priority)
- Auto-detect any `[A-Z]{2,}-\d+` pattern — no prefix configuration required
- When multiple keys are found, first match wins (branch > title, left-to-right within each)
- Detected key is **cached in the database** alongside the PR record; re-extracted when the PR is polled and updated (not re-parsed on every detail render)

**Comment posting UX**
- Comment input lives **inside the expanded Jira card**, below the existing comments
- After successful post: refresh the Jira card inline via HTMX (new comment appears immediately)
- Comment input is **hidden completely** when no Jira credentials are configured (not disabled with a prompt)
- Auth-gate per request via `credStore.Get` (same pattern as GitHub write handlers); additionally validate that the Jira base URL is reachable before attempting the post — return a 422 HTML fragment on connectivity failure

**Multi-Jira connections and per-repo mapping**
- Multiple Jira connections supported, each stored as a named entry (e.g., "Work Jira", "Client Jira")
- Each connection stores: display name, base URL, email, API token
- Per-repo assignment configured in the Settings drawer, in the same repo-level settings section as thresholds (consistent UX with repo thresholds UI)
- A repo can be assigned one Jira connection or "none"; unassigned repos show "No linked issue"
- A "default" Jira connection can be designated as the fallback for repos with no explicit assignment

**Error states**
- Jira API failures (unreachable, 401, 404): show inline error inside the card with a retry button — "Could not load Jira issue: [reason]"
- Credential validation: validate on save (same pattern as GitHub token) — POST triggers a live Jira API call to verify credentials, returns success/error HTML fragment

### Claude's Discretion
- Exact Jira card visual design (colors, icons, spacing) — consistent with existing dashboard aesthetics
- Loading skeleton while Jira data is fetching
- Exact retry mechanism implementation
- DB schema for multi-connection storage
- How Jira connections are listed/managed in the Settings drawer (add/remove/edit flow)

### Deferred Ideas (OUT OF SCOPE)
- Creating Jira issues from the dashboard — future phase
- Jira issue search/browse — future phase
- Transitioning Jira issue status from the dashboard — future phase
- Jira project prefix filtering — auto-detection without config is sufficient for now
</user_constraints>

---

## Summary

Phase 9 integrates Jira Cloud into an existing Go hexagonal application with a templ/HTMX/Alpine.js frontend. The core work involves: (1) a new `jira_connections` table for multi-connection credential storage, (2) a `jira_key` column on pull_requests for cached key detection, (3) a JiraClient port and adapter that calls Jira Cloud REST API v3 using Basic auth (email:token base64-encoded), and (4) a collapsible JiraCard templ component injected above the existing PRDetail tabs.

The existing codebase provides all necessary patterns to follow. The credStore (`CredentialRepo`) handles encrypted individual key-value pairs — but multi-connection Jira storage requires a dedicated `jira_connections` table (similar in concept to `repo_thresholds`). The GitHub write handler auth-gate pattern (`credStore.Get` → 422 on missing creds) applies directly to the Jira comment POST handler.

The main external API complexity is Jira's ADF (Atlassian Document Format): the v3 API returns descriptions and comments as JSON doc nodes rather than plain text, and POSTing a comment requires wrapping the text in ADF envelope JSON. Rate limiting for API token auth is burst-only (existing limits, not affected by the new March 2026 points-based enforcement which only covers OAuth apps).

**Primary recommendation:** Build a thin stdlib `net/http` JiraClient adapter (no third-party library) — the API surface is small (2 endpoints: GET issue, POST comment), and the existing codebase uses no Atlassian libraries. Keep ADF rendering simple: extract plain text from ADF for display; wrap plain text in minimal ADF for posting.

---

## Standard Stack

### Core (no new libraries needed)
| Component | Approach | Why |
|-----------|----------|-----|
| Jira HTTP client | `net/http` stdlib (no external library) | Only 2 endpoints needed; existing project avoids unnecessary deps; go-atlassian adds 15+ transitive deps for 2 API calls |
| ADF rendering | Manual JSON marshal/unmarshal with Go structs | ADF is a simple JSON tree; no library needed for the minimal subset used here |
| Credential encryption | Existing `CredentialRepo` AES-256-GCM | Already established; new `jira_connections` table reuses same encryption infra |
| DB migration | `golang-migrate/migrate/v4` embedded SQL | Already established (currently at migration 000012) |
| UI | `templ` + HTMX + Alpine.js | Already established; JiraCard follows same collapsible pattern used elsewhere |

### No New Dependencies
The phase can be implemented entirely with existing dependencies. The `net/http` client is sufficient for Jira Cloud REST API v3. No `go get` commands needed.

### Alternative Considered and Rejected
| Instead of | Rejected Alternative | Why Rejected |
|------------|---------------------|--------------|
| `net/http` + manual structs | `github.com/ctreminiom/go-atlassian/v2` | go-atlassian v2 has 15+ transitive deps for 2 API calls; project prefers minimal deps |
| `net/http` + manual structs | `github.com/andygrunwald/go-jira` | go-jira v1.17.0 does not explicitly document v3 API support; adds unnecessary abstraction |

---

## Architecture Patterns

### New Files to Create

```
internal/
  domain/
    model/
      jiraconnection.go      # JiraConnection, JiraIssue, JiraComment domain models
    port/
      driven/
        jiraconnectionstore.go  # JiraConnectionStore port interface
        jiraclient.go           # JiraClient port interface
  adapter/
    driven/
      sqlite/
        jiraconnectionrepo.go       # SQLite adapter for jira_connections + repo_jira_mapping
        jiraconnectionrepo_test.go
        migrations/
          000013_add_jira_connections.up.sql
          000013_add_jira_connections.down.sql
          000014_add_jira_key_to_prs.up.sql
          000014_add_jira_key_to_prs.down.sql
      jira/
        client.go           # JiraClient adapter (net/http, ADF handling)
        client_test.go
  adapter/
    driving/
      web/
        templates/
          components/
            jira_card.templ       # Collapsible JiraCard component
            jira_card_templ.go    # Generated
          partials/
            jira_card_content.templ      # HTMX swap target for card refresh
            jira_card_content_templ.go
        viewmodel/
          # Add JiraCardViewModel to existing viewmodel.go
```

### Pattern 1: JiraClient Port Interface

```go
// Source: internal/domain/port/driven/jiraclient.go
// Mirrors the GitHubClient port: minimal interface, only what Phase 9 uses.

package driven

import "context"

// JiraClient defines the driven port for Jira Cloud REST API access.
type JiraClient interface {
    // GetIssue fetches a Jira issue by key (e.g., "PROJ-123").
    // Returns ErrJiraNotFound if the issue does not exist or is not accessible.
    // Returns ErrJiraUnauthorized if credentials are invalid.
    // Returns ErrJiraUnavailable if the Jira instance is unreachable.
    GetIssue(ctx context.Context, key string) (model.JiraIssue, error)

    // AddComment posts a plain-text comment to the given Jira issue.
    // The adapter wraps the text in ADF format before sending.
    AddComment(ctx context.Context, key, body string) error

    // Ping validates connectivity to the Jira instance (e.g., GET /rest/api/3/myself).
    // Used for credential validation on save.
    Ping(ctx context.Context) error
}

// Sentinel errors returned by JiraClient implementations.
var (
    ErrJiraNotFound     = errors.New("jira: issue not found")
    ErrJiraUnauthorized = errors.New("jira: invalid credentials")
    ErrJiraUnavailable  = errors.New("jira: instance unreachable")
)
```

### Pattern 2: JiraConnectionStore Port Interface

The existing `CredentialStore` (service key-value pairs) is insufficient for multi-connection Jira data. Introduce a dedicated port.

```go
// Source: internal/domain/port/driven/jiraconnectionstore.go

package driven

import "context"

// JiraConnectionStore defines the driven port for Jira connection persistence.
type JiraConnectionStore interface {
    // Create adds a new Jira connection. Returns the assigned ID.
    Create(ctx context.Context, conn model.JiraConnection) (int64, error)

    // Update replaces an existing connection by ID.
    Update(ctx context.Context, conn model.JiraConnection) error

    // Delete removes a connection by ID. Cascades to repo mappings.
    Delete(ctx context.Context, id int64) error

    // List returns all stored connections (tokens decrypted).
    List(ctx context.Context) ([]model.JiraConnection, error)

    // GetByID returns a single connection by ID.
    // Returns (zero, nil) if not found.
    GetByID(ctx context.Context, id int64) (model.JiraConnection, error)

    // GetForRepo returns the Jira connection assigned to a repo (by full name).
    // Falls back to the default connection if no explicit mapping exists.
    // Returns (zero, nil) if no connection applies.
    GetForRepo(ctx context.Context, repoFullName string) (model.JiraConnection, error)

    // SetRepoMapping assigns a Jira connection (by ID) to a repo.
    // Pass connectionID=0 to clear the mapping (repo uses default).
    SetRepoMapping(ctx context.Context, repoFullName string, connectionID int64) error

    // SetDefault designates a connection as the default fallback.
    // Pass id=0 to clear the default.
    SetDefault(ctx context.Context, id int64) error
}
```

### Pattern 3: JiraConnection Domain Model

```go
// Source: internal/domain/model/jiraconnection.go

package model

import "time"

// JiraConnection represents a named Jira Cloud connection.
// The Token field holds the plaintext API token at the domain boundary;
// the SQLite adapter encrypts before write and decrypts after read.
type JiraConnection struct {
    ID          int64
    DisplayName string    // e.g., "Work Jira"
    BaseURL     string    // e.g., "https://mycompany.atlassian.net"
    Email       string    // Atlassian account email
    Token       string    // API token (plaintext at domain boundary; encrypted in DB)
    IsDefault   bool      // Whether this is the fallback connection
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// JiraIssue holds the display-ready fields from a Jira issue.
type JiraIssue struct {
    Key         string
    Summary     string
    Description string // Extracted plain text from ADF; empty if no description
    Status      string // e.g., "In Progress"
    Priority    string // e.g., "High"
    Assignee    string // Display name or empty
    Comments    []JiraComment
}

// JiraComment represents a single comment on a Jira issue.
type JiraComment struct {
    Author    string
    Body      string // Plain text extracted from ADF
    CreatedAt time.Time
}
```

### Pattern 4: DB Migration — Multi-Connection Storage

Migration 000013 (new table, next in sequence after 000012):

```sql
-- 000013_add_jira_connections.up.sql
CREATE TABLE IF NOT EXISTS jira_connections (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    display_name TEXT     NOT NULL,
    base_url     TEXT     NOT NULL,
    email        TEXT     NOT NULL,
    token        TEXT     NOT NULL DEFAULT '',   -- AES-256-GCM encrypted
    is_default   INTEGER  NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS repo_jira_mapping (
    repo_full_name    TEXT    NOT NULL PRIMARY KEY,
    jira_connection_id INTEGER,
    FOREIGN KEY (repo_full_name) REFERENCES repositories(full_name) ON DELETE CASCADE,
    FOREIGN KEY (jira_connection_id) REFERENCES jira_connections(id) ON DELETE SET NULL
);
```

Migration 000014 (add `jira_key` to PRs):

```sql
-- 000014_add_jira_key_to_prs.up.sql
ALTER TABLE pull_requests ADD COLUMN jira_key TEXT NOT NULL DEFAULT '';
```

### Pattern 5: Jira Key Detection in PollService

Detection runs during polling (not per-render). Regex: `[A-Z]{2,}-\d+`. Branch takes priority over title.

```go
// Source: internal/application/ (pollservice.go or jirakey.go)
var jiraKeyPattern = regexp.MustCompile(`[A-Z]{2,}-\d+`)

// ExtractJiraKey returns the first Jira issue key found in branch name,
// then PR title. Returns "" if none found.
func ExtractJiraKey(branch, title string) string {
    if m := jiraKeyPattern.FindString(branch); m != "" {
        return m
    }
    return jiraKeyPattern.FindString(title)
}
```

The PR domain model needs a `JiraKey string` field added. The `PRRepo.Upsert` method and DB query need updating.

### Pattern 6: JiraClient HTTP Adapter (net/http)

```go
// Source: internal/adapter/driven/jira/client.go

// Auth header construction — Basic auth with email:token base64-encoded.
func basicAuthHeader(email, token string) string {
    creds := email + ":" + token
    return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

// GET issue: GET {baseURL}/rest/api/3/issue/{key}?fields=summary,description,status,priority,assignee,comment
// POST comment: POST {baseURL}/rest/api/3/issue/{key}/comment
//   Body: {"body": <ADF doc>}

// Minimum ADF body for a plain-text comment:
type adfDoc struct {
    Version int         `json:"version"`
    Type    string      `json:"type"`
    Content []adfBlock  `json:"content"`
}
type adfBlock struct {
    Type    string      `json:"type"`
    Content []adfInline `json:"content,omitempty"`
}
type adfInline struct {
    Type string `json:"type"`
    Text string `json:"text,omitempty"`
}

func plainTextToADF(text string) adfDoc {
    return adfDoc{
        Version: 1,
        Type:    "doc",
        Content: []adfBlock{{
            Type: "paragraph",
            Content: []adfInline{{Type: "text", Text: text}},
        }},
    }
}
```

### Pattern 7: JiraCard Templ Component

The card is inserted into `PRDetail` component above the info section. It receives a `JiraCardViewModel`:

```go
// JiraCardViewModel in viewmodel/viewmodel.go
type JiraCardViewModel struct {
    HasCredentials bool
    JiraKey        string       // "" when not detected
    Issue          *JiraIssueVM // nil on load error or when no key
    LoadError      string       // non-empty when Jira fetch failed
}

type JiraIssueVM struct {
    Key         string
    Summary     string
    Description string
    Status      string
    Priority    string
    Assignee    string
    Comments    []JiraCommentVM
    JiraURL     string // link to issue in Jira
}

type JiraCommentVM struct {
    Author    string
    Body      string
    CreatedAt string
}
```

The HTMX swap target for card refresh after comment POST:
- `hx-target="#jira-card"` with `hx-swap="morph"` for Alpine-state-preserving refresh

### Pattern 8: Write Handler Auth-Gate for Jira Comment

Follows identical structure to `CreateIssueComment`:

```go
// POST /app/prs/{owner}/{repo}/{number}/jira-comment
func (h *Handler) CreateJiraComment(w http.ResponseWriter, r *http.Request) {
    // 1. Parse form + CSRF
    // 2. Check credStore nil guard
    // 3. credStore.Get("jira_connection_id") — or derive connection from repo mapping
    // 4. If missing: 422 with HTML fragment
    // 5. Validate base URL reachable (Ping) before post; 422 on connectivity failure
    // 6. jiraClient.AddComment(ctx, jiraKey, body)
    // 7. On success: re-render JiraCard partial via morph swap
}
```

### Pattern 9: Settings Drawer — Jira Connections Section

The existing settings drawer has a single "Jira" subsection with a single-connection form stub (Phase 8 placeholder). Phase 9 needs to replace that stub with a multi-connection UI. The existing tabs are: `credentials` and `thresholds`.

The existing stub form posts to `POST /app/settings/jira`. In Phase 9:
- The credentials tab Jira subsection becomes a list of named connections + "Add connection" form
- Per-repo assignment (which connection to use) is added to the repo settings popover (alongside the existing threshold controls in `repo_threshold_popover.templ`)

### Anti-Patterns to Avoid

- **Storing Jira credentials in the existing `credentials` table**: The `credentials` table uses `service TEXT NOT NULL UNIQUE` — it's a key-value store, not a multi-row connection store. Multiple Jira connections need their own table.
- **Re-parsing jira_key on every detail render**: The key must be cached in the DB (polled + upserted), not extracted from PR fields in the HTTP handler.
- **Injecting JiraClient into PollService for fetching**: PollService only stores the detected key — it does NOT call the Jira API. The Jira API call happens in the web handler when PR detail is requested.
- **Using Jira REST API v2 instead of v3**: v2 returns Wiki Markup in description; v3 returns ADF JSON. The project should use v3 consistently.
- **Treating ADF as plain text**: ADF is JSON, not a string. Description and comment bodies returned by v3 are `{"type":"doc","version":1,"content":[...]}`. Must unmarshal and extract text nodes.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| AES-256-GCM encryption for Jira token | New encryption code | Existing `CredentialRepo.encrypt/decrypt` methods (or extract to shared util) | Already battle-tested in Phase 8; same key management |
| Jira key regex | Complex parsing logic | `regexp.MustCompile(\`[A-Z]{2,}-\d+\`)` | Standard pattern; all Jira project keys match this format |
| ADF-to-text extraction | Full ADF parser | Walk `content` array, collect `text` nodes recursively | ADF is a tree; a 15-line recursive function covers all display cases |

**Key insight:** The Jira REST API surface for this phase is exactly 3 calls (GET issue, POST comment, GET myself for ping). A thin adapter with manual JSON structs is more maintainable than importing a full Atlassian client library.

---

## Common Pitfalls

### Pitfall 1: ADF Description Is JSON, Not a String
**What goes wrong:** Handler tries to use `issue.Fields.Description` directly as a string — it's actually a raw JSON blob (`{"type":"doc",...}`).
**Why it happens:** Jira REST API v3 changed the description field from Wiki Markup (v2) to ADF JSON. The field type is `interface{}` or `json.RawMessage` in Go structs.
**How to avoid:** Define explicit ADF Go structs. Unmarshal `description` into `adfDoc`. Walk the content tree to extract text nodes for display.
**Warning signs:** Description shows up as `[object Object]` or raw JSON in the UI.

### Pitfall 2: Rate Limiting Not Signaled Until 429
**What goes wrong:** Handler doesn't check `429 Too Many Requests` responses from Jira; silently fails or panics.
**Why it happens:** Jira's rate limiting is opaque — no pre-flight warning, just a sudden 429.
**How to avoid:** JiraClient adapter must explicitly handle HTTP 429 by returning `ErrJiraUnavailable` with the `Retry-After` header value logged. The card shows "Could not load Jira issue: rate limit exceeded" with retry button.
**Note on API token auth:** API token-based auth is subject to burst limits only (100 RPS for GET, 50 for PUT/DELETE). The new points-based quota enforcement starting March 2026 applies to OAuth apps only — API tokens are exempt. For a single-user dashboard, rate limiting is unlikely in practice.

### Pitfall 3: Multi-Connection Default Resolution
**What goes wrong:** `GetForRepo` returns no connection for a repo that has no explicit mapping AND no default is set — handler panics on nil JiraConnection.
**Why it happens:** Both `repo_jira_mapping` and the `is_default` flag can be absent.
**How to avoid:** `GetForRepo` must return `(zero, nil)` when no connection applies. The handler checks: if `conn.ID == 0`, skip Jira fetch entirely and render "No linked issue" placeholder (same as no jira_key).

### Pitfall 4: JiraKey Column Missing from PR Upsert
**What goes wrong:** Migration adds `jira_key` column but `PRRepo.Upsert` doesn't include it — key is never written, always reads as empty string.
**Why it happens:** The upsert query is a long explicit column list; new columns must be added manually.
**How to avoid:** Update `PRRepo.Upsert` INSERT and ON CONFLICT DO UPDATE to include `jira_key`. Also update `PRRepo.Get`/`ListAll` scan to populate `PR.JiraKey`.

### Pitfall 5: JiraCard Triggers Double Fetch on Load
**What goes wrong:** The PR detail partial renders the JiraCard with an `hx-get` that fires on load, causing two sequential Jira API calls (one on page render, one via HTMX).
**Why it happens:** "auto-fetch on PR detail load" could be misimplemented as an HTMX trigger rather than a server-side fetch.
**How to avoid:** The Jira issue is fetched server-side when the PR detail partial renders (`GET /app/prs/{owner}/{repo}/{number}`). The handler calls `jiraClient.GetIssue` and passes the result to the JiraCard template. HTMX is only used for the comment POST refresh — not for initial load.

### Pitfall 6: ADF Comment POST Body
**What goes wrong:** POST to `POST /rest/api/3/issue/{key}/comment` with `{"body": "plain text"}` gets rejected with 400 "Comment body is not valid!".
**Why it happens:** Jira REST API v3 requires ADF format for comment bodies, not plain text strings.
**How to avoid:** Always wrap the plain-text body in `plainTextToADF(text)` before posting. The ADF envelope is minimal (see Pattern 6 above).

### Pitfall 7: Settings Drawer Jira Stub Must Be Replaced, Not Patched
**What goes wrong:** Phase 9 tries to incrementally update the Phase 8 single-connection Jira stub — results in confusing dual-form state.
**Why it happens:** The existing `settings_drawer.templ` has a single "Jira" form stub (lines 131-201) marked "(Phase 9)".
**How to avoid:** In Phase 9 plans, explicitly replace the entire Jira subsection of the credentials tab with the multi-connection UI. The handler `SaveJiraCredentials` (existing stub) also needs to be replaced by separate Create/Update/Delete connection handlers.

---

## Code Examples

### Jira Basic Auth Header
```go
// Source: https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/
import "encoding/base64"

func basicAuthHeader(email, token string) string {
    return "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+token))
}
```

### GET Issue Request (stdlib net/http)
```go
// GET {baseURL}/rest/api/3/issue/{key}?fields=summary,description,status,priority,assignee,comment
req, err := http.NewRequestWithContext(ctx, http.MethodGet,
    c.baseURL+"/rest/api/3/issue/"+key+"?fields=summary,description,status,priority,assignee,comment",
    nil)
req.Header.Set("Authorization", basicAuthHeader(c.email, c.token))
req.Header.Set("Accept", "application/json")
resp, err := c.httpClient.Do(req)
// Handle 401 → ErrJiraUnauthorized, 404 → ErrJiraNotFound, 429 → ErrJiraUnavailable
```

### POST Comment with ADF Body
```go
// POST {baseURL}/rest/api/3/issue/{key}/comment
// Body: {"body": {"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"..."}]}]}}
type commentRequest struct {
    Body adfDoc `json:"body"`
}
payload, _ := json.Marshal(commentRequest{Body: plainTextToADF(text)})
req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
    c.baseURL+"/rest/api/3/issue/"+key+"/comment",
    bytes.NewReader(payload))
req.Header.Set("Authorization", basicAuthHeader(c.email, c.token))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json")
```

### Ping (Credential Validation)
```go
// GET {baseURL}/rest/api/3/myself
// Returns 200 with user info on success, 401 on bad creds
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/rest/api/3/myself", nil)
req.Header.Set("Authorization", basicAuthHeader(c.email, c.token))
req.Header.Set("Accept", "application/json")
resp, err := c.httpClient.Do(req)
if resp.StatusCode == 401 { return ErrJiraUnauthorized }
```

### ADF Text Extraction
```go
// Recursively extract all text nodes from ADF content tree.
func extractADFText(doc adfDoc) string {
    var sb strings.Builder
    var walk func(nodes []adfBlock)
    walk = func(nodes []adfBlock) {
        for _, node := range nodes {
            for _, inline := range node.Content {
                if inline.Type == "text" {
                    sb.WriteString(inline.Text)
                }
            }
            // Note: adfBlock.Content is []adfInline here — for nested blocks,
            // the real ADF tree has blocks-within-blocks; use json.RawMessage
            // for recursive nodes in the actual implementation.
        }
    }
    walk(doc.Content)
    return sb.String()
}
// In practice: use json.RawMessage for content nodes and unmarshal recursively,
// or use a flat text extraction that handles paragraph/text/hardBreak node types.
```

### Jira Key Regex (in PollService/application layer)
```go
// Source: standard Jira project key format, verified against official docs
var jiraKeyPattern = regexp.MustCompile(`[A-Z]{2,}-\d+`)

func ExtractJiraKey(branch, title string) string {
    if m := jiraKeyPattern.FindString(branch); m != "" {
        return m
    }
    return jiraKeyPattern.FindString(title)
}
```

### HTMX JiraCard Refresh After Comment POST
```html
<!-- jira_card.templ snippet — comment form inside expanded card -->
<form
  hx-post="/app/prs/{owner}/{repo}/{number}/jira-comment"
  hx-target="#jira-card"
  hx-swap="morph"
  hx-ext="alpine-morph"
  hx-indicator="#jira-comment-spinner"
  class="mt-3"
>
  <textarea name="body" ...></textarea>
  <button type="submit">Comment</button>
  <span id="jira-comment-spinner" class="htmx-indicator">...</span>
</form>
```

### Settings Drawer — Jira Connections Management
The existing `settings_drawer.templ` has a single-connection Jira stub at lines 131-201. In Phase 9 it becomes a connection list with "Add connection" functionality. The structure follows the existing tabs pattern:

```go
// New routes to add to routes.go:
mux.HandleFunc("POST /app/settings/jira/connections", h.CreateJiraConnection)
mux.HandleFunc("PUT /app/settings/jira/connections/{id}", h.UpdateJiraConnection)
mux.HandleFunc("DELETE /app/settings/jira/connections/{id}", h.DeleteJiraConnection)
mux.HandleFunc("POST /app/settings/jira/connections/{id}/default", h.SetDefaultJiraConnection)
mux.HandleFunc("POST /app/settings/jira/repo-mapping", h.SaveJiraRepoMapping)
// Existing (to remove/replace):
// POST /app/settings/jira  → replaced by POST /app/settings/jira/connections
```

---

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| Jira REST API v2 (Wiki Markup) | Jira REST API v3 (ADF JSON) | Description/comment bodies are JSON trees, not strings |
| OAuth app rate limits | API token exempt from March 2026 points quota | API token auth not affected by new enforcement |
| Single Jira connection (Phase 8 stub) | Multi-named connections with per-repo assignment | Requires dedicated table, not key-value credential store |

**Jira REST API versions:**
- v2: Supported but returns Wiki Markup — do not use for new code
- v3: Current standard; use this — returns ADF for rich text fields

---

## Open Questions

1. **ADF recursive content tree depth**
   - What we know: ADF is a recursive tree (blocks can contain blocks, e.g., bullet lists with nested paragraphs). A simple two-level walk (doc → paragraph → text) covers most cases.
   - What's unclear: Whether issue descriptions in typical Jira usage use deeply nested ADF (tables, code blocks, nested lists) — these would be lost by a shallow text extraction.
   - Recommendation: Implement a simple recursive text extractor that handles `text`, `paragraph`, `bulletList`, `listItem`, `hardBreak`, `codeBlock` node types. Display result as plain text — markdown rendering is not required for this phase.

2. **JiraClient injection into web Handler**
   - What we know: The web Handler receives its dependencies via constructor injection. JiraClient must be injected. However, JiraClient needs connection-specific credentials (baseURL, email, token) which vary per-repo.
   - What's unclear: Whether to inject a single JiraClient factory (`func(conn model.JiraConnection) driven.JiraClient`) or inject the JiraConnectionStore and construct the client per-request.
   - Recommendation: Inject a `jiraClientFactory func(conn model.JiraConnection) driven.JiraClient` — mirrors the existing `writerFactory func(token string) driven.GitHubWriter` pattern. This keeps the Handler decoupled from the concrete adapter.

3. **Phase 8 stub handler replacement**
   - What we know: `SaveJiraCredentials` in `handler.go` stores three keys (`jira_url`, `jira_email`, `jira_token`) in the existing credentials table. This must be replaced.
   - What's unclear: Whether any production data lives in these credential rows that needs migrating.
   - Recommendation: Migration 000013 creates `jira_connections`. A note in the plan should indicate that any values stored under `jira_url`/`jira_email`/`jira_token` in the credentials table are orphaned by this migration (they can be deleted). The existing `SaveJiraCredentials` handler is replaced by `CreateJiraConnection`.

---

## Existing Codebase Patterns to Follow

| Pattern | Where It Lives | How Phase 9 Uses It |
|---------|---------------|---------------------|
| Credential encryption (AES-256-GCM) | `sqlite/credentialrepo.go` | Encrypt `jira_connections.token` with same encrypt/decrypt logic; extract to shared util or copy to `jiraconnectionrepo.go` |
| Write handler auth-gate (422 on missing creds) | `handler.go: CreateIssueComment` | Jira comment POST handler follows identical check pattern |
| writerFactory closure injection | `handler.go: Handler.writerFactory` | `jiraClientFactory func(conn model.JiraConnection) driven.JiraClient` mirrors this |
| Port interface compile-time check | `sqlite/credentialrepo.go: var _ driven.CredentialStore = ...` | Add to `sqlite/jiraconnectionrepo.go` and `jira/client.go` |
| morph swap for partial re-render | `routes.go: hx-swap="morph"`, multiple handlers | JiraCard refresh after comment POST uses same pattern |
| HTMX fragment response (HTML, not JSON) | All write handlers | Jira comment POST returns updated JiraCard HTML fragment |
| Alpine x-data collapsible | `repo_threshold_popover.templ` | JiraCard uses `x-data="{ expanded: false }"` + `x-show="expanded"` for collapsed-by-default behavior |
| DB migration naming | `migrations/000012_...` | Phase 9 adds `000013_add_jira_connections` and `000014_add_jira_key_to_prs` |
| HTMX spinner pattern | `settings_drawer.templ: htmx-indicator` | Jira card uses same loading indicator while fetching |
| Per-repo settings popover | `repo_threshold_popover.templ` | Per-repo Jira connection assignment added to same popover |

---

## Sources

### Primary (HIGH confidence)
- Codebase: `/home/esfisher/dev/mygitpanel/internal/adapter/driven/sqlite/credentialrepo.go` — full encryption pattern
- Codebase: `/home/esfisher/dev/mygitpanel/internal/adapter/driving/web/handler.go` — auth-gate, writerFactory, SaveJiraCredentials stub
- Codebase: `/home/esfisher/dev/mygitpanel/internal/adapter/driving/web/templates/components/` — all templ patterns
- Codebase: `/home/esfisher/dev/mygitpanel/internal/adapter/driving/web/routes.go` — route registration
- Codebase: `/home/esfisher/dev/mygitpanel/internal/adapter/driven/sqlite/migrations/` — migration naming convention
- [Jira Cloud Rate Limiting](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/) — confirmed headers, 429 behavior, API token exemption from new quota
- [Jira Basic Auth](https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/) — confirmed Base64(email:token) Authorization header format
- [ADF Structure](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) — confirmed minimal doc/paragraph/text JSON structure

### Secondary (MEDIUM confidence)
- [go-atlassian pkg.go.dev](https://pkg.go.dev/github.com/ctreminiom/go-atlassian) — confirmed library exists, `github.com/ctreminiom/go-atlassian/v2/jira/v3` import path; rejected for this project due to dependency weight
- WebSearch community: ADF comment POST format — cross-verified with official ADF docs; "body must be ADF not plain string" confirmed

### Tertiary (LOW confidence)
- WebSearch: Jira Cloud burst rate limits are 100 RPS GET / 50 RPS PUT/DELETE — mentioned in community posts, not directly verified from official rate limit page (which confirms the 3-tier model and API token exemption but not specific numbers)

---

## Metadata

**Confidence breakdown:**
- Standard stack (no new deps): HIGH — all patterns verified from codebase
- Jira API authentication: HIGH — official Atlassian docs verified
- ADF format: HIGH — official Atlassian docs + community cross-verification
- Rate limiting: MEDIUM — official docs confirm token exemption from new quota; specific burst numbers LOW
- Multi-connection DB schema: HIGH — design follows established migration patterns
- Architecture patterns: HIGH — all derived from existing codebase conventions

**Research date:** 2026-02-24
**Valid until:** 2026-03-24 (stable domain; Jira API v3 is stable; rate limit changes March 2026 do not affect API token users)
