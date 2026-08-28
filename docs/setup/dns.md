# DNS for quizzivy.com

**Not needed for Phase 0 or Phase 1.** Local development runs entirely on
`localhost`, and the Google OAuth client already has the localhost redirect
registered and verified. This blocks the first real deploy, nothing before it.

Tracked as O-14.

---

## Current state

Checked 2026-08-28:

| | |
|---|---|
| Nameservers | `ns1–ns4.zonedns.vn` — **not Cloudflare yet** |
| A / CNAME records | none, on the apex or any subdomain |
| Google OAuth | ✅ `https://app.quizzivy.com/auth/google/callback` **is registered** |

The Google half is confirmed mechanically, not by memory: posting a deliberately
invalid authorization code to Google's token endpoint with that redirect URI
returns `invalid_grant` (credentials and redirect accepted) rather than
`redirect_uri_mismatch`. `make verify-google` does the same check for whatever
`GOOGLE_REDIRECT_URI` is set to.

---

## Step 0 — move the domain to Cloudflare

Do this first and independently of everything else: nameserver changes take up
to 24 hours to propagate, so starting it early costs nothing and removes it from
the critical path.

1. Cloudflare dashboard → **Add a site** → `quizzivy.com` → **Free** plan.
2. Cloudflare scans for existing records. There are none, so the list will be
   empty — that is expected, not an error.
3. Cloudflare shows two nameservers, e.g. `xxx.ns.cloudflare.com`.
4. At the current registrar (the one behind `zonedns.vn`), replace all four
   `nsN.zonedns.vn` entries with Cloudflare's two.
5. Wait for Cloudflare to report **Active**. Verify from here with:

   ```bash
   dig +short NS quizzivy.com @1.1.1.1
   ```

Nothing breaks while this propagates, because nothing is served from the domain
yet.

**Why Cloudflare rather than staying at the current registrar:** R2 is already
on this account, so the media bucket and DNS end up in one place; and apex
CNAME flattening removes the usual "you cannot CNAME the apex" problem if the
SPA ever moves to `quizzivy.com` itself.

---

## The layout

| Host | Serves |
|---|---|
| `app.quizzivy.com` | the SPA |
| `api.quizzivy.com` | the Go API |
| `quizzivy.com` | nothing yet — kept free for a landing page |

Both hosts must stay under `quizzivy.com`. That is not cosmetic: it is what
makes them same-*site*, which is what makes §5.2's
`httpOnly; Secure; SameSite=Lax; Path=/auth` refresh cookie work on a
cross-origin request (`docs/plan/00-overview.md` §4.1).

Split them across two registrable domains and `Lax` cookies stop being sent
entirely — sessions die after 15 minutes with no error anywhere. That is R-07,
and it is the kind of failure that survives code review because nothing looks
wrong.

---

## Step 1 — the records

| Name | Type | Target | Who creates it |
|---|---|---|---|
| `app` | CNAME | `<project>.pages.dev` | **Cloudflare, automatically** |
| `api` | CNAME | `quizzivy-api.fly.dev` | you, by hand |

### `app` — Cloudflare Pages

**You do not create this record.** That is the part worth being explicit about:
Pages owns both the DNS record and the certificate. The flow is

1. Create the Pages project.
2. In the project → **Custom domains** → add `app.quizzivy.com`.
3. Cloudflare creates a CNAME to `<project>.pages.dev` and issues the cert.

There is no value for you to type into a Target field; if you create the record
by hand first, Pages will refuse the custom domain because the name is already
taken.

Two ways to create the project:

- **Git integration** — connect the GitHub repo, build command `pnpm build`,
  output directory `dist`, root directory `web`. Requires the repo to be pushed,
  and gives preview deploys per branch.
- **Direct upload** — `pnpm build && npx wrangler pages deploy dist
  --project-name quizzivy-web` from `web/`. No GitHub needed, good for a first
  smoke test.

### `api` — Fly.io, and the certificate bootstrap

There is one ordering trap here. Fly needs to issue a certificate for
`api.quizzivy.com`, and if the record is already proxied, Cloudflare answers the
ACME challenge instead of Fly, so issuance hangs.

Do it in this order:

1. `fly launch --no-deploy` (or `fly apps create quizzivy-api`), then
   `fly deploy`. `fly.toml` is already in the repo and pins `primary_region = "sin"`.
2. Add the CNAME in Cloudflare as **DNS only** (grey cloud):
   `api` → `quizzivy-api.fly.dev`
3. `fly certs add api.quizzivy.com`. Fly prints the exact records it wants —
   follow them; a `_acme-challenge` CNAME may be requested, which is fine to add
   as DNS-only.
4. Wait for `fly certs show api.quizzivy.com` to report the certificate issued.
5. **Then** switch the `api` record to **proxied** (orange cloud), and set
   Cloudflare SSL/TLS mode to **Full (strict)** — Fly now has a real certificate,
   so strict verification passes.

If you skip step 5 and leave `api` on DNS-only, that is a defensible choice
(fewer moving parts, one less hop). It changes one setting — see below.

### The setting that depends on the proxy decision

`CLIENT_IP_HEADER` must name the header set by whatever sits directly in front
of the app:

| `api` record | `CLIENT_IP_HEADER` |
|---|---|
| proxied (orange) | `CF-Connecting-IP` |
| DNS only (grey) | `Fly-Client-IP` |

`fly.toml` currently sets `CF-Connecting-IP`, matching the proxied setup above.
Change it if you leave the record grey.

**Never set this to `X-Forwarded-For.`** Proxies *append* to that header, so a
client can send its own value and have the real address appended after it —
letting it pick a fresh rate-limit bucket per request and defeat §6.5 entirely.
`config.Load` refuses to start if `CLIENT_IP_HEADER` is `X-Forwarded-For`, and
`internal/ratelimit/clientip_test.go` covers the forging case end to end.

### Cloudflare proxy limits worth knowing

Neither binds today, but check them before blaming the app:

- Request body: 100 MB on the Free plan, well above §11.1's 10 MB media cap.
- Origin timeout: 100 seconds. The longest thing this API does is a media
  upload, which is capped at 10 MB.

---

## Secrets on Fly

Set these with `fly secrets set`, never in `fly.toml` — that file is committed.

```
DATABASE_URL           postgres://quizzivy_app:...@<neon-host>/quizzivy?sslmode=require
MIGRATE_DATABASE_URL   postgres://quizzivy_migrate:...@<neon-host>/quizzivy?sslmode=require
JWT_SIGNING_KEY        openssl rand -base64 48
GOOGLE_CLIENT_SECRET   from the OAuth client
R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY
```

Two roles, two URLs, deliberately (§13.5). `MIGRATE_DATABASE_URL` is used only
by the release command; the running API connects as `quizzivy_app`, which cannot
run DDL. `sslmode=require` because Neon is over the public internet.

## Deployment shape

`Dockerfile` builds two static binaries into a distroless image (36 MB): the API
and a small migration runner. The runner exists instead of shipping the goose
CLI, which links drivers for MySQL, SQLite, Turso, Vertica and YDB — a dozen
dependencies this project does not use, in a binary that runs against a public
deployment. It uses the same goose library at the same version, with only the
Postgres driver attached.

`fly.toml` runs `migrate up` as a `release_command`, so the schema is applied
before a new version takes traffic, and rolls back the deploy if it fails.

`min_machines_running = 1` and `auto_stop_machines = false`: this app has a
server-authoritative timer and a 1.5-second autosave loop, so a cold start when
a class all begins a test at once is both slow and visible. At one
shared-cpu-1x machine, never sleeping is a couple of dollars a month.

## Verify after the first deploy

```bash
curl -s https://api.quizzivy.com/healthz            # {"database":"ok","status":"ok"}
GOOGLE_REDIRECT_URI=https://app.quizzivy.com/auth/google/callback make verify-google
make verify-r2
```
