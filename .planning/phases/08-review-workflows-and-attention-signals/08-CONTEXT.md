# Phase 8: Review Workflows and Attention Signals - Context

**Gathered:** 2026-02-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Write operations, credential management, configurable attention thresholds, and a PR ignore list — all accessible from the dashboard. Users can enter a GitHub token, submit PR reviews (approve/request changes/comment) with line-level diff comments, reply to review threads, toggle PRs between draft and ready-for-review, configure per-repo attention thresholds, and ignore PRs from the feed. Reading PR data is Phase 7; Jira-specific features are Phase 9.

</domain>

<decisions>
## Implementation Decisions

### Credential storage UX
- Settings live in a slide-in drawer/panel accessible from the main dashboard (no navigation away)
- The settings drawer is a **general settings hub** — covers credentials, thresholds, and other config
- Token entry field; on save, validate immediately by making a test GitHub API call and show success/error inline before closing
- After saving, display only a status indicator ("GitHub token: configured") — no masked token value shown
- Credentials stored in SQLite (same DB as PR data), encrypted at rest
- Jira credentials stored the same way: URL, email, and token fields in the same settings drawer
- If a write operation is attempted without a valid token, open the settings drawer with an inline error message explaining the issue

### Review submission flow
- Review flow should mirror GitHub's UX as closely as possible
- Full GitHub parity: users can add line-level comments on the diff, accumulate pending comments, then submit them all together as a review (approve / request changes / comment)
- Inline reply boxes below each existing comment thread (collapsed, expands on click) — same as GitHub
- After submitting a review or reply: HTMX partial refresh — only the comments/review section updates, no full page reload

### Draft/ready toggle
- Prominent button near the PR title/status in the PR detail view
- Only visible on PRs authored by the authenticated user (hidden entirely on others' PRs)
- Single click — no confirmation dialog
- Wait for API response before updating the badge (loading state shown during the call)

### Attention threshold config
- Global defaults in the settings drawer; per-repo overrides accessible inline (icon/popover next to each repo in the feed)
- Four configurable threshold types:
  1. **Review count** — flag PRs with fewer than N approvals (all PRs)
  2. **Age-based urgency** — flag PRs open longer than X days (all PRs)
  3. **Stale review** — flag when a PR has been updated since the authenticated user's last review (only the authenticated user's reviews)
  4. **CI failure** — flag PRs with failing CI checks (only PRs authored by the authenticated user)
- Flagged PRs shown with **both** a colored border/accent (communicates urgency level) and specific icons per threshold type (identifies which threshold was crossed)

### PR ignore list
- Claude's discretion: standard pattern — ignore hides PR from feed, view ignored list accessible somewhere in the UI, restore from the list. Claude decides the exact placement and interaction.

### Claude's Discretion
- Exact encryption mechanism for SQLite credential storage
- Loading/spinner treatment during API calls (review submit, draft toggle, token validation)
- Exact color scheme for urgency levels (orange vs red gradations)
- Icon choices for each threshold type
- Exact ignore list placement and interaction pattern

</decisions>

<specifics>
## Specific Ideas

- "Should function/flow the same as GitHub" — review composer and thread interactions should feel immediately familiar to GitHub users
- CI failure threshold applies only to the authenticated user's own PRs (not others' failing PRs)
- Stale review signal tracks only the authenticated user's own reviews being outdated (not others')

</specifics>

<deferred>
## Deferred Ideas

- Jira credential fields (URL, email, token) are captured in the settings drawer here, but Jira-specific features (issue display, commenting from PR view, auto-linking) belong to Phase 9

</deferred>

---

*Phase: 08-review-workflows-and-attention-signals*
*Context gathered: 2026-02-19*
