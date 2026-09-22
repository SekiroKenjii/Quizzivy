# Open issue audit — 2026-09-22

Repository: [SekiroKenjii/Quizzivy](https://github.com/SekiroKenjii/Quizzivy). All **15 open issues** and their comments were retrieved with authenticated `gh`, and the list was rechecked at the end. This is an investigation, not an implementation or a GitHub triage mutation.

Audited `develop` at `4f32d63f13509c761777eb400765690b063e92d3`. GitHub's branch head matches the local checkout. `main` is `4b84de3bf468fcb4b0f477caacbf47071b18f2d3`; both have tree `139608245e61f2997c1638c0780abd6e70e5110a`, so the reviewed source is identical. [Latest develop CI](https://github.com/SekiroKenjii/Quizzivy/actions/runs/35625232022), [main CI](https://github.com/SekiroKenjii/Quizzivy/actions/runs/35625026369), and [production deployment](https://github.com/SekiroKenjii/Quizzivy/actions/runs/35625494698) succeeded. Deployment success is supporting provenance, not proof of every UI behavior.

## Disposition of every issue

| Issue | Verdict | Recommended disposition |
|---|---|---|
| [#78](https://github.com/SekiroKenjii/Quizzivy/issues/78) — headers/body cap | Confirmed; production headers partially differ from the original report | Keep open. Prioritize body cap and CSP/HSTS; correct the CSP assumptions below. |
| [#79](https://github.com/SekiroKenjii/Quizzivy/issues/79) — backups | Repository evidence missing; actual Neon project state unverified | Keep open pending project-settings evidence and an isolated restore drill. |
| [#80](https://github.com/SekiroKenjii/Quizzivy/issues/80) — deck CI | Confirmed | Keep open; checker works but CI never invokes it. |
| [#81](https://github.com/SekiroKenjii/Quizzivy/issues/81) — outage notification | No documented decision/configuration; external monitors unverified | Keep open. Correct the false claim that Fly health checks restart machines. |
| [#82](https://github.com/SekiroKenjii/Quizzivy/issues/82) — retention/erasure | Decision still unrecorded; proposed implementation needs revision | Keep open as a policy decision, then derive implementation issues. |
| [#85](https://github.com/SekiroKenjii/Quizzivy/issues/85) — publish violations | Reproduced; primary bug and secondary gaps remain | Keep open; prioritize before visual polish. |
| [#87](https://github.com/SekiroKenjii/Quizzivy/issues/87) — desktop engine | Resolved in current source | Closure recommended with Phase 4.5 evidence. |
| [#88](https://github.com/SekiroKenjii/Quizzivy/issues/88) — sections/instructions | Resolved through contract, backend, UI | Closure recommended with Phase 4.5 evidence. |
| [#89](https://github.com/SekiroKenjii/Quizzivy/issues/89) — unnamed switches | Contradicted by current rendered DOM and Chromium | Closure as not reproducible recommended; require a specific browser/AT reproduction if retained. |
| [#90](https://github.com/SekiroKenjii/Quizzivy/issues/90) — dashboard | Confirmed; one diagnosis is wrong | Keep open; fix list query as well as dashboard read model. |
| [#91](https://github.com/SekiroKenjii/Quizzivy/issues/91) — take-test copy/back | Two of three items resolved | Narrow to the papers back-label only. |
| [#92](https://github.com/SekiroKenjii/Quizzivy/issues/92) — integrity question context | Reproduced | Keep open; attach the event-time current question to non-audio client events. |
| [#93](https://github.com/SekiroKenjii/Quizzivy/issues/93) — papers order/roster | Both claims confirmed | Keep open; sort papers separately and compute actual recipient union. |
| [#95](https://github.com/SekiroKenjii/Quizzivy/issues/95) — 62 deck rows | Mixed and duplicated | 11 resolved rows, 49 remaining deviation rows (including 4 duplicate rows), 2 design decisions. See full ledger. |
| [#96](https://github.com/SekiroKenjii/Quizzivy/issues/96) — typography | Confirmed by built-CSS measurement | Keep open; choose the shared scale before per-component polish. |

## Verification performed

- **356 frontend tests / 56 files passed** across assignments, attempts, builder, take-test, integrity, student, settings sections, dashboard, tokens, contract, and date formatting.
- **Four Go test packages passed**: attempts/domain, tests/domain, core/router, platform/httpx. These were unit tests; no database integration or live end-to-end test was run locally in this audit.
- `pnpm typecheck` passed. A production Vite build to `/tmp/quizzivy-audit-dist` passed.
- `node docs/design/mockups/check.mjs` passed on every sheet; this does not establish CI integration.
- Two additional form tests passed, including checking all seven real assignment switches by accessible name.
- Two local router probes confirmed missing headers and a 2,097,214-byte valid JSON request reaching an unconfigured handler (501), rather than being rejected as 413.
- Exploratory probes: three assertions passed (#85 loss of violations, #89 implicit labeling, #92 missing question context); the lowercase `hôm qua` hypothesis failed with actual `Hôm qua`, confirming #95 row 44. This diagnostic failure is preserved, not described as a green regression test. Its subsequent weekday assertion did not execute. A separate Node invocation of the installed date-fns locale returned `08:00 · Thứ Ba, 22/09`, independently confirming weekday casing.
- Chromium 151.0.7922.34 independently read seven switch names from the actual React-rendered form DOM and measured app/deck CSS. The browser check used that serialized DOM, not a live authenticated production session; no full screen-reader test was performed.
- Production received only read-only GET requests to API `/healthz` and the SPA `/`. No oversized request, outage simulation, restore, deletion, or account change was performed against production.

Evidence and the exact diagnostic test sources are retained in [2026-09-22-evidence](2026-09-22-evidence/). They are archived outside test discovery. No existing test was changed or removed. Most #95 rows are source/deck comparisons, not fresh pixel screenshots of every state.

## Findings and corrections

### #78 — security headers and JSON cap

[router.go:74](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/server/internal/core/router/router.go#L74) adds request IDs, logging and CORS but no hardening middleware or body limit. [validate.go:25](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/server/internal/platform/httpx/validate.go#L25) validates the JSON schema; it does not replace a byte cap. The local probe sends valid login JSON padded with 2 MiB of whitespace: schema validation succeeds and the request reaches the unconfigured login handler with **501**, not **413**. This is a real missing generic limit, not merely an absent symbol found by grep.

Read-only production observations:

| Header | `api.quizzivy.com/healthz` | `app.quizzivy.com/` |
|---|---|---|
| HTTP status | 200; database ok | 200 |
| HSTS | Absent | Absent |
| CSP | Absent | Absent |
| X-Content-Type-Options | Absent | nosniff |
| Referrer-Policy | Absent | strict-origin-when-cross-origin |
| Permissions-Policy | Absent | Absent |

`web/public/_headers` is absent, and deploy has no `_headers` artifact preflight. Authenticated JSON cache behavior was not checked on production. A few local endpoints already set no-store, so the issue should not imply that every response lacks cache controls.

Corrections to its acceptance criteria:

- The app intentionally does **not** load GIS (`accounts.google.com/gsi/client`); Google authorization uses a PKCE popup. Do not grant script/frame access merely by copying the issue's old GIS assumption. Derive required origins from the actual auth flow.
- [probe.ts:17](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/media/probe.ts#L17) probes uploads with a blob URL: a restrictive media CSP needs to account for `blob:`. The audio progress bar and engine safe-area padding use inline styles; removing inline-style permission solely because Tailwind emits static CSS would break real UI.
- HSTS alone does not protect a browser's first unprotected visit; the issue's rationale overstates this. See [MDN HSTS](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Strict-Transport-Security). Preloading is a separate operational decision.

### #79 — backup evidence, not a proven absence of backups

`docs/setup/deploy.md` contains no retention window or restore-drill record; [50-nfr-registry.md:78](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/docs/plan/50-nfr-registry.md#L78) still marks this unknown. No authenticated Neon control-plane access was established during this audit, so the actual configured window and existence of an out-of-repo drill remain **unknown**.

Correct two assumptions before implementing the checklist. R2 objects are durable application state too; a database restore does not restore deleted media objects. PITR history and keeping a term's student records are different requirements: seven days of restore history is not seven days of application data retention. Also, `seed/99-assert.sql` checks seed-specific question validity, including bank drafts. It is not sufficient proof of production recovery and can reject a legitimate unfinished draft. A useful drill needs a known recovery timestamp, expected records, migration compatibility, and media-reference checks on an isolated branch. Do not increase a billable retention setting as a side effect of this audit.

### #80 — deck check missing from CI

The checker passes locally. [ci.yml:32](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/.github/workflows/ci.yml#L32) defines the pipeline but contains neither `check.mjs` nor `build-artifact.mjs`, and no Deck job. [50-nfr-registry.md:153](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/docs/plan/50-nfr-registry.md#L153) still records the gap. The issue is valid. Branch-protection required-check settings were not inspected; adding a job alone would not establish that part of the checklist.

### #81 — no recorded alert decision; Fly premise is false

The issue has no decision comment; the deployment docs and repository do not document a monitor service, recipient, test notification or error aggregator. This does not prove that no external monitor was created elsewhere.

[fly.toml:67](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/fly.toml#L67) polls health, but Fly documents that a failing health check changes routing and **does not automatically stop or restart the machine**. Correct both the issue and NFR-A05 wording. See [Fly health-check documentation](https://fly.io/docs/reference/health-checks/). Do not run the issue's suggested production stop/start just to audit it; notification verification belongs in an approved drill.

### #82 — unresolved retention policy and unsafe shortcut in the proposal

[50-nfr-registry.md:53](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/docs/plan/50-nfr-registry.md#L53) remains unknown; no accepted retention decision or erasure flow was found. The existing scheduled prune is [prune.go:24](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/server/internal/core/jobs/prune.go#L24), covering expired refresh-token families, not attempts/events/accounts. Therefore “nothing is ever deleted” is too broad, while the student-data retention gap is real.

The proposed hard delete is not implementable as written: [00020_create_attempts.sql:8](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/migrations/00020_create_attempts.sql#L8) and [00019_create_assignment_targets.sql:19](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/migrations/00019_create_assignment_targets.sql#L19) restrict user deletion. Events belong to attempts, not directly to users; keeping anonymized attempts does not automatically erase their events. Events and audit rows also deny application-role UPDATE/DELETE. Any retention design must preserve those app-role restrictions and specify a separate authorized maintenance path. Retaining a tombstone plus free-text answers, notes or audit diffs is not automatically full anonymization.

Its legal framing also needs updating before treating it as a compliance requirement: the Ministry of Public Security confirms Law **91/2025/QH15** took effect on **2026-01-01**. A September 2026 policy should not rely only on the issue's 2023-decree summary. This audit does not establish the legal sufficiency of 13 months or any proposed deletion rule. [Official effective-date notice](https://bocongan.gov.vn/chinh-sach-phap-luat/bai-viet/luat-bao-ve-du-lieu-ca-nhan-chinh-thuc-co-hieu-luc-thi-hanh-tu-ngay-01-01-2026-1767186124).

### #85 — publish errors disappear in the client

The contract [openapi.yaml:326](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/api/openapi.yaml#L326) puts `violations` beside `error`. [errors.ts:104](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/api/errors.ts#L104) only carries `error.details`. [TestBuilderPage.tsx:610](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/pages/TestBuilderPage.tsx#L610) then looks for `details.violations`. The diagnostic reproduces the discarded array using a real-shaped 409 response.

The secondary gaps also remain: a single Later button, no warnings block, no section-only jump, no rendered question/section location, and outline problems populated only after a failed publish. The quick reproduction should use an empty test/section; a zero-point question may already be rejected during question saving by earlier contract/database constraints. Preserve structured publish errors, then cover the real error decoder and builder/dialog together. A dialog-only fixture would miss the actual loss.

### #87/#88 — implemented but never closed

Implementation is present on both current branches:

- Contract: [openapi.yaml:885](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/api/openapi.yaml#L885), [openapi.yaml:899](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/api/openapi.yaml#L899), [openapi.yaml:1291](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/api/openapi.yaml#L1291) carry sections/instructions and question sectionId; StudentBlank has caseSensitive.
- Backend: [store.go:258](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/server/internal/modules/attempts/repositories/store.go#L258), [attempts.go:97](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/server/internal/modules/attempts/http/attempts.go#L97), and [deal.go:20](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/server/internal/modules/attempts/domain/deal.go#L20) project frozen sections and keep the deal within sections.
- UI: [EngineHeader.tsx:46](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/EngineHeader.tsx#L46) provides the one-row desktop header; [TakeTestPage.tsx:398](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/pages/TakeTestPage.tsx#L398) limits the footer to phones; [SectionInstructions.tsx:16](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/SectionInstructions.tsx#L16) shows first-question instructions; [Navigator.tsx:46](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/Navigator.tsx#L46) groups navigation.
- Existing tests include desktop navigation/section instructions, grouping, fill-blank matching, and Go section shuffle. These passed in this audit. Student-payload contract checks were included; live payload E2E was not rerun locally.

Relevant commits: `8de9a12` (contract), `458b40c` (engine), `7839a6a` and `8b242a8` (follow-up parity), release `0dc1c25`. The Phase 4.5 plan explicitly lists closing #87/#88 as release exit work. Closure is warranted without another implementation branch.

### #89 — false positive on the current implementation

[AssignmentFormPage.tsx:668](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/assignments/pages/AssignmentFormPage.tsx#L668) wraps the actual Radix switch button in a native label. Native buttons can receive their name from an associated label; an empty button subtree does not invalidate that mechanism. [HTML-AAM naming rules](https://www.w3.org/TR/html-aam/#button-accessible-name-computation) describe this association.

Both the real AssignmentFormPage test and Chromium expose:

```text
switch "Trộn thứ tự câu hỏi trong từng phần"
switch "Trộn thứ tự lựa chọn"
switch "Xem điểm"
switch "Xem đáp án đúng"
switch "Xem giải thích"
switch "Bắt buộc toàn màn hình"
switch "Chặn sao chép / dán"
```

Changing to explicit id/htmlFor may be a stylistic improvement, but the reported seven unnamed controls are not established. No Safari/VoiceOver or Firefox/NVDA session was run; if the report was specific to one of those, retain that narrower claim only with a reproduction.

### #90 — genuine dashboard gaps; wrong query identified in issue

The third tile is still openAssignments; the waiting hint and flagged hint are static; the date line uses formatDateTime; activeStudents has no total denominator. These agree with the issue and deck mismatch.

The table is **not** fed by GET /admin/dashboard as its final paragraph suggests. [AdminDashboardPage.tsx:60](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/app/pages/AdminDashboardPage.tsx#L60) calls `listAssignments({ limit: 10 })` with no status filter despite the query key containing “open.” [AdminDashboardPage.tsx:293](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/app/pages/AdminDashboardPage.tsx#L293) renders a progress fraction for every state. Fix filtering before pagination at the list request/server query boundary, so filtering only ten mixed rows client-side does not omit legitimate open assignments. Define “closing soon” (time window) explicitly before adding its count; the deck example of three hours is not a complete counting rule.

### #91 — only the papers back label remains

Matching-rule hints now render conditionally from caseSensitive, and the short-answer footer is a shared row. Tests cover both behaviors. [PageHeader.tsx:44](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/shared/PageHeader.tsx#L44) still hard-codes the generic label for [AssignmentAttemptsPage.tsx:125](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/attempts/pages/AssignmentAttemptsPage.tsx#L125). Narrow the issue to a page-specific “Về bài giao” label. The same remainder appears in #95 row 48.

### #92 — missing event-time question remains reproducible

[useIntegrityMonitor.ts:48](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/integrity/useIntegrityMonitor.ts#L48) records kind/meta only, while recordAudioEvent supplies questionId. Dispatching an offline event through the real hook produces a network_offline record with no questionId. The contract and timeline already accept/render the field.

Capture the current question at the moment an event is recorded, not later when the buffer is flushed. Handle review/submitted screens deliberately rather than inventing a question there. Historical records without questionId cannot be reliably reconstructed. The issue's “every event except audio” should be narrowed to the non-audio client signals from this hook; its own screenshot includes a start event with a question.

### #93 — both discrepancies still exist

[monitor.go:76](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/server/internal/modules/attempts/repositories/monitor.go#L76) sorts monitor rows by state and deadline before name. [AssignmentAttemptsPage.tsx:105](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/attempts/pages/AssignmentAttemptsPage.tsx#L105) preserves that order for the papers screen. Re-sort a copy in the papers view using the chosen Vietnamese roster rule and a stable tie-break; do not mutate the shared monitor cache.

[AssignmentFormPage.tsx:241](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/assignments/pages/AssignmentFormPage.tsx#L241) is still constant. The selections contain class summaries plus explicitly selected students, which is not enough to subtract overlapping class memberships. Summing studentCount values would be wrong when classes overlap. Use actual member IDs or a server-side recipient-preview count; retain the server's union semantics.

### #96 — typography mismatch measured, including line heights

Chromium measurement of the current production-build CSS against kit.css at a 16px root:

| Utility | App font / line height | Deck font / line height |
|---|---|---|
| text-xs | 12 / 16px | 12 / 16.8px |
| text-sm | 14 / 20px | 13 / 19.5px |
| text-base | 16 / 24px | 14 / 21px |
| text-lg | 18 / 28px | 17 / 24.65px |
| text-xl | 20 / 28px | 20 / 27px |

tracking-tight is -0.025em in the app and -0.015em in the kit. **xs/xl only agree in font size, not line height.** [label.tsx:14](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/ui/label.tsx#L14) separately overrides label leading; changing text-sm alone will not fix that comment's concern. [index.css:59](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/index.css#L59) defines no text scale. Keep #96 as the shared decision; #95 row 30 should follow it rather than independently hard-code a button font size. Recheck student reading screens after the chosen scale changes.

## #95 — complete 62-row ledger

Rows are numbered in the order of the original issue table, starting with the dashboard closing-date row. **Resolved** is implementation evidence, not a new pixel-perfect certification. **Open** means the described source/deck difference remains. **Design decision** means a visible difference exists but the proposed change lacks a complete behavior requirement.

There are **11 resolved rows**: 24, 50, 53–55, 57–62. The **51 other rows** include **4 duplicate rows** (8→7, 14→11, 23→17, 28→27) and **2 design decisions** (45, 52): **47 distinct remaining groups**, not 62 independent bugs. Rows 2/4/5 overlap #90; 11/12/13/14 overlap #85; 48 overlaps the remainder of #91; 30 overlaps #96. Group their fixes to avoid doing the same work twice.

| Row | Board | Verdict | Current evidence / correction |
|---|---|---|---|
| 01 | A-01 | Open | Dashboard closing dates remain the long comma/year format. [AdminDashboardPage.tsx:314](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/app/pages/AdminDashboardPage.tsx#L314) |
| 02 | A-01 | Open · #90 | Dashboard subtitle still uses a clock/date, not weekday/date. [AdminDashboardPage.tsx:70](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/app/pages/AdminDashboardPage.tsx#L70) |
| 03 | A-01 | Open | Flagged hint still begins with a capital and ends with a period. [vi.json:727](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/locales/vi.json#L727) |
| 04 | A-01 | Open · #90 | Waiting hint is static; no distinct-student/oldest-wait context. [AdminDashboardPage.tsx:121](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/app/pages/AdminDashboardPage.tsx#L121) |
| 05 | A-01 | Open · #90 | Third tile still counts open assignments, not closing-soon assignments. [AdminDashboardPage.tsx:133](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/app/pages/AdminDashboardPage.tsx#L133) |
| 06 | A-03 | Open | Archived tab still shares the longer row-status translation. [TestsListPage.tsx:184](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/pages/TestsListPage.tsx#L184) |
| 07 | A-03a | Open | Tests status starts at all; the status URL parameter is not read. [TestsListPage.tsx:75](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/pages/TestsListPage.tsx#L75) |
| 08 | A-03a | Duplicate of 7 | Same status URL state problem; do not schedule a second fix. [TestsListPage.tsx:75](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/pages/TestsListPage.tsx#L75) |
| 09 | A-04a | Open | Remove question has aria-label but no tooltip/title or tooltip primitive. [OutlineTree.tsx:614](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/components/OutlineTree.tsx#L614) |
| 10 | A-05 | Open | Upload state still shows progress + percentage + outline cancel, without file glyph/status line. [UploadPanel.tsx:114](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/media/components/UploadPanel.tsx#L114) |
| 11 | A-05 | Open · #85 | Publish footer still has only the full-width Later button. [PublishDialog.tsx:70](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/components/PublishDialog.tsx#L70) |
| 12 | A-05 | Open · #85 | No non-blocking warning input/rendering or warning contract. [PublishDialog.tsx:14](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/components/PublishDialog.tsx#L14) |
| 13 | A-05 | Open · #85 | Jump button still requires questionId; section-only violations cannot jump. [PublishDialog.tsx:45](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/components/PublishDialog.tsx#L45) |
| 14 | A-05 | Duplicate of 11 | Same missing disabled Publish button. [PublishDialog.tsx:70](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/tests/components/PublishDialog.tsx#L70) |
| 15 | A-05 | Open | Rejected-file retry still uses sm (32px), not xs (28px). [UploadPanel.tsx:158](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/media/components/UploadPanel.tsx#L158) |
| 16 | A-06 | Open | Question-type facet still includes an additional All row. [QuestionBankPage.tsx:499](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/question-bank/pages/QuestionBankPage.tsx#L499) |
| 17 | A-06 | Open | Sidebar close label remains Đóng menu. [vi.json:71](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/locales/vi.json#L71) |
| 18 | A-06a | Open | Row-menu trigger has no persistent open-state background. [RowMenu.tsx:23](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/shared/RowMenu.tsx#L23) |
| 19 | A-06a | Open | Destructive menu ink still uses the raw destructive token; no ink token defined. [dropdown-menu.tsx:51](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/ui/dropdown-menu.tsx#L51) |
| 20 | A-07 | Open | Media upload action still says Tải tệp lên. [vi.json:431](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/locales/vi.json#L431) |
| 21 | A-07 | Open | Media no-overwrite/delete note is inside the blocked-delete dialog only. [MediaLibraryPage.tsx:210](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/media/pages/MediaLibraryPage.tsx#L210) |
| 22 | A-07 | Open | Unused media still gets a Badge rather than a plain muted cell. [MediaLibraryPage.tsx:304](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/media/pages/MediaLibraryPage.tsx#L304) |
| 23 | B-04 | Duplicate of 17 | Same sidebar label. [vi.json:71](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/locales/vi.json#L71) |
| 24 | B-04 | Resolved | Student shell uses classesNav = Lớp on both widths. [StudentLayout.tsx:50](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/layouts/StudentLayout.tsx#L50) |
| 25 | E-01 | Open | Auth/error card remains p-5 at all widths. [AuthLayout.tsx:42](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/auth/AuthLayout.tsx#L42) |
| 26 | E-04 | Open | ErrorActions changes height only; it does not apply the lg button typography/padding. [ErrorScreen.tsx:39](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/app/pages/ErrorScreen.tsx#L39) |
| 27 | F-01 | Open | Notification dot remains destructive red. [NotificationsButton.tsx:43](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/dashboard/NotificationsButton.tsx#L43) |
| 28 | F-02 | Duplicate of 27 | Same notification dot. [NotificationsButton.tsx:43](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/dashboard/NotificationsButton.tsx#L43) |
| 29 | F-04 | Open | Question-bank title still renders question.prompt verbatim, including blank placeholders. [QuestionBankPage.tsx:410](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/question-bank/pages/QuestionBankPage.tsx#L410) |
| 30 | F-05 | Open · #96 | Small buttons inherit 14px text-sm; kit small buttons are 13px. [button.tsx:24](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/ui/button.tsx#L24) |
| 31 | F-05 | Open | Icon-dependent has-[>svg] padding overrides remain. [button.tsx:22](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/ui/button.tsx#L22) |
| 32 | F-09 | Open | Untouched navigator dots still lack muted-foreground. [Navigator.tsx:101](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/Navigator.tsx#L101) |
| 33 | F-10 | Open | Narrow PageAside still uses centered DialogContent. [PageAside.tsx:61](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/shared/PageAside.tsx#L61) |
| 34 | G-02b | Open | Shared confirm still inherits p-6 and sm:max-w-lg; intervention deck uses a smaller dialog. [dialog.tsx:61](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/ui/dialog.tsx#L61) |
| 35 | G-04 | Open | Reveal names remains a bordered button outside the notice sentence. [GradeByQuestion.tsx:192](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/attempts/components/GradeByQuestion.tsx#L192) |
| 36 | G-04 | Open | Grading keyboard hint still sits in the rubric column. [GradeByQuestion.tsx:210](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/attempts/components/GradeByQuestion.tsx#L210) |
| 37 | G-05 | Open | Finish grading is still built into actions for both tabs. [AttemptReviewPage.tsx:224](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/attempts/pages/AttemptReviewPage.tsx#L224) |
| 38 | G-05 | Open | Private-note placeholder still omits the example prefix and period. [vi.json:1380](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/locales/vi.json#L1380) |
| 39 | G-06 | Open | Weekday formatter keeps the date-fns title-cased second word. [datetime.ts:31](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/datetime.ts#L31) |
| 40 | G-07 | Open | Drawer header only renders provider/disabled badges; join-source information is lower in class rows. [StudentDrawer.tsx:267](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/students/components/StudentDrawer.tsx#L267) |
| 41 | G-07 | Open | Drawer submitted tile still reuses students.submitted. [StudentDrawer.tsx:130](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/students/components/StudentDrawer.tsx#L130) |
| 42 | G-09 | Open; fix proposal needs correction | Pending hint is per-paper manualCount. A total should sum outstanding manual questions, not simply multiply papers by manualCount. [AssignmentDetailPage.tsx:455](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/assignments/pages/AssignmentDetailPage.tsx#L455) |
| 43 | G-09 | Open | Review hint still branches only on showCorrectAnswers, not assignment state. [AssignmentDetailPage.tsx:722](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/assignments/pages/AssignmentDetailPage.tsx#L722) |
| 44 | G-10 | Open · reproduced | Current runtime returns Hôm qua; deck uses hôm qua. [datetime.ts:88](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/datetime.ts#L88) |
| 45 | G-11 | Design decision | No-attempt row has no menu. Deck draws a kebab, but issue names no valid action; decide its behavior before adding a control. [AssignmentAttemptsPage.tsx:323](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/attempts/pages/AssignmentAttemptsPage.tsx#L323) |
| 46 | G-11 | Open | Papers submitted-at still uses the long formatDateTime. [AssignmentAttemptsPage.tsx:293](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/attempts/pages/AssignmentAttemptsPage.tsx#L293) |
| 47 | G-11 | Open | Papers uses shared SearchInput defaults: 36px, md:text-sm, 16px search icon. [SearchInput.tsx:25](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/shared/SearchInput.tsx#L25) |
| 48 | G-11 | Open · #91 | Back control still uses generic common.back; no page-specific backLabel. [PageHeader.tsx:44](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/components/shared/PageHeader.tsx#L44) |
| 49 | S-01 | Open | Join input moved to JoinCodeForm; text-lg still loses to Input md:text-sm. [JoinCodeForm.tsx:45](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/join/components/JoinCodeForm.tsx#L45) |
| 50 | S-03 | Resolved · same as 24 | Student nav now uses Lớp. [StudentLayout.tsx:50](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/layouts/StudentLayout.tsx#L50) |
| 51 | S-04 | Open | Copy/paste rule still omits trong lúc làm bài. [vi.json:840](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/lib/i18n/locales/vi.json#L840) |
| 52 | S-04 | Design decision | Extra attempts rule remains only when maxAttempts > 1. This is accurate policy disclosure, not an unconditional extra row; decide whether duplicate intro copy should be removed. [studentRules.ts:55](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/assignments/studentRules.ts#L55) |
| 53 | S-05 | Resolved | Phone body has no second counter; flag is beside the question. [TakeTestPage.tsx:363](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/pages/TakeTestPage.tsx#L363) |
| 54 | S-05 | Resolved · #91 | Short-answer worth and word count share a flex row. [QuestionBody.tsx:205](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/QuestionBody.tsx#L205) |
| 55 | S-05 | Resolved · #91 | Fill-blank now displays a case-sensitive/insensitive matching rule. [QuestionCard.tsx:46](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/QuestionCard.tsx#L46) |
| 56 | S-06 | Open | Review footnote still contains only submitNote; remaining retakes are not supplied. [ReviewScreen.tsx:163](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/ReviewScreen.tsx#L163) |
| 57 | S-08 | Resolved · #87/#88 | Wide meta includes section title, position and worth. [TakeTestPage.tsx:440](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/pages/TakeTestPage.tsx#L440) |
| 58 | S-08 | Resolved · #87 | Wide navigation is inline; footer is rendered only when !wide. [TakeTestPage.tsx:398](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/pages/TakeTestPage.tsx#L398) |
| 59 | S-10 | Resolved · same as 24 | Student nav now uses Lớp. [StudentLayout.tsx:50](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/layouts/StudentLayout.tsx#L50) |
| 60 | S-10 | Resolved | ProfileSection mounts on phone and desktop after the #94 decision. [StudentSettingsPage.tsx:38](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/auth/pages/StudentSettingsPage.tsx#L38) |
| 61 | S-10 | Resolved · duplicate of 60 | Same profile block; both render branches now include it. [StudentSettingsPage.tsx:38](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/auth/pages/StudentSettingsPage.tsx#L38) |
| 62 | S-12 | Resolved · duplicate of 55 | Same fill-blank matching hint on the narrow board. [QuestionCard.tsx:46](https://github.com/SekiroKenjii/Quizzivy/blob/4f32d63f13509c761777eb400765690b063e92d3/web/src/features/take-test/components/QuestionCard.tsx#L46) |

## Work order after triage

1. Close or narrow stale reports: #87, #88, #89, #91; remove resolved/duplicate rows from #95.
2. Fix behavioral gaps #85 and #92, and hardening #78. For #78 retain separate upload limits and an explicit 413 translation before schema buffering.
3. Obtain the external evidence/decisions for #79, #81, #82. They are not completed merely because repository tests are green.
4. Address #90/#93 data correctness and #80 CI coverage.
5. Settle #96's shared typography decision, then polish the remaining #95 groups with targeted visual checks. Do not implement its inaccurate pending-question multiplication or add a nonfunctional no-attempt menu merely to resemble a static fixture.

## Reproduction notes

Run from the repository root unless noted:

```sh
node docs/design/mockups/check.mjs
cd web
pnpm typecheck
pnpm vitest run --maxWorkers=2 tests/units/assignments tests/units/attempts tests/units/builder tests/units/take-test tests/units/integrity tests/units/student tests/units/auth/settings-sections.test.tsx tests/units/dashboard tests/units/styles tests/units/contract tests/units/i18n/datetime.test.ts
pnpm exec vite build --outDir /tmp/quizzivy-audit-dist
```

From `server/`:

```sh
GOCACHE=/tmp/quizzivy-audit-go-cache go test ./internal/modules/attempts/domain/tests ./internal/modules/tests/domain/tests ./internal/core/router/tests ./internal/platform/httpx/tests
```

To reproduce the extra probes, copy the archived `.txt` sources back to these temporary locations and run only those files/test names:

- `exploratory-probes.test.tsx.txt` → `web/tests/units/issue-audit-probes.test.tsx`
- `assignment-form.test.tsx.txt` → `web/tests/integration/issue-audit-form.test.tsx`
- `router-probes_test.go.txt` → `server/internal/core/router/tests/issue_audit_test.go` (`go test ./internal/core/router/tests -run TestIssueAudit -v`)

The exploratory casing assertion is intentionally the original hypothesis and currently fails. These probes observe existing behavior; they are not proposed regression tests pinning the bugs as desired behavior.
