# Team ledger

The team's checkpoint. Only the Tech Lead edits this file, and only in the Tech Lead's own
pull requests, never inside a task's PR, so parallel branches never conflict on it.
Between checkpoints the live state is in the session's task list. After an interruption,
start at [README.md](README.md) "Resuming".

## Checkpoint

- **As of:** 2026-10-08, afternoon.
- **Base:** `work/redesign-r4` at `d6c5f8cc` (T-R4.16a merged, #422).
- **Done today:** the team merged into `work/redesign-r4` (#420) and cherry-picked onto
  `develop` (#421); the open items F-1 to F-9 are closed or owned; W0 started.
- **Objective (the user, 2026-10-08):** the team takes over the project and drives R4 to
  v0.10.0 by the waves below. Merging to `main` still waits for the user's go.
- **In flight:**

  | Task | PR | Owner | State |
  |---|---|---|---|
  | T-R4.16b diff and `unpublishedChanges` | #423 | backend | review round 1 fixed and confirmed by the Tech Lead; one nit (the 422 before the 404); tester next |
  | T-R4.63 content editor frame | #424 (draft) | frontend | Principal's code review (REV-63) |
  | T-R4.31a builder frame | #414 (draft) | frontend | synced with R4, green; tester's browser acceptance (VER-DRAFTS) |
  | T-R4.28 Grading | #416 (draft) | frontend | synced with R4, green; VER-DRAFTS |
  | T-R4.35 media library | none yet | frontend | implementing |
  | T-R4.11 review options, note, live lock | none yet | backend | implementing, after the 16b nit |
  | W0-64, the editor chain's next approach | none | principal | with REV-63 |

## Roster and models

Requested is what the user asked for. Configured is how this session launched the
agent. Resolved is what the transcript metadata recorded on every turn (README,
"Models").

| Agent | Requested | Configured | Resolved | State |
|---|---|---|---|---|
| Tech Lead | Opus 5.5, high or more | primary session | `claude-opus-5-5`, xhigh (session metadata) | active |
| `principal_swe` | Fable 5.1, high or more | its definition (from 2026-10-08) | `claude-fable-5-1`, high | active |
| `senior_swe_backend` | Sonnet 5.5, xhigh | its definition (from 2026-10-08) | `claude-sonnet-5-5`, xhigh | active |
| `senior_swe_frontend` | Opus 5.5, high or more | its definition (from 2026-10-08) | `claude-opus-5-5`, high | active |
| `senior_swe_platform` | Sonnet 5.5, high or more | its definition (from 2026-10-08) | `claude-sonnet-5-5`, high | active |
| `senior_tester` | Sonnet 5.5, high or more | its definition (from 2026-10-08) | `claude-sonnet-5-5`, high | active |

The definitions in `.claude/agents/` pin the full model ids. On 2026-10-07, the session
that created the directory launched by family alias and effort; from 2026-10-08 the agents
launch from their definitions, and the resolved models match the pins. On 2026-10-07 the
platform engineer was relaunched twice with a checkpoint after server-side API 500s.

## R4 status

Taken from merged pull requests on 2026-10-07. The plan's "Done when" boxes lag this (F-1).

- **Merged:** T-R4.1a–c, 2a–c, 3a–c, 4, 5a–b, 7, 10a, 14, 15, 16a, 17a, 17b, 19, 21, 22,
  23, 25, 36, 37, 45a, 51a, 51b, 53, 54, 55, 56, plus the plan corrections (#351, titled
  T-R4.0, not a plan task), the develop syncs (#344, #352, #364, #379, #396, #402) and the
  team (#420, #421).
- **In flight:** see the checkpoint.
- **Ready (every dependency merged):** backend T-R4.8, 9, 12 (after 11), 13, 20; frontend
  T-R4.27a, 32, 46, 57. T-R3.1 to T-R3.3 (v0.9.1) become eligible on 2026-10-10.
- **Blocked:** the rest. Close-out (T-R4.48 to 52) comes last.
- **Migration numbers:** 00092 is taken (16a). Reserved by W1-APPROACH: 00093 and 00094
  for 11, 00095 for 9, 00096 for 12; each is renumbered if another PR merges first (T-5).
  Per-task readiness, sizes and open questions are in
  [r4-readiness.md](r4-readiness.md). 16 is nearer L. 12 is nearer XL and is proposed as
  two PRs: 12a the overrides table, extend and the override operations; 12b the attempt
  deadline recompute, the monitor and the maintenance windows. The split is decided when
  12 starts and written into `74-r4.md` T-R4.12 by its first PR (CONTRIBUTING, 70 §3).

| Task | Waits for |
|---|---|
| T-R4.6 command palette | 13 |
| T-R4.10b notification producers | 11, 12 |
| T-R4.18 class schedule and room | 8 |
| T-R4.62 alt text | 16 |
| T-R4.24 assignments list | 12, 13 |
| T-R4.26 assignment Questions and Settings | 11, 12, 13 |
| T-R4.27b wizard schedule and rules | 27a, 11 |
| T-R4.29 tests list, T-R4.30 test detail | 16 (and 29 for 30) |
| T-R4.64 → 66 → 65 editor chain | 63; 65 also 35 and 62 |
| T-R4.31b builder editor pane, T-R4.33 question editor | 65, 66 (and 31a, 32) |
| T-R4.34 question groups | 63, 35 |
| T-R4.38 import review, T-R4.39 preview | 63 (and 30 for 39) |
| T-R4.40 students | 20 |
| T-R4.41, 42 classes | 18 |
| T-R4.43, 44 settings | 8, 9 |
| T-R4.45b bell and student settings | 10b, 8 |
| T-R4.47 attempt review | 46, 28 |

## Environment baseline

The container reaches CI's gates by the recipe in [environment.md](environment.md):
dockerd, `postgres:18` and MinIO under compose, `GOTOOLCHAIN=go1.27.0`, and CI's
provisioning for the Go integration tier. At `bb4d4000` every gate run here passes:
deck, goose up/down/up, `make gen-check`, `make lint`, the Go unit, integration and
e2e tiers, `pnpm lint`, `typecheck`, `format:check`, `test:unit` (3936 tests),
`test:integration` (120 tests) and `build`. Not run here: the Word converter tests,
`pnpm e2e`, `e2e:content` and `e2e:live`. PostgreSQL (5432) and MinIO (9000, 9001)
stay running for the team.

## Critical path and waves

The Principal's assessment (2026-10-07). The dependency-critical path is
16 → 62 → 63 → 64 → 66 → 65 → 31b and 33 → 48 → 50, 51 → 52: eight serial steps, six of
them frontend. The capacity-critical path is the one frontend engineer, who has 24
frontend tasks left; the backend runs out of Go work after about two waves.

| Wave | Backend | Frontend | Platform | Tester |
|---|---|---|---|---|
| W0 | T-R4.16 | T-R4.63 | environment, baseline, migration register | checks for 16, 63, 35, 11 |
| W1 | 11 → 12 → 13, serial (both 11 and 12 edit `assignments/repositories/student.go`) | 64 → 66, and #414's fixture-only fix if the user makes it ours | as needed | verify 16, 63 |
| W2 | 8, 20, 9, then 62, 18, 10b | 35 → 65 → 62 web | as needed | verify 11, 12, 13 |
| W3+ | review, then a second frontend stream | 31b, 33, 38, 39 | close-out support | route matrix (T-R4.51) |

Approach points to agree before each bundle starts: one `EffectiveClose` in
`assignments/domain/schedule.go` shared by 11, 12 and 10b (settled by W1-APPROACH as
`shared/schedule.CloseOf` and `Close`); 16's question-identity rule and a null
`unpublishedChanges` when the group graph is refused (settled by W0-APPROACH); 8 uses the
standard library and `DecodeConfig` before decoding; `QuestionEditor` as the one component
(DG-108), with its structure reviewed before 66 starts.

## What the user settled (2026-10-08)

The user told the team to clear the open items, merge, and take over the whole project.
That settles the decisions owed on 2026-10-07:

1. **Objective:** all of R4, by the waves above; the Principal's W0 (T-R4.16 and T-R4.63)
   goes first, with the two drafts.
2. **v0.9.1 (F-3):** the plan's own provision applies by default: T-R3.1 to T-R3.3 run as
   T-R4.49, numbered on `work/redesign-r4` (`73-r3.md`, "Migration numbers"). The user
   can still ask for a separate v0.9.1, which first renumbers R4's merged migrations.
3. **Defaults on ready tasks:** built as the plan states them (AGENTS.md: "build the
   default and move on"): DG-115, DG-110, the note to students, the Admin's own-row lists.
   The plan keeps each marked for the user's confirmation, and a later "no" is a change.
4. **#414 and #416:** the team drives them.
5. **Capacity:** the five specialists; `senior_swe_backend` takes frontend tasks once
   the Go work runs out (W3).
6. **Criteria the tester could not make testable:** the tester's proposals in
   `verification.md` §9 are the working rule until the user says otherwise.

Still the user's: the go for every merge to `main`, the production steps of the release
checklist (Cloudflare, Fly, Neon, the smoke tests on real devices), reading production
logs (T-R4.49 needs seven days of `legacy_admin_path` counts before the `/admin` alias goes),
and any reversal of the defaults above.

## Decisions

- **T-1 (2026-10-07, amended 2026-10-08).** The team's artifacts are `.claude/agents/` and
  `docs/team/`. They are on `chore/agent-team`, cut from `work/redesign-r4` and merged into
  it, because the team works on R4 now; at the user's request they are also cherry-picked
  onto `develop`, so the next develop-into-R4 sync meets the same files on both sides.
- **T-2 (2026-10-07).** Models are pinned by full id in the definitions and checked from
  transcript metadata, never from an agent's own account.
- **T-3 (2026-10-07).** At most four sub-agents run at once (70 §3, "Machine").
- **T-4 (2026-10-07).** The ledger changes only in the Tech Lead's pull requests.
- **T-5 (2026-10-07).** A migration takes its number before its PR runs CI, because goose
  refuses `NNNNN_` (T-R4.10a, "As built"); it is renumbered if another PR merges first.
- **T-6 (2026-10-08).** T-R3.1 to T-R3.3 ship inside R4 as T-R4.49 by default (F-3).
- **T-7 (2026-10-08).** The team takes over #414 and #416. Their branches are brought up to
  date by merging `work/redesign-r4` into them, never by rebase or force-push.
- **T-8 (2026-10-08).** The Principal's approach notes bind the tasks they name:
  W0-APPROACH (T-R4.16a/b, T-R4.63) and W1-APPROACH (T-R4.11, 12, 9, 20). Each task records
  what it took from them in its "As built", so the plan, not a scratch note, keeps them.
  In short: 16 matches questions by source id, then by a fingerprint with gap ids rewritten
  to ordinals, and counts one change per question; 63 places its props on the host by
  profile, keeps images out of a paste and counts them (DG-115); 11 makes PATCH partial
  for the new fields, withholds through the result's existing path and adds `releasesAt`;
  12 ships as 12a and 12b and recomputes in-progress deadlines under the assignment's lock;
  9 builds the location label only behind `CF-Connecting-IP`; 20 resets each student in
  its own transaction.
- **T-9 (2026-10-08).** Download in the media library (T-R4.35) opens the signed URL in a
  new tab; the file keeps its storage name until the backend signs a content disposition,
  which is a follow-up, not part of 35.
- **T-10 (2026-10-08).** Ports: the tester keeps the defaults (8080, 5173, 4173, 5175,
  4175), the frontend engineer 5185, 4185 and 4183, the backend engineer 8090, 5195 and
  4195. An agent stops only servers it started.

## Findings and open items

| ID | Finding | Owner | State |
|---|---|---|---|
| F-1 | Plan drift: "Done when" boxes unticked on merged tasks (T-R4.1, 4, 10a, 22, 23, 36, 53, 54, 56 among others); the schema table still says `NNNNN_` for 7, 15 and 55 though 00085 to 00091 exist; T-R4.10b still routes the bell to Grading although T-R4.45a moved that rule to the web; `73-r3.md` says the newest migration is 00079; `AGENTS.md` still describes the teacher console as not yet rebuilt; T-R4.11 asks for a "new DG entry" that exists as DG-71; T-R4.20's first Done-when bullet has shipped; several "Touches" lists omit files (`r4-readiness.md` names them). | senior_tester, Tech Lead | fixed: text in `76a39e1c`, 116 verified ticks after it; 9 boxes stay open on purpose (QA-F1) |
| F-2 | Underscores in a definition's `name` are not documented as valid (the docs show hyphens). The next session must confirm that `principal_swe` loads, or rename all five to hyphens. | Tech Lead | closed 2026-10-08: the session loaded the five definitions and launched from them |
| F-3 | v0.9.1 cannot take 00080: `work/redesign-r4` already merged 00080 to 00091, and `develop` ends at 00079. If v0.9.1 takes 00092 and deploys first, production's `cmd/migrate` (`goose.Up`, missing versions not allowed) refuses R4's 00080 to 00091. | Tech Lead | decided by default (T-6, `e4c2916e`) |
| F-4 | #414 is red because two new test files build a `Test` without `skills`, which #415 made required; the contract validator rejects the stub. Fixture-only fix. | senior_swe_frontend | taken over (T-7) |
| F-5 | #416's run 718 ended `failure` with every job green and no **CI result** job, apparently the gate never ran. "Re-run failed jobs" on run 718 is the next step. | senior_swe_frontend | taken over (T-7); GitHub refused a re-run of run 718, so the next push starts a fresh one |
| F-6 | `70` §6 names the teacher sidebar key `quizzivy.column.sidebar`; the code stores `quizzivy.sidebar` (`web/src/layouts/shell/sidebarState.ts`). | Tech Lead | fixed (`76a39e1c`): 70 §6 names both keys and their layouts |
| F-7 | This container has Node 22 and pnpm 10.28; CI uses Node 24, pnpm 11.25.0 and Go 1.27. Only Chromium is available, so the Firefox and WebKit paste checks of T-R4.51 cannot run here. | senior_swe_platform | accepted: `environment.md` lists the differences, and CI stays the authority for Node 24, pnpm 11 and other browsers |
| F-8 | T-R4.5b's ticked "Done when" item (`74-r4.md:1331-1336`) says the `router-chunks` media pattern was narrowed to `features/media/(pages/\|components/(UploadPanel\|AssetLibraryDialog))`; the code still names all of `features/media/` (`router-chunks.test.ts:51`). So T-R4.35 needs no canary edit. | Tech Lead | fixed (`76a39e1c`) |
| F-9 | Setting up the container takes a session's first half hour by hand. The platform engineer proposes a SessionStart hook (dockerd, `.env`, the MinIO image, compose, goose, `pnpm install`, `goose up`, the toolchain pin); it would change every session, so it waits for the user. | senior_swe_platform | fixed (`21a161c0`): `.claude/hooks/session-start.sh` |
| QA-F1-1 | T-R4.36: the imports history row does not draw a pasted import (clipboard tile, "Pasted text", characters); PR #410 deferred it and no task owned it. Major for the R4 exit. | senior_swe_frontend, in T-R4.57 (a "Done when" line added) | open |
| QA-F1-2 | T-R4.36: `ImportsListPage.tsx` still uses the legacy `Pager`, which T-R4.1b meant the deck `data/Pager` to replace there; ten rebuilt pages do, and T-R4.48 cannot delete it while they remain. | senior_swe_frontend, by T-R4.48 | open |
| QA-F1-3 | T-R4.19: the seeded-volume `EXPLAIN` that shows the five named indexes is still owed; PR #397 made it a release gate. | senior_swe_platform, at T-R4.52 | open |
| QA-F1-4 | T-R4.25: the header has no Extend button; the "As built" moved it to T-R4.26. | T-R4.26 | open |
| QA-F1-5 | T-R4.25: the sheet's "Grade answers" opens the full review, not Grading filtered to the student, until T-R4.28. | T-R4.28 (#416) | open |
| QA-F1-6 | T-R4.23 and T-R4.25: the keys exist in both locales (`parity.test.ts`), but the PRs do not list them as the box asks. | Tech Lead, at the release review | open |
| QA-F1-7 | T-R4.17a: PR #386 records the canaries at its last commit only, not at the branch point. | Tech Lead, at the release review | open |
| QA-16a-1 | T-R4.16a: `Test.assignments`' three correlated counts lift `listTests` at `limit=100` past `jit_above_cost`, so the first five runs per connection take 82–131 ms instead of about 15 ms. | backend, in T-R4.16b (one `LATERAL` aggregate) | fixed on #423; the tester re-measures in VER-16b |
| F-10 | `/opt/pw-browsers` holds Chromium 1194; Playwright 1.62.1 looks for headless shell 1234 and fails to launch. | senior_swe_platform | worked around (`environment.md`, differences table): `executablePath` in an untracked wrapper; a helper script is open |
| F-11 | Agent worktrees under `.claude/worktrees/` showed as untracked in the main checkout. | Tech Lead | fixed: `.gitignore` |

## Reviews

- **REV-ARTIFACTS (2026-10-07, `principal_swe`, at `50964df5`).** Ten defects: a missing
  full code reviewer for frontend PRs, two schedules that contradicted the ledger, an
  unrecorded split of T-R4.12, two canary runs the plan requires, the owner of #414's fix,
  an unverified recipe step, unmarked tool assertions, and plan corrections kept in a
  survey. All are fixed in the commit after it, except trimming `verification.md` where it
  restates the plan, which is left for its next edit.

- **REV-PLAN (2026-10-08, `principal_swe`, at `76a39e1c`).** Six findings on the plan
  corrections and the F-3 default. The v0.9.1 release checklist had no owner under the
  default; the release train, R4's header and schema table, and R4's decision list did not
  say it; the numbering wording was loose; T-R4.10a's "As built" still left a decided
  question open. All six are fixed (`2fdf2c8d` and the commit after the ticks).
- **F1-TICKS (2026-10-08, `senior_tester`).** 116 of 125 boxes verified and ticked;
  seven findings, QA-F1-1 to 7, above. The tester also proposed that the PR template carry a
  keys table and a canary-runs line, so these two duties stop slipping.
- **W0-APPROACH and W1-APPROACH (2026-10-08, `principal_swe`).** Binding approach notes
  (T-8). Three product defaults in W1 are built unless the user reverses them: the class
  average is a percent shown once at least three students are graded; `after_close` also
  hides the grader's comment until release; the devices list shows the location of the
  latest refresh, not of the first sign-in.
- **T-R4.16a (#422, merged 2026-10-08).** Reviewed by the Principal (architecture) and in
  a full code review; the tester verified it and raised QA-16a-1, carried into 16b.
- **T-R4.16b (#423).** The Principal's architecture review (keep one entry per question;
  the context wording) and a code review with three findings: a version that no longer
  reads back failed `getTest`, `diff_paper.go` discarded two errors, and `DiffSide` said
  more than the schema holds. Round 1 fixed all of them (`93f6f314` to `c2541355`). The
  Tech Lead confirmed the fixes and found one more: the 422 must be checked before the 404,
  because the wrapped error matches both.

## Next action

As each hand-off arrives:
- **VER-DRAFTS** → the frontend engineer finishes #414 and #416 (Done-when boxes, "As
  built", fixes), then they merge.
- **REV-63** → its fix round, then the tester's VER-63, then #424 merges.
- **W0-64** → T-R4.64 after 63 merges.
- **The 16b nit** → the tester's VER-16b, then #423 merges.
- **T-R4.11** → its review and verification, then 12a, 12b and 13; 9 and 20 run beside
  them when a slot is free.
