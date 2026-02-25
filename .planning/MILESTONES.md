# Milestones

## v1.0 MVP (Shipped: 2026-02-14)

**Phases:** 1-6 | **Plans:** 16 | **LOC:** 9,052 Go | **Timeline:** 5 days (2026-02-10 → 2026-02-14)

**Delivered:** Dockerized Go API that tracks GitHub PRs and formats review comments with code context for AI agent consumption.

**Key accomplishments:**
- Hexagonal Go API with domain model, port interfaces, and SQLite persistence (WAL mode, dual reader/writer)
- GitHub polling engine with ETag caching, rate limiting, pagination, and PR discovery (author/reviewer/team)
- 10 REST endpoints for PR listing, detail, attention, repository management, bot config, and health
- Review intelligence with comment threading, suggestion extraction, bot/outdated detection, GraphQL thread resolution
- PR health signals with CI/CD status (Checks + Status API), diff stats, staleness, merge conflict detection
- Docker deployment with 17MB scratch image, adaptive 4-tier polling, and volume persistence

**Archives:**
- [v1.0-ROADMAP.md](milestones/v1.0-ROADMAP.md)
- [v1.0-REQUIREMENTS.md](milestones/v1.0-REQUIREMENTS.md)
- [v1.0-MILESTONE-AUDIT.md](milestones/v1.0-MILESTONE-AUDIT.md)

---

## 2026.2.0 Web GUI (Shipped: 2026-02-24)

**Phases:** 7-9 | **Plans:** 12 | **Tasks:** 23 | **LOC:** 18,782 Go (+9,730 over v1.0) | **Timeline:** 10 days (2026-02-14 → 2026-02-24)

**Delivered:** Web GUI with full PR review workflows, configurable attention signals, and Jira integration — transforming the API-only v1.0 into a complete developer dashboard.

**Key accomplishments:**
- Full-stack web GUI with templ/HTMX/Alpine.js/Tailwind/GSAP — PR feed sidebar, detail panel with tabs, search/filter, dark/light theme, GSAP animations, repo management
- Full PR review workflows — threaded comments, inline replies, approve/request-changes reviews, draft toggle via GraphQL with optimistic UI
- Credential management with zero-restart PollService hot-swap — GitHub token GUI entry, AES-256-GCM persistence, tokenProvider closure
- Configurable attention signals — per-repo review thresholds, urgency days, PR ignore list with re-add; 4-severity visual flagging
- Jira integration — JiraHTTPClient (ADF handling, Basic auth), multi-connection credential management, collapsible JiraCard (4 states), in-dashboard comment posting, PollService jira_key extraction

**Tech debt:**
- Dockerfile Tailwind CLI musl/glibc incompatibility — tailwindcss-linux-x64 fails on Alpine scratch; local dev works, Docker build needs fix

**Archives:**
- [2026.2.0-ROADMAP.md](milestones/2026.2.0-ROADMAP.md)
- [2026.2.0-REQUIREMENTS.md](milestones/2026.2.0-REQUIREMENTS.md)
- [2026.2.0-MILESTONE-AUDIT.md](milestones/2026.2.0-MILESTONE-AUDIT.md)

---

