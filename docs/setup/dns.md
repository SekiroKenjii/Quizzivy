# DNS for quizzivy.com

**Not needed for Phase 0 or Phase 1.** Local development runs entirely on
`localhost`, and the Google OAuth client already has the localhost redirect
registered and verified. This blocks the first real deploy, nothing before it.

Tracked as O-14.

---

## The layout

| Host | Serves | Why |
|---|---|---|
| `app.quizzivy.com` | the SPA | |
| `api.quizzivy.com` | the Go API | |
| `quizzivy.com` | nothing yet | kept free for a landing page |

Both hosts must stay under `quizzivy.com`. That is not cosmetic: it is what
makes them same-*site*, which is what makes §5.2's
`httpOnly; Secure; SameSite=Lax; Path=/auth` refresh cookie work on a
cross-origin request (`docs/plan/00-overview.md` §4.1).

Split them across two registrable domains and `Lax` cookies stop being sent
entirely — sessions die after 15 minutes with no error anywhere. That is R-07,
and it is the kind of failure that survives code review because nothing looks
wrong.

Serving the SPA from the apex instead of `app.` is equally valid and equally
same-site. See O-14 if you would rather do that; decide before students have
bookmarks.

## Records

Nameservers are already Cloudflare's if the domain is registered there. If not,
moving DNS to Cloudflare is worth it here — R2 is already on the account, and
apex CNAME flattening removes the usual A-record awkwardness.

Two records, both proxied (orange cloud) so TLS terminates at the edge:

```
CNAME  app   -> <frontend host>     proxied
CNAME  api   -> <backend host>      proxied
```

Hosting is not chosen yet, so the targets stay blank for now. Whatever they end
up being, keep both under this one domain.

## Before the first deploy

1. **Google OAuth console.** The client currently has only the localhost origin
   verified. Add:
   ```
   Authorized JavaScript origins:  https://app.quizzivy.com
   Authorized redirect URIs:       https://app.quizzivy.com/auth/google/callback
   ```
   Without these Google rejects the exchange with `redirect_uri_mismatch` —
   which `make verify-google` will catch if you point `GOOGLE_REDIRECT_URI` at
   the production URL and re-run it.

2. **Production `.env`.**
   ```
   CORS_ALLOWED_ORIGINS=https://app.quizzivy.com
   VITE_API_BASE_URL=https://api.quizzivy.com
   REFRESH_COOKIE_SECURE=true
   ```
   `CORS_ALLOWED_ORIGINS` is an exact allowlist and must never become `*` — that
   is illegal with credentials, and this API always sends them.

3. **Re-run both verifications** against production values before announcing the
   URL to anyone.
