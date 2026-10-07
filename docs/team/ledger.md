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
- **Next objective:** not chosen. The candidates are under "Candidate objectives".

## Roster and models

Requested is what the user asked for. Configured is how this session launched the
agent. Resolved is what the transcript's metadata recorded (README, "Models").

| Agent | Requested | Configured (2026-10-07) | Resolved | State |
|---|---|---|---|---|
| Tech Lead | Opus 5.5, high or more | primary session | `claude-opus-5-5`, xhigh (session metadata) | active |
| `principal_swe` | Fable 5.1, high or more | general-purpose, `model: fable`, `effort: high` | `claude-fable-5-1`, high | onboarding |
| `senior_swe_backend` | Sonnet 5.5, xhigh | general-purpose, `model: sonnet`, `effort: xhigh` | `claude-sonnet-5-5`, xhigh | onboarding |
| `senior_swe_frontend` | Opus 5.5, high or more | general-purpose, `model: opus`, `effort: high` | pending | queued |
| `senior_swe_platform` | Sonnet 5.5, high or more | general-purpose, `model: sonnet`, `effort: high` | `claude-sonnet-5-5` | onboarding |
| `senior_tester` | Sonnet 5.5, high or more | general-purpose, `model: sonnet`, `effort: high` | `claude-sonnet-5-5` | onboarding |

The definitions in `.claude/agents/` pin the full model ids. This session created that
directory, so it could not launch from them and passed the family alias and effort on each
launch instead. The aliases resolved to the requested versions.

## R4 status

Taken from merged pull requests on 2026-10-07. The plan's "Done when" boxes lag this.

- **Merged:** T-R4.0, 1a–c, 2a–c, 3a–c, 4, 5a–b, 7, 10a, 14, 15, 17a, 17b, 19, 21, 22, 23,
  25, 36, 37, 45a, 51a, 51b, 53, 54, 55, 56.
- **Open drafts, red on their latest run:** #416 T-R4.28 Grading, #414 T-R4.31a builder
  frame and outline. Both are treated as in-flight work that is not this team's.
- **Ready (every dependency merged):** backend T-R4.8, 9, 11, 12, 13, 16, 20; frontend
  T-R4.27a, 32, 35, 46, 57, 63.
- **Blocked:** the rest. Close-out (T-R4.48 to 52) comes last.

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

## Candidate objectives

Offered to the user on 2026-10-07. None is chosen.

- **A. Backend wave:** T-R4.16, 11, 12, 13, then 8, 9, 20. Unblocks most of the screens.
- **B. Content editor chain:** T-R4.63 → 64 → 66. The longest serial chain.
- **C. Ready screens:** T-R4.57, 35, 32, 27a.
- **D. The two red drafts:** #416 and #414, if nobody else is driving them.

## Decisions

- **T-1 (2026-10-07).** The team's artifacts are `.claude/agents/` and `docs/team/`. They
  are on `chore/agent-team`, cut from `work/redesign-r4` and aimed at it, because the
  team works on R4 now. They reach `develop` with R4's merge. CONTRIBUTING cuts a `chore/`
  branch from `develop`; this is the one exception, made because of that timing.
- **T-2 (2026-10-07).** Models are pinned by full id in the definitions and checked from
  transcript metadata, never from an agent's own account.
- **T-3 (2026-10-07).** At most four sub-agents run at once (70 §3, "Machine").
- **T-4 (2026-10-07).** The ledger changes only in the Tech Lead's pull requests.

## Findings and open items

| ID | Finding | Owner | State |
|---|---|---|---|
| F-1 | The plan's "Done when" boxes and "As built" notes lag merged work (T-R4.1, 4, 23, 36, 56 among others). | Tech Lead | open |
| F-2 | Underscores in a definition's `name` are not documented as valid (the docs show hyphens). The next session must confirm that `principal_swe` loads, or rename all five to hyphens. | Tech Lead | open |

## Next action

Collect the onboarding reports, then ask the user to choose the next objective.
