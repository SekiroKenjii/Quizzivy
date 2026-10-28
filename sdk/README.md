# @quizzivy/sdk

The Quizzivy API contract as a package: `api/openapi.yaml` and what is generated
from it, for any client of the API. Types and constants only; it has no
runtime client and no dependencies. Decision D-080.

| File | What it is |
|---|---|
| `openapi.yaml`, `openapi.json` | the contract, as written and as JSON |
| `schema.d.ts` | `paths`, `operations`, `components` (openapi-typescript) |
| `operations.js` / `.d.ts` | `OPERATIONS` (every operationId with its method and path), `OperationId`, and `CONTRACT_VERSION` (the contract's `info.version`) |
| `index.js` / `.d.ts` | the entry point, re-exporting the above (hand-written) |

Everything except `index.*`, this README and `package.json`'s fields other than
`version` is written by `make gen`. Never edit it by hand: change
`api/openapi.yaml` and run `make gen`. CI fails on drift.

## Versions

The SDK's version is the app release its contract ships in: `info.version` in
`api/openapi.yaml`, set only in the release PR next to `web/package.json`'s
version. Releases only add to the contract, so **SDK X works with any server at
X or later**. A release that has to break the contract bumps the minor while
0.x (the major from 1.0), and a patch hotfix never breaks it.

| Branch | Published as | dist-tag |
|---|---|---|
| `main`, once production serves it | the release, when the version and the contract are new | `latest` |
| `develop` | `0.13.3-dev.202610281200.gabc1234`, on pushes that change the contract | `next` |

`.github/workflows/sdk-publish.yml` publishes to GitHub Packages, only from the
Quizzivy org. A release publishes from `deploy.yml`, after the API is live, so no
client adopts a contract production does not serve yet (D-081). A `-dev` version's `CONTRACT_VERSION` is still the release it
builds on.

## In this repository

The web links the package (`"@quizzivy/sdk": "link:../sdk"`) and imports the
types through `web/src/lib/api/schema.ts`, so a contract change is one PR and
`make gen`, with nothing to publish or install.

## In another repository

```ini
# .npmrc: GitHub Packages needs a token even to install
@quizzivy:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${GITHUB_TOKEN}
```

```jsonc
// package.json
"devDependencies": { "@quizzivy/sdk": "0.13.2" },
"scripts": {
  // "gen sdk": take the newest release and let the compiler say what changed
  "sdk:sync": "pnpm up @quizzivy/sdk@latest && pnpm typecheck",
  // try the contract on develop before it ships
  "sdk:next": "pnpm up @quizzivy/sdk@next && pnpm typecheck"
}
```

```ts
import type { components, paths } from "@quizzivy/sdk";
import { OPERATIONS, CONTRACT_VERSION } from "@quizzivy/sdk";

type Test = components["schemas"]["Test"];
```

`openapi-fetch` turns `paths` into a typed fetch client if you want one. In CI,
give the workflow `packages: read`, and in the org's package settings give the
client repository read access. Renovate or Dependabot watching `@quizzivy/sdk`
opens the bump PR on its own, so syncing becomes merging that PR once its
typecheck is green.

A client in another language pins a version and generates its own code from the
contract, for example:

```bash
npm pack @quizzivy/sdk@0.13.2 && tar -xzf quizzivy-sdk-0.13.2.tgz package/openapi.yaml
openapi-generator-cli generate -i package/openapi.yaml -g dart -o lib/api   # or swift5, kotlin, ...
```

## Changing the contract

1. Edit `api/openapi.yaml`, then `make gen`, and commit the generated files.
2. `make sdk-check` (also run by CI's Contract job on every PR) reports
   breaking changes since the merge-base with `SDK_BASE` (default
   `origin/develop`), plus the changelog.
3. A breaking change needs the `contract-breaking` label, a D-entry or the
   owner's approval cited in the PR body, and the release-notes line for tabs
   loaded before the deploy. Prefer expand, then contract: add the new field or
   operation, ship the clients, remove the old one in a later release.
4. False alarms from oasdiff go in `api/oasdiff-ignore.txt`, with a comment.

The gate is report-only until it has been run over the contract's history; it
then blocks.
