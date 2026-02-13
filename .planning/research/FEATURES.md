# Feature Landscape

**Domain:** GitHub PR Tracking and Review Management API (machine-consumable, AI-agent-oriented)
**Researched:** 2026-02-10
**Confidence:** MEDIUM (based on training data knowledge of GitHub API, Graphite, PullApprove, ReviewBot, and similar tools; no live web verification available during this research session)

## Methodology Note

Web search and fetch tools were unavailable during this research session. All findings are based on training data knowledge of the GitHub PR tooling ecosystem, which is a mature and well-documented space. Features of GitHub's native PR interface, Graphite, PullApprove, ReviewBot, and similar tools are well-represented in training data through May 2025. The core GitHub API surface and PR data model have been stable for years, so confidence in table-stakes features is relatively high. Newer differentiating features from tools like Graphite may have evolved since training cutoff.

---

## Table Stakes

Features that any PR tracking tool must have. If ReviewHub is missing these, users will consider it fundamentally broken for its stated purpose.

### PR Discovery and Listing

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| List PRs authored by user | Core use case stated in PROJECT.md | Low | GitHub API `GET /search/issues?q=author:{user}+type:pr` or per-repo pulls endpoint |
| List PRs where user is requested reviewer | Core use case stated in PROJECT.md | Low | GitHub API `GET /search/issues?q=review-requested:{user}+type:pr` |
| Filter by repository | Users need scoped views | Low | Already planned via CRUD repo management |
| Filter by PR state (open/closed/merged) | Fundamental triage need | Low | GitHub API `state` parameter; merged is a sub-state of closed |
| Unique PR identification | Prevent duplicates across polls | Low | Use `{owner}/{repo}#{number}` or GitHub node ID |

### PR Status and Metadata

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| PR title and description | Minimum context for identification | Low | Direct from GitHub API |
| PR state: open, closed, merged | Every PR tool shows this | Low | `state` + `merged_at` presence distinguishes closed vs merged |
| Author and assignees | Attribution is fundamental | Low | Direct from GitHub API |
| Branch info (head/base) | Needed to understand what the PR targets | Low | `head.ref`, `base.ref` from API |
| Created/updated timestamps | Temporal context is universal | Low | Direct from GitHub API |
| PR URL / web link | Users need to jump to GitHub | Low | `html_url` from API |
| Labels | Used for workflow categorization everywhere | Low | Direct from GitHub API |

### Review Status

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Review state per reviewer (approved/changes_requested/commented/pending) | Core review workflow signal | Medium | GitHub API `GET /repos/{owner}/{repo}/pulls/{number}/reviews`; must deduplicate to latest review per reviewer |
| Overall review decision (approved/changes requested/no reviews) | Aggregate signal for PR readiness | Medium | Computed: latest review from each requested reviewer |
| Requested reviewers list | Who still needs to review | Low | `requested_reviewers` field on PR object |
| Review comment count | Signal of review activity/complexity | Low | `review_comments` field on PR object |

### CI/CD Status

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Combined commit status (success/failure/pending) | Universal merge-readiness signal | Medium | GitHub API `GET /repos/{owner}/{repo}/commits/{ref}/status` for legacy statuses + `GET /repos/{owner}/{repo}/commits/{ref}/check-runs` for GitHub Actions checks |
| Individual check names and states | Users need to know WHICH check failed | Medium | Must combine both Status API and Checks API; they are separate systems |
| Required checks passing/failing | Distinguishes blocking vs informational checks | High | Requires fetching branch protection rules via `GET /repos/{owner}/{repo}/branches/{branch}/protection` to know which checks are required |

### Polling and Data Freshness

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Configurable poll interval | Users have different latency needs | Low | Already planned |
| Rate limit awareness | GitHub API has strict limits (5000 req/hr for authenticated) | Medium | Must track `X-RateLimit-Remaining` and `X-RateLimit-Reset` headers; back off when approaching limits |
| Conditional requests (ETags/If-Modified-Since) | Avoid wasting rate limit on unchanged data | Medium | GitHub supports `ETag` and `304 Not Modified`; significant rate-limit savings |
| Last polled timestamp | Users need to know data freshness | Low | Track per-repo or globally |

---

## Differentiators

Features that go beyond what basic PR tracking tools offer. These create competitive advantage, especially for ReviewHub's AI-agent-oriented niche.

### AI-Agent-Oriented Comment Formatting (Core Differentiator)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Review comments with targeted code snippets | THE core differentiator -- AI agent can read comment + code and generate fix | High | Must map `diff_hunk` and `position`/`line` from review comment API to actual file content; GitHub provides `diff_hunk` in comment payload but it may need enrichment |
| Multi-line comment context (surrounding code) | Single-line snippets are often insufficient for AI understanding | High | Fetch file content at PR head SHA via `GET /repos/{owner}/{repo}/contents/{path}?ref={sha}`, extract lines around comment target |
| Comment-to-file-path mapping | AI agent needs to know exactly which file and line range to edit | Medium | `path` and `line`/`original_line` fields from review comment API |
| Suggested changes extraction | GitHub supports `suggestion` blocks in comments; extracting these gives AI a direct fix proposal | Medium | Parse markdown suggestion blocks from comment body: ````suggestion\n...\n```` |
| Inline vs general comment distinction | General comments need different handling than line-specific ones | Low | Review comments have `path` field; its absence or PR-level comments indicate general discussion |
| Conversation threading (reply chains) | AI needs full context of a discussion, not just individual comments | Medium | `in_reply_to_id` field links reply chains; must reconstruct thread tree |

### Review Intelligence

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Thread resolution tracking | Know which feedback is addressed vs outstanding | Medium | GitHub API provides `isResolved` on review threads (GraphQL API: `pullRequest.reviewThreads`); REST API does not expose this directly -- may need GraphQL |
| Coderabbit detection and status | Distinguish AI-generated reviews from human reviews | Low | Check review author login against `coderabbitai` (or configurable bot usernames) |
| Actionable vs informational comment classification | AI agent should prioritize actionable feedback | High | Heuristic or NLP-based; changes_requested reviews are more actionable than comment-only reviews |
| Review freshness (outdated reviews) | Reviews on old commits may be stale after force-push | Medium | Compare review `commit_id` against current PR head SHA; GitHub marks reviews as "outdated" when code changes beneath them |
| Pending reviews detection | Pending (draft) reviews are not yet submitted | Low | Review state `PENDING` in API; typically only visible to the review author |

### PR Health and Readiness

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Staleness tracking (days since open, days since last activity) | Highlight aging PRs that need attention | Low | Computed from `created_at` and `updated_at` |
| Merge conflict detection | PRs with conflicts cannot be merged | Medium | `mergeable` and `mergeable_state` fields on PR; requires GitHub to compute -- may be null initially, need retry logic |
| Diff stats (files changed, additions, deletions) | Quick complexity signal | Low | `changed_files`, `additions`, `deletions` on PR object |
| Draft PR detection | Draft PRs have different workflow expectations | Low | `draft` boolean field on PR object |
| Merge readiness score (composite) | Single signal combining reviews + checks + conflicts + staleness | Medium | Computed from multiple signals; opinionated but highly useful for triage |

### Repository and Configuration Management

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Runtime repo CRUD | Add/remove repos without restart | Low | Already planned; store in SQLite |
| Per-repo poll interval override | High-activity repos may need faster polling | Low | Optional override in repo config |
| Bot user configuration | Different teams use different review bots (Coderabbit, Copilot, custom) | Low | Configurable list of bot usernames per repo or globally |
| Repository health summary | Aggregate stats across all watched repos | Low | Computed from individual PR data |

### Data Enrichment

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| File content at PR head SHA | AI agent can see the full file, not just diff hunks | Medium | `GET /repos/{owner}/{repo}/contents/{path}?ref={sha}`; cache aggressively per SHA (immutable) |
| Diff between base and head | Full diff context beyond individual comments | Medium | `GET /repos/{owner}/{repo}/pulls/{number}/files` returns patch per file |
| PR timeline events | Understand the full lifecycle (review requested, force pushed, etc.) | Medium | `GET /repos/{owner}/{repo}/issues/{number}/timeline` provides rich event history |
| Commit list per PR | Useful for understanding PR evolution, especially after force pushes | Low | `GET /repos/{owner}/{repo}/pulls/{number}/commits` |

---

## Anti-Features

Features to explicitly NOT build. These are common in PR dashboard tools but wrong for ReviewHub's scope and audience.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Web dashboard / UI | PROJECT.md explicitly scopes this out; primary consumer is CLI agent, not humans. Building UI doubles complexity for zero audience value | Serve clean JSON API endpoints. Let consumers build their own views |
| Review assignment / routing | Tools like PullApprove and CODEOWNERS handle this. Duplicating it adds complexity with no differentiation | Report who is assigned/requested; do not manage assignments |
| Notification system (email, Slack, push) | Polling-based tool for machine consumption; notifications are a human-facing concern. PROJECT.md excludes this | Expose "needs attention" flags in API; let consumers decide alerting |
| AI review summarization / generation | The downstream AI agent (Claude Code) handles this. ReviewHub provides data, not intelligence | Format data cleanly so the AI agent can summarize |
| GitHub webhook receiver | Adds deployment complexity (public endpoint, secret management, delivery guarantees). Polling is simpler for v1 | Poll at configurable intervals; webhooks can be a v2 optimization |
| Multi-user / multi-tenant support | Single-user, single-token design is explicitly scoped. Multi-tenant adds auth, isolation, and complexity | Single user config via env vars; if needed later, it is a v2 concern |
| OAuth / SSO authentication | Localhost-only, no auth needed. Adding OAuth is massive scope creep | Accept PAT via environment variable |
| PR creation or modification | ReviewHub is read-only tracking. Writing PRs is a different tool entirely | Only read operations against GitHub API |
| Code analysis or linting | Out of scope; many dedicated tools exist (CodeClimate, SonarQube, etc.) | Report CI check status which includes linter results |
| Merge automation | Tools like Mergify and GitHub's auto-merge handle this. Not ReviewHub's job | Report merge readiness; do not perform merges |
| Custom review workflows / policies | PullApprove's territory. ReviewHub tracks state, not enforces policy | Report current state; let teams use CODEOWNERS and branch protection for policies |
| Historical analytics / trends | Metrics dashboards (LinearB, Sleuth, etc.) own this space. Storage and query complexity is high | Track current state only; expire old data |
| Comment reply / interaction | ReviewHub is read-only. Posting comments requires different auth scope and is out of purpose | Read and format comments; never post |

---

## Feature Dependencies

```
Repository CRUD (foundation)
  |
  +---> PR Discovery (requires repos to watch)
  |       |
  |       +---> PR Status & Metadata (enriches discovered PRs)
  |       |       |
  |       |       +---> Diff Stats (per-PR enrichment)
  |       |       +---> Draft Detection (per-PR enrichment)
  |       |       +---> Staleness Tracking (per-PR enrichment)
  |       |       +---> Merge Conflict Detection (per-PR enrichment)
  |       |
  |       +---> Review Status (requires PR list)
  |       |       |
  |       |       +---> Coderabbit Detection (specialized review filtering)
  |       |       +---> Thread Resolution Tracking (per-review enrichment, may need GraphQL)
  |       |       +---> Review Freshness / Outdated Detection (compare review SHA to head)
  |       |
  |       +---> CI/CD Check Status (requires PR head SHA)
  |       |       |
  |       |       +---> Required Checks (needs branch protection data)
  |       |
  |       +---> Review Comment Formatting (THE differentiator)
  |               |
  |               +---> Comment-to-file mapping (foundation for snippets)
  |               +---> Diff hunk extraction (from comment payload)
  |               +---> File content at SHA (enrichment for surrounding context)
  |               +---> Suggested changes extraction (parse suggestion blocks)
  |               +---> Conversation threading (reply chain reconstruction)
  |
  +---> Polling Engine (runs independently, feeds all above)
          |
          +---> Rate Limit Awareness (required for reliable polling)
          +---> Conditional Requests / ETags (optimization, not blocking)
          +---> Per-repo Poll Intervals (optional override)

Merge Readiness Score (composite)
  -- depends on: Review Status + CI/CD Status + Merge Conflict Detection + Staleness
```

### Critical Path

The shortest path to core value is:

1. **Repository CRUD** + **Polling Engine** (no data without these)
2. **PR Discovery** (find PRs to track)
3. **PR Status & Metadata** (basic useful output)
4. **Review Status** (review workflow awareness)
5. **Review Comment Formatting with Code Snippets** (core differentiator)

Everything else is enrichment layered on top of this critical path.

---

## MVP Recommendation

For MVP, prioritize the critical path plus essential enrichment:

### Must Have (MVP)

1. **Repository CRUD** -- foundation; without repo config, nothing works
2. **Polling engine with rate-limit awareness** -- data ingestion backbone
3. **PR discovery (authored + review-requested)** -- core use case
4. **PR status and metadata** -- title, state, author, timestamps, URL, labels, branch info
5. **Review status per reviewer** -- approved/changes_requested/commented
6. **Review comments with targeted code snippets** -- THE differentiator; without this, ReviewHub is just another PR list
7. **CI/CD combined check status** -- merge readiness signal
8. **Diff stats** -- trivially cheap, immediately useful
9. **Staleness tracking** -- trivially cheap, immediately useful
10. **Coderabbit detection** -- trivially cheap, explicitly planned

### Defer to Post-MVP

- **Thread resolution tracking**: May require GraphQL API (REST doesn't expose `isResolved` cleanly); adds API complexity. Worth investigating in a research spike before committing.
- **Merge conflict detection**: `mergeable` field requires GitHub backend computation and may be `null` on first request; retry logic adds complexity.
- **Required checks identification**: Needs branch protection API which requires admin-level token permissions; may not be available.
- **Suggested changes extraction**: Useful but parsing markdown suggestion blocks is fiddly; comment body is already provided.
- **File content at head SHA**: Valuable for richer AI context but significantly increases API calls and storage; optimize with caching.
- **Review freshness/outdated detection**: Nice signal but not critical for initial value.
- **Merge readiness composite score**: Depends on multiple inputs being stable first.
- **Conditional requests (ETags)**: Optimization; poll naively first, optimize when rate limits become a real constraint.
- **PR timeline events**: Rich but verbose; adds storage and API cost.
- **Per-repo poll interval overrides**: YAGNI for single-user MVP.

---

## Competitive Landscape Context

### GitHub Native PR Interface

GitHub's own interface provides rich PR browsing, review, and merge tooling. Table stakes for any external tool is matching the data GitHub already surfaces: PR state, reviews, checks, diffs, comments. ReviewHub's value is NOT replacing GitHub's UI -- it is reformatting GitHub's data for machine consumption, specifically for an AI coding agent that needs code context alongside review comments.

### Graphite

Graphite focuses on stacked diffs (dependent PR chains), PR dashboard with review queue, and merge workflow automation. Its differentiators are stacked PR management and fast merge queues. ReviewHub does not compete with Graphite on workflow -- it competes on data formatting for AI agents, which Graphite does not address.

### PullApprove

PullApprove handles review assignment policies: who needs to review what, based on file ownership rules. ReviewHub should not replicate this -- just report the state that PullApprove (or CODEOWNERS) has already established.

### ReviewBot / Other Bots

Various bots (Coderabbit, GitHub Copilot for PRs, ReviewBot) post automated reviews. ReviewHub's job is to identify these bot reviews, distinguish them from human reviews, and present them alongside human feedback. The bot detection feature (configurable bot username list) serves this need.

### Key Insight

None of these tools format review data for consumption by AI coding agents. ReviewHub's niche is the **AI-agent-as-consumer** paradigm: an API where every endpoint is designed so that an LLM can read the response and take action (generate code fixes, understand review feedback, prioritize work). This is genuinely unserved territory.

---

## GitHub API Considerations

### REST vs GraphQL

GitHub offers both REST and GraphQL APIs. Key tradeoffs for ReviewHub:

| Concern | REST API | GraphQL API |
|---------|----------|-------------|
| Simplicity | Simpler, well-documented endpoints | More complex query construction |
| Data efficiency | Multiple requests for related data (N+1 patterns) | Single query for nested data (PR + reviews + comments) |
| Thread resolution | `isResolved` NOT available in REST | `isResolved` available via `reviewThreads` |
| Rate limiting | 5000 requests/hour | 5000 points/hour (different cost model) |
| Go client libraries | `google/go-github` (mature, REST) | `shurcooL/githubv4` (GraphQL) |

**Recommendation:** Start with REST API using `google/go-github` for simplicity. Add targeted GraphQL queries only for features that REST cannot serve (thread resolution). This hybrid approach is common in production GitHub integrations.

### Pagination

All GitHub list endpoints are paginated (default 30, max 100 per page). The polling engine must handle pagination correctly for repos with many PRs or many review comments.

### Token Scopes

ReviewHub needs a personal access token (classic) with at minimum:
- `repo` scope (read access to private repos, PRs, reviews, checks)
- Or fine-grained token with: `Pull requests: Read`, `Contents: Read`, `Checks: Read`

Branch protection rules require `Administration: Read` which is a higher privilege -- another reason to defer required-checks detection to post-MVP.

---

## Sources

- GitHub REST API documentation (training data, HIGH confidence for API surface stability)
- GitHub GraphQL API documentation (training data, HIGH confidence for core schema)
- `google/go-github` library (training data, MEDIUM confidence -- verify current version)
- Graphite, PullApprove, Coderabbit product knowledge (training data, MEDIUM confidence -- features may have evolved)
- GitHub PR data model has been stable since 2019+ (HIGH confidence for field names and structures)

**Gaps:** Could not verify current versions of Go client libraries, latest Graphite/PullApprove features, or any tooling changes since May 2025. Recommend validating `google/go-github` version and API compatibility during stack research phase.
