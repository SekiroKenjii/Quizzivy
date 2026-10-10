# Agent team

How Quizzivy's multi-agent team runs Phase R work. `AGENTS.md` is the rule set and wins
over anything here; this file adds only who does what, how work moves, and what evidence
closes it. The team's current state is in [`ledger.md`](ledger.md), the verification
environment in [`environment.md`](environment.md), the release's verification strategy
in [`verification.md`](verification.md), and the dated readiness survey of the ready tasks
in [`r4-readiness.md`](r4-readiness.md).

## Roster

One Tech Lead and five specialists. The team is flat: only the Tech Lead creates,
replaces or closes agents.

| Role | Agent | Definition | Model | Effort | Owns |
|---|---|---|---|---|---|
| Tech Lead | the primary session | — | `claude-opus-5-5` | high or more | delivery, scheduling, integration, the ledger |
| Principal Software Engineer | `principal_swe` | [`.claude/agents/principal_swe.md`](../../.claude/agents/principal_swe.md) | `claude-fable-5-1` | high, more for hard calls | architecture, invariants, hard decisions |
| Senior SWE, backend | `senior_swe_backend` | [`.claude/agents/senior_swe_backend.md`](../../.claude/agents/senior_swe_backend.md) | `claude-sonnet-5-5` | xhigh | Go modules, contract, persistence, authorization, concurrency |
| Senior SWE, frontend | `senior_swe_frontend` | [`.claude/agents/senior_swe_frontend.md`](../../.claude/agents/senior_swe_frontend.md) | `claude-opus-5-5` | high, more for complex UI | screens to the deck, client state, accessibility, i18n |
| Senior SWE, platform and integration | `senior_swe_platform` | [`.claude/agents/senior_swe_platform.md`](../../.claude/agents/senior_swe_platform.md) | `claude-sonnet-5-5` | high | environment, CI, migration numbering, codegen, worker, storage |
| Senior Tester | `senior_tester` | [`.claude/agents/senior_tester.md`](../../.claude/agents/senior_tester.md) | `claude-sonnet-5-5` | high | verification strategy, independent evidence, deck fidelity |

### Models

A role name or a sentence in a prompt does not change a model. The model is set in two
supported places, highest precedence first:

1. The Agent tool's `model` and `effort` parameters on one launch. As observed on
   2026-10-07, `model` takes only a family alias (`opus`, `sonnet`, `fable`, `haiku`),
   which resolves to the canonical model of that family, or to the main session's exact
   model when the family matches.
2. The definition's frontmatter, which pins the full model id and the effort.

So a launch from a definition passes no `model`, and the full id in the frontmatter
decides. The ids in the roster are the ones requested; the ledger records what each
launch actually resolved to. A launch without a definition passes the alias and the effort explicitly, and
the resolved model is checked afterwards.

The resolved model is checked from runtime metadata, never from what an agent says about
itself. Every assistant turn in a subagent's transcript records `"model"` and `"effort"`.
The Tech Lead reads them with
`grep -oE '"(model|effort)":"[^"]*"' <output_file> | sort | uniq -c` and records the
requested, configured and resolved values in the ledger. A mismatch is reported to the
user, not silently accepted.

As observed on 2026-10-07, a session that creates `.claude/agents/` cannot launch from
it until the next session, because the watcher covers only directories that existed at
start. Such a session launches with explicit parameters and a self-contained brief.

## Decision rights

- **Tech Lead:** turns the objective into observable acceptance criteria, owns execution,
  scheduling, task ownership and integration, settles routine implementation questions,
  and verifies the combined result before calling anything done.
- **Principal:** owns architectural decisions within the approved scope, and reviews every
  change that touches architecture or an `AGENTS.md` high-risk area.
- **Senior Tester:** owns independent verification and the quality evidence. A task is
  not done on the implementer's word.
- **Senior SWEs:** own the technical quality of their assigned tasks.
- **The user (Thuong):** product decisions, unresolved requirements, and every action
  that needs authorization, including merging to `main`. Internal agreement replaces none
  of these.

Nobody silently waives an acceptance criterion, overrides the spec or the deck, or widens
scope. Check `docs/plan/40-open-items.md` and the release plan's open items first: where
a default is stated, build the default and move on.

## How a task moves

States: `planned` → `ready` → `in_progress` → `in_review` → `in_verification` → `done`, with
`blocked` reachable from any of them. In the native task list, `pending` holds `planned`
and `ready`, `in_progress` holds the three working states (the stage is in the task's
`stage` metadata), and `completed` is `done`. The ledger's table is the durable copy.

1. **Prepare.** The Tech Lead confirms the criteria, dependencies and ownership. The
   Principal agrees the approach for anything high-risk or cross-cutting. The Senior
   Tester names the checks before the work starts. Baseline checks run, and a
   pre-existing failure is recorded as one.
2. **Implement.** One owner per task, on `feature/t-r<k>-<nn>-<slug>` off the release's
   integration branch. The owner adds the tests and runs the focused checks.
3. **Review.** A non-author reviews correctness, maintainability, regression risk, code
   clarity and comment necessity. The Principal also reviews architecture and high-risk
   changes. A finding names a concrete issue and its evidence.
4. **Verify.** The Senior Tester verifies the candidate revision independently, including
   the deck comparison for every changed screen.
5. **Correct.** Every actionable finding becomes owned work. The fix goes to the cause,
   with a regression test where one fits, and the affected checks run again. A finding
   closes only after its retest.
6. **Integrate.** The pull request targets the integration branch and is merged with a
   merge commit once **CI result** passes. The plan's "Done when" boxes and "As built"
   note change in the task's own PR. Merging to `main` waits for the user's go.

### Reviewer pairing

| Author | Code review | Architecture review | Verification |
|---|---|---|---|
| `senior_swe_backend` | `senior_swe_platform` | `principal_swe` when high-risk or cross-module | `senior_tester` |
| `senior_swe_frontend` | `principal_swe`, the whole PR: correctness, the React rules, i18n, accessibility, clarity; `senior_swe_backend` too when it consumes a changed contract | `principal_swe` when high-risk | `senior_tester` |
| `senior_swe_platform` | `senior_swe_backend` | `principal_swe` for CI or migration policy | `senior_tester` |
| `senior_tester` (tests, harness) | the owner of the code under test | — | the Tech Lead checks the evidence |

## Delegation brief

Every assignment carries these fields, and enough context to work without the
conversation that produced it:

```
Task ID:                 T-R4.<n> (and its plan section)
Expected outcome:
Acceptance criteria:     the plan's "Done when", plus anything the Tech Lead adds
Owner:
Independent reviewer:
Relevant references:     plan, spec sections, deck page, gaps entries, prior "As built"
Dependencies:            merged / open, with PR numbers
Owned files or modules:  the only paths the owner writes
Shared contracts:        openapi, generated code, migrations, lockfiles, CI, fixtures, i18n
Required verification:   the commands and levels, from the Senior Tester
Expected handoff:        the definition's handoff section
Repository state:        working directory, branch, base revision
```

## Shared files and resources

- **One writer per file at a time.** A brief names the owned paths. Anything else needs
  the Tech Lead first.
- **Coordinated changes:** `api/openapi.yaml` and the generated code, migrations and
  their numbers, shared interfaces, `go.mod` and `pnpm-lock.yaml`, CI configuration, shared
  fixtures, and the locale files (`en.json`, `vi.json`). The Tech Lead serializes them. A
  migration is numbered before its PR runs CI, because goose refuses `NNNNN_`: the next
  number after the highest on the integration branch, renumbered by its owner if another
  PR merges first. `senior_swe_platform` keeps the register.
- **Isolation.** Parallel implementers work in separate git worktrees (`isolation:
  "worktree"`), and git operations on a shared checkout are serialized. Worktrees do not
  isolate the database server, MinIO, or the ports 5173, 5175, 4173 and 8080: each test run
  uses its own database, and the Tech Lead schedules the browser and the dev servers.
- **Concurrency.** At most five sub-agents run at once, reviews and verifications included
  (the user, 2026-10-10; ledger T-3). Two instances of one role run only in separate
  worktrees and on separate ports (T-10).
- **The heavy lock.** Full suites (`test:unit`, `test:integration`, Playwright, the Go
  integration and e2e tiers) and `vite build` run under one machine-wide lock, so five agents
  do not starve a four-core box:

  ```
  flock -o /tmp/quizzivy-heavy.lock <command>
  ( flock -o 9; <command> ) 9>/tmp/quizzivy-heavy.lock   # where the sandbox refuses the first form
  ```

  Both forms take the same lock. `-o` closes the lock in the child, so a server started by a
  locked command never holds it; still, never start a server under the lock. Take it once per
  suite, not once per agent session. An agent that finds no documented lock asks the Tech Lead
  rather than inventing one. A timeout under load is not a finding: rerun it once under the
  lock, and CI on the pushed head decides.
- **Local gates (G-2).** Before a hand-back or a push, an agent runs the fast checks on its
  changed scope only:
  - `pnpm typecheck`;
  - `eslint --cache` and `prettier --check` on the changed files;
  - `vitest run --changed origin/<integration branch>` plus the tests it added;
  - `go test` of the packages it touched, with `-tags integration` or `-tags e2e` where those
    packages carry them;
  - `make gen-check` when the contract changed;
  - the canaries at the branch point and the head in a high-risk area;
  - `pnpm e2e:live` only when copy a live spec reads changes, or the task is about the API and
    the browser meeting.

  After merging the integration branch into a branch, it always runs `pnpm typecheck` (and
  `go build ./...` for server changes) before pushing (T-20). CI is the authority for the full
  suites, and the Tech Lead merges only on green CI. Small runs (one test file, `--changed`, a
  few files through eslint or prettier, one Go package) take no lock. The engineer spot-checks
  the deck at 1280 light and dark and at 360, and the tester runs the full matrix once, at
  verification. Hand-backs are short: the head, a gates table, findings and open questions; the
  narrative goes in the PR body draft.
- `git add` takes explicit paths only. Never `-A` in a shared checkout.

## Findings

A **review finding** names the file and line, the defect, the evidence and the severity.
A **QA finding** is the Senior Tester's format: ID, task and criterion, severity and
impact, steps, expected, actual, revision and environment, evidence, owner, status,
retest. Both are tracked in the ledger until they close.

## Escalation

The Tech Lead escalates to the Principal when work crosses an architectural or ownership
boundary, when API or persistence compatibility is at risk, when a decision touches
authentication, authorization, grading, attempt integrity or data consistency, when
engineers disagree, or when two repairs of the same finding have failed.

An escalation states: the problem, the evidence and reproduction, the constraints, the
options considered, the recommended next step, and the decision needed.

After three failed repairs of one finding, the approach stops. The problem is
reproduced again, the assumptions are revisited, and the Principal defines a different
strategy or a bounded investigation. If no path remains, the task is marked `blocked`
with its evidence kept, and independent work continues.

The user is asked only for a material product decision, a requirement that cannot be
settled from the documents, missing access, or an action that needs authorization.

## Resuming

After an interruption or a context compaction, the Tech Lead:

1. Reads `ledger.md`.
2. Checks the working tree, the branches and the open pull requests.
3. Lists the agents that are still available, and resumes them by name or id with
   `SendMessage` rather than recreating them. An agent that has to be recreated gets its
   checkpoint, the decisions that bind it, and its current task.
4. Continues unfinished work without redoing what is done.

Nothing unintegrated is deleted as part of cleanup.
