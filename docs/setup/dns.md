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

**Blocked on choosing where the API runs.** A CNAME needs a target, and there
is no deployment yet. See "Hosting" below.

Once targets exist, both records are proxied (orange cloud) so TLS terminates
at the edge:

```
CNAME  app   ->  <SPA host>       proxied
CNAME  api   ->  <API host>       proxied
```

One caveat for the API record: Cloudflare's proxy imposes a request-body limit
(100 MB on Free, well above §11.1's 10 MB cap) and its own timeouts. Neither
binds here, but if a future upload grows, check them before blaming the app.

---

## Hosting

### Database — decided: Neon, Singapore, PostgreSQL 18

§13.7 already assumes Neon ("On Neon, a branch per migration, reset from parent
between runs"), and the two things that could have invalidated that are both
fine as of 2026-08-28:

- Neon runs **18.6** — the same minor this project has been developed and
  tested against locally — as a normally supported release, no longer preview.
  §13 hard-requires 18: `uuidv7()` and virtual generated columns do not exist
  before it, and `pg18_test.go` asserts the major version, so a provider stuck
  on 17 would fail immediately rather than subtly.
- Region `aws-ap-southeast-1` (Singapore) is the closest to Vietnam.

The region is **fixed at project creation and cannot be changed**, so pick
Singapore when creating the project.

Neon's branching is worth using for what §13.7 asks: a branch per migration,
reset from the parent between runs, so migrations are tested against
production-shaped data without touching production.

### SPA — Cloudflare Pages

Static output from `pnpm build`. Already on the account, free, and the custom
domain is one click once DNS is on Cloudflare.

One required setting: the SPA uses client-side routing, so every unmatched path
must serve `index.html`. Add `web/public/_redirects`:

```
/*  /index.html  200
```

Without it, a deep link like `/join/K7M3-P9QR` — the one URL students are most
likely to open cold, from a QR code or a message — 404s at the edge before React
ever loads.

### API — open (O-16)

The one piece still undecided. See `docs/plan/40-open-items.md`.
