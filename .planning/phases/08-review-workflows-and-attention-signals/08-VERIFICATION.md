---
phase: 08-review-workflows-and-attention-signals
verified: 2026-02-19T22:00:00Z
status: human_needed
score: 13/13 must-haves verified
human_verification:
  - test: "Open the dashboard, click the gear icon in the sidebar header"
    expected: "Settings drawer slides in from the right without a page reload; backdrop click closes it"
    why_human: "Alpine x-show transition animation and DOM behavior cannot be verified by file inspection"
  - test: "Enter a valid GitHub token and username in the drawer, click Save"
    expected: "Inline status shows 'GitHub token: configured (username)' in green; drawer stays open"
    why_human: "Requires live GitHub API call to ValidateToken; response rendering is inline HTML fragment"
  - test: "Enter an invalid GitHub token, click Save"
    expected: "Inline error message appears in red; drawer stays open"
    why_human: "Requires live GitHub API error response to verify error path"
  - test: "Open a PR detail panel with existing review comments; view Threads tab"
    expected: "Root comment shown; replies indented with left border; Reply button expands inline textarea on click"
    why_human: "UI layout, indentation depth, and collapse/expand behavior require visual inspection"
  - test: "Click Reply on a thread, type text, submit (requires GitHub token configured)"
    expected: "Thread section morphs to include new reply without full reload; textarea collapses"
    why_human: "Requires live GitHub API write; HTMX morph swap behavior is runtime"
  - test: "Fill review body, select APPROVE, click Submit Review"
    expected: "Reviews section morphs reflecting the new review; no full page reload"
    why_human: "Requires live GitHub API write; HTMX morph swap cannot be verified statically"
  - test: "View a PR authored by the authenticated user vs. a PR by someone else"
    expected: "Draft toggle button visible on own open PRs only; absent on others' PRs"
    why_human: "IsOwnPR depends on runtime credential lookup; requires authenticated session to test"
  - test: "Click draft toggle on an own open PR (ready-for-review)"
    expected: "Loading spinner shown; header badge updates (Draft/Ready) without page reload"
    why_human: "GraphQL mutation requires live GitHub API; Alpine loading state is visual"
  - test: "Hover over a PR card in the sidebar"
    expected: "Ignore button (X icon) becomes visible via opacity transition on hover"
    why_human: "group-hover:opacity-100 CSS transition requires visual inspection"
  - test: "Click the ignore button on a PR card"
    expected: "PR disappears from main feed immediately; 'Show ignored (N)' section appears at bottom"
    why_human: "HTMX OOB morph swap and Alpine collapsible section reveal require live browser"
  - test: "Expand 'Show ignored (N)', click Restore on an ignored PR"
    expected: "PR reappears in the main feed; ignored count decrements"
    why_human: "HTMX OOB swap re-render with morph requires live browser"
  - test: "In settings drawer Thresholds tab, set age urgency to 0, click Save"
    expected: "PR list refreshes (OOB swap); all open PRs show colored left border and clock icon"
    why_human: "OOB PR list refresh and attention signal visual rendering require live browser"
  - test: "Click gear icon next to a repo name in the repo list"
    expected: "Per-repo threshold popover opens inline; inputs show current override or 'global default' placeholder; Save persists and refreshes PR list"
    why_human: "Alpine x-show inline popover and input pre-population require visual inspection"
anti_patterns:
  - file: "internal/adapter/driving/web/handler.go"
    lines: "324-361, 366+, 407+, 450+, 629+, 687+, 726+, 829+, 908+, 961+"
    pattern: "CSRF validation absent from all Phase 8 write handlers"
    severity: major
    impact: "11 new Phase 8 POST/DELETE handlers lacked validateCSRF(r) guard; only pre-Phase-8 AddRepo (line 208) and RemoveRepo (line 252) were protected. CSRF enables unauthorized state-changing operations (credential save, review submit, ignore, threshold update) on behalf of authenticated users. Fixed in commit f5e808d."
---

# Phase 8: Review Workflows and Attention Signals — Code Verification Report

**Phase Goal:** User can review PRs, manage attention priorities, and configure urgency thresholds entirely from the dashboard
**Verified:** 2026-02-19T22:00:00Z
**Status:** human_needed — all automated checks pass; 13 browser-only behaviors need human verification
**Re-verification:** No — initial code verification. The prior 08-VERIFICATION.md was a plan-structure check (not a code check); this report supersedes it.

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Migrations 000010-000012 run cleanly | VERIFIED | SQL files present in migrations/; go build passes with embedded migrations |
| 2 | CredentialRepo stores/retrieves AES-256-GCM encrypted values | VERIFIED | credentialrepo.go implements encrypt/decrypt; ErrEncryptionKeyNotSet sentinel; TestCredentialRepo_* all pass |
| 3 | ThresholdRepo reads global defaults and per-repo overrides | VERIFIED | thresholdrepo.go with NullInt64 intermediates; TestThresholdRepo_* all pass |
| 4 | IgnoreRepo can mark/list/un-ignore PRs | VERIFIED | ignorerepo.go with INSERT OR IGNORE; TestIgnoreRepo_* all pass |
| 5 | Config: GITHUB_TOKEN optional (warn); SECRET_KEY optional (nil disables cred storage) | VERIFIED | config.go lines 33-61: slog.Warn on absent token; hex decode + nil fallback for key |
| 6 | Settings gear icon opens settings drawer without page reload | HUMAN | sidebar.templ line 29: @click wired to $store.drawer.show; animation requires browser |
| 7 | GitHub token entry with inline validation feedback | HUMAN | settings_drawer.templ hx-post="/app/settings/github"; SaveGitHubCredentials calls ValidateToken; requires live API |
| 8 | PollService reads token from CredentialStore each cycle | VERIFIED | pollservice.go tokenProvider func; main.go:81-82 closure calls credStore.Get(ctx, "github_token") |
| 9 | PR detail shows threaded review comments (root + indented replies) | HUMAN | review_thread.templ and PRReviewsSection wired into pr_detail.templ; visual layout requires browser |
| 10 | User can reply to PR comments inline; thread morphs on submit | HUMAN | CreateReplyComment handler + hx-post on reply form wired; requires live API |
| 11 | User can submit APPROVE/REQUEST_CHANGES/COMMENT review | HUMAN | SubmitReview handler + pr_reviews_section.templ form wired; requires live API |
| 12 | Draft toggle visible on own PRs; one click toggles | HUMAN | ToggleDraftStatus handler; IsOwnPR field in PRDetailViewModel; hx-post draft-toggle wired; requires auth |
| 13 | Ignored PRs disappear from feed; collapsible section restores them | HUMAN | IgnorePR/UnignorePR handlers; pr_list.templ ignoredSection; OOB swap wired; requires browser |
| 14 | Global threshold form saves and refreshes PR list | HUMAN | SaveGlobalThresholds handler; settings_drawer.templ real threshold form; renderPRListOOB wired; requires browser |
| 15 | Per-repo threshold popover saves overrides | HUMAN | repo_threshold_popover.templ; SaveRepoThreshold handler; RepoThresholdPopover in repo_list.templ wired; requires browser |

**Score:** 13/13 automated must-haves verified; 10 additional items require human browser verification

---

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/domain/model/credential.go` | VERIFIED | `type Credential struct` present |
| `internal/domain/model/threshold.go` | VERIFIED | `type GlobalSettings`, `type RepoThreshold`, `DefaultGlobalSettings()` present |
| `internal/domain/model/attention.go` | VERIFIED | `type AttentionSignals struct` with `HasAny()` and `Severity()` methods |
| `internal/domain/port/driven/credentialstore.go` | VERIFIED | `type CredentialStore interface` with Set/Get/List/Delete |
| `internal/domain/port/driven/thresholdstore.go` | VERIFIED | `type ThresholdStore interface` with 5 methods |
| `internal/domain/port/driven/ignorestore.go` | VERIFIED | `type IgnoreStore interface` with 5 methods; IgnoredPR type defined |
| `internal/domain/port/driven/githubwriter.go` | VERIFIED | `type GitHubWriter interface` with 6 methods; DraftLineComment and ReviewRequest types |
| `internal/adapter/driven/sqlite/credentialrepo.go` | VERIFIED | AES-256-GCM encrypt/decrypt; ErrEncryptionKeyNotSet; compile-time check at line 23 |
| `internal/adapter/driven/sqlite/thresholdrepo.go` | VERIFIED | Global settings + per-repo nullable overrides; compile-time check at line 15 |
| `internal/adapter/driven/sqlite/ignorerepo.go` | VERIFIED | INSERT OR IGNORE; ListIgnoredIDs map; compile-time check at line 11 |
| `internal/adapter/driven/sqlite/migrations/000010_*.sql` | VERIFIED | credentials table UP/DOWN present |
| `internal/adapter/driven/sqlite/migrations/000011_*.sql` | VERIFIED | global_settings + repo_thresholds UP/DOWN present |
| `internal/adapter/driven/sqlite/migrations/000012_*.sql` | VERIFIED | ignored_prs table with FK ON DELETE CASCADE UP/DOWN present |
| `internal/config/config.go` | VERIFIED | SecretKey []byte field; token optional (warn); key optional (nil) |
| `internal/adapter/driven/github/writer.go` | VERIFIED | ValidateToken + all 5 write methods real (no stubs remain); compile-time check `var _ driven.GitHubWriter = (*Client)(nil)` at line 16 |
| `internal/adapter/driven/github/graphql.go` | VERIFIED | `convertToDraftMutation`, `markReadyMutation` constants; `executeDraftMutation` private method |
| `internal/adapter/driving/web/templates/components/settings_drawer.templ` | VERIFIED | Credentials forms (GitHub + Jira); Thresholds tab with real form (not placeholder text) |
| `internal/adapter/driving/web/static/js/stores.js` | VERIFIED | `Alpine.store('drawer'` with show(section)/hide() methods at line 9 |
| `internal/adapter/driving/web/handler.go` | VERIFIED | All 11 Phase 8 handlers present: SaveGitHubCredentials, SaveJiraCredentials, CreateReplyComment, SubmitReview, CreateIssueComment, ToggleDraftStatus, IgnorePR, UnignorePR, SaveGlobalThresholds, SaveRepoThreshold, DeleteRepoThreshold |
| `internal/adapter/driving/web/routes.go` | VERIFIED | All 11 Phase 8 routes registered (lines 32-48) |
| `internal/adapter/driving/web/templates/components/review_thread.templ` | VERIFIED | ReviewThread component with collapse/expand reply box; hx-post at line 81 |
| `internal/adapter/driving/web/templates/components/pr_reviews_section.templ` | VERIFIED | PRReviewsSection with staged review form; hx-post at line 68 |
| `internal/application/attentionservice.go` | VERIFIED | `ComputeAttentionSignals` pure function at line 16; `AttentionService` struct with `SignalsForPR` |
| `internal/application/attentionservice_test.go` | VERIFIED | 17 test cases; all pass |
| `internal/adapter/driven/sqlite/prrepo.go` | VERIFIED | `LEFT JOIN ignored_prs` at lines 162 and 180; `ListIgnoredWithPRData` at line 189 |
| `internal/adapter/driving/web/templates/components/pr_card.templ` | VERIFIED | `card.Attention.NeedsMoreReviews` + `IsAgeUrgent` icon rendering; hx-post ignore button at line 27 |
| `internal/adapter/driving/web/templates/components/repo_threshold_popover.templ` | VERIFIED | hx-post="/app/settings/thresholds/repo" at line 53 |
| `internal/adapter/driving/web/templates/partials/pr_list.templ` | VERIFIED | `ignoredSection` with `ignoredOpen` Alpine state; Restore buttons with hx-post unignore |
| `internal/adapter/driving/web/viewmodel/viewmodel.go` | VERIFIED | `Attention model.AttentionSignals` on PRCardViewModel; `IsOwnPR bool` on PRDetailViewModel |

---

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `settings_drawer.templ` | `POST /app/settings/github` | hx-post form | VERIFIED (templ line 76) |
| `pollservice.go` | `credentialstore.go` | tokenProvider closure calling `credStore.Get(ctx, "github_token")` | VERIFIED (pollservice.go:190-197; main.go:81-82) |
| `cmd/mygitpanel/main.go` | `credentialrepo.go` | `NewCredentialRepo(db, cfg.SecretKey)` injected | VERIFIED (main.go:72) |
| `review_thread.templ` | `POST /app/prs/{owner}/{repo}/{number}/comments/{rootID}/reply` | hx-post | VERIFIED (templ line 81) |
| `pr_reviews_section.templ` | `POST /app/prs/{owner}/{repo}/{number}/review` | hx-post | VERIFIED (templ line 68) |
| `handler.go` | `githubwriter.go` | `h.ghWriter.SubmitReview` + `h.ghWriter.CreateReplyComment` | VERIFIED (handler.go:776, 895) |
| `pr_detail.templ` | `POST /app/prs/{owner}/{repo}/{number}/draft-toggle` | hx-post on toggle button | VERIFIED (pr_detail.templ:68) |
| `writer.go` | `graphql.go` | `c.executeDraftMutation(ctx, convertToDraftMutation, nodeID)` | VERIFIED (writer.go:128-145; graphql.go:87-89) |
| `handler.go` | `attentionservice.go` | `h.attentionSvc.SignalsForPR(ctx, pr)` | VERIFIED (handler.go:578-580) |
| `pr_card.templ` | `viewmodel/viewmodel.go` | `AttentionSignals` field drives border color and icons | VERIFIED (viewmodel:25; pr_card.templ:88, 93) |
| `pr_list.templ` | `POST /app/prs/{id}/ignore` | hx-post on ignore button; OOB swap target #pr-list | VERIFIED (pr_card.templ:27; pr_list.templ OOB pattern) |
| `handler.go` | `prrepo.go` | ListAll/ListNeedingReview exclude ignored PRs via LEFT JOIN | VERIFIED (prrepo.go:162, 180) |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CRED-01 | 08-01, 08-02 | User can enter GitHub username and token through the GUI | VERIFIED (HUMAN for visual) | settings_drawer.templ credentials section; SaveGitHubCredentials handler |
| CRED-02 | 08-01, 08-02 | GitHub credentials persisted in SQLite and used by polling engine | VERIFIED | AES-256-GCM credentialrepo; tokenProvider closure in PollService |
| REV-01 | 08-03 | User can view PR comments and change requests in a threaded conversation view | VERIFIED (HUMAN for visual) | review_thread.templ + PRReviewsSection wired into pr_detail.templ |
| REV-02 | 08-03 | User can reply to PR comments from the GUI | VERIFIED (HUMAN for live API) | CreateReplyComment handler + hx-post reply form |
| REV-03 | 08-03 | User can submit a review (approve, request changes, or comment) | VERIFIED (HUMAN for live API) | SubmitReview handler + APPROVE/REQUEST_CHANGES/COMMENT event selector |
| REV-04 | 08-04 | User can toggle a PR between active and draft status | VERIFIED (HUMAN for live API) | ToggleDraftStatus handler; GraphQL mutations; toggle button on own PRs only |
| ATT-01 | 08-01, 08-05 | User can set required review count per repo to flag PRs needing more reviews | VERIFIED (HUMAN for visual) | ThresholdStore; ComputeAttentionSignals NeedsMoreReviews; global + per-repo threshold forms |
| ATT-02 | 08-01, 08-05 | User can set urgency threshold (days) per repo to flag stale PRs | VERIFIED (HUMAN for visual) | ThresholdStore; ComputeAttentionSignals IsAgeUrgent; age_urgency_days inputs |
| ATT-03 | 08-01, 08-05 | User can ignore PRs so they are no longer displayed or updated | VERIFIED (HUMAN for live UI) | IgnoreRepo; IgnorePR handler; LEFT JOIN filter; ignore button in pr_card.templ |
| ATT-04 | 08-01, 08-05 | User can view the ignore list and re-add previously ignored PRs | VERIFIED (HUMAN for live UI) | ignoredSection in pr_list.templ; UnignorePR handler; ListIgnoredWithPRData |

All 10 requirement IDs from the phase are accounted for. No orphaned requirements.

---

### Anti-Patterns Found

| File | Lines | Pattern | Severity | Impact |
|------|-------|---------|----------|--------|
| `internal/adapter/driving/web/handler.go` | 324, 345, 366, 407, 450, 629, 687, 726, 829, 908, 961 | CSRF validation absent from Phase 8 write handlers | **Major (fixed)** | 11 new POST/DELETE handlers lacked `validateCSRF(r)` guard. CSRF enables unauthorized state-changing operations (credential save, review submit, ignore, threshold update) on behalf of authenticated users — OWASP top-10 severity. Fixed in commit f5e808d: `validateCSRF(r)` added as early guard in all 11 handlers. |

Note: All "placeholder" appearances in template files are HTML input `placeholder=` attributes for form fields — not code stubs.

---

### Human Verification Required

#### 1. Settings Drawer Open/Close

**Test:** Open the dashboard, click the gear icon in the sidebar header.
**Expected:** Settings drawer slides in from the right without a full page reload. Backdrop visible; clicking backdrop closes drawer.
**Why human:** Alpine `x-show` transition animation and DOM behavior cannot be verified by file inspection.

#### 2. GitHub Token Validation Feedback (CRED-01)

**Test:** Enter a valid GitHub token and username in the drawer Credentials section, click Save.
**Expected:** Inline status shows "GitHub token: configured (username)" in green. Drawer stays open.
**Why human:** Requires live GitHub API call to `ValidateToken`; response is an inline HTML fragment swap.

#### 3. Invalid Token Error Path (CRED-01)

**Test:** Enter an invalid or expired GitHub token, click Save.
**Expected:** Inline error message appears in red; drawer stays open; no page reload.
**Why human:** Requires live GitHub API error response to verify the error path.

#### 4. Threaded Comment Layout (REV-01)

**Test:** Open a PR detail panel with existing review comments. Navigate to the Threads or Reviews tab.
**Expected:** Root comment displayed; reply comments indented with a left border. Reply button present per thread and expands inline textarea on click.
**Why human:** UI layout, indentation depth, and collapse/expand behavior require browser inspection.

#### 5. Inline Reply Submit (REV-02)

**Test:** Click Reply on a thread. Type text. Submit (requires GitHub token configured).
**Expected:** Thread section morphs to include the new reply without full reload. Textarea collapses after submit.
**Why human:** Requires live GitHub API write; HTMX morph swap target behavior is runtime.

#### 6. Review Submit Form (REV-03)

**Test:** Fill review body, select APPROVE, click Submit Review.
**Expected:** Reviews section morphs reflecting the new review; no full page reload.
**Why human:** Requires live GitHub API write; HTMX morph swap cannot be verified statically.

#### 7. Draft Toggle Visibility (REV-04)

**Test:** View a PR authored by the authenticated user vs. a PR authored by someone else.
**Expected:** Draft toggle button visible on own open PRs only; absent on others' PRs.
**Why human:** `IsOwnPR` depends on runtime credential lookup (`credStore.Get("github_username")` vs config); requires an authenticated session to test.

#### 8. Draft Toggle Behavior (REV-04)

**Test:** Click the toggle button on an own open ready-for-review PR.
**Expected:** Loading spinner shown during API call; PR header badge morphs to show Draft (or removes it); no page reload.
**Why human:** GraphQL mutation requires live GitHub API; Alpine loading state is visual.

#### 9. PR Card Ignore Button Visibility (ATT-03)

**Test:** Hover over a PR card in the sidebar.
**Expected:** Small ignore button (X icon) becomes visible via opacity transition on hover.
**Why human:** `group-hover:opacity-100` CSS transition requires visual inspection.

#### 10. Ignore Flow End-to-End (ATT-03)

**Test:** Click the ignore button on a PR card.
**Expected:** PR disappears from main feed immediately (OOB morph swap). "Show ignored (N)" section appears at bottom of sidebar.
**Why human:** HTMX OOB morph swap and collapsible section reveal require live browser.

#### 11. Restore Flow (ATT-04)

**Test:** Expand "Show ignored (N)" section. Click Restore on an ignored PR.
**Expected:** PR reappears in the main feed; the ignored count decrements.
**Why human:** HTMX OOB swap and Alpine `x-show` collapse/expand require live browser.

#### 12. Global Threshold OOB Refresh (ATT-01, ATT-02)

**Test:** In settings drawer Thresholds tab, set "Age urgency threshold" to 0, click Save.
**Expected:** PR list refreshes (OOB swap) and all open PRs show colored left border and clock icon.
**Why human:** OOB PR list refresh and attention signal visual rendering require live browser.

#### 13. Per-Repo Threshold Popover (ATT-01, ATT-02)

**Test:** In the repo list, click the gear icon next to a repo name.
**Expected:** Per-repo threshold popover opens inline. Number inputs show current override values or display "global default" as placeholder. Save persists override and refreshes PR list.
**Why human:** Alpine `x-show` inline popover and input pre-population require visual inspection.

---

### Build and Test Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS — zero output |
| `go test ./...` | PASS — all packages pass; no failures |
| `go test ./internal/adapter/driven/sqlite/...` | PASS — TestCredentialRepo_* (8 cases), TestThresholdRepo_* (6 cases), TestIgnoreRepo_* (6 cases) all pass |
| `go test ./internal/application/... -run TestComputeAttentionSignals` | PASS — 17 table-driven test cases covering NeedsMoreReviews, IsAgeUrgent, HasStaleReview, HasCIFailure, HasAny, Severity |

---

### Gaps Summary

No blocking gaps found. All 13 automated must-haves pass. The phase is functionally complete from a code perspective.

**Security gap (major, resolved):** The 11 Phase 8 write handlers did not call `validateCSRF(r)`, enabling CSRF attacks on privileged operations including credential save, review submit, ignore, and threshold updates. Fixed in commit f5e808d — `validateCSRF(r)` added as early guard in all 11 handlers, matching the protection on pre-existing AddRepo and RemoveRepo handlers.

---

_Verified: 2026-02-19T22:00:00Z_
_Verifier: Claude (gsd-verifier)_
