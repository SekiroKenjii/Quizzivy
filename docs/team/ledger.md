# Team ledger

The team's checkpoint. Only the Tech Lead edits this file, and only in the Tech Lead's own
pull requests, never inside a task's PR, so parallel branches never conflict on it.
Between checkpoints the live state is in the session's task list. After an interruption,
start at [README.md](README.md) "Resuming".

## Checkpoint

- **As of:** 2026-10-07.
- **Base:** `work/redesign-r4` at `bb4d4000`, CI green (run 716).
- **Working branch:** `chore/agent-team`, which carries these team artifacts.
- **Objective now:** establish the team and its artifacts. The user asked for no
  implementation in this step.
- **Next objective:** not chosen. The candidates and the decisions the user owes are
  below.

## Roster and models

Requested is what the user asked for. Configured is how this session launched the
agent. Resolved is what the transcript metadata recorded on every turn (README,
"Models").

| Agent | Requested | Configured (2026-10-07) | Resolved | State |
|---|---|---|---|---|
| Tech Lead | Opus 5.5, high or more | primary session | `claude-opus-5-5`, xhigh (session metadata) | active |
| `principal_swe` | Fable 5.1, high or more | general-purpose, `model: fable`, `effort: high` | `claude-fable-5-1`, high | onboarded |
| `senior_swe_backend` | Sonnet 5.5, xhigh | general-purpose, `model: sonnet`, `effort: xhigh` | `claude-sonnet-5-5`, xhigh | onboarded |
| `senior_swe_frontend` | Opus 5.5, high or more | general-purpose, `model: opus`, `effort: high` | `claude-opus-5-5`, high | onboarded |
| `senior_swe_platform` | Sonnet 5.5, high or more | general-purpose, `model: sonnet`, `effort: high`; relaunched twice with a checkpoint after server-side API 500s | `claude-sonnet-5-5`, high | onboarded |
| `senior_tester` | Sonnet 5.5, high or more | general-purpose, `model: sonnet`, `effort: high` | `claude-sonnet-5-5`, high | onboarded |

The definitions in `.claude/agents/` pin the full model ids. This session created that
directory, so it could not launch from them and passed the family alias and effort on each
launch instead. Every alias resolved to the requested version.

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
  [r4-readiness.md](r4-readiness.md); 16 is nearer L and 12 nearer XL (split 12a/12b).

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
| W1 | 11 → 12 → 13, serial (both 11 and 12 edit `assignments/repositories/student.go`) | 64 → 66 | #414 if the user makes it ours | verify 16, 63 |
| W2 | 8, 20, 9, then 62, 18, 10b | 35 → 65 → 62 web | as needed | verify 11, 12, 13 |
| W3+ | review, then a second frontend stream | 31b, 33, 38, 39 | close-out support | route matrix (T-R4.51) |

Approach points to agree before each bundle starts: one `EffectiveClose` in
`assignments/domain/schedule.go` shared by 11, 12 and 10b; 16's question-identity rule
and a null `unpublishedChanges` when the group graph is refused; 8 uses the standard
library and `DecodeConfig` before decoding; `QuestionEditor` as the one component
(DG-108), with its structure reviewed before 66 starts.

## Candidate objectives

Offered to the user on 2026-10-07. None is chosen.

- **A. Backend wave:** T-R4.16, 11, 12, 13, then 8, 9, 20. Unblocks most of the screens.
- **B. Content editor chain:** T-R4.63 → 64 → 66. The longest serial chain.
- **C. Ready screens:** T-R4.57, 35, 32, 27a.
- **D. The two red drafts:** #416 and #414, if nobody else is driving them.

The Principal recommends starting the heads of A and B together (W0).

## Decisions the user owes

1. **v0.9.1 and migration numbers (F-3).** Ship T-R3.1 to T-R3.3 as v0.9.1 from
   `develop` and renumber R4's merged migrations after them, or carry them inside R4
   as T-R4.49.
2. **Unconfirmed defaults on ready tasks:** T-R4.63 leaves pasted images out and counts
   them, changing spec §7.1 (DG-115); T-R4.64 removes `VITE_RICH_QUESTION_EDITOR` for every
   teacher and adds up to four dependencies on the student path (DG-110); T-R4.11's note to
   students; T-R4.54's own-row lists for the Admin (already built).
3. **Who drives #414 and #416.**
4. **Capacity:** a second frontend-capable implementer once the Go work runs out. The
   frontend survey sizes T-R4.57 as L and 63, 35 and 27a as M+.
5. **Criteria the tester could not make testable** (`verification.md` §9): the deck
   tolerance, the effective close under `after_close` with overrides, the class-average
   counting rule, the limits in T-R4.13, the Neon compute-hours threshold.

## Decisions

- **T-1 (2026-10-07).** The team's artifacts are `.claude/agents/` and `docs/team/`. They
  are on `chore/agent-team`, cut from `work/redesign-r4` and aimed at it, because the
  team works on R4 now. They reach `develop` with R4's merge. CONTRIBUTING cuts a `chore/`
  branch from `develop`; this is the one exception, made because of that timing.
- **T-2 (2026-10-07).** Models are pinned by full id in the definitions and checked from
  transcript metadata, never from an agent's own account.
- **T-3 (2026-10-07).** At most four sub-agents run at once (70 §3, "Machine").
- **T-4 (2026-10-07).** The ledger changes only in the Tech Lead's pull requests.
- **T-5 (2026-10-07).** A migration takes its number before its PR runs CI, because goose
  refuses `NNNNN_` (T-R4.10a, "As built"); it is renumbered if another PR merges first.

## Findings and open items

| ID | Finding | Owner | State |
|---|---|---|---|
| F-1 | Plan drift: "Done when" boxes unticked on merged tasks (T-R4.1, 4, 10a, 22, 23, 36, 53, 54, 56 among others); the schema table still says `NNNNN_` for 7, 15 and 55 though 00085 to 00091 exist; T-R4.10b still routes the bell to Grading although T-R4.45a moved that rule to the web; `73-r3.md` says the newest migration is 00079; `AGENTS.md` still describes the teacher console as not yet rebuilt. | Tech Lead, with T-R4.50 or a docs PR | open |
| F-2 | Underscores in a definition's `name` are not documented as valid (the docs show hyphens). The next session must confirm that `principal_swe` loads, or rename all five to hyphens. | Tech Lead | open |
| F-3 | v0.9.1 cannot take 00080: `work/redesign-r4` already merged 00080 to 00091, and `develop` ends at 00079. If v0.9.1 takes 00092 and deploys first, production's `cmd/migrate` (`goose.Up`, missing versions not allowed) refuses R4's 00080 to 00091. | the user decides; platform executes | open |
| F-4 | #414 is red because two new test files build a `Test` without `skills`, which #415 made required; the contract validator rejects the stub. Fixture-only fix. | owner of #414 | reported |
| F-5 | #416's run 718 ended `failure` with every job green and no **CI result** job, apparently the gate never ran. "Re-run failed jobs" on run 718 is the next step. | owner of #416 | reported |
| F-6 | `70` §6 names the teacher sidebar key `quizzivy.column.sidebar`; the code stores `quizzivy.sidebar` (`web/src/layouts/shell/sidebarState.ts`). | Tech Lead, docs | open |
| F-8 | T-R4.5b's ticked "Done when" item (`74-r4.md:1331-1336`) says the `router-chunks` media pattern was narrowed to `features/media/(pages/\|components/(UploadPanel\|AssetLibraryDialog))`; the code still names all of `features/media/` (`router-chunks.test.ts:51`). So T-R4.35 needs no canary edit. | Tech Lead, docs | open |
| F-9 | Setting up the container takes a session's first half hour by hand. The platform engineer proposes a SessionStart hook (dockerd, `.env`, the MinIO image, compose, goose, `pnpm install`, `goose up`, the toolchain pin); it would change every session, so it waits for the user. | the user decides | open |
| F-7 | This container has Node 22 and pnpm 10.28; CI uses Node 24, pnpm 11.25.0 and Go 1.27. Only Chromium is available, so the Firefox and WebKit paste checks of T-R4.51 cannot run here. | senior_swe_platform | open |

## Next action

The Principal reviews the whole artifact set; then ask the user to choose the next
objective and settle the decisions above.
