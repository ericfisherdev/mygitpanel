# Requirements: MyGitPanel

**Defined:** 2026-02-14
**Core Value:** A single dashboard where a developer can see all PRs needing attention, review and comment on them, and link to Jira context

## 2026.2.0 Requirements

Requirements for the Web GUI milestone. Each maps to roadmap phases.

### GUI Foundation

- [ ] **GUI-01**: User can view a unified PR feed across all watched repos
- [ ] **GUI-02**: User can view PR detail including description, branch info, reviewers, and CI status
- [ ] **GUI-03**: User can search and filter PRs by status, repo, and text
- [ ] **GUI-04**: User can toggle between dark and light theme
- [ ] **GUI-05**: User can collapse/expand the sidebar PR list
- [ ] **GUI-06**: GUI displays GSAP-animated transitions on PR selection, tab switching, and new data arrival
- [ ] **GUI-07**: User can manage watched repos (add/remove) through the GUI

### Credential Management

- [ ] **CRED-01**: User can enter GitHub username and token through the GUI
- [ ] **CRED-02**: GitHub credentials are persisted in SQLite and used by the polling engine
- [ ] **CRED-03**: User can enter Jira connection details (URL, email, token) through the GUI
- [ ] **CRED-04**: Jira credentials are persisted in SQLite

### Review Workflows

- [ ] **REV-01**: User can view PR comments and change requests in a threaded conversation view
- [ ] **REV-02**: User can reply to PR comments from the GUI
- [ ] **REV-03**: User can submit a review on others' PRs (approve, request changes, or comment)
- [ ] **REV-04**: User can toggle a PR between active and draft status

### Attention Signals

- [ ] **ATT-01**: User can set required review count per repo to flag PRs needing more reviews
- [ ] **ATT-02**: User can set urgency threshold (days) per repo to flag stale PRs
- [ ] **ATT-03**: User can ignore PRs so they are no longer displayed or updated
- [ ] **ATT-04**: User can view the ignore list and re-add previously ignored PRs

### Jira Integration

- [ ] **JIRA-01**: User can view linked Jira issue details (description, comments, priority, status, assignee)
- [ ] **JIRA-02**: User can post comments to linked Jira issues from the GUI
- [ ] **JIRA-03**: Jira issues are auto-linked by extracting issue keys from PR branch names

## Future Requirements

Deferred to future milestone. Tracked but not in current roadmap.

### Jira Write Operations

- **JIRA-W01**: User can update Jira issue status from the GUI
- **JIRA-W02**: User can create Jira issues from the GUI
- **JIRA-W03**: User can update Jira issue priority/assignee from the GUI

### Notifications

- **NOTF-01**: User receives in-app toast notifications for new PR activity
- **NOTF-02**: User receives browser push notifications for urgent PRs

### Webhooks

- **HOOK-01**: System receives GitHub webhook events for real-time updates
- **HOOK-02**: System receives Jira webhook events for issue changes

## Out of Scope

| Feature | Reason |
|---------|--------|
| Multi-user support | Single user, single GitHub token for this milestone |
| OAuth login flows | Token-based configuration only |
| React/Vue/Svelte frameworks | HTMX/Alpine.js stack per user preference |
| Review summary/digest generation | AI agent handles summarization |
| Push notifications | Pull-based only for this milestone |
| Jira issue creation/status changes | Read + comment only |
| Mobile-specific responsive design | Desktop-first, basic responsive only |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| GUI-01 | Phase 7 | Pending |
| GUI-02 | Phase 7 | Pending |
| GUI-03 | Phase 7 | Pending |
| GUI-04 | Phase 7 | Pending |
| GUI-05 | Phase 7 | Pending |
| GUI-06 | Phase 7 | Pending |
| GUI-07 | Phase 7 | Pending |
| CRED-01 | Phase 8 | Pending |
| CRED-02 | Phase 8 | Pending |
| CRED-03 | Phase 9 | Pending |
| CRED-04 | Phase 9 | Pending |
| REV-01 | Phase 8 | Pending |
| REV-02 | Phase 8 | Pending |
| REV-03 | Phase 8 | Pending |
| REV-04 | Phase 8 | Pending |
| ATT-01 | Phase 8 | Pending |
| ATT-02 | Phase 8 | Pending |
| ATT-03 | Phase 8 | Pending |
| ATT-04 | Phase 8 | Pending |
| JIRA-01 | Phase 9 | Pending |
| JIRA-02 | Phase 9 | Pending |
| JIRA-03 | Phase 9 | Pending |

**Coverage:**
- 2026.2.0 requirements: 22 total
- Mapped to phases: 22
- Unmapped: 0

---
*Requirements defined: 2026-02-14*
*Last updated: 2026-02-14 after roadmap creation*
