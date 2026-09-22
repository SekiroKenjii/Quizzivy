# September 2026 remediation evidence

This follows the immutable [baseline audit](2026-09-22-open-issues.md).
It records repository changes; it is not evidence of a production deployment.
The user approved the retention policy and confirmed that Neon/monitoring access
is not configured.

## Issue disposition

| Issue | Repository outcome | Remaining acceptance |
|---|---|---|
| #78 | API security headers, bounded JSON bodies, Pages CSP/headers, deployment probe. | Deploy and check real login/private media under the production CSP. |
| #79 | Read-only recovery verifier and isolated restore procedure. | Confirm actual Neon history and run production PITR/media drill with project access. |
| #80 | Deck checker and generated review artifact added to CI. | CI run on the PR. |
| #81 | Opt-in API/database/SPA monitor and notification-delivery test workflow. | Owner configures notifications and proves delivery before enabling the schedule. |
| #82 | Approved O-23/spec v0.4 policy, privileged dry-run/batch CLI, anonymization and migration 00029. | First production run remains an explicitly authorized operation. |
| #85 | Publish errors, locations, jumps, warnings and immediate outline diagnostics. | Included in validation below. |
| #87–#88 | Already implemented in released baseline; retained and regression-checked. | No duplicate redesign. |
| #89 | Existing switch accessible names reproduced in Chromium; no defect found. | No speculative change to native labels. |
| #90 | Closing-soon/waiting/active context; open+scheduled list and correct distinct-student counts. | Included in validation below. |
| #91 | Contextual papers back label; existing answer metadata retained. | Included in validation below. |
| #92 | Integrity events capture the current question at event time. | Included in validation below. |
| #93 | Stable Vietnamese name order and de-duplicated enabled recipient union across paginated classes and individuals. | Included in validation below. |
| #95 | All 62 source-review rows accounted for below, including duplicates and two explicit design decisions. | Device-specific QA remains distinct from Chromium checks. |
| #96 | Shared mockup type scale, line heights and button spacing. | Physical-device font rendering not measured. |

## Student defects found beyond the issue text

- Clicking an answer disabled navigation shortcuts because radios were treated
  as text inputs. Arrow/A–D/F shortcuts now work after a selection, while typing,
  sliders, composition and dialogs keep their own keys.
- The fill-blank Markdown component remounted per keystroke, losing focus after
  one character. A stable renderer keeps focus and ordinary caret editing.
- Submission could overtake autosave or continue after its failure. Submission
  now waits for outstanding and newer edits; stale session responses cannot
  clear another attempt's answers.
- Unsaved answers have a student/session/deadline-scoped local draft. Failed
  saves remain visible; navigation retries saving, reload restores pending edits,
  and sign-out/confirmed save/closed or superseded sessions clear drafts.

## UI decisions

Keep the mockup's neutral visual system and phone/desktop layouts. Improve the
actual input flow: 44px primary touch targets, 16px phone text inputs, flag beside
the prompt, question focus/scroll after navigation, and save status on review.
At 1024px the intro avoids two cramped rules columns. These changes are in the
spec; they do not introduce a second design system.

## Complete #95 ledger

Row numbers refer to the baseline audit, which preserves the original claims.
“Already correct” rows were checked, not counted as newly fixed.

| Rows | Boards | Result |
|---|---|---|
| 01–05 | A-01 | Compact date, weekday subtitle, calm flag copy, waiting-student/age context and closing-soon count. Dashboard repository regression covers partial grading and retakes. |
| 06–08 | A-03/A-03a | Short archive tab and URL-backed status selection, including browser navigation. |
| 09 | A-04a | Keyboard-accessible remove tooltip in the outline. |
| 10,15 | A-05 | File icon/name, status and percentage, icon cancel, compact retry. |
| 11–14 | A-05 | Top-level publish violations survive parsing; blocking rows have location and question/section jumps, warning section and disabled Publish action. |
| 16–19,23 | A-06/A-06a/B-04 | Type facet, sidebar copy, open-menu background and semantic destructive ink. |
| 20–22 | A-07 | Upload copy, persistent immutable-media note and plain unused state. |
| 24,50,59 | B-04/S-03/S-10 | Already correct: student navigation uses Lớp. |
| 25–26 | E-01/E-04 | Responsive auth spacing and error-action typography/touch targets. |
| 27–28 | F-01/F-02 | Neutral notification dot. |
| 29 | F-04 | Blank placeholders become underscores in the question-bank list. |
| 30–31 | F-05 | Shared 13px small type and consistent button padding. |
| 32 | F-09 | Muted unanswered navigator state; 44px phone targets. |
| 33 | F-10 | PageAside becomes an edge sheet below the desktop breakpoint. |
| 34 | G-02b | Compact shared confirmation dialog. |
| 35–36 | G-04 | Inline name-reveal action and keyboard hint under the answers column. |
| 37–38 | G-05 | Integrity tab hides grading actions; private-note example copy. |
| 39,44 | G-06/G-10 | Vietnamese weekday casing and lowercase relative-day labels. |
| 40–41 | G-07 | Join-code provenance badge and concise submitted tile. |
| 42–43 | G-09 | Actual outstanding manual questions and state-aware review hint. |
| 45 | G-11 | Decision: each roster row offers View student; reset/void remain conditional on an attempt. |
| 46–48 | G-11 | Compact submission time, dense search and contextual back label. |
| 49 | S-01 | Join-code typography remains large at both breakpoints. |
| 51–52 | S-04 | Copy/paste timing is explicit. Decision: remove duplicate attempts rule while retaining allowance and best-score disclosure. |
| 53–55,62 | S-05/S-12 | Existing counter/word-count/matching-hint parity retained. Prompt flag no longer squeezes answer options; fill-blank focus bug fixed. |
| 56 | S-06 | Server-calculated remaining retakes and visible save state on review. |
| 57–58 | S-08 | Existing desktop section/meta/inline navigation retained and exercised at 1024 and 1440px. |
| 60–61 | S-10 | Existing profile block retained on phone and desktop. |

## Validation

- Full web unit/integration run: 667/667 tests, 117 files, one worker.
- Full Go unit, database integration and API end-to-end suites passed. Latest
  retention and HTTP middleware changes were rerun in their own packages.
- Contract lint, 26 structural assertions, Go/TypeScript generation and generated
  package test passed.
- Production-build Chromium: 19/19, including student interaction at 360, 1024
  and 1440px. Screenshots are in `2026-09-22-evidence/fill-blank-*.png`.
- Deck check: all seven sheets clean; single-file artifact built successfully.
- Availability probe passed against the local API, database and SPA.

- Live API browser scenarios: all seven passed (six in the full run, then the
  persistence case rerun after updating its assertion for the newly answered
  manual question). Reload during failed saves restored the entire draft and
  submitted it; the result correctly showed 2/4 provisional points and one
  manual answer waiting for grading. No test was skipped.
- Direct browser walk: home/classes at phone and desktop, intro/results at
  360/1024px, settings at 360/1440px. No horizontal overflow; inputs measured
  16px on phone and 13px on desktop. Result filtering worked. Engine review and
  submission are covered by the three-width interaction test and live tests.
- Local PG18 logical-backup drill on 2026-09-22, completed at 11:02 +07: restored
  to a new isolated database, compared all verifier fields, rolled migration
  00029 down/up, and compared again. Both comparisons passed. Checkpoint counts:
  users 54, versions 35, questions 49, attempts 25, answers 16, events 29, audit
  rows 98. Migration 29; zero unvalidated constraints; app cannot rewrite logs.
  The unrelated `zz_review` scratch schema was excluded because the migrate
  role does not own it. The temporary recovery database was removed after
  validation. This did not exercise Neon PITR or private-object restoration.

- `EXPLAIN (ANALYZE, BUFFERS)` captured for dashboard summary, nearest closing
  assignment, assignment counts, attempt tally, retention candidates and the
  retention batch (rolled back). See `2026-09-22-evidence/query-plans.txt`.
  Observed execution times were 0.039–1.016ms on the small local dataset;
  these are review evidence, not the deferred volume/p95 benchmark.

Final lint/CI results are recorded below when complete.

## Resource discipline

After the desktop crash, checks ran sequentially with one web/browser worker,
Go concurrency limited to two CPUs and one package build, and Node heap capped
at 1 GiB. A local process-group guard refuses a heavy run below 2.5 GiB available
RAM and stops its own task below 2 GiB; unrelated applications are untouched.
The guard stopped one redundant E2E rebuild when host memory dipped. The
remaining browser check reused the finished build and passed. Playwright starts
its web server in a separate process group, so the guard was extended to track
owned descendants as well; the leftover preview was explicitly cleaned up.
