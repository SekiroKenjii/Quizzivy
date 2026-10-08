# Team ledger

The team's checkpoint. Only the Tech Lead edits this file, and only in the Tech Lead's own
pull requests, never inside a task's PR, so parallel branches never conflict on it.
Between checkpoints the live state is in the session's task list. After an interruption,
start at [README.md](README.md) "Resuming".

## Checkpoint

- **As of:** 2026-10-08.
- **Base:** `work/redesign-r4` at `bb4d4000`, CI green (run 716).
- **Working branch:** `chore/agent-team`, which carries these team artifacts.
- **Objective now (the user, 2026-10-08):** clear the open items, merge this branch into
  `work/redesign-r4`, cherry-pick the team onto `develop`, then take over the project.
- **Next objective:** the team drives R4 to v0.10.0 by the waves below, starting with W0
  and the two red drafts. Merging to `main` still waits for the user's go.

## Roster and models

Requested is what the user asked for. Configured is how this session launched the
agent. Resolved is what the transcript metadata recorded on every turn (README,
"Models").

| Agent | Requested | Configured | Resolved | State |
|---|---|---|---|---|
| Tech Lead | Opus 5.5, high or more | primary session | `claude-opus-5-5`, xhigh (session metadata) | active |
| `principal_swe` | Fable 5.1, high or more | its definition (from 2026-10-08) | `claude-fable-5-1`, high | active |
| `senior_swe_backend` | Sonnet 5.5, xhigh | its definition (from 2026-10-08) | `claude-sonnet-5-5`, xhigh (2026-10-07, by alias) | onboarded |
| `senior_swe_frontend` | Opus 5.5, high or more | its definition (from 2026-10-08) | `claude-opus-5-5`, high (2026-10-07, by alias) | onboarded |
| `senior_swe_platform` | Sonnet 5.5, high or more | its definition (from 2026-10-08) | `claude-sonnet-5-5`, high | active |
| `senior_tester` | Sonnet 5.5, high or more | its definition (from 2026-10-08) | `claude-sonnet-5-5`, high | active |

The definitions in `.claude/agents/` pin the full model ids. On 2026-10-07, the session
that created the directory launched by family alias and effort; from 2026-10-08 the agents
launch from their definitions, and the resolved models match the pins. On 2026-10-07 the
platform engineer was relaunched twice with a checkpoint after server-side API 500s.

## R4 status

Taken from merged pull requests on 2026-10-07. The plan's "Done when" boxes lag this (F-1).

- **Merged:** T-R4.1a–c, 2a–c, 3a–c, 4, 5a–b, 7, 10a, 14, 15, 17a, 17b, 19, 21, 22, 23, 25,
  36, 37, 45a, 51a, 51b, 53, 54, 55, 56, plus the plan corrections (#351, titled T-R4.0,
  not a plan task) and the develop syncs (#344, #352, #364, #379, #396, #402).
- **Open drafts, red on their latest run:** #416 T-R4.28 Grading, #414 T-R4.31a builder
  frame and outline. Both are treated as in-flight work that is not this team's (F-4, F-5).
- **Ready (every dependency merged):** backend T-R4.8, 9, 11, 12, 13, 16, 20; frontend
  T-R4.27a, 32, 35, 46, 57, 63. T-R3.1 to T-R3.3 (v0.9.1) become eligible on 2026-10-10.
- **Blocked:** the rest. Close-out (T-R4.48 to 52) comes last.
- **Next migration number:** 00092. Per-task readiness, sizes and open questions are in
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
`assignments/domain/schedule.go` shared by 11, 12 and 10b; 16's question-identity rule
and a null `unpublishedChanges` when the group graph is refused; 8 uses the standard
library and `DecodeConfig` before decoding; `QuestionEditor` as the one component
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

## Findings and open items

| ID | Finding | Owner | State |
|---|---|---|---|
| F-1 | Plan drift: "Done when" boxes unticked on merged tasks (T-R4.1, 4, 10a, 22, 23, 36, 53, 54, 56 among others); the schema table still says `NNNNN_` for 7, 15 and 55 though 00085 to 00091 exist; T-R4.10b still routes the bell to Grading although T-R4.45a moved that rule to the web; `73-r3.md` says the newest migration is 00079; `AGENTS.md` still describes the teacher console as not yet rebuilt; T-R4.11 asks for a "new DG entry" that exists as DG-71; T-R4.20's first Done-when bullet has shipped; several "Touches" lists omit files (`r4-readiness.md` names them). | senior_tester (boxes); Tech Lead (text, `76a39e1c`) | text fixed; boxes being verified |
| F-2 | Underscores in a definition's `name` are not documented as valid (the docs show hyphens). The next session must confirm that `principal_swe` loads, or rename all five to hyphens. | Tech Lead | closed 2026-10-08: the session loaded the five definitions and launched from them |
| F-3 | v0.9.1 cannot take 00080: `work/redesign-r4` already merged 00080 to 00091, and `develop` ends at 00079. If v0.9.1 takes 00092 and deploys first, production's `cmd/migrate` (`goose.Up`, missing versions not allowed) refuses R4's 00080 to 00091. | Tech Lead | decided by default (T-6, `e4c2916e`) |
| F-4 | #414 is red because two new test files build a `Test` without `skills`, which #415 made required; the contract validator rejects the stub. Fixture-only fix. | senior_swe_frontend | taken over (T-7) |
| F-5 | #416's run 718 ended `failure` with every job green and no **CI result** job, apparently the gate never ran. "Re-run failed jobs" on run 718 is the next step. | senior_swe_frontend | taken over (T-7); GitHub refused a re-run of run 718, so the next push starts a fresh one |
| F-6 | `70` §6 names the teacher sidebar key `quizzivy.column.sidebar`; the code stores `quizzivy.sidebar` (`web/src/layouts/shell/sidebarState.ts`). | Tech Lead | fixed (`76a39e1c`): 70 §6 names both keys and their layouts |
| F-7 | This container has Node 22 and pnpm 10.28; CI uses Node 24, pnpm 11.25.0 and Go 1.27. Only Chromium is available, so the Firefox and WebKit paste checks of T-R4.51 cannot run here. | senior_swe_platform | accepted: `environment.md` lists the differences, and CI stays the authority for Node 24, pnpm 11 and other browsers |
| F-8 | T-R4.5b's ticked "Done when" item (`74-r4.md:1331-1336`) says the `router-chunks` media pattern was narrowed to `features/media/(pages/\|components/(UploadPanel\|AssetLibraryDialog))`; the code still names all of `features/media/` (`router-chunks.test.ts:51`). So T-R4.35 needs no canary edit. | Tech Lead | fixed (`76a39e1c`) |
| F-9 | Setting up the container takes a session's first half hour by hand. The platform engineer proposes a SessionStart hook (dockerd, `.env`, the MinIO image, compose, goose, `pnpm install`, `goose up`, the toolchain pin); it would change every session, so it waits for the user. | senior_swe_platform | decided: build it; in progress |

## Reviews

- **REV-ARTIFACTS (2026-10-07, `principal_swe`, at `50964df5`).** Ten defects: a missing
  full code reviewer for frontend PRs, two schedules that contradicted the ledger, an
  unrecorded split of T-R4.12, two canary runs the plan requires, the owner of #414's fix,
  an unverified recipe step, unmarked tool assertions, and plan corrections kept in a
  survey. All are fixed in the commit after it, except trimming `verification.md` where it
  restates the plan, which is left for its next edit.

## Next action

Ask the user to choose the next objective and settle "Decisions the user owes".
