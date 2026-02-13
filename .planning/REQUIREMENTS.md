# Requirements: ReviewHub

**Defined:** 2026-02-10
**Core Value:** Review comments formatted with enough code context that an AI agent can understand and fix the code

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### PR Discovery

- [ ] **DISC-01**: System polls GitHub for all open PRs authored by the configured user
- [ ] **DISC-02**: System polls configured repositories for PRs where the user's review is requested
- [ ] **DISC-03**: System detects and flags draft PRs separately from ready PRs
- [ ] **DISC-04**: System detects PRs where the user's team is requested as reviewer (not just individual)
- [ ] **DISC-05**: System deduplicates PRs that appear in both authored and review-requested queries

### PR Status & Metadata

- [ ] **STAT-01**: Each PR shows current status: open, merged, closed
- [ ] **STAT-02**: Each PR shows review-derived status: changes requested, approved, ready to merge
- [ ] **STAT-03**: Each PR shows staleness: days since opened, days since last activity
- [ ] **STAT-04**: Each PR shows diff stats: files changed, lines added, lines removed
- [ ] **STAT-05**: Each PR shows merge conflict status (mergeable/conflicted/unknown)
- [ ] **STAT-06**: Each PR shows boolean flags: needs review, reviewed, Coderabbit reviewed, awaiting Coderabbit
- [ ] **STAT-07**: Each PR includes title, author, branch, base branch, URL, labels

### Review Intelligence

- [ ] **REVW-01**: System tracks review state per reviewer: approved, changes requested, commented
- [ ] **REVW-02**: System detects Coderabbit reviews by checking for @coderabbitai author
- [ ] **REVW-03**: System detects Coderabbit nitpick comments by parsing for the nitpick marker in comment body
- [ ] **REVW-04**: System tracks resolved vs open comment threads per PR
- [ ] **REVW-05**: System detects outdated reviews (review posted on a commit that is no longer the PR head)

### Comment Formatting (AI-Ready)

- [ ] **CFMT-01**: Each review comment includes the targeted diff hunk and surrounding code lines
- [ ] **CFMT-02**: Each review comment includes file path and line number(s)
- [ ] **CFMT-03**: Comments are grouped into conversation threads (original + all replies)
- [ ] **CFMT-04**: GitHub suggestion blocks are extracted and presented as structured proposed changes
- [ ] **CFMT-05**: Inline (line-specific) comments are distinguished from general PR-level comments
- [ ] **CFMT-06**: Coderabbit nitpick comments are flagged separately from regular review comments
- [ ] **CFMT-07**: Each comment includes reviewer name, timestamp, and review action (approve/request changes/comment)

### CI/CD Status

- [ ] **CICD-01**: Each PR shows combined check status: passing, failing, pending
- [ ] **CICD-02**: Each PR lists individual check runs with name, status, and conclusion
- [ ] **CICD-03**: System identifies required checks vs optional checks (when token permissions allow)

### Repository Configuration

- [ ] **REPO-01**: API endpoint to add a watched repository
- [ ] **REPO-02**: API endpoint to remove a watched repository
- [ ] **REPO-03**: API endpoint to list all watched repositories
- [ ] **REPO-04**: API endpoint to configure bot usernames to detect (Coderabbit, Copilot, custom)

### Polling & Data Management

- [ ] **POLL-01**: System polls GitHub at configurable intervals (default 5 minutes)
- [ ] **POLL-02**: System respects GitHub API rate limits (track remaining budget, back off when low)
- [ ] **POLL-03**: System uses adaptive polling: active PRs polled more frequently, stale ones less
- [ ] **POLL-04**: API endpoint to trigger manual refresh for a specific repo or PR
- [ ] **POLL-05**: System handles GitHub API pagination correctly (all list endpoints)
- [ ] **POLL-06**: System uses conditional requests (ETags) to minimize rate limit consumption
- [ ] **POLL-07**: System tracks updated_at timestamps to skip re-processing unchanged PRs

### Configuration & Infrastructure

- [ ] **INFR-01**: GitHub token configurable via environment variable
- [ ] **INFR-02**: GitHub username configurable via environment variable
- [ ] **INFR-03**: Application runs in a Docker container
- [ ] **INFR-04**: SQLite database persisted via Docker volume
- [ ] **INFR-05**: Graceful shutdown on SIGTERM/SIGINT (drain requests, stop polling, close DB)
- [ ] **INFR-06**: Database migrations run automatically on startup
- [ ] **INFR-07**: API accessible on localhost only

### API Endpoints

- [ ] **API-01**: GET endpoint returning all tracked PRs with status flags (the "git-status" endpoint)
- [ ] **API-02**: GET endpoint returning a single PR with full review comments and code context
- [ ] **API-03**: GET endpoint returning only PRs needing attention (changes requested, needs review)
- [ ] **API-04**: Health check endpoint

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Webhooks

- **HOOK-01**: GitHub webhook receiver for real-time PR updates
- **HOOK-02**: Webhook signature verification

### Analytics

- **ANLT-01**: PR turnaround time tracking
- **ANLT-02**: Review response time metrics

### Notifications

- **NOTF-01**: Configurable notification when PR status changes
- **NOTF-02**: Summary digest of pending reviews

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Web dashboard / frontend UI | API-only, consumed by Claude Code CLI |
| Multi-user / multi-tenant support | Single user, single token design |
| OAuth / SSO authentication | Localhost-only, token via env var |
| PR creation or modification | Read-only tracking tool |
| Code analysis or linting | Dedicated tools exist; report CI status instead |
| Merge automation | Mergify/GitHub auto-merge handles this |
| Review assignment / routing | CODEOWNERS and PullApprove handle this |
| AI review summarization | Downstream AI agent (Claude Code) handles this |
| Comment reply / interaction | Read-only; never post to GitHub |
| Historical analytics / trends | Metrics dashboards own this space |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DISC-01 | Phase 2 | Complete |
| DISC-02 | Phase 2 | Complete |
| DISC-03 | Phase 2 | Complete |
| DISC-04 | Phase 2 | Complete |
| DISC-05 | Phase 2 | Complete |
| STAT-01 | Phase 3 | Complete |
| STAT-02 | Phase 4 | Pending |
| STAT-03 | Phase 5 | Pending |
| STAT-04 | Phase 5 | Pending |
| STAT-05 | Phase 5 | Pending |
| STAT-06 | Phase 4 | Pending |
| STAT-07 | Phase 3 | Complete |
| REVW-01 | Phase 4 | Pending |
| REVW-02 | Phase 4 | Pending |
| REVW-03 | Phase 4 | Pending |
| REVW-04 | Phase 4 | Pending |
| REVW-05 | Phase 4 | Pending |
| CFMT-01 | Phase 4 | Pending |
| CFMT-02 | Phase 4 | Pending |
| CFMT-03 | Phase 4 | Pending |
| CFMT-04 | Phase 4 | Pending |
| CFMT-05 | Phase 4 | Pending |
| CFMT-06 | Phase 4 | Pending |
| CFMT-07 | Phase 4 | Pending |
| CICD-01 | Phase 5 | Pending |
| CICD-02 | Phase 5 | Pending |
| CICD-03 | Phase 5 | Pending |
| REPO-01 | Phase 3 | Complete |
| REPO-02 | Phase 3 | Complete |
| REPO-03 | Phase 3 | Complete |
| REPO-04 | Phase 4 | Pending |
| POLL-01 | Phase 2 | Complete |
| POLL-02 | Phase 2 | Complete |
| POLL-03 | Phase 6 | Pending |
| POLL-04 | Phase 2 | Complete |
| POLL-05 | Phase 2 | Complete |
| POLL-06 | Phase 2 | Complete |
| POLL-07 | Phase 2 | Complete |
| INFR-01 | Phase 1 | Complete |
| INFR-02 | Phase 1 | Complete |
| INFR-03 | Phase 6 | Pending |
| INFR-04 | Phase 6 | Pending |
| INFR-05 | Phase 1 | Complete |
| INFR-06 | Phase 1 | Complete |
| INFR-07 | Phase 1 | Complete |
| API-01 | Phase 3 | Complete |
| API-02 | Phase 3 | Complete |
| API-03 | Phase 3 | Complete |
| API-04 | Phase 3 | Complete |

**Coverage:**
- v1 requirements: 49 total
- Mapped to phases: 49
- Unmapped: 0

---
*Requirements defined: 2026-02-10*
*Last updated: 2026-02-10 after roadmap creation*
