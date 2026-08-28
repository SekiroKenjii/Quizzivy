# Contributing

Read [`AGENTS.md`](AGENTS.md) first. It is the standing rule set and it is short.

## Branch model

Gitflow.

| Branch | Purpose |
|---|---|
| `main` | Released only, tagged `vX.Y.Z`. Never committed to directly. |
| `develop` | Integration. Every feature PR targets this. |
| `feature/t-<phase>-<n>-<slug>` | One task from a phase file. Branch off `develop`. |
| `release/phase-<n>` | Cut when a phase's tasks are merged. Stabilization only. |
| `hotfix/<slug>` | Off `main`. Merged to **both** `main` and `develop`. |
| `chore/<slug>` | Off `develop`. Repo-wide work that is not a plan task — tooling, layout, dependency bumps. |

```bash
git checkout -b feature/t-1-04-google-oauth develop
# ... work, commit ...
git checkout develop && git merge --no-ff feature/t-1-04-google-oauth
git branch -d feature/t-1-04-google-oauth
```

`--no-ff` always, so each task stays a visible unit in the history.

## One task, one PR

Tasks are defined in `docs/plan/10-phase-0.md` … `15-phase-5.md`. Each carries
its own **Done when** list; that list is the acceptance criteria, and the PR
template repeats the parts that apply to every change.

If a task turns out to be bigger than its stated size (S < 2h, M half day,
L full day), split it and record the split in the phase file. Do not let a task
grow past L.

Order matters: tasks are sequenced so the phase builds and deploys at any task
boundary. Do not merge a task whose dependencies are unmerged.

## Phase completion

When every task in a phase is merged to `develop`:

```bash
git checkout -b release/phase-2 develop
# fix only what the exit criteria surface
git checkout main && git merge --no-ff release/phase-2 && git tag v0.2.0
git checkout develop && git merge --no-ff release/phase-2
git branch -d release/phase-2
```

A phase is done when its **exit criteria** pass — the E2E tests named in the
phase file, not a judgement call. Do not start the next phase with the current
one red (spec §16).

## Commit messages

`T-<phase>.<n>: <imperative summary>`, then a body explaining *why*. English.

Reference the spec by section number (`§13.3`), never by quoting it back.

## Before opening a PR

```bash
make gen        # if api/openapi.yaml changed — commit the generated files
make test
make lint
```

CI runs the same checks plus a codegen drift check and a migration up/down/up
against `postgres:18`. A red CI blocks merge.

## Things that will get a PR sent back

- A generated file edited by hand (`server/gen/`, `web/src/lib/api/schema.d.ts`)
- A new dependency with no reason in the description
- English-only user-facing text
- A public endpoint without both §6.5 boxes ticked in the same PR
- A failing test deleted or skipped rather than fixed
- `SELECT *` in application code
- DDL that contradicts `docs/plan/20-data-model.md` without a new `D-nn` entry

## Open questions

Check `docs/plan/40-open-items.md` before asking. Most questions are already
there with a stated default — build the default and move on. Ask one precise
question only when the answer changes what you build.
