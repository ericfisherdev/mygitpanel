# Feature Landscape

**Domain:** Web GUI for PR Review Dashboard with Jira Integration
**Researched:** 2026-02-14
**Confidence:** HIGH (verified against GitHub REST API docs, Jira REST API v3 docs, Graphite/ReviewStack competitive analysis, and existing v1.0 codebase)

---

## Table Stakes

Features users expect from any PR review dashboard with a web GUI. Missing these means the product feels broken.

### Unified PR Feed

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| PR list across all watched repos | Core purpose of a dashboard; Graphite, GitHub all provide this | Low | Existing `GET /api/v1/prs` endpoint |
| Filter by repo, status, author, draft | Every PR dashboard has filters; GitHub's native filters set the bar | Medium | Existing PR data in SQLite |
| Search by PR title/branch name | Text search is basic expectation for any list view | Low | SQLite LIKE query on existing columns |
| Sort by updated date, age, review status | Users need to triage; Graphite sorts by "needs attention" | Low | Existing fields (`updated_at`, `days_since_opened`) |
| Visual status indicators (CI, merge, review) | GitHub shows green check/red X; users expect at-a-glance status | Low | Existing `ci_status`, `mergeable_status`, `review_status` fields |
| PR count badges per repo | Orientation signal; how many PRs need attention per repo | Low | Aggregate query on existing data |
| Responsive layout (desktop primary, tablet acceptable) | Single-user tool likely used on dev workstation | Medium | Tailwind CSS responsive utilities |

### PR Detail View

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| Full PR metadata display (title, author, branch, labels, diff stats) | Every PR tool shows this context | Low | Existing `GET /api/v1/repos/{owner}/{repo}/prs/{number}` |
| Review thread display with code context | The existing API already formats this beautifully | Medium | Existing `threads`, `suggestions`, `reviews` response fields |
| CI/CD check status with individual run details | Users need to see which check failed, not just pass/fail | Low | Existing `check_runs` response field |
| Link to GitHub PR for full diff view | Users will want to jump to GitHub for the full diff | Low | Existing `url` field |
| Reviewer status list (who approved, who requested changes) | Core review workflow signal | Low | Existing `reviews` data |

### Repo Management

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| Add/remove watched repos from UI | Currently API-only; GUI must surface this | Low | Existing `POST /api/v1/repos`, `DELETE /api/v1/repos/{owner}/{repo}` |
| Bot configuration from UI | Currently API-only | Low | Existing `POST /api/v1/bots`, `DELETE /api/v1/bots/{username}` |

### Theme and Appearance

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| Dark mode (default) | Developers overwhelmingly prefer dark mode; GitHub, VS Code default dark | Low | Tailwind CSS dark mode classes |
| Light mode toggle | Accessibility and preference; some users work in bright environments | Low | Alpine.js state + localStorage persistence |

---

## Differentiators

Features that set MyGitPanel apart. Not universally expected, but create real value.

### Review Workflow Actions (Core Differentiator)

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Approve PR from dashboard | Eliminates context switch to GitHub; one-click workflow | Medium | New: `POST /repos/{owner}/{repo}/pulls/{number}/reviews` with `event: "APPROVE"` via GitHub API |
| Request changes with comment body | Submit structured feedback without leaving the dashboard | Medium | New: same endpoint, `event: "REQUEST_CHANGES"`, body required |
| Comment-only review submission | Lightweight feedback path | Medium | New: same endpoint, `event: "COMMENT"`, body required |
| Reply to review comments | Continue conversation threads inline | Medium | New: `POST /repos/{owner}/{repo}/pulls/{number}/comments/{comment_id}/replies` -- note: replies to replies not supported by GitHub API |
| Post general PR comment | Issue-level discussion without line-specific context | Low | New: `POST /repos/{owner}/{repo}/issues/{number}/comments` |

**API constraint:** GitHub requires `repo` scope (classic PAT) or `Pull requests: Write` (fine-grained) for review submission. The existing read-only token will need upgrading. This is a breaking configuration change for existing users.

### Draft Toggle

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Convert PR to draft | Signal "not ready for review" without leaving dashboard | Low | `PATCH /repos/{owner}/{repo}/pulls/{number}` with `{"draft": true}` -- works via REST API |
| Mark PR as ready for review | Resume review workflow | Medium | **GraphQL-only**: `markPullRequestReadyForReview` mutation. REST API does NOT support this. Requires adding `shurcooL/githubv4` or shelling out to `gh pr ready` |

**Critical finding:** Draft-to-ready is GraphQL-only. This is asymmetric -- converting TO draft works via REST, but converting FROM draft requires GraphQL. The existing codebase uses only REST (`go-github`). This needs a new adapter or CLI wrapper.

### PR Ignore List

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Ignore/hide specific PRs from feed | Reduce noise from PRs you do not care about (e.g., dependabot, long-lived feature branches) | Low | New: `ignored_prs` SQLite table with `(repo_full_name, number)` composite key |
| Re-add ignored PRs (undo) | Users make mistakes; need recovery path | Low | Delete from `ignored_prs` table |
| View ignored PRs list | Audit what is being hidden | Low | Filter query on `ignored_prs` table |
| Bulk ignore by label or author | Power user feature for ignoring all dependabot PRs at once | Medium | Filter + batch insert |

### Configurable Urgency Thresholds

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Per-repo review count threshold (how many approvals needed) | Different repos have different policies (1 approval vs 2 vs 3) | Low | New: column on `repositories` table, default 1 |
| Configurable age-based urgency levels | Graphite and DevDynamics use tiered thresholds: <1d, <3d, <7d, <14d, >30d | Medium | New: `urgency_config` table or JSON column with per-repo overrides |
| Visual urgency indicators (color coding by age/status) | At-a-glance triage; red/yellow/green is universal dashboard pattern | Low | Computed in templ templates from existing `days_since_opened` |
| Attention score (composite) | Single number combining: needs review + age + unresolved threads + CI failure | Medium | Computed from existing fields; new scoring algorithm |

### Jira Integration

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Configure Jira connection (URL, email, API token) | Foundation for all Jira features; stored encrypted in SQLite | Medium | New: `jira_config` table, Jira REST API v3 basic auth (email + API token) |
| View linked Jira issue details from PR | PRs typically reference Jira keys in title/branch (e.g., `PROJ-123`); auto-extract and show issue status, assignee, priority | High | New: regex extraction of Jira keys from PR title/branch, `GET /rest/api/3/issue/{issueKey}` |
| Post comment on linked Jira issue | Update Jira from PR dashboard ("PR approved, ready to merge") | Medium | New: `POST /rest/api/3/issue/{issueKey}/comment` with ADF (Atlassian Document Format) body |
| Jira issue status badge in PR list | At-a-glance project management context alongside PR status | Medium | Cached Jira issue data; poll or fetch-on-view |

**Jira API notes:**
- Authentication: Basic auth with email + API token (not password). Header: `Authorization: Basic base64(email:token)`
- Cloud URL format: `https://{your-domain}.atlassian.net`
- JQL search: `GET /rest/api/3/search/jql?jql=key IN (PROJ-123, PROJ-456)` for batch issue lookup
- Comment body uses ADF (Atlassian Document Format), not plain markdown. Must construct JSON document nodes.
- Rate limits: Jira Cloud has undocumented rate limits; implement exponential backoff

### GitHub Credential Management

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Store GitHub PAT in SQLite (encrypted) | Currently requires env var; GUI needs persistent config | Medium | New: `credentials` table with AES-256-GCM encryption; derive key from machine-specific secret |
| Token validation on save | Verify token works before persisting; test with `GET /user` | Low | GitHub API call on save |
| Token scope display | Show what the token can do; warn if missing write scopes for review features | Low | Parse `X-OAuth-Scopes` response header |

### GSAP Animations

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Page transition animations | Polish; smooth route changes feel professional | Low | GSAP `gsap.from()`/`gsap.to()` on HTMX `htmx:afterSwap` events |
| PR card entrance animations (stagger) | Visual delight on feed load; Graphite does smooth list renders | Low | GSAP `stagger` on PR card elements |
| Status change animations | Draw attention to CI pass/fail, review state changes | Low | GSAP color/scale tweens on status badges |
| Reduced motion support | Accessibility requirement; respect `prefers-reduced-motion` | Low | `window.matchMedia('(prefers-reduced-motion: reduce)')` check |

---

## Anti-Features

Features to explicitly NOT build. Common in PR dashboards but wrong for MyGitPanel's scope.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Full diff viewer in-app | Massive complexity (syntax highlighting, side-by-side, virtual scroll for large files); GitHub does this perfectly | Show diff stats and code snippets from review comments; link to GitHub for full diff |
| PR creation | Out of scope; users create PRs from git CLI or IDE | Track existing PRs only |
| Merge automation / merge button | Dangerous from a dashboard; GitHub's merge button has safety rails (branch protection, required checks) that are hard to replicate | Show merge readiness; link to GitHub for actual merge |
| Review assignment / routing | PullApprove, CODEOWNERS handle this; duplicating it adds complexity with zero differentiation | Show who is assigned; do not manage assignments |
| Multi-user / multi-tenant | Single-user tool; adding auth, user isolation, and role management is massive scope creep | Single user, single token, local access only |
| Notification system (email, Slack, push) | Dashboard is the notification; polling + visual indicators suffice for single user | Urgency indicators and attention scores in the feed |
| AI review summarization | The downstream AI agent (Claude Code) handles this; MyGitPanel provides data, not intelligence | Format review data cleanly for both human and AI consumption |
| Jira issue creation | Write operations beyond comments add Jira project config complexity (issue types, custom fields, workflows) | View Jira issues and post comments only |
| Webhook receiver (GitHub or Jira) | Adds deployment complexity (public endpoint, TLS, secret validation); polling works fine for single user | Poll GitHub on existing adaptive schedule; fetch Jira on-demand |
| Historical analytics / trends | LinearB, Sleuth, Graphite Insights own this space; storage and charting complexity is high | Show current state only |
| OAuth flow for GitHub/Jira | Localhost single-user tool; OAuth adds redirect URI handling, token refresh, PKCE for no audience benefit | Accept tokens directly via settings UI |
| Real-time WebSocket updates | Polling-based backend; adding WebSocket layer adds bidirectional complexity for marginal latency improvement | HTMX polling (`hx-trigger="every 30s"`) for near-real-time feel |

---

## Feature Dependencies

```
GitHub Credential Management (foundation for write operations)
  |
  +---> Review Workflow Actions (approve, request changes, comment)
  |       |
  |       +---> Reply to Review Comments (extends review workflow)
  |       +---> Post General PR Comment (extends review workflow)
  |
  +---> Draft Toggle (requires write-scoped token)
          |
          +---> Mark Ready for Review (requires GraphQL adapter -- HIGH complexity spike)

Jira Credential Management
  |
  +---> Jira Key Extraction (regex from PR title/branch)
  |       |
  |       +---> Jira Issue Detail Fetch (depends on valid keys)
  |       |       |
  |       |       +---> Jira Status Badge in PR List (cached issue data)
  |       |
  |       +---> Post Jira Comment (requires issue key + Jira connection)

Existing v1.0 API (all read data is already available)
  |
  +---> Unified PR Feed (renders existing /api/v1/prs data)
  |       |
  |       +---> Search/Filter (client-side or server-side on existing data)
  |       +---> Sort (on existing fields)
  |       +---> Urgency Indicators (computed from existing days_since_*, ci_status, review_status)
  |
  +---> PR Detail View (renders existing /api/v1/repos/{o}/{r}/prs/{n} data)
  |       |
  |       +---> Review Thread Display (existing threads response)
  |       +---> CI Check Display (existing check_runs response)
  |
  +---> PR Ignore List (new table, filters existing PR list)
  |
  +---> Configurable Urgency Thresholds (new config, applied to existing age/status data)

Theme (independent, no API dependency)
  |
  +---> Dark/Light Toggle (Alpine.js + localStorage + Tailwind)
  +---> GSAP Animations (progressive enhancement, no data dependency)
```

### Critical Path

Shortest path to a usable dashboard:

1. **Theme + Layout Shell** -- Tailwind dark/light, nav, empty states
2. **Unified PR Feed** -- Renders existing API data; immediate value
3. **PR Detail View** -- Shows reviews, threads, CI; leverages existing rich data
4. **Search/Filter/Sort** -- Triage capability
5. **Repo Management UI** -- Currently API-only; GUI must surface this

Everything else layers on after this critical path delivers a functional read-only dashboard.

---

## MVP Recommendation

### Phase 1: Read-Only Dashboard (builds on entire v1.0 API)

Prioritize:
1. **Unified PR feed with search/filter/sort** -- immediate daily-driver value
2. **PR detail view with review threads and CI status** -- leverages existing rich API
3. **Repo and bot management UI** -- currently CLI/API only
4. **Dark/light theme** -- developer expectation
5. **Visual urgency indicators** -- color-coded age/status badges using existing data
6. **GSAP entrance animations** -- polish, low effort

### Phase 2: Review Workflows (requires token scope upgrade)

Prioritize:
1. **GitHub credential management** (store PAT with write scope in SQLite)
2. **Approve/Request Changes/Comment** submission
3. **Reply to review comments** inline
4. **Draft toggle** (convert to draft via REST; ready-for-review via GraphQL)
5. **PR ignore list** with undo

### Phase 3: Jira + Configuration

Prioritize:
1. **Jira credential management** (URL, email, API token)
2. **Jira key extraction** from PR title/branch
3. **Jira issue detail display** (status, assignee, priority)
4. **Configurable urgency thresholds** per repo
5. **Configurable review count requirements** per repo
6. **Post Jira comments** from PR detail view

### Defer Beyond MVP

- **Attention score (composite)**: Useful but needs UX iteration to get the formula right; ship simple urgency colors first, evolve based on usage
- **Bulk ignore by label/author**: Power user feature; ship single-PR ignore first
- **Jira issue status badge in PR list**: Requires caching strategy; ship detail-view integration first

---

## Competitive Landscape Context

### Graphite

Graphite's PR inbox is the closest competitor pattern. Key features to learn from:
- **Five pre-made sections**: Needs your review, Approved, Changes requested, Your PRs, Watched. MyGitPanel should have similar categorized views, not just a flat list.
- **Custom sections with filters**: Author, date, status filters saved as named views. Worth considering post-MVP.
- **Keyboard shortcuts**: Power user feature that Graphite emphasizes. Worth noting for later.
- **Live check status with re-run**: Re-running checks from dashboard is out of scope (anti-feature), but live status display is table stakes.

### GitHub Native

GitHub's PR list page has a known UX problem: draft PR status is a tiny text label that blends with metadata (confirmed in GitHub community discussions). MyGitPanel should make draft status highly visible with distinct visual treatment.

### GitClear

GitClear focuses on PR review analytics (time-to-review, review depth). MyGitPanel is not an analytics tool -- it is an operational dashboard. Do not chase metrics features.

---

## Sources

- [GitHub REST API - Pull Request Reviews](https://docs.github.com/en/rest/pulls/reviews) -- verified endpoints for `POST /repos/{owner}/{repo}/pulls/{number}/reviews` (HIGH confidence)
- [GitHub REST API - Review Comments](https://docs.github.com/en/rest/pulls/comments) -- verified reply endpoint (HIGH confidence)
- [GitHub Community - Convert PR to Draft](https://github.com/orgs/community/discussions/45174) -- confirmed REST PATCH works for draft=true (HIGH confidence)
- [GitHub Community - Ready for Review](https://github.com/orgs/community/discussions/70061) -- confirmed GraphQL-only for markReadyForReview (HIGH confidence)
- [Jira REST API v3 - Issue Search](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/) -- JQL search endpoint (MEDIUM confidence, did not verify exact response schema)
- [Jira REST API - Add Comment](https://developer.atlassian.com/server/jira/platform/jira-rest-api-example-add-comment-8946422/) -- comment endpoint structure (MEDIUM confidence)
- [Jira Basic Auth](https://developer.atlassian.com/cloud/jira/software/basic-auth-for-rest-apis/) -- email + API token authentication (HIGH confidence)
- [Graphite Features](https://graphite.dev/features) -- inbox sections and dashboard patterns (MEDIUM confidence)
- [GitHub Community - Draft PR Visibility](https://github.com/orgs/community/discussions/165497) -- draft label UX problem (MEDIUM confidence)
- [DevDynamics - Open PR Age](https://docs.devdynamics.ai/features/metrics/git-dashboard/open-pr-age) -- age threshold tiers: <1d, <3d, <7d, <14d, <1mo, >1mo (MEDIUM confidence)
- [HTMX + Alpine.js + Go patterns](https://ntorga.com/full-stack-go-app-with-htmx-and-alpinejs/) -- integration patterns for the chosen stack (MEDIUM confidence)
- Existing MyGitPanel v1.0 codebase: `internal/adapter/driving/http/handler.go`, `response.go`, domain models (HIGH confidence, direct inspection)
