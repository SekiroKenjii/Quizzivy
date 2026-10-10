# Team ledger

The team's checkpoint. Only the Tech Lead edits this file, and only in the Tech Lead's own
pull requests, never inside a task's PR, so parallel branches never conflict on it.
Between checkpoints the live state is in the session's task list. After an interruption,
start at [README.md](README.md) "Resuming".

## Checkpoint

- **As of:** 2026-10-10, 16:40 UTC.
- **Base:** `work/redesign-r4` at `5b4e7355` (T-R4.30 merged, #469).
- **Objective:** R4 to v0.10.0, then D4 (v0.11.0), which absorbs the design team's v2 export
  and builds the AI assistant (T-24). Done pull requests into the integration branch merge
  without asking the user (T-22); a release to `main`, a hotfix or a critical PR waits for
  the user's go.
- **In flight** (five agents, T-3):

  | Task | PR | Owner | State |
  |---|---|---|---|
  | T-R4.34 question groups | #471 | frontend, lane 3 | VER-34 PASS WITH FINDINGS; round 2 (QA-34-1, QA-34-3, copy, and observation 6, a possible content loss); QA-34-2 is closed by #474 |
  | T-R4.33 question editor | #474 | frontend, lane 1 | CI green; VER-33 running |
  | T-R4.8 profile photo | #472 | backend 1 | security review: SR-472-1 (major) to SR-472-4 in a fix round; then the user (T-22) |
  | T-R4.9 signed-in devices | #473 | backend 2 | security review approved; waits on the user (T-22) |
  | T-R4.38 import review | none yet | frontend, lane 2 | implementing (DG-147 to 149) |
  | T-R4.46 teacher integrity | none yet | frontend, lane 3 | parked while #471's round 2 runs (DG-150 to 152) |
  | F-46 next version on `Test` | none yet | backend 2 | implementing |

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

Taken from merged pull requests on 2026-10-10. The plan's "Done when" boxes lag this (F-1).

- **Merged:** T-R4.1a–c, 2a–c, 3a–c, 4, 5a–b, 7, 10a, 11 (#429), 12a (#447), 14, 15, 16a
  (#422), 16b (#423), 17a, 17b, 19, 21, 22, 23, 25, 28 (#416), 31a (#414), 35 (#428), 36, 37,
  45a, 51a, 51b, 53, 54, 55, 56, 62a (#426), 62b (#450), 63 (#424), 64a–d (#430, #433, #435,
  #436), 66a (#446); the fixes #427 (`x/tools`), #437 (F-27), #438 (F-23), #439 (F-30), #440
  (T-19), #441 (F-31), #442 (FLAKE-1), #443 (F-34) and #449 (F-35, by the user); on
  2026-10-10, 10b (#462), 12b (#451), 13 (#456), 27a (#458), 27b (#468), 29 (#448), 30
  (#469), 31b (#467), 31b-2 (#470), 32 (#455), 57 (#459), 65 (#461) and 66b (#457), with the
  fixes #452 (QA-66a-1), #454 (FONT-1), #460 and #463 (F-20), #464 (F-36), #465 (F-43) and
  #466 (the gates), and the docs #453; the D4
  plan (#444, #445); and, from before the team, the plan corrections (#351, titled T-R4.0,
  not a plan task), the develop syncs (#344, #352, #364, #379, #396, #402) and the team
  (#420, #421).
- **In flight:** see the checkpoint.
- **Order:** W3-PLAN (T-23). Lane 1: 33, then a task from lane 3's tail (24 or 40) when it
  frees. Lane 2: 38 → 39 → 43 → 44 → 45b → 6. Lane 3: 34 → 46 → 47 → 26a → 24 → 26b → 40 →
  41 → 42. Backend: stream A is done (12b, 13, 10b, F-43); stream B is 8 (#472) and 9 (#473),
  then F-46, 20 and 18 (18 waits for 8). Close-out (T-R4.48 to 52) comes last.
- **Migration numbers:** 00092 (16a), 00097 and 00098 (62a), 00099 and 00100 (11), 00101
  (12a), 00102 (9, #473). The next free number is 00103; T-5 governs.

Still waiting on an unmerged dependency:

| Task | Waits for |
|---|---|
| T-R4.18 class schedule and room | 8 (#472) |
| T-R4.40 students | 20 |
| T-R4.41, 42 classes | 18 |
| T-R4.43, 44 settings | 8 (#472), 9 (#473) |
| T-R4.45b bell and student settings | 8 (#472) |
| T-R4.47 attempt review | 46 |

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
- **T-3 (2026-10-07, amended 2026-10-09 and 2026-10-10).** At most four sub-agents ran at
  once (70 §3, "Machine"). On 2026-10-09 the user cut that to two, after the weekly usage
  limit stopped the team (F-14); on 2026-10-10 the user raised it to five. Reviews and
  verifications count.
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
  4175); a frontend engineer 5185, 4185 and 4183, and a second frontend instance 5187,
  4187 and 4188; the backend engineer 8090, 5195 and 4195. An agent stops only servers it
  started. Two instances of one role run only in separate worktrees, within T-3's four.
- **T-11 (2026-10-08, corrected 2026-10-09).** Grading is last-write-wins in R4: the
  contract carries no concurrency token for a grade, and the second write replaces the
  score, the comment, the grader and the time (QA-416-5). The first version of this entry
  said the audit log keeps both writes; it does not, because grade writes are not audited
  (`review_grade.go` is a single UPDATE), and spec §13.4's required entries do not include
  grading (QA-416b-1). Whether a regrade should be audited was offered to the user as a
  possible §13.4 addition; the default stands (T-22): not audited in R4. T-R4.28's "As
  built" and `76-r6.md` say the same.
- **T-12 (2026-10-08).** In the builder, the product's section keeps the word "section" in
  English, where the deck says "group", because the outline also shows the product's
  question groups (QA-414-2). It is DG-135, written by #414; the rest of the builder's
  English follows the deck.
- **T-13 (2026-10-09, amended the same day).** Heavy runs take the machine-wide lock
  (README, "The heavy lock"), with `-o`, so a server started by a locked command never holds
  it: the tester's API once held it for 45 minutes (F-19). A timeout under load is not a
  finding; CI on the pushed head is the authority for the full suites.
- **T-14 (2026-10-09).** Upload's play-limit default is "2 plays", per DG-69 and the editor.
- **T-15 (2026-10-09).** Commit trailers name the model that wrote the commit, from each
  agent's own harness attribution; T-R4.11's mixed trailers stay.
- **T-16 (2026-10-09).** `bundle.test` builds with the production export conditions (what
  the student downloads, as the plan states the budgets), not vitest's development ones;
  the limits stay 160, 220 and 64 KiB.
- **T-17 (2026-10-09).** T-R4.64 shipped as four stacked PRs (64a to 64d). An empty field
  opens in rich text; any other field opens in its stored form.
- **T-18 (2026-10-09).** R64c-1 keeps that interim rule: an empty rich editor over a stored
  starter prompt would show something other than what is stored (DG-110). T-R4.31b and
  T-R4.34 each create the starter as rich content.
- **T-19 (2026-10-09).** A true/false question's options are read by their canonical texts,
  "True" and "False", never by position, because the deal shuffles them. One helper maps
  the canonical texts to `t()` labels on every surface (DG-139, #440).
- **T-20 (2026-10-10).** After merging the integration branch into a PR that changes a
  shared type, run `pnpm typecheck` before pushing, even when the merge is clean: #429's
  clean merge failed `tsc -b` on a fixture #440 added.
- **T-21 (2026-10-10).** Gate policy G-2 (README, "Local gates"): changed-scope local gates,
  CI as the authority for the full suites, no lock for small runs, the deck matrix once by
  the tester, short hand-backs. Measured before it: six commands queued on the one lock,
  a full `test:unit` waiting 16 minutes and a Playwright run 15.
- **T-22 (2026-10-10, the user).** A done pull request into an integration branch merges
  without asking; a release to `main`, a hotfix or a critical PR (security, data loss,
  authentication, a migration rewriting populated tables) goes to the user first. The
  pending R4 product decisions take their recommended defaults: the class-average floor is
  three, DG-135's wording stays as built, grade changes are not audited, and alt text is
  optional.
- **T-23 (2026-10-10).** W3-PLAN: the remaining frontend tasks run in three file-disjoint
  lanes (R4 status, "Order"). Frontend PRs merge one at a time. Router edits land in the
  order 27a, 39, 43, 40/41, 48; 30 precedes 31b (`StudentPreview`), and 34 precedes 66b
  (`gap-bindings.test`).
- **T-24 (2026-10-10, the user).** D4 absorbs the v2 export and builds the AI assistant on
  Anthropic's API, sending each feature only what it needs, from a $200 monthly pool that
  only an Admin's approval extends (70 §1, D19 to D22). Q28 to Q35 in `74d` are accepted as
  recommended; Q35 keeps two items open, the Test intro sentence (the user's wording) and
  the data-retention setting on the provider account. No model identifier appears in code
  or docs: the tiers are named by their environment variables.
- **T-25 (2026-10-10).** The tests list (T-R4.29) restores "Select all on this page" in the
  bulk bar once a card is ticked; its selection stays global across pages, tabs and
  searches, and the bar says how many selected items are not shown (QA-29-2, QA-29-3; F-36).
- **T-26 (2026-10-10).** Every character the UI draws is in the loaded font faces, and a
  symbol the font lacks is an icon; content is shown in NFC. FONT-1 writes both into spec
  §12 and pins the first with `font-coverage.test.ts`; the server composes stored text to
  NFC in F-37.
- **T-27 (2026-10-10).** A PR's CI runs on its merge ref, so a fix on the base reaches a
  PR's next run without merging the base into the branch (F-29). Merge the base into a
  branch for a conflict, before a reviewer's or the tester's run, or before the merge
  when the base has moved since the last green run.

- **T-28 (2026-10-10).** After INC-1, every commit is checked before it is made:
  `git status` and `git diff --cached --stat` list only the files meant, paths are added
  explicitly (no `add -A`, `add .` or `commit -a`), commits someone else pushed are taken
  with `fetch` and `merge --ff-only` after committing one's own work (never `reset --soft`,
  `update-ref` or `branch -f` under an older working tree), and before a push the diff
  against the integration branch lists only the task's files. The Tech Lead's worktrees
  are detached and push with `HEAD:<branch>`, so no branch is checked out twice.
- **T-29 (2026-10-10).** A security-sensitive PR (authentication, sessions, decoding of an
  upload, credentials) gets the Principal's read-only security review before it goes to the
  user (T-22). Its verdict and findings are posted on the PR, and a major finding is fixed
  in the PR, not followed up.
- **T-30 (2026-10-10).** When several lanes add rows at once, the Tech Lead reserves
  design-gap numbers per lane (DG-146 for T-R4.33, 147 to 149 for lane 2, 150 to 152 for
  lane 3). Spec versions are settled at merge: the second of two PRs to merge renumbers.
- **T-31 (2026-10-10).** Teacher wall-clock inputs stay in Asia/Ho_Chi_Minh until T-R4.43
  moves them all to the account's zone at once (spec v0.57). T-R4.27b's wizard follows this,
  and T-R4.43 carries the Done-when line.
- **T-32 (2026-10-10).** A tester on a live stack logs in once per role and reuses the
  Playwright `storageState`, so the sign-in limiter never stops a verification. An agent
  refused a permission stops and reports it; the Tech Lead does not perform that action for
  it and tells the user (VER-30).
- **T-33 (2026-10-10).** A PR that changes no behaviour and whose live spec CI runs
  (T-R4.31b-2) may merge on the Tech Lead's review without a VER round; T-R4.51 measures
  every route against the deck again.

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
| F-13 | After a container restart the session-start hook can fail to start the database and MinIO ("container with given ID already exists", stale runc state); `docker compose up -d --force-recreate db minio` recovers with the volumes intact. | senior_swe_platform | open: the hook should detect it and recreate |
| F-14 | The weekly usage limit stopped every agent on 2026-10-08 at about 04:15 UTC until 2026-10-09 11:00 UTC, with four agents running. | Tech Lead | closed: T-3 |
| F-15 | The shared SegField and Segmented fill labels sit left where the deck centres them. | senior_swe_frontend | open: the next task touching forms, or T-R4.48 |
| F-16 | `copy-field.test.tsx` "copies the value and says so for two seconds" failed once on CI (fake timers with `shouldAdvanceTime`). | senior_swe_frontend | open |
| F-17 | T-R4.47 must also carry `mediaAlt` on the server side of the attempt review (`review_review.go`, `toAPIReviewQuestion`); its plan section names only web files. | T-R4.47 | open |
| F-19 | The tester's API inherited the heavy lock from a locked start and held it for 45 minutes. | Tech Lead | fixed: T-13's `-o` |
| F-20 | The reader's bundle imports `@/lib/i18n`, which bundles both locale dictionaries, so every string added anywhere grows the student's budget. Proposal: load the inactive locale lazily. | senior_swe_frontend, principal_swe | fixed (#460, #463) |
| F-21 | Dialogs opened from a menu item without `returnFocus`: the rebuilt AssignmentDetailPage (Close early, Reopen) and the pages not yet rebuilt. | each rebuild task; a small fix for AssignmentDetailPage | open |
| F-22 | A new accessible name containing an existing field label shadows a bare `getByLabel` in `*.live.spec.ts`, which `pnpm e2e` does not run. | every brief | closed: grep the live specs when a label changes; prefer `getByRole` with `exact` |
| F-23 | The join-code rotation and a redemption took the class and code rows in opposite orders and deadlocked; the retry could deadlock again. | senior_swe_backend | fixed (#438): the writers take the class row `FOR NO KEY UPDATE` |
| F-24 | A NUL character in any free-text field answers 500 (PostgreSQL refuses 0x00 in `text`). | senior_swe_platform | open: refuse it in `httpx.ValidateRequests` |
| F-25 | The content editor says "ô trống" where the question editor and the blanks list say "chỗ trống". | T-R4.65, or whoever next touches the content editor | open |
| F-26 | The Vietnamese grading score's accessible name reads "2.5", not "2,5". | the next grading copy change | open |
| F-27 | Docker Hub's anonymous pull limit refused `postgres:18` in CI. | senior_swe_platform | fixed on r4 (#437, `mirror.gcr.io`); `develop` and `main` still need the change |
| F-28 | The grading card's prompt is 14/21 where the deck draws 15/24. | R5 deck pass | open |
| F-29 | A PR's CI runs on the merge ref; the planner can skip a later push whose merged tree is the same. | Tech Lead | closed: T-27 |
| F-30 | `fill-blank-editor.test.tsx` timed out on the lazy rich editor's cold import. | senior_swe_frontend | fixed (#439) |
| F-31 | No check caught a Tailwind class that resolves to no token (`bg-accent-soft`). | senior_swe_platform | fixed (#441): `tailwind-classes.test.ts` |
| F-32 | The import review's `changeType` writes "Đúng"/"Sai" for true/false options instead of the canonical "True"/"False". | the imports owner | open |
| F-33 | Spec §7's answer type still lists `{type:'true_false'; value: boolean}`; the engine writes `choice` and the server grades both. | Tech Lead | open: a sentence that the boolean form is legacy input, in the next spec change of the Tech Lead's |
| F-34 | `assignment-sheet-presence.test.tsx`'s cold first test missed `findBy`'s one second under load. | senior_tester | fixed (#443) |
| F-35 | `builder-group-recovery.spec.ts` failed when Back came within ~60 ms of the URL change. | the user | fixed (#449); an app-side guard for the six `useBlocker` callers is optional |
| F-36 | The tests list: select-all, the "not shown" count, and a stubbed e2e for a failed Duplicate and a failed bulk delete (T-25). | senior_swe_frontend, lane 2 | fixed (#464) |
| F-37 | Decomposed (NFD) text from the server still falls back in plain fields such as a test title. Proposal: `content.Normalize` and `norm.NFC` in the command handlers (never passwords, tokens, emails or join codes), and a one-off backfill that skips published snapshots. | senior_swe_backend | open: waits on the user's go, because the backfill rewrites populated tables (T-22) |
| QA-66a-1 | A failed builder autosave crashed the page (React #185). The cause was `MarqueeText` flipping as the "Not saved" label shrank the title. | senior_swe_frontend, lane 1 | fixed (#452) |
| R12b-1 | `updateAssignment` and `reopenAssignment` change an assignment's window without recomputing in-progress deadlines or taking the windows lock. | senior_swe_backend | fixed (#451) |
| F-38 | The tag search box's radius is 10px where the deck draws 8. | the next task touching `TagCombobox` | open |
| F-39 | The long relative time ("Updated …") wraps in table cells; a short form for cells is a decision across screens. | Tech Lead | open |
| F-40 | Builder and integration tests time out under parallel load (`editor-pane`, `grading-finish-recovery`, `assignment-note-recovery-focus`, `intervention-return-focus`); each passes alone. | senior_tester | open: raise a timeout in its own commit, never loosen an assertion |
| F-41 | Teacher-only namespaces sit in the student's dictionary (about half of it). | R5 | open |
| F-42 | The export limiter is a token bucket; the contract's wording could say so. | senior_swe_backend | open |
| F-44 | Media copy: raw bytes in a size refusal, "This file cannot be used" for a server failure, and `media.empty` naming listening files in the image library. | senior_swe_frontend | open |
| F-45 | A phone photo (4032 x 3024) is over the avatar's 2048 side; the client downscales on a canvas before uploading. | T-R4.43 | open |
| F-46 | "Publish version {n}" counts from the versions listed, not from `last_published_version + 1`, so it is wrong after the newest version is deleted (measured in VER-30). | senior_swe_backend | in progress: `Test.nextVersion` |
| F-47 | `Segmented` is 30px under `data-scale="deck"`, where the deck draws 32 (QA-27b-2). | senior_swe_frontend | open |
| F-48 | `DateTimePicker` edits through a browser-local `Date` (a DST hour is unreachable in New York), a save drops seconds, and `NumberStepper` does not select on Enter. | senior_swe_frontend | open |
| F-49 | Two tabs of one device refresh separately, so an epoch bump can trip reuse detection (R-06 residual, about 0.5% per revoke per extra visible tab). | senior_swe_frontend with the Principal | open: a cross-tab refresh lock in `client.ts`, a canary area |
| F-50 | `FormDialog` puts initial focus on Close, not on the first field. | senior_swe_frontend | open |
| F-51 | `listTestVersions` answers 200 with no items and `previewTest` 409 for a missing or another teacher's test; consistent with a missing id, but not a 404. | senior_swe_backend | open |
| INC-1 | `909115d4` on #469 reverted 49 files: it was committed from a working tree older than the Tech Lead's merge of the integration branch. The gates passed, because the old tree is self-consistent. | Tech Lead | fixed (`31c37362`) before the merge; T-28 |
| SR-472-1 to 4 | The Principal's security review of #472: a progressive JPEG of near-empty scans holds a decode slot for minutes (major); the PNG estimate misses Adam7 and `tRNS`; a decoder panic leaves no trace; a 2 MiB file cannot be uploaded. | senior_swe_backend | fix round on #472 |
| QA-34-6 | In the material editor, an image inserted at an audio block's cursor seemed to replace the audio block (VER-34, observation 6). | senior_swe_frontend, lane 3 | investigating in #471's round 2 |

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
  because the wrapped error matches both. Fixed in `4ff93029`.
- **REV-63 (2026-10-08, `principal_swe`, #424 at `e8c1ef5a`).** Approve after F1, no split:
  F1 (major) the link popover selects its "https://" prefix on open, so every bare address
  typed is refused; F2 the word count reads the whole document on each transaction; F3 the
  link button announces as a toggle and a dialog opener; F4 the paste notice hard-codes
  Ctrl+Shift+V. The lazy link panel (156 KiB against the 160 KiB editor budget) and the
  notice kinds are accepted. Lesson for 65 and 66: 63 ran 1,110 lines against 70 §3's ~800,
  and the paste half was the seam.
- **W0-64 (2026-10-08, `principal_swe`).** T-R4.64 registers only the GFM table and
  strikethrough extensions in one `gfmSubset` plugin (not `remark-gfm`, whose autolinks
  would turn a student prompt's URL into a link), states both bundle budgets before and
  after, keeps the stored form as the mode, and removes `VITE_RICH_QUESTION_EDITOR` from
  code, config and docs. `OptionField` is not 64's.
- **VER-DRAFTS (2026-10-08, `senior_tester`, #414 at `1f270726`, #416 at `0f7eefd1`).**
  Both stay drafts. #414: the title bar wraps where the deck keeps one row, and the English
  copy is not the deck's (major); geometry, drag feedback, ARIA values, the latency of
  "Add section" and focus return (minor). #416: the workspace does nothing under StrictMode,
  "Save & next" skips the student's own next answer, and the shortcuts stop after the first
  key (major); copy and geometry, the two-tab overwrite (T-11), and the test gaps that let
  them through (minor). The autosave flushes, the drag paths, the split pane, Finish and
  Retry, and reload recovery beyond 100 candidates all pass.

- **2026-10-09 to 2026-10-10, in brief.** Every merge above had a review and an
  independent verification, recorded in its PR:
  - REV-64a and REV-64bcd (the Principal): one change required, R64c-1 (T-18);
  - REV-11: R11-1 became the user's class-average floor (T-22);
  - REV-F23 (join codes, high-risk);
  - REV-TF: RTF-1, true/false options with plain content;
  - REV-416: R416-1, an unsaved grading comment lost on navigation;
  - REV-414: R414-1, the dead `bg-accent-soft` that led to F-31;
  - REV-12a: R12a-5 to 7, carried into 12b;
  - the Tech Lead's own reviews: R416-6, R66a-1, R12b-1.

  The tester's VER-35, 62a, 64, 11, 414b and 414c, 416b to 416d, 66a and 29 each passed
  with findings, and every major one was fixed before its merge. The two drafts the team
  took over (#414, #416) never had a recorded code review before that; they got REV-414 and
  REV-416 before merging.

- **2026-10-10, afternoon.** VER-27b, VER-31b, VER-30 and VER-34 each passed with findings,
  and no finding was major; each minor one was fixed before its merge or filed (F-46 to
  F-48, F-50, F-51). VER-30 stopped early when the tester's restart of its own API was
  refused (T-32); CI's live job and T-R4.51 cover what it did not measure. The Principal's
  security review (T-29): #472 approve with changes (SR-472-1, major), #473 approve, with
  SR-473-1 and SR-473-2 recorded in T-R4.9's "As built".

## Next action

As each hand-off arrives:
- **#471** merges after round 2 and green CI; lane 3 then resumes T-R4.46.
- **#474** merges after VER-33; lane 1 takes the next task (24 or 40).
- **#472** goes to the user once SR-472-1 to 4 are fixed and CI is green; **#473** waits on
  the user. Whichever of the two merges second resolves the pinned counts, the golden, the
  per-actor table and the generated code.
- **F-46** opens as a PR; backend 2 then takes T-R4.20.
- **Lane 2** finishes T-R4.38, then 39.
- **F-37** waits on the user's go.
