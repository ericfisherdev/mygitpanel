# Roadmap: ReviewHub

## Overview

ReviewHub delivers a Dockerized Go API that tracks GitHub pull requests and formats review comments with code context for AI agent consumption. Milestone 2026.2.0 adds a web GUI using templ/HTMX/Alpine.js/Tailwind/GSAP, full PR review workflows, configurable attention signals, and Jira integration for issue context and commenting.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-6 (shipped 2026-02-14)
- 🚧 **2026.2.0 Web GUI** — Phases 7-9 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-6) — SHIPPED 2026-02-14</summary>

- [x] Phase 1: Foundation (3/3 plans) — completed 2026-02-10
- [x] Phase 2: GitHub Integration (2/2 plans) — completed 2026-02-11
- [x] Phase 3: Core API (2/2 plans) — completed 2026-02-11
- [x] Phase 4: Review Intelligence (4/4 plans) — completed 2026-02-12
- [x] Phase 5: PR Health Signals (3/3 plans) — completed 2026-02-13
- [x] Phase 6: Docker Deployment (2/2 plans) — completed 2026-02-14

Full details: [milestones/v1.0-ROADMAP.md](milestones/v1.0-ROADMAP.md)

</details>

### 🚧 2026.2.0 Web GUI (In Progress)

**Milestone Goal:** Add a web GUI that surfaces all existing API capabilities and extends them with full PR review workflows, Jira integration, and customizable attention configuration.

- [x] **Phase 7: GUI Foundation** — Read-only dashboard with templ/HTMX/Alpine.js/Tailwind/GSAP
- [x] **Phase 8: Review Workflows and Attention Signals** — Write operations, credential management, configurable thresholds, PR ignore list
- [ ] **Phase 9: Jira Integration** — Jira API adapter, issue viewing, commenting, auto-linking

## Phase Details

### Phase 7: GUI Foundation
**Goal:** User can browse all PR activity across repos in an interactive web dashboard without leaving a single page
**Depends on:** Phase 6 (Docker deployment — existing v1.0 foundation)
**Requirements:** GUI-01, GUI-02, GUI-03, GUI-04, GUI-05, GUI-06, GUI-07
**Success Criteria** (what must be TRUE):
  1. User can open the web dashboard and see a unified feed of PRs from all watched repos, with status indicators for CI, review state, and merge readiness
  2. User can click a PR to view its full detail including description, branch info, reviewers, CI checks, diff stats, and review comments with code context
  3. User can search PRs by text and filter by status or repo, with results updating without full page reload
  4. User can toggle between dark and light theme, with the preference persisting across sessions
  5. User can add and remove watched repos through the GUI, with the PR feed updating to reflect changes
**Plans:** 3 plans

Plans:
- [ ] 07-01-PLAN.md — Scaffolding: vendored JS, Tailwind, templ layout, web adapter skeleton, Dockerfile update
- [ ] 07-02-PLAN.md — PR feed sidebar + PR detail panel with reviews, threads, CI checks, collapsible sidebar
- [ ] 07-03-PLAN.md — Search/filter, theme toggle, repo management, GSAP animations

### Phase 8: Review Workflows and Attention Signals
**Goal:** User can review PRs, manage attention priorities, and configure urgency thresholds entirely from the dashboard
**Depends on:** Phase 7
**Requirements:** CRED-01, CRED-02, REV-01, REV-02, REV-03, REV-04, ATT-01, ATT-02, ATT-03, ATT-04
**Success Criteria** (what must be TRUE):
  1. User can enter a GitHub token through the GUI and the polling engine uses it for subsequent API calls without requiring a container restart
  2. User can view PR comments in a threaded conversation layout, reply to specific comments, and submit full reviews (approve, request changes, comment) on others' PRs
  3. User can toggle a PR between active and draft status from the PR detail view
  4. User can configure per-repo review count thresholds and age-based urgency days, and PRs are visually flagged when they exceed these thresholds
  5. User can ignore a PR to hide it from the feed, view the ignore list, and re-add previously ignored PRs
**Plans:** 4 plans

Plans:
- [ ] 08-01-PLAN.md — Domain models, ports, migrations, SQLite adapters for credentials, repo settings, ignore list, and PR node ID
- [ ] 08-02-PLAN.md — GitHub write methods (review, comment, reply, draft toggle), GitHubClientProvider hot-swap, config fallback
- [ ] 08-03-PLAN.md — Credential management UI, composition root rewire, review submission/comment reply/draft toggle UI
- [ ] 08-04-PLAN.md — Per-repo attention thresholds with visual flags, PR ignore/restore with ignore list page

### Phase 9: Jira Integration
**Goal:** User can see linked Jira issue context alongside PRs and post comments to Jira without leaving the dashboard
**Depends on:** Phase 8 (credential storage pattern established)
**Requirements:** CRED-03, CRED-04, JIRA-01, JIRA-02, JIRA-03
**Success Criteria** (what must be TRUE):
  1. User can enter Jira connection details (URL, email, token) through the GUI and credentials are persisted for use across sessions
  2. When viewing a PR whose branch name contains a Jira issue key, the dashboard automatically displays the linked Jira issue details (description, status, priority, assignee, comments)
  3. User can post a comment to a linked Jira issue directly from the PR detail view, and the comment appears in Jira
**Plans:** TBD

Plans:
- [ ] 09-01: TBD
- [ ] 09-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 7 → 8 → 9

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-02-10 |
| 2. GitHub Integration | v1.0 | 2/2 | Complete | 2026-02-11 |
| 3. Core API | v1.0 | 2/2 | Complete | 2026-02-11 |
| 4. Review Intelligence | v1.0 | 4/4 | Complete | 2026-02-12 |
| 5. PR Health Signals | v1.0 | 3/3 | Complete | 2026-02-13 |
| 6. Docker Deployment | v1.0 | 2/2 | Complete | 2026-02-14 |
| 7. GUI Foundation | 2026.2.0 | 3/3 | Complete | 2026-02-15 |
| 8. Review Workflows and Attention Signals | 2026.2.0 | 4/4 | Complete | 2026-02-15 |
| 9. Jira Integration | 2026.2.0 | 0/TBD | Not started | - |
