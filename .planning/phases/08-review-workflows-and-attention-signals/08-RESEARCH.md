# Phase 8 Research

**Date:** 2026-02-19
**Phase:** Review Workflows and Attention Signals

---

## 1. go-github v82 Write Operations

### 1.1 Submit a Review (CreateReview)

`PullRequestsService.CreateReview` creates a review with optional line comments in a single API call.

```go
review, _, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, prNumber,
    &gh.PullRequestReviewRequest{
        CommitID: gh.Ptr("abc123sha"),    // HEAD SHA to attach review to
        Body:     gh.Ptr("LGTM overall"),
        Event:    gh.Ptr("APPROVE"),      // "APPROVE", "REQUEST_CHANGES", or "COMMENT"
        Comments: []*gh.DraftReviewComment{
            {
                Path:     gh.Ptr("internal/foo/bar.go"),
                Position: gh.Ptr(5),      // diff position (not line number)
                Body:     gh.Ptr("Nit: rename this"),
            },
        },
    },
)
```

**Important:** `Position` is the diff position (1-indexed line in the unified diff), not the file line number. GitHub returns `DiffHunk` in review comments which includes the diff context — this is what must be used to render line context in the UI.

**Alternative fields** for line comments (newer API):
- `Line int` — source file line number (requires `Side: "RIGHT"` or `"LEFT"`)
- `StartLine int` — for multi-line comments

### 1.2 Reply to a Review Thread (CreateComment)

To reply to an existing thread, use `PullRequests.CreateComment` with `InReplyTo`:

```go
comment, _, err := c.gh.PullRequests.CreateComment(ctx, owner, repo, prNumber,
    &gh.PullRequestComment{
        Body:        gh.Ptr("Agreed, will fix."),
        InReplyTo:   gh.Ptr(int64(987654321)), // ID of root comment in thread
        CommitID:    gh.Ptr("abc123sha"),       // required even for replies
        Path:        gh.Ptr("internal/foo/bar.go"), // required even for replies
        SubjectType: gh.Ptr("line"),
    },
)
```

### 1.3 Top-Level PR Comment (Issues.CreateComment)

For a general comment (not attached to a diff line):

```go
comment, _, err := c.gh.Issues.CreateComment(ctx, owner, repo, prNumber,
    &gh.IssueComment{Body: gh.Ptr("Looks good to me!")},
)
```

### 1.4 Token Validation (Users.Get)

Use `GET /user` to validate a token — cheapest authenticated call:

```go
user, _, err := c.gh.Users.Get(ctx, "") // empty string = authenticated user
// err != nil (401) → invalid token
// err == nil → token valid, user.GetLogin() = username
```

This is the correct endpoint for the "validate immediately on save" requirement. A 401 response means the token is invalid; any 2xx means it works.

### 1.5 Draft Status Toggle: REST Does Not Exist

**Confidence: HIGH** — The GitHub REST API has no `ConvertPullRequestToDraft` or `MarkPullRequestReadyForReview` endpoints. Both mutations are GraphQL-only. The go-github library has no methods for these operations.

The GitHub GraphQL mutations are:
- `convertPullRequestToDraft(input: ConvertPullRequestToDraftInput!)` — requires the PR's **node ID** (not database integer ID)
- `markPullRequestReadyForReview(input: MarkPullRequestReadyForReviewInput!)` — same

Both take `{ pullRequestId: String! }` where the string is the GraphQL global node ID (e.g., `"PR_kwABC..."`). The REST API `GET /repos/{owner}/{repo}/pulls/{number}` returns `node_id`. The current `mapPullRequest` function does **not** capture `node_id` — this field needs to be added to the domain model and stored in the DB.

### 1.6 Fetching PR Node ID

```go
// go-github PullRequest struct has GetNodeID()
pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, number)
nodeID := pr.GetNodeID() // returns "PR_kwABCxyz..."
```

The `FetchPRDetail` method already calls `PullRequests.Get`. Adding `GetNodeID()` to what it returns is minimal work. Alternatively, `FetchPullRequests` (list) also returns node IDs and they could be stored at poll time.

---

## 2. SQLite Encryption (No CGO)

### 2.1 Ruling Out CGO-Based Options

SQLCipher requires CGO and is incompatible with `modernc.org/sqlite` (the project's pure-Go driver). Ruled out.

### 2.2 Recommended Approach: Application-Level AES-256-GCM

**Confidence: HIGH** — Standard Go stdlib approach, no new dependencies.

Encrypt the credential value in Go before writing to SQLite; decrypt after reading.

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "fmt"
    "io"
)

// Encrypt encrypts plaintext with AES-256-GCM using the provided 32-byte key.
// Returns a base64-encoded string of: nonce (12 bytes) || ciphertext.
func Encrypt(key []byte, plaintext string) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", fmt.Errorf("aes.NewCipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("cipher.NewGCM: %w", err)
    }
    nonce := make([]byte, gcm.NonceSize()) // 12 bytes for GCM
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("rand nonce: %w", err)
    }
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
func Decrypt(key []byte, encoded string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(encoded)
    if err != nil {
        return "", fmt.Errorf("base64 decode: %w", err)
    }
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", fmt.Errorf("aes.NewCipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("cipher.NewGCM: %w", err)
    }
    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", errors.New("ciphertext too short")
    }
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", fmt.Errorf("gcm.Open: %w", err)
    }
    return string(plaintext), nil
}
```

### 2.3 Key Management Strategy

- Store key in a new env var: `MYGITPANEL_SECRET_KEY` (64-char hex = 32 bytes)
- Fail fast at startup if key is absent or malformed (consistent with existing config philosophy)
- Do NOT derive key from machine ID or hostname — makes the app non-portable and complicates Docker deployment (Phase 6)
- If `MYGITPANEL_SECRET_KEY` is not set, credential storage is disabled; write operations that require credentials return an appropriate error but app starts

```go
// In config package:
keyHex := os.Getenv("MYGITPANEL_SECRET_KEY")
key, err := hex.DecodeString(keyHex)
if err != nil || len(key) != 32 {
    return nil, errors.New("MYGITPANEL_SECRET_KEY must be a 64-character hex string (32 bytes)")
}
```

### 2.4 Database Schema for Credentials

New migration — `credentials` table (key-value, extensible for Phase 9 Jira):

```sql
CREATE TABLE IF NOT EXISTS credentials (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    service    TEXT NOT NULL UNIQUE,   -- e.g., 'github_token', 'github_username', 'jira_url'
    value      TEXT NOT NULL DEFAULT '',  -- encrypted value (base64 AES-256-GCM)
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Use `service='github_token'` and `service='github_username'` for GitHub. Store Jira fields (`jira_url`, `jira_email`, `jira_token`) now for Phase 9 without additional migrations.

### 2.5 Hot-Swappable GitHub Client

The existing `NewClient(token, username)` creates a static client at startup from env vars. After Phase 8, the token may come from the database.

**Recommended approach (Option A):** Inject a `CredentialStore` port into `PollService` and `web.Handler`. Each poll cycle, fetch the current token from the store and use it. Cache the resolved client and invalidate on credential update.

This fits the hexagonal pattern — add a `CredentialStore` driven port, implement in the SQLite adapter.

**Startup behavior:** If no token in env var AND no token in DB, app starts but polling is disabled until credentials are set via the GUI. This removes the "required at startup" constraint on `MYGITPANEL_GITHUB_TOKEN` (or makes it optional with DB fallback).

---

## 3. Draft Status: GraphQL Decision

### 3.1 Options Compared

| Option | Pros | Cons |
|--------|------|------|
| Extend hand-rolled GraphQL client (`graphql.go`) | No new dep, same auth pattern already used, small mutations | More boilerplate structs for response parsing |
| Add `shurcooL/githubv4` | Type-safe GraphQL, less manual JSON | New dependency, different auth setup, overkill for 2 mutations |
| Subprocess `gh CLI` | Zero new code | Requires gh CLI installed; not suitable for a server |

**Recommendation: extend the hand-rolled GraphQL client.** The project already has a functioning raw GraphQL client in `internal/adapter/driven/github/graphql.go` with the same `bearer` token auth. Adding two mutations follows the exact same pattern.

### 3.2 The Two GraphQL Mutations

```graphql
# Convert ready PR to draft
mutation ConvertToDraft($prNodeId: ID!) {
    convertPullRequestToDraft(input: { pullRequestId: $prNodeId }) {
        pullRequest { isDraft }
    }
}

# Convert draft PR to ready for review
mutation MarkReadyForReview($prNodeId: ID!) {
    markPullRequestReadyForReview(input: { pullRequestId: $prNodeId }) {
        pullRequest { isDraft }
    }
}
```

**Note:** Verify the variable name (`pullRequestId` vs `id`) against GitHub's GraphQL Explorer before implementing — this is a LOW confidence detail.

### 3.3 Getting the PR Node ID

**Recommendation:** Fetch node ID on demand at toggle time via `PullRequests.Get` — one extra REST call is acceptable for a user-initiated action. No schema change needed for node_id storage.

Alternative: add `NodeID string` to `model.PullRequest` + new DB column, store at poll time. More work but avoids the extra API call.

### 3.4 Implementation Shape

```go
// In graphql.go — add alongside FetchThreadResolution

func (c *Client) ConvertPullRequestToDraft(ctx context.Context, nodeID string) error {
    return c.executeDraftMutation(ctx, convertToDraftMutation, nodeID)
}

func (c *Client) MarkPullRequestReadyForReview(ctx context.Context, nodeID string) error {
    return c.executeDraftMutation(ctx, markReadyMutation, nodeID)
}

func (c *Client) executeDraftMutation(ctx context.Context, mutation, nodeID string) error {
    reqBody := graphqlRequest{
        Query: mutation,
        Variables: map[string]any{"prNodeId": nodeID},
    }
    // ... same HTTP POST pattern as FetchThreadResolution
    return nil
}
```

### 3.5 New GitHubWriter Port (ISP)

Separate write port recommended to keep read/write concerns separate:

```go
// domain/port/driven/githubwriter.go
type GitHubWriter interface {
    SubmitReview(ctx context.Context, repoFullName string, prNumber int, review ReviewRequest) error
    CreateReplyComment(ctx context.Context, repoFullName string, prNumber int, inReplyTo int64, body, path, commitSHA string) error
    CreateIssueComment(ctx context.Context, repoFullName string, prNumber int, body string) error
    ConvertPullRequestToDraft(ctx context.Context, repoFullName string, prNumber int) error
    MarkPullRequestReadyForReview(ctx context.Context, repoFullName string, prNumber int) error
    ValidateToken(ctx context.Context, token string) (username string, err error)
}
```

The `Client` struct implements both `GitHubClient` (read) and `GitHubWriter` (write). Web handler only gets `GitHubWriter` injected.

---

## 4. HTMX/Alpine.js Patterns

### 4.1 Slide-In Settings Drawer

**Pattern:** Fixed-position overlay with Alpine.js `x-show` + CSS transitions. Drawer state in an Alpine store (same pattern as existing `theme` store).

```javascript
// stores.js — add drawer store
Alpine.store('drawer', {
    open: false,
    section: 'credentials',
    show(section) { this.section = section || 'credentials'; this.open = true; },
    hide() { this.open = false; }
});
```

```html
<!-- Backdrop overlay -->
<div
    x-show="$store.drawer.open"
    x-transition:enter="transition ease-out duration-200"
    x-transition:enter-start="opacity-0"
    x-transition:enter-end="opacity-100"
    x-transition:leave="transition ease-in duration-150"
    x-transition:leave-start="opacity-100"
    x-transition:leave-end="opacity-0"
    @click="$store.drawer.hide()"
    class="fixed inset-0 bg-black/40 z-40"
></div>

<!-- Drawer panel — slides from right -->
<div
    x-show="$store.drawer.open"
    x-transition:enter="transition ease-out duration-300"
    x-transition:enter-start="translate-x-full"
    x-transition:enter-end="translate-x-0"
    x-transition:leave="transition ease-in duration-200"
    x-transition:leave-start="translate-x-0"
    x-transition:leave-end="translate-x-full"
    class="fixed right-0 top-0 h-full w-96 bg-white dark:bg-gray-800 shadow-xl z-50 overflow-y-auto"
>
    <!-- Settings drawer content -->
</div>
```

**Important:** Render the drawer in the initial page HTML (layout.templ or dashboard.templ), NOT inside any HTMX swap target. This preserves Alpine state across partial refreshes.

**Trigger:** Settings gear icon button in the sidebar header → `@click="$store.drawer.show('credentials')"`.

### 4.2 Credential Form with Inline Validation

```html
<form
    hx-post="/app/settings/github"
    hx-target="#cred-status"
    hx-swap="innerHTML"
    hx-indicator="#cred-spinner"
>
    <input type="text" name="github_token" placeholder="ghp_..." />
    <button type="submit">Save</button>
    <span id="cred-spinner" class="htmx-indicator"><!-- spinner SVG --></span>
</form>
<div id="cred-status"></div>
```

Server responds with HTML fragment: success (`<span class="text-green-600">GitHub token: configured</span>`) or error (`<span class="text-red-600">Invalid token: 401 Unauthorized</span>`). Drawer stays open; status div updates in place.

### 4.3 hx-indicator for Loading States

```css
.htmx-indicator { display: none; }
.htmx-indicator.htmx-request { display: inline; }
```

**Draft toggle — disable during request:**

```html
<button
    hx-post="/app/prs/{owner}/{repo}/{number}/draft-toggle"
    hx-target="#pr-detail-header"
    hx-swap="morph"
    hx-indicator="this"
    x-data="{ loading: false }"
    @htmx:before-request="loading = true"
    @htmx:after-request="loading = false"
    :disabled="loading"
>
    <span x-show="!loading">Convert to Draft</span>
    <span x-show="loading">...</span>
</button>
```

### 4.4 Pending Comment Accumulation (Staged Review)

GitHub's review flow stages line comments before submit — this is local Alpine state only. Server never sees individual staged comments until submit.

```javascript
// x-data on the PR detail container
{
    pendingComments: [],
    addPendingComment(path, line, side, body) {
        this.pendingComments.push({ path, line, side, body });
    },
    removePendingComment(index) {
        this.pendingComments.splice(index, 1);
    },
    clearPending() { this.pendingComments = []; }
}
```

```html
<!-- Review submit form -->
<form
    hx-post="/app/prs/{owner}/{repo}/{number}/review"
    hx-target="#pr-reviews-section"
    hx-swap="morph"
    @submit="$el.querySelector('[name=comments]').value = JSON.stringify(pendingComments)"
    @htmx:after-request="clearPending()"
>
    <input type="hidden" name="comments" />
    <textarea name="body"></textarea>
    <select name="event">
        <option value="APPROVE">Approve</option>
        <option value="REQUEST_CHANGES">Request Changes</option>
        <option value="COMMENT">Comment</option>
    </select>
    <button type="submit">Submit Review</button>
</form>
```

The server handler parses the JSON `comments` field and constructs `[]*gh.DraftReviewComment` for `CreateReview`.

**Note:** Pending comments are lost on page reload — same behavior as GitHub's web UI when not explicitly saving a draft review. Acceptable.

### 4.5 OOB Swaps for Threshold Updates

After saving a threshold, update the PR card list sidebar with the same OOB pattern already used for repo mutations:

```go
// Server writes primary target (threshold save confirmation), then OOB:
// hx-swap-oob="morph:#pr-list" on the OOB element
```

### 4.6 Inline Reply Box Pattern

```html
<!-- Inside each thread card -->
<div x-data="{ replyOpen: false, replyBody: '' }">
    <button
        @click="replyOpen = !replyOpen"
        class="text-xs text-indigo-500 hover:underline px-4 py-2"
        x-text="replyOpen ? 'Cancel' : 'Reply'"
    ></button>
    <div x-show="replyOpen" x-transition>
        <form
            hx-post="/app/prs/{owner}/{repo}/{number}/comments/{rootCommentID}/reply"
            hx-target="#thread-{threadID}"
            hx-swap="morph"
            @htmx:after-request="replyOpen = false; replyBody = ''"
        >
            <textarea name="body" x-model="replyBody" rows="3"></textarea>
            <button type="submit">Reply</button>
        </form>
    </div>
</div>
```

---

## 5. Attention Signal Computation

### 5.1 Data Already Available in SQLite

All signal data is already persisted from prior phases:

| Signal | Data Source |
|--------|------------|
| Review count | `reviews` table: count rows where `state = 'approved'` per PR |
| Age-based urgency | `pull_requests.opened_at` — already stored |
| Stale review (my reviews) | `reviews.commit_id` vs `pull_requests.head_sha` — both stored (migrations 003, 007) |
| CI failure (my PRs) | `pull_requests.ci_status` + `pull_requests.author` — both stored (migration 008) |

**No new DB columns needed for signal computation.**

Stale review check: `reviews.commit_id != pull_requests.head_sha` for the authenticated user's reviews — fully computable from existing data.

### 5.2 Threshold Storage Schema

Two new tables:

```sql
-- Global defaults
CREATE TABLE IF NOT EXISTS global_settings (
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO global_settings (key, value) VALUES
    ('review_count_threshold', '1'),
    ('age_urgency_days', '7'),
    ('stale_review_enabled', '1'),
    ('ci_failure_enabled', '1');

-- Per-repo overrides (NULL = use global)
CREATE TABLE IF NOT EXISTS repo_thresholds (
    repo_full_name       TEXT NOT NULL PRIMARY KEY,
    review_count         INTEGER,  -- NULL = use global
    age_urgency_days     INTEGER,  -- NULL = use global
    stale_review_enabled INTEGER,  -- NULL = use global (0 or 1)
    ci_failure_enabled   INTEGER,  -- NULL = use global (0 or 1)
    FOREIGN KEY (repo_full_name) REFERENCES repositories(full_name) ON DELETE CASCADE
);
```

### 5.3 Attention Signal Computation: Where

**Recommendation: compute in the application layer (pure Go), not in SQL.**

- Signals require cross-table data (PRs + reviews + thresholds)
- Authenticated username is known at runtime (from config or credential store)
- Pure function = easily unit-tested

```go
// application layer

type AttentionSignals struct {
    NeedsMoreReviews bool // fewer than threshold approvals
    IsAgeUrgent      bool // open longer than threshold days
    HasStaleReview   bool // user's last review is on an outdated commit
    HasCIFailure     bool // own PR with failing CI
}

type EffectiveThresholds struct {
    ReviewCount         int
    AgeUrgencyDays      int
    StaleReviewEnabled  bool
    CIFailureEnabled    bool
}

func ComputeAttentionSignals(
    pr model.PullRequest,
    reviews []model.Review,
    thresholds EffectiveThresholds,
    authenticatedUser string,
) AttentionSignals
```

### 5.4 PR Ignore List Schema

```sql
CREATE TABLE IF NOT EXISTS ignored_prs (
    pr_id      INTEGER NOT NULL PRIMARY KEY,
    ignored_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE CASCADE
);
```

`PRStore.ListAll` and `PRStore.ListNeedingReview` should JOIN to exclude ignored PRs. Add `ListIgnored` method to PRStore for the ignore list view.

### 5.5 Ignore List UI (Claude's Discretion — Recommendation)

- "Ignore" button on PR card (small X icon or "..." menu)
- At the bottom of the sidebar PR list: collapsible "Show ignored (N)" section — same pattern as the existing collapsible repo manager
- Re-adding: single click from the ignore list
- Avoids new routes; keeps everything in the sidebar scroll

### 5.6 Attention Signal Display

| Signal | Icon | Color |
|--------|------|-------|
| Needs more reviews | Users/people icon | `text-orange-500` |
| Age urgency | Clock icon | `text-red-500` |
| Stale review | Outdated/refresh icon | `text-yellow-500` |
| CI failure | X-circle icon | `text-red-600` |

PR card: `border-l-4` with color reflecting highest-severity signal. Icons shown inline on the card.

---

## 6. Planning Recommendations

### 6.1 New Ports to Define

| Port | Purpose |
|------|---------|
| `driven.GitHubWriter` | Write: submit review, create comment/reply, draft toggle, validate token |
| `driven.CredentialStore` | CRUD for encrypted credentials (key-value by service name) |
| `driven.ThresholdStore` | Read/write global settings and per-repo threshold overrides |
| `driven.IgnoreStore` | Ignore/un-ignore PRs, list ignored PRs |

### 6.2 New Domain Models

| Model | Notes |
|-------|-------|
| `model.Credential` | service, encrypted value, updated_at |
| `model.RepoThreshold` | per-repo threshold overrides (NULLable fields) |
| `model.GlobalSettings` | key-value settings |
| `model.IgnoredPR` | pr_id, ignored_at |
| `model.AttentionSignals` | Transient — computed, not stored |
| `model.ReviewRequest` | Input to GitHubWriter.SubmitReview |

### 6.3 New Migrations (minimum 3)

| Migration | Content |
|-----------|---------|
| `000010_add_credentials.up.sql` | `credentials` table |
| `000011_add_repo_thresholds.up.sql` | `repo_thresholds` + `global_settings` |
| `000012_add_ignored_prs.up.sql` | `ignored_prs` table |

### 6.4 New Config

Add to `config/` package:
- `MYGITPANEL_SECRET_KEY` — 64-char hex (32 bytes). Optional for startup; required for credential read/write. App starts with warning if missing.
- `MYGITPANEL_GITHUB_TOKEN` — becomes optional if DB credential exists (DB takes precedence)

### 6.5 Hot-Swap Client Architecture

**Risk:** Existing architecture wires GitHub client at startup. After Phase 8, token may come from DB.

**Solution:** `PollService` re-reads from `CredentialStore` at start of each poll cycle (or on demand for write ops). The startup env var token is used as a fallback/seed only.

### 6.6 Planner Decision: Review Composer Phasing

Recommended task ordering for the review submission work (most to least complex):

1. Top-level issue comment (single `CreateIssueComment` call) — simplest
2. Reply to thread (single `CreateComment` with `InReplyTo`) — straightforward
3. Full staged review with line comments and submit — requires diff context + pending state + `CreateReview`

This delivers value incrementally and reduces risk.

### 6.7 Key Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Draft toggle fails for PRs user doesn't own | HIGH | Button visibility gated on `pr.Author == authenticatedUser` in viewmodel |
| Pending comments lost on HTMX swap | MEDIUM | Keep PR detail container outside HTMX swap targets; only swap inner sections |
| Encrypted token unreadable if `MYGITPANEL_SECRET_KEY` changes | HIGH | Document clearly; surface specific error on decrypt failure; prompt to re-enter credentials |
| `head_sha` mismatch on review submit (force-push race) | MEDIUM | Re-fetch head SHA before submit; surface 422 as "PR was updated, refresh and retry" |
| SQLite writer contention (poll + review submit simultaneous) | LOW | Already handled by single-connection writer with busy_timeout(5000) |

### 6.8 Stale Review Signal Correctness

`reviews.commit_id != pull_requests.head_sha` is the correct stale review check. `commit_id` is the SHA when the user submitted their review; `head_sha` is the current PR head. If they differ, the review is on an outdated commit — exactly GitHub's "Outdated" badge behavior. Both fields are already in the DB. No new data needed.

---

## Sources

**HIGH confidence (read from codebase):**
- `internal/adapter/driven/github/client.go` — confirmed go-github v82 usage patterns
- `internal/adapter/driven/github/graphql.go` — confirmed existing raw GraphQL client shape
- `internal/adapter/driven/sqlite/migrations/` — confirmed exact DB schema
- `internal/adapter/driving/web/` — confirmed HTMX OOB swap pattern, Alpine.js patterns in use
- `go.mod` — confirmed dependency versions

**MEDIUM confidence (training knowledge consistent with codebase):**
- go-github v82 API shapes for CreateReview, CreateComment, Issues.CreateComment, Users.Get
- GitHub GraphQL schema for draft mutations
- HTMX 2.x hx-indicator, OOB swap attributes
- Alpine.js 3.x store pattern

**LOW confidence — verify before implementing:**
- GitHub GraphQL `convertPullRequestToDraft` variable name (`pullRequestId` vs `id`)
- Whether `PullRequest.NodeID` is accessible via go-github v82 `GetNodeID()` — check source

---

*Research completed: 2026-02-19*
