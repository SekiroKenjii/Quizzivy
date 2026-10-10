# 70 — Redesign overview (Phase R)

The programme that rebuilds Quizzivy to the new design deck (`docs/design/deck/`), splits the
Admin role from the Teacher role, makes every resource owned by a teacher, and builds every
capability the deck draws. Release files are `71-r1.md` … `81-r11.md`, with
`74d-d4-deck-update.md` for the deck update between R4 and R5; groundwork (R0) is recorded here.
Design requests are in `docs/design/gaps.md`.

Where this file and a release file disagree, this file wins until the release file is corrected.
Where either disagrees with the spec, the spec wins and the plan is corrected (AGENTS.md).

---

## 1. Decisions (Thuong, 2026-09-26)

| # | Area | Decision |
|---|---|---|
| D1 | Scope | The new deck replaces `docs/design/mockups` and every rule that came with it. Everything it draws is built, including modules the backend lacks. |
| D2 | Roles | Admin and Teacher are separate roles with separate consoles; an Admin can switch to the Teacher workspace. Assistant, custom roles and the permission matrix follow the Admin page. |
| D3 | Ownership | Everything is owned per teacher: classes, tests, questions and question sets, media, courses, word lists, imports. Admin sees everything. |
| D4 | Sharing | An owner shares with one or more teachers, per recipient **Can use** or **Can edit** (the deck's share dialog and "Shared with me"). Recipients cannot share further or delete. Revoking keeps assignments already made working. |
| D5 | Join codes | Stored encrypted (reversible, server key) and always shown in full to their teacher and to admins (copy, QR, link). The §6 alphabet stays. The public preview may add schedule and room, never counts. Every legacy hashed code is **rotated at the R4 release**; the release note and an in-app notice tell teachers to share the new code. |
| D6 | API | Teaching operations move from `/admin/*` to `/teacher/*`; `/admin/*` becomes the Admin console; students stay at `/app/*`. Every operation declares `x-permission` in `api/openapi.yaml`, and one contract-driven middleware enforces it on every request. No `/v1`. |
| D7 | Rollout | Phased releases. Production stays coherent between them: no console is half old, half new. |
| D8 | Integrity | Follow the deck: flags, "Flagged", the red timer under five minutes, "Your teacher has been told", the wizard's defaults. The calm-integrity rule in AGENTS.md and spec §10/§12 is rewritten. |
| D9 | Email | Resend over its HTTP API (`RESEND_API_KEY`), sending domain `quizzivy.com` verified by DNS, open and click tracking off. From R7, Resend's Svix-signed webhook reaches the open operation `POST /public/email-events` (`RESEND_WEBHOOK_SECRET`). Hard bounces and complaints add the address to `app.email_suppressions`, which the dispatcher skips. |
| D10 | Landing | On the apex `quizzivy.com`, from the same web build and Pages project, as a lazy public route at `/`. Signed-in visitors go to their console. The app keeps working at `app.quizzivy.com`. |
| D11 | Copy | We write Vietnamese first and English from the deck (only Landing and Splash carry vi). Thuong reviews the new keys per PR. |
| D12 | Add student | A new matrix row, **Create student accounts** (Admin and Teacher). A teacher creates students into their own classes. |
| D13 | Admin rows | The People and System rows may be granted to any role, under the **subset rule** (§4.3). |
| D14 | Assistant | Attached per class (class staff); hidden from role pickers until the deck draws it (DG-04). |
| D15 | Credentials by email | A one-time set-password link valid 24 hours. A password never sits in an inbox. |
| D16 | Retention | The Admin retention settings drive `cmd/maintenance retention-sweep`, run nightly at 03:00 ICT by the GitHub Actions workflow `retention-sweep.yml` as the `quizzivy_maintenance` role. Its credential lives in the GitHub environment `production-maintenance`, never on Fly. Runs are audited as System, and a scheduled run applies only once `RETENTION_SWEEP_ENABLED` is set after a reviewed dry run. Every `cmd/maintenance` command runs as that role, and a release that adds or widens a command grants exactly what it needs in its own migration. "Delete disabled accounts" anonymises (the O-23 method). Supersedes O-23. |
| D17 | Messages | Direct messages are readable by their participants only. Admins see counts, never contents. |
| D18 | Leads | A consent checkbox and a privacy notice (Decree 13/2023); leads kept 12 months. |

Decided by Thuong on 2026-10-10 for the AI assistant the design team's v2 export draws
(`74d-d4-deck-update.md`, section G):

| # | Area | Decision |
|---|---|---|
| D19 | AI provider | Anthropic's API for every AI feature in v2; cloud processing is approved and enabled. This replaces the spec's "cloud and private AI processing remain candidates … not enabled by default" for these features. The provider sits behind a module port (`core/adapters`, as Google and the audio probe do); with `ANTHROPIC_API_KEY` unset the AI operations answer 501 and the web shows the deck's "AI didn't respond" fallback where it draws one, never a 500. Model choice is configuration, as tiers (a fast tier and a capable tier with vision), never a model identifier in a document. |
| D20 | AI data | Data minimisation, decided per feature: each feature sends exactly what it needs and the plan lists it. Personal identifiers never reach the provider unless a feature cannot work without them (none does), and the plan says so wherever that would happen. |
| D21 | AI budget | The centre has $200 of API credit a month; that is the pool. Use beyond it happens only when an Admin approves extra credit, which Thuong pays for (the deck's "Add credits"). The accounting is a hard stop, never a soft warning: a credit unit mapped to the provider's token cost, metered per call from the reported usage, with per-teacher allowances under the pool. |
| D22 | AI release | Built in D4, the release right after R4, not a release of its own; D4's lanes, tasks and version are reshaped to fit. |

## 2. The release train

| Rel | Version | Name | Plan file |
|---|---|---|---|
| R0 | rides in v0.7.0 | Groundwork: deck imported, docs rewritten, CI, gap register, external setup | this file, §7 |
| R1 | v0.7.0 | Foundations and front door | `71-r1.md` |
| R2 | v0.8.0 | Access: permissions, ownership, `/teacher` rename, encrypted join codes (no screen changes) | `72-r2.md` |
| R3 | v0.9.0 | Student console and the take-test engine | `73-r3.md` |
| R3 | v0.9.1 | R2's contract steps (T-R3.1 to T-R3.3): the legacy role, the ownership constraints and the `/admin` alias. No earlier than 2026-10-10. By default carried inside v0.10.0 as T-R4.49 (2026-10-08; `73-r3.md`, "Migration numbers") | `73-r3.md` |
| R4 | v0.10.0 | Teacher workspace | `74-r4.md` |
| D4 | v0.11.0 (decided 2026-10-10, `74d` Q29) | Deck update: the fourth export (2026-10-04) and v2 (2026-10-10). The take-test engine, the builder's shared content and the three previews, rebuilt to it; the AI assistant (D19 to D22); the four R4 screens v2 redraws | `74d-d4-deck-update.md` |
| R5 | v0.12.0 | Admin console and email; the AI credits page on D4's operations | `75-r5.md` |
| R6 | v0.13.0 | Question types and scoring | `76-r6.md` |
| R7 | v0.14.0 | Collaboration: sharing, messages, email notifications | `77-r7.md` |
| R8 | v0.15.0 | Schedule: sessions, calendar, attendance, terms | `78-r8.md` |
| R9 | v0.16.0 | Insights: gradebook, reports, student grades | `79-r9.md` |
| R10 | v0.17.0 | Learn: vocabulary and flashcards, courses and lessons | `80-r10.md` |
| R11 | v1.0.0 | Landing, leads, and v1.0 hardening | `81-r11.md` |

The release files `75-r5.md` to `80-r10.md` still carry their first-planned version in their
titles; D4's release task (T-D4.18) corrects them to the versions above (Q29, decided 2026-10-10).

**Ordering rules.**

1. Foundations (R1) before any screen is rebuilt. R1's primitives are additive, so old screens
   take the new palette and font without shifting.
2. Access and ownership (R2) before a second teacher can exist. The first UI that creates one is
   R5's Users page; `cmd/seedadmin` must not create teachers before R2 is live.
3. R2 ships alone and changes no screen, so any regression is unambiguously an access bug.
4. R3 and R4 are independent after R2 and may swap.
5. A console never shows a destination whose module has not shipped: nav items, palette entries
   and controls are gated by module availability and permission.
6. Resend lands with its first consumers (R5). The notification platform lands in R4 (in-app)
   and gains its email channel in R7.
7. Schedule before gradebook (attendance feeds it); vocabulary before courses (a lesson can be a
   flashcards lesson).
8. The Landing ships last because its copy markets every module.
9. A capability waiting on the deck lands in the first release that starts after the deck draws
   it. Class staff (D14, DG-04) is the case today: until then every release only hides
   Assistant from role pickers, and if DG-04 is still open at T-R11.14 class staff goes on the
   post-1.0 list (§10).
10. D4 follows R4 and comes before R5 (Thuong, 2026-10-04). R4 builds from the deck of record
    as it stands. The v2 export of 2026-10-10, which carries the fourth export, is imported by
    D4's first task, after v0.10.0, and no R4 task is amended to it while R4 is in flight. The
    AI assistant ships inside D4 (D22); the Admin "AI credits" page, which needs R5's console,
    is R5's T-R5.35 on operations D4 ships (`74d` Q28, decided 2026-10-10); until then the
    owner uses `cmd/maintenance ai-credits`.

## 3. How the work is run

- **Task ids** `T-R<k>.<n>`; branches `feature/t-r<k>-<nn>-<slug>`.
- **Integration branches.** For R1–R11, `work/redesign-r<k>` is cut from `develop`; feature PRs
  target it; `develop` is merged into it at least weekly and before every verification run.
  When every task is ticked, `work/redesign-r<k>` → `develop` (`--no-ff`), then
  `release/0.<x>.0` → `main`. R0 PRs go straight to `develop`. D4 runs the same way under its
  own names: tasks `T-D4.<n>`, branches `feature/t-d4-<nn>-<slug>`, the integration branch
  `work/deck-d4`, and `release/0.10.1`. Merging to `main` deploys, so it waits for Thuong's go
  every time.
- **One task, one branch, one PR**, under ~800 non-generated lines; XL tasks split into lettered
  sub-PRs under one id.
- **Mechanical sweeps are their own commits** with no behaviour inside: path renames, test
  helper swaps, comment sweeps, i18n key renames.
- **Generated code** only from `api/openapi.yaml` via `make gen`, committed as its own commit in
  the same PR. A contract rename carries the web call-site sweep in the same PR, or
  `pnpm typecheck` fails.
- **Expand, then contract one release later.** Deploy runs the API (and its migrations) before
  the web, and old tabs keep the old SPA, so every release's API and schema change is additive.
  A new NOT NULL column on a populated table is added nullable, backfilled, constrained
  `NOT NULL … NOT VALID` with a trigger that fills it for the old binary, and validated one
  release later. Indexes on populated tables go in their own `-- +goose NO TRANSACTION` file
  with `CONCURRENTLY`. An integration test inserts rows the old binary's way.
- **Migrations** are numbered at merge time from `00053`; a later integration branch renumbers
  on rebase. Reference data the app cannot run without (built-in roles, the permission
  catalogue) goes in a migration and is recorded as a deviation in `20-data-model.md` §12.
- **Docs change in the release that changes behaviour**: spec section, AGENTS.md, this plan.
  Never batched at the end.
- **Copy.** Every string through `t()`; vi written first, en from the deck; each PR lists its
  new keys. The ~124 test files that pin Vietnamese copy are rewritten with the screen they
  cover, never in bulk.
- **Machine.** At most four concurrent agents; only the compose services a step needs; stop
  Vite and the API after each verification; `git add` explicit paths only.

## 4. Access model

### 4.1 Permission catalogue

One `PermissionKey` enum in `api/openapi.yaml`; the same rows seeded into `app.permissions`; an
integration test asserts they are equal. Labels and defaults are the deck's
(`Quizzivy Admin.dc.html`, `PERMS`), plus D12's row.

| Key | Matrix row | Group | Admin | Teacher | Assistant | Student |
|---|---|---|---|---|---|---|
| `content.tests.write` | Create and edit tests | Content | ✓ | ✓ | ✓ | |
| `content.tests.publish` | Publish tests | Content | ✓ | ✓ | | |
| `content.questions.write` | Manage question bank | Content | ✓ | ✓ | ✓ | |
| `content.media.write` | Upload and delete media | Content | ✓ | ✓ | | |
| `content.share` | Share own content with other teachers | Content | ✓ | ✓ | | |
| `teaching.classes.write` | Create classes | Teaching | ✓ | ✓ | | |
| `teaching.assignments.write` | Assign tests to classes | Teaching | ✓ | ✓ | ✓ | |
| `teaching.grading` | Grade answers | Teaching | ✓ | ✓ | ✓ | |
| `teaching.attempts.intervene` | Reset, void or extend attempts | Teaching | ✓ | ✓ | | |
| `teaching.attendance` | Take attendance | Teaching | ✓ | ✓ | ✓ | |
| `people.students.read` | See students in own classes | People | ✓ | ✓ | ✓ | |
| `people.students.create` | Create student accounts (D12, DG-03) | People | ✓ | ✓ | | |
| `people.students.reset_password` | Reset student passwords | People | ✓ | ✓ | | |
| `people.users.manage` | Add and disable users | People | ✓ | | | |
| `people.roles.manage` | Change roles | People | ✓ | | | |
| `system.audit.read` | View audit log | System | ✓ | | | |
| `system.settings.write` | Change system settings | System | ✓ | | | |
| `learning.take_tests` | Take tests and view own results | System | toggle | | | ✓ |
| `ai.use` | Use AI features (D19 to D22; D4, T-D4.32) | Content | ✓ | ✓ | | |
| `ai.credits.manage` | Manage AI credits (D21; D4, T-D4.32) | System | ✓ | | | |

The two AI rows are not in the deck's matrix (DG+18 in `74d`, asked of the design team); they are
drawn in the matrix's own style until then. `ai.use` gates every `/teacher/ai/*` operation and the
import's "Read with AI"; `ai.credits.manage` gates `/admin/ai/*` (the pool, allowances, rules and
requests). Neither is a pseudo-key: a role may hold every teaching key and no AI.

- **Admin is a wildcard**: it holds every key, including the hidden ones below, except
  `learning.take_tests`, the one Admin cell the matrix may toggle (DG-51). New keys added by
  later migrations reach Admin automatically.
- **Hidden keys**, never shown in the matrix and never grantable, held only by the Admin
  wildcard: `scope.all`, `system.api_reference`, `system.data_export`, `system.leads`.
  `scope.all` reads and edits any teacher's content, classes and people by id, and powers the
  Admin console's cross-teacher views. In the teacher workspace, content lists (tests, bank,
  media, imports, assignments) show the caller's own rows for everyone, admins included;
  Students and Classes keep `scope.all` lists until R5's Admin Classes and Users take them over
  (DG-53, T-R4.54).
- **Pseudo-keys**, evaluated by the middleware, not stored: `self` (any active signed-in user);
  `workspace.teacher` (holds any `content.*`, `teaching.*` or `people.students.*` key);
  `workspace.admin` (holds `people.users.manage`, `people.roles.manage`, `system.audit.read`,
  `system.settings.write` or `scope.all`).
- **Capabilities the matrix does not draw** (DG-50) map as follows until the deck draws rows:
  imports, from a file or from pasted text → `content.tests.write`; question sets →
  `content.questions.write`; word lists → `content.questions.write`; courses and lessons →
  `content.tests.write`; class sessions and calendar → view `workspace.teacher`, edit
  `teaching.classes.write`; messages → staff `workspace.teacher`, students
  `learning.take_tests`; announcements → `teaching.classes.write`; gradebook and exports →
  `teaching.grading`; reports → `people.students.read`; terms → `system.settings.write`; data
  export → `system.data_export`; leads → `system.leads`; API reference →
  `system.api_reference`; assigning a word list or course to classes →
  `teaching.assignments.write`; word-list stats and course progress → `people.students.read`;
  attendance session lists → `teaching.attendance`.
- **One role per user.** Built-in roles carry an immutable `builtin_key` (admin, teacher,
  assistant, student); their name, description, icon and colour are editable. Custom roles copy
  a role's grants on creation. There is no delete (DG-52).

### 4.2 Enforcement

- Each operation in `api/openapi.yaml` declares `x-permission`: one key, one pseudo-key, or a
  list meaning any of them. A list is used only where one operation serves two matrix rows:
  the question and question-group writes that serve both the bank and the test builder
  (creating, updating and duplicating a question; creating, updating, copying and deleting a
  group: `[content.questions.write, content.tests.write]`) and the attempt-review reads
  (`[teaching.grading, teaching.attempts.intervene]`). The group writes narrow the check under
  the row lock, once the destination or the stored group shows which row applies: the bank
  needs `content.questions.write`, a test section `content.tests.write` (T-R2.12a). Open
  operations, those that do not require the bearer token, declare none, and every key belongs
  to its path's tree (T-R2.7). `router.New` refuses to start on any operation that breaks
  this, and `permissions_test.go`, `testdata/permissions.golden` and the web contract suite
  fail the build.
- **Open operations.** There are six at v0.6.0 (login, Google, refresh, logout,
  `/join/preview`, the integrity beacon). R1 adds `getPublicStatus` (7). R5 adds
  `checkSetPasswordLink`, `setPasswordWithLink`, `getPublicOrganization` and
  `getOrganizationLogo` (11). R7 adds the Resend webhook `POST /public/email-events` (12). R11
  adds `submitLead` (13). Each is rate-limited, leak-reviewed and added to the pinned list in its
  own PR.
- `httpx.RequirePermission` replaces `RequireRole`'s `/admin/` prefix gate. It resolves the
  caller's role, permission set, `disabled_at` and session epoch on every request, from a
  10-second in-process cache keyed by user (10,000 entries, least recently used out first),
  with each role's grants cached by `(role_id, revision)`. A write forgets the entry at once on
  the machine that made it, so a matrix edit, role change or disable applies on that machine's
  next request and within 10 seconds on any other, and Neon is not woken per poll. An unknown
  or disabled user, or a token whose epoch is older than the user's, gets 401; an unmet
  requirement gets 403.
- **Ending access is one rule.** Every password reset (single, bulk, reset-link issue and link
  use), disable, sign-out-everywhere and role change does three things in one command: it
  revokes the user's refresh families, bumps `session_epoch` and calls
  `Principals.Forget(userID)`. `setRolePermissions` and `updateRole` call `ForgetAll`.
- Resource scoping is separate from permission: repositories filter by
  `access.Scope{UserID, All}` (owner, share grants, class membership), with `All` only for
  `scope.all`. Another teacher's id answers exactly as a missing id does, never 403. The
  two-teacher isolation suite (T-R2.16) sends another owner's id into every id an operation
  under `/teacher/*`, `/app/*` or `/me/*` takes, as the contract's `x-resource` names them.

### 4.3 Guards that are not permissions

- **Subset rule (D13):** for editing or deleting an account, reset password, disable, enable,
  sign out everywhere, change role, create user, every user-import row, bulk actions, "copy
  from" when creating a role, and every key a matrix save grants or revokes, the target's
  permissions, ignoring `learning.take_tests` (which gives no power over another person), must
  be a subset of the actor's (`access.CanActOn`). Nobody edits their own role's permissions.
  R2 applies it to the student writes it has: update (a disable included), reset and delete
  (T-R2.13).
- **Student targets stay strict:** the Students list and record, every write to a student, the
  dashboard's student counts, class membership, assignment targets and the anonymiser take only
  the built-in Student role or a custom role whose grants are a subset of
  `{learning.take_tests}`. `app.student_like_roles` is the one definition, and
  `access.IsStudentLike` the same predicate in Go. An Admin with "Take tests" turned on is
  never a student target.
- **Disabling and shared students (T-R2.13):** a disable needs `people.users.manage`
  (`403 FORBIDDEN`). So do a password reset and an email change, unless no one else reaches
  the student: every class of theirs, archived ones included, is one the actor teaches, no
  other account created them or targets them individually, and with no class the actor
  created them. Otherwise the answer is `403 STUDENT_SHARED`.
- **Last admin:** the last active Admin cannot be demoted, disabled or deleted, by any path:
  the `users_last_admin` trigger refuses it (`20-data-model.md` §30). `409 LAST_ADMIN` joins
  the contract with R5's first operation that can reach it.
- **Sign-in policy:** a settings change that would leave no active Admin able to sign in is
  refused (`422 SIGN_IN_POLICY_LOCKOUT`); the response counts users who would lose their only
  sign-in method.
- The built-in Student role cannot lose `learning.take_tests`.

## 5. Route trees

| Tree | Audience | Shell |
|---|---|---|
| `/` | Anyone (signed-in → console) | Landing (R11) |
| `/login`, `/forgot-password`, `/join`, `/join/:code`, `/set-password`, `/change-password` | Front door | `AuthLayout` |
| `/teacher/*` | `workspace.teacher` | `TeacherLayout` |
| `/admin/*` | `workspace.admin` | `AdminLayout` |
| `/app/*` | `learning.take_tests` | `StudentLayout`; `/app/attempts/:id` in `FocusLayout` |
| `/403`, `*`, error boundary, maintenance | Everyone | System pages |

API prefixes follow the same split (`/teacher/*`, `/admin/*`, `/app/*`, `/auth/*`, `/me/*`,
`/public/*`). Every tree is a lazy chunk kept out of the entry chunk;
`web/tests/integration/router-chunks.test.ts` is updated, never weakened.

## 6. Production data

| Item | Plan | Release |
|---|---|---|
| The owner's account | `role='admin'` maps to the built-in Admin role: every permission, both workspaces. No manual step. | R2 (done) |
| Students | `role='student'` → built-in Student. Memberships, attempts, answers and events are untouched. | R2 (done) |
| Ownership | `owner_id` backfilled from `created_by` / `uploaded_by`; classes' `teacher_id` from the oldest active Admin, the teacher v0.7.0 shows, because `app.classes` has never recorded a creator (D-23, decided 2026-09-28). Each backfill raises if a row would stay NULL. | R2 (done) |
| Join codes | New codes are encrypted from R2. Every legacy hashed code is rotated at the R4 release, when the new class screens can show codes (D5); legacy codes redeem until then. | R2 (done), R4 |
| Tokens | Refresh tokens unchanged; access-token claims change additively; no forced re-login. `users.role` and `app.user_role` are dropped in v0.9.1 (T-R3.1). | R2 (done), v0.9.1 |
| Attempts in flight | R3 reads the existing local answer drafts unchanged. Deploys happen outside exam windows. | R3 |
| Assignment integrity | Existing assignments keep their stored policy; the wizard's new defaults apply to new assignments. | R4 |
| Retention | O-23's fixed rules apply until an admin changes the R5 settings. | R5 |
| Browser storage | The student has no `quizzivy.column.*` key since R3: the engine's footer deletes `quizzivy.column.studentNavigator`, the only one, when it mounts. The theme is `quizzivy.theme` since R1; `quizzivy.locale` stays. The student's keys are listed below. | R1 (done), R3 |

The student's browser storage in v0.9.0, as `web/src` reads and writes it:

| Key | Store | Holds |
|---|---|---|
| `quizzivy.theme` | local | The theme preference: light, dark or system (R1). |
| `quizzivy.locale` | local | The language, `vi` or `en`. |
| `quizzivy.testText` | local | "Larger text in tests", `large` or `default`, until R4's `users.preferences` takes it (`73-r3.md`). |
| `quizzivy.answer-draft.<attemptId>` | local | Answers the server has not confirmed. The format is v0.8.0's. |
| `quizzivy.group-play.<playId>` | local | A play of a shared recording the server has not confirmed. |
| `quizzivy.flags.<attemptId>` | session | The questions flagged for review. |
| `quizzivy.open-question.<attemptId>` | session | The id of the question that was open, so a reload returns to it. Removed when the attempt is submitted. |
| `quizzivy.integrity.<attemptId>` | session | The integrity event buffer and its sequence number. |
| `quizzivy.join` | session | The class being joined, across sign-in, for 30 minutes. |
| `quizzivy.oauth.pending` | session | The Google authorization in flight (PKCE). |

Two keys are gone since R3: `quizzivy.column.studentNavigator` (local), and
`quizzivy.material-collapsed.<groupId>` (session), which nothing writes now
that the phone has no collapse button. The teacher console keeps
`quizzivy.column.sidebar`, `rail`, `panel`, `outline` and `importSource`. Since T-R4.4
`TeacherLayout` stores its sidebar's collapsed state in `quizzivy.sidebar`;
`quizzivy.column.sidebar` is the old `AdminLayout`'s width and goes with it at T-R4.48.

## 7. R0 — Groundwork

No production change; PRs straight to `develop`, riding to production in v0.7.0.

| Task | What | Status |
|---|---|---|
| T-R0.1 | Import the deck into `docs/design/deck/` with `MANIFEST.sha256`; remove `docs/design/mockups/`; CI Deck job runs `scripts/check-design-deck.mjs`; launch config; `work/redesign-*` in CI triggers | PR #152 |
| T-R0.2 | PR template: x-permission, isolation test, deck comparison, vi keys | with T-R0.4 |
| T-R0.3 | Spec rewrite in parts, version bumped per §18: §1–§4; §5–§6; §8–§10, §12; §13, §15, §16. Future capabilities carry "Availability: R<k>" | |
| T-R0.4 | AGENTS.md (Design, Authentication and authorization, student layout, repository map, six open operations, "Redesign in progress" note); NFR registry; risks; open items (O-23 superseded) | |
| T-R0.5 | This file and `71-r1.md` … `81-r11.md` | this PR |
| T-R0.6 | Design-gap register `docs/design/gaps.md` | PR #153 |
| T-R0.7 | Comment sweep: ~108 web files cite old board ids; formatting-only PR | |
| T-R0.8 | External setup (Thuong): Resend domain DNS; a second Google OAuth redirect URI for the apex; `JOIN_CODE_KEY` and `SECRETS_KEY` as Fly secrets; `.env.example`. Later, in each release's checklist: `quizzivy_maintenance` provisioned on Neon and the GitHub environment `production-maintenance` (R5); `RESEND_WEBHOOK_SECRET` as a Fly secret (R7) | |
| T-R0.9 | Refresh the agent memory notes that point at the old deck | |

## 8. Verification, every release

1. Every CI job step, locally, in order, judged by exit code: `make lint`, `make gen-check`,
   `make test-api` (unit, integration, e2e; `docker compose up -d db minio`), goose up/down/up,
   `pnpm lint`, `pnpm typecheck`, `pnpm format:check`, `pnpm test:unit`,
   `pnpm test:integration`, `pnpm build`, `pnpm e2e`, `pnpm e2e:content`, and `pnpm e2e:live`
   when take-test, imports, auth or join changed. `node scripts/check-design-deck.mjs`.
2. The app beside the deck in the browser: `design-deck` on 5175, the app on 5173 with the API;
   every changed screen at 360, 768, 1024, 1280 and 1440, light and dark, with data shaped like
   the deck's fixtures. Confirm the dev server is not stale; measure with
   `getBoundingClientRect`, not screenshots alone.
3. Security: the isolation suite (R2 on), escalation tests (R2, R5), the leak E2E that asserts
   student payloads carry no `isCorrect`, `sampleAnswer`, `acceptedAnswers` or `transcript` in
   every release that adds or changes an `/app/*` or `/me/*` response (R3, R4, D4, R6, R7, R8,
   R9, R10), rate-limit tests for every new public operation.
4. Release: migrations rehearsed on a Neon branch; deploy API then Pages; smoke `/healthz`,
   `/livez` and a sign-in per role; roll forward only; release notes; tag.

## 9. Risks

| # | Risk | Mitigation |
|---|---|---|
| PR-1 | A teacher sees another teacher's data after R2 | Landed in R2: every teacher repository filters by `access.Scope`, and another teacher's id answers as a missing one (T-R2.12a–f); the isolation suite sends another owner's id into every id a `/teacher/*`, `/app/*` or `/me/*` operation takes, and a scoped uuid without an `x-resource` kind fails the build (T-R2.16, NFR-S17); no UI creates a second teacher before R5 (§2 rule 2) |
| PR-2 | A role or permission edit escalates privilege | Landed in R2: the subset rule (`access.CanActOn`) on every student write; strict student targets through `app.student_like_roles`; `people.users.manage` for a disable and for resetting or re-addressing a shared student; the `users_last_admin` trigger; grant triggers that keep the hidden keys ungranted and the Student's "Take tests" (T-R2.1, T-R2.2, T-R2.13); `escalation_test.go` with an Admin who takes tests and a custom role per matrix key as targets. R5 adds the role and matrix edits, the sign-in lockout guard and `admin_escalation_flow_test.go` |
| PR-3 | The rebuilt take-test engine loses work | The five canaries before and after every PR; local drafts read unchanged; live E2E 5–8; deploy outside exam windows |
| PR-4 | Rolling deploy breaks inserts from the old binary | Expand/contract with fill triggers; the old-binary insert test |
| PR-5 | The path rename breaks old tabs or drops a rate limit | Landed in R2: the outer `/admin` alias rewrites the 78 v0.7.0 paths, logs `legacy_admin_path` and lasts until v0.9.1 (T-R2.8, T-R3.3); every rate-limit key moved, and `credential_limits_test.go` fails on a credential-minting operation without a limit (T-R2.7); `ratelimit_contract_test.go` pins every `x-rate-limit` block to the registry (T-R2.15) |
| PR-6 | Legacy join codes stop working unannounced | Rotation only at R4, with the release note and an in-app notice |
| PR-7 | Polling keeps Neon awake | Principal cache; polling stops after idle, not only when hidden; compute hours measured after R4 and R7 |
| PR-8 | English-only deck leaks English into the product | vi first per PR; `no-hardcoded-strings` and parity tests; a native review before v1.0 |
| PR-9 | The deck keeps changing under the work | The manifest pins the imported version; re-imports are PRs that say which screens changed |
| PR-10 | Scope (eleven releases) outruns one developer | Each release is shippable alone; later modules can slip without leaving a console incoherent |

## 10. Open items

- **The Admin's lists in the teacher workspace (DG-53, D3).** §4.1 states R4's default
  (T-R4.54): content lists show the caller's own rows, admins included, and everything stays
  reachable by id and, from R5, in the Admin console. Default: build it so. Thuong confirms this
  reading of D3's "Admin sees everything".
- **The subset rule and "Take tests" (D13).** §4.3 compares the target's permissions without
  `learning.take_tests`. Read literally, no Admin could act on a student, and an Admin without
  "Take tests" could never grant it to a custom role. Default: build as written (R2, R5).
  Thuong confirms this reading of D13.
- **What D9 and D16 now record.** They carry what R5 and R7 designed: the sweep runs from GitHub
  Actions with the maintenance credential in the GitHub environment `production-maintenance`,
  and R7 adds the open operation `POST /public/email-events` and `app.email_suppressions`.
  Default: build as written. Thuong confirms both, since one places a production credential in
  GitHub and the other adds a public endpoint.
- **Question bank Import (D1).** The deck's Import dialog (CSV, XLSX, `.quizzivy`, a tag for
  every question, "Skip questions that already exist") has no release yet; R4 leaves the button
  absent. Default: built, as D1 requires, in R6 after T-R6.3 so the importer knows every type:
  `POST /teacher/questions/import` (`content.questions.write`, formula-safe, bounded in the API
  or run by the worker) and the dialog. `76-r6.md` gains the task when Thuong confirms. The
  alternative is a D1 exception recorded in §2 with a DG entry.
- **Import controls without a backend (DG-67, D1).** Saved profiles, source conventions, the
  Word template, the audio slot, Move / Split / Merge, the processing history, "Create another
  test from this file", "Delete import" and "Process again" after a cancel. R4 gates them all.
  Default: built, as D1 requires, in R6 beside T-R6.13; `76-r6.md` gains their backend and UI
  tasks when Thuong confirms. The alternative is an approved post-1.0 exception in §2, with
  DG-67's "Needed by" moved to it. Pasting a test, which the deck draws since 2026-10-03, is not
  one of them: R4 builds it (`74-r4.md` T-R4.55–T-R4.57). The answer conventions its page
  states are fixed recognizer rules for text sources, not the gated source-conventions control.
- **The content editor of the 2026-10-04 export (DG-108 to DG-116, D1).** The third deck import
  redrew the builder's editor pane and the import review's "Question" field around one rich
  content editor. R4 builds it (`74-r4.md` T-R4.62–T-R4.66, with T-R4.31b, T-R4.33 and T-R4.38)
  on defaults that Thuong has not confirmed, all listed in that file's open items. The main
  six: the editor ships to every teacher and its flag goes; the editing mode is the form the field is stored in; one question editor serves the
  builder and the bank; the type menu offers the five stored types; "Students can pause" is not
  built; and a prompt holds no media in R4. The last is the large one. Images and audio inside
  a prompt or an explanation need a contract change that runs through the publish snapshot,
  the student payload and the engine. Default: built, as D1 requires, in R6; `76-r6.md` gains
  the tasks when Thuong confirms. The alternative is to build them in R4. "Students can pause"
  (DG-111) is unbuilt until D4, which builds it as `allowPause` on its own default
  (`74d-d4-deck-update.md`, T-D4.13 and Q4). One default leaves something drawn unbuilt in
  every release: the review's "Markdown" mode (DG-114). If Thuong confirms it, it is a D1
  exception recorded in §2 with its DG entry; so is "Students can pause" if he declines it
  in D4.
- **Class staff (D14, DG-04).** §2 rule 9. Default: class staff lands in the first release that
  starts after the deck draws DG-04; if DG-04 is still open at T-R11.14, v1.0 ships with
  Assistant hidden from role pickers and class staff on the post-1.0 list. Thuong confirms the
  fallback.
- **The AI assistant's questions (D19 to D22; `74d` Q28 to Q35).** Thuong answered Q28 to Q34
  on 2026-10-10 as recommended: the Admin page waits for R5 and D4 ships its operations and a
  maintenance command; D4 is v0.11.0; the default allowance is 20,000 credits; the pool's rule
  has one value; a variant is a new draft test and the clean-up merges nothing; "Let AI read
  student answers" is a per-teacher switch on by default; paused or exhausted buttons are visible
  and disabled with the reason. **Still open (Q35):** the student-facing sentence on the Test
  intro about cloud processing, which waits for Thuong's wording, and the provider account's
  data-retention setting, which he confirms. The import page's sentence (D4) and the privacy
  notice (R11) are decided.
