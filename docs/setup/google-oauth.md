# T-0.2 — Google OAuth client

Blocks the Phase 1 Google path (T-1.5 onward), which is **every student's
sign-in** and the only self-signup route (§6.3). There is no workaround: the
authorization-code exchange needs a real client secret, and E2E 3 mocks the GIS
widget, not the exchange.

Owner: **Thuong**. Verify with `make verify-google` when done.

---

## Before you start: one thing the spec assumes that needs deciding

Spec §2 picks **Google Identity Services** as the library and §5.3 requires
**Authorization Code + PKCE**. Those two may not be compatible, and it is worth
knowing before you configure the console.

What was verified, from Google's own discovery document:

```
code_challenge_methods_supported: ["plain", "S256"]
```

So **Google's authorization server fully supports PKCE**. The problem is the GIS
JavaScript wrapper: `google.accounts.oauth2.initCodeClient` has no documented
parameter for passing a `code_challenge`. Google's own docs for the code model
and for the web-server flow do not mention PKCE at all.

Two ways forward:

| | Approach | Trade-off |
|---|---|---|
| **A** | GIS `initCodeClient`, no PKCE | Keeps §2's library. Drops §5.3's PKCE. The code passes through browser JS, so an XSS could steal it — though redeeming it still needs the client secret, which never leaves the backend. |
| **B** | Build the authorization request ourselves | Keeps §5.3's PKCE. Deviates from §2's stated library, which AGENTS.md says needs your approval. About 40 lines: generate a verifier, S256 challenge, `state`, open the popup, handle the callback. |

**Recommendation: B.** PKCE is cheap to do properly, it is what §5.3 asked for,
and it removes a dependency on undocumented GIS behaviour. We would still render
a "Tiếp tục với Google" button — §12 dictates our own button styling anyway.

**This does not block the console setup.** The instructions below register both
the JavaScript origins (option A needs these) and the redirect URIs (option B
needs these), so either choice works afterwards. Decide before T-1.5.

---

## Console steps

<https://console.cloud.google.com>

### 1. Project

Create one, or reuse an existing one. Name it `quizzivy`.

### 2. OAuth consent screen

- **User type: External.** Students are outside any Workspace org.
- App name `Quizzivy`, your support email, your developer contact email.
- **Scopes: `openid`, `email`, `profile` — and nothing else.**

  This matters. Those three are Google's *non-sensitive* scopes, and apps that
  request only non-sensitive scopes do not go through OAuth verification review.
  Adding anything else (Drive, Classroom, Calendar) puts the app into
  "sensitive" or "restricted" territory and triggers a review that can take
  weeks. §5.3 needs `sub`, `email`, `email_verified`, `name`, `picture` — all
  covered by these three.

### 3. Publishing status — do not skip this

**Set the app to "In production", not "Testing".**

In Testing status, only users you add by hand to the test-user list can sign in
at all. With ~50 students that means maintaining an allowlist, and every new
student is blocked until you remember to add them — a support burden that will
land on you at the worst possible moment, in the middle of a test.

Because the app requests only non-sensitive scopes (step 2), publishing does not
require submitting for verification. The console tells you inline whether a
review is needed; if it offers to submit for verification, something other than
`openid`/`email`/`profile` crept into the scope list.

Unverified apps show an "unverified app" interstitial for sensitive scopes; with
non-sensitive scopes only, students see the ordinary consent screen.

### 4. Create the client

**Credentials → Create credentials → OAuth client ID → Web application.**
Name it `quizzivy-web`.

**Authorized JavaScript origins** — origins only, no path, no trailing slash:

```
http://localhost:5173
https://app.quizzivy.com
```

**Authorized redirect URIs** — full URLs:

```
http://localhost:5173/auth/google/callback
https://app.quizzivy.com/auth/google/callback
```

Register all four even though only one pair will end up being used. They cost
nothing, and it means the option A/B decision above does not send you back here.

Replace `app.quizzivy.com` with the real host once O-01 is settled. Both must sit
under one registrable domain as the API — that is what makes the `SameSite=Lax`
refresh cookie work (`docs/plan/00-overview.md` §4.1).

### 5. Put the values in `.env`

```
VITE_GOOGLE_CLIENT_ID=<the client id>.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-<the secret>
GOOGLE_REDIRECT_URI=http://localhost:5173/auth/google/callback
```

`.env` is gitignored and must stay that way. The client ID is public and ships
in the frontend bundle (§5.3); **the secret is backend-only** and must never
reach `web/`, a commit, or this chat.

### 6. Verify

```bash
make verify-google
```

This needs no browser and no consent screen. It POSTs a deliberately invalid
authorization code to Google's token endpoint using your real credentials.
Google validates the *client* before the *code*, so the error it returns
identifies the problem exactly:

| Google's response | Meaning |
|---|---|
| `invalid_grant` | **Pass.** The credentials are correct; only the dummy code was rejected. |
| `invalid_client` | The client ID or secret is wrong. |
| `redirect_uri_mismatch` | `GOOGLE_REDIRECT_URI` is not registered on this client. |

---

## Notes for whoever implements T-1.5

- Verify the ID token properly: `iss`, `aud`, `exp`, and the signature via JWKS
  at `https://www.googleapis.com/oauth2/v3/certs`. Cache the JWKS and refetch on
  an unknown `kid`.
- **`email_verified: false` is rejected outright** — no link, no create, even
  with a valid join code. This closes an account-takeover path and §5.1 states
  it without exception.
- Google's `sub` is the stable identity; email can change. `user_identities`
  keys on `(provider, provider_user_id)` and stores `email_at_link` only as a
  record of what it was.
