# Phase 9: Jira Integration - Context

**Gathered:** 2026-02-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Jira integration surfaces linked issue context alongside PRs in the dashboard and enables posting comments to Jira from the PR detail view. This includes: credential management for multiple Jira connections, branch/title-based Jira key detection with DB caching, auto-fetching linked issue details on PR load, and a comment input inside the Jira card. Creating Jira issues, managing Jira workflows, and Jira search are out of scope.

</domain>

<decisions>
## Implementation Decisions

### Jira issue display
- Collapsible card at the **top** of the PR detail panel (above description/reviews)
- Card auto-fetches Jira issue data on PR detail load (not deferred to expand)
- Collapsed by default; shows issue key + summary in the header
- Expanded view shows: summary, description, status, priority, assignee, and existing comments
- When no Jira key is detected: show a subtle "No linked issue" placeholder (muted, non-intrusive) — do NOT hide the card entirely
- When Jira credentials are not configured: card shows collapsed with "Configure Jira in Settings" and a link to the Settings drawer

### Issue key detection
- Scan branch name first, then PR title (branch takes priority)
- Auto-detect any `[A-Z]{2,}-\d+` pattern — no prefix configuration required
- When multiple keys are found, first match wins (branch > title, left-to-right within each)
- Detected key is **cached in the database** alongside the PR record; re-extracted when the PR is polled and updated (not re-parsed on every detail render)

### Comment posting UX
- Comment input lives **inside the expanded Jira card**, below the existing comments
- After successful post: refresh the Jira card inline via HTMX (new comment appears immediately)
- Comment input is **hidden completely** when no Jira credentials are configured (not disabled with a prompt)
- Auth-gate per request via `credStore.Get` (same pattern as GitHub write handlers); additionally validate that the Jira base URL is reachable before attempting the post — return a 422 HTML fragment on connectivity failure

### Multi-Jira connections and per-repo mapping
- Multiple Jira connections supported, each stored as a named entry (e.g., "Work Jira", "Client Jira")
- Each connection stores: display name, base URL, email, API token
- Per-repo assignment configured in the Settings drawer, in the same repo-level settings section as thresholds (consistent UX with repo thresholds UI)
- A repo can be assigned one Jira connection or "none"; unassigned repos show "No linked issue"
- A "default" Jira connection can be designated as the fallback for repos with no explicit assignment

### Error states
- Jira API failures (unreachable, 401, 404): show inline error inside the card with a retry button — "Could not load Jira issue: [reason]"
- Credential validation: validate on save (same pattern as GitHub token) — POST triggers a live Jira API call to verify credentials, returns success/error HTML fragment

### Claude's Discretion
- Exact Jira card visual design (colors, icons, spacing) — consistent with existing dashboard aesthetics
- Loading skeleton while Jira data is fetching
- Exact retry mechanism implementation
- DB schema for multi-connection storage
- How Jira connections are listed/managed in the Settings drawer (add/remove/edit flow)

</decisions>

<specifics>
## Specific Ideas

- Jira credential management should feel like the repo-threshold UI in Phase 8: consistent placement in the Settings drawer, similar patterns for per-repo assignment
- The Jira card should be clearly distinguishable from GitHub review content — Jira's visual identity (colors) could be referenced subtly

</specifics>

<deferred>
## Deferred Ideas

- Creating Jira issues from the dashboard — future phase
- Jira issue search/browse — future phase
- Transitioning Jira issue status from the dashboard — future phase
- Jira project prefix filtering — auto-detection without config is sufficient for now

</deferred>

---

*Phase: 09-jira-integration*
*Context gathered: 2026-02-24*
