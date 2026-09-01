# Deploying

Merging `release/*` into `main` ships. Nothing else does, and nothing deploys
from `develop`.

```
feature/*  →  develop  →  release/x.y  →  main  →  CI  →  Deploy
                                                    │
                                          only if CI is green
```

`.github/workflows/deploy.yml` runs on `workflow_run` after **CI** concludes on
`main`, not on the push itself. That ordering is the point: a merge that breaks
something is not deployed while its own test run is still red.

## What it does

| Job | Target | How |
|---|---|---|
| `api` | Fly.io `quizzivy-api` | `flyctl deploy --remote-only` |
| `web` | Cloudflare Pages `quizzivy-web` | `pnpm build` then `wrangler pages deploy` |

The API goes first. The SPA is the half that calls the other, so the window
between the two deploys is old-SPA-against-new-API rather than the reverse — and
the OpenAPI contract only ever grows, so that direction is safe. The reverse is
not: a new SPA calling an endpoint that has not shipped yet is a broken screen.

Migrations ride along with the API. `fly.toml`'s `release_command` applies them
before the new version takes traffic and rolls the deploy back if they fail,
which is why a schema change and the code that needs it must ship as one merge.

## What you have to set up once

The workflow fails with a named error before touching production if any of these
is missing, rather than half way through.

```bash
# Fly — a deploy-scoped token, not your personal one
fly tokens create deploy -a quizzivy-api
gh secret set FLY_API_TOKEN

# Cloudflare — an API token with "Cloudflare Pages: Edit" on this account
gh secret set CLOUDFLARE_API_TOKEN
gh secret set CLOUDFLARE_ACCOUNT_ID

# Not a secret: it travels in the authorization URL the browser follows.
# A variable, so it is readable in logs when a build looks wrong.
gh variable set VITE_GOOGLE_CLIENT_ID --body '<client-id>.apps.googleusercontent.com'
```

`VITE_GOOGLE_CLIENT_ID` is worth the fuss because a build without it does not
fail — it just hides the Google button, which is the kind of breakage nobody
reports and everybody works around.

Check what is set:

```bash
gh secret list && gh variable list
```

## The preflight

Before anything is built, the API job unions `fly.toml`'s `[env]` with
`flyctl secrets list` and checks that the result is a configuration the app
accepts. It reports missing names in seconds instead of after a build, a
migration, a rollout and a health check.

It exists because `config.Load` has two all-or-nothing groups — Google sign-in
and object storage — and each exits 1 on a partial set. Both shipped partial,
one after the other, and the cause was legible only in the app's own stderr:
Fly reports it as "the app appears to be crashing" and prints an empty log tail,
because the machine is gone before it attaches.

Names only. A secret set to the wrong *value* passes here; that is a different
failure with a different symptom.

## The build-time trap this guards against

`VITE_*` values are inlined into the bundle at build time; there is no runtime
override. `src/lib/api/client.ts` falls back to `http://localhost:8080` when
`VITE_API_BASE_URL` is unset, so a mis-scoped variable produces a bundle that
builds, uploads, serves, and cannot reach anything — with no error anywhere.

The `web` job greps the built bundle for `https://api.quizzivy.com` and fails if
it is absent. Do not remove that step.

## Running it by hand

Actions → **Deploy** → *Run workflow*, with a target of `all`, `api` or `web`.

Two reasons this exists:

- **A failed deploy can be retried on its own.** An expired token or a Fly
  region hiccup should not cost twenty minutes of tests that already passed on
  the same commit.
- **The very first merge may not trigger it.** `workflow_run` reads the workflow
  definition from the default branch, so `deploy.yml` has to already be on
  `main` for a run to fire. It arrives there in the same merge it would fire on.
  If nothing happens after the first release, dispatch it by hand; subsequent
  merges are automatic.

## Cutting a release

```bash
git checkout -b release/0.1 develop
# only fixes on this branch -- no new features
git checkout main && git merge --no-ff release/0.1
git push origin main            # CI runs, then Deploy
git checkout develop && git merge --no-ff release/0.1
git branch -d release/0.1
```

Merge back into `develop` too, or fixes made on the release branch are lost from
the next one.

## Verifying afterwards

The workflow checks `/healthz` reports `database:ok` before it calls the API
deploy done. The rest is by hand:

```bash
curl -s https://api.quizzivy.com/healthz
curl -o /dev/null -w '%{http_code}\n' https://app.quizzivy.com/join/K7M3-P9QR   # 200, the SPA shell
GOOGLE_REDIRECT_URI=https://app.quizzivy.com/auth/google/callback make verify-google
```

The deep-link check is not decoration: `web/public/_redirects` is what makes a
QR-code link work on a cold load, and it fails silently at the CDN if it is ever
dropped from the build output.

## Still manual

Both need dashboard access the project's tokens do not have. See `dns.md`:

1. `api` — add the A/AAAA records **DNS only**, wait for
   `fly certs check api.quizzivy.com`, then switch to proxied.
2. `app` — Pages project → Custom domains → add `app.quizzivy.com`. Cloudflare
   creates the record itself; making it by hand returns 522.
