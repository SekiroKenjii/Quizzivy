# Phase 1 — Auth, join, shells

**Deliverable (§16):** password login, Google login, refresh + rotation,
join-code flow end-to-end, guards, both layouts, settings.

**Exit criteria — corrected.** §16 states "E2E 2, 3, 4 pass". E2E 2 is
*"Student logs in with password → starts → answers → reloads mid-test → answers
persist → submits → sees result"*, which requires test authoring (Phase 2), the
take-test engine (Phase 3) and the result page (Phase 4). It cannot pass here.
The phase exits on **E2E 3, E2E 4, and a new E2E 2a** — password login lands the
student on `/app` with an empty assignment list — which is the part of E2E 2 that
is actually in scope. E2E 2 in full becomes a Phase 4 exit criterion.

Ordered so the phase builds at every boundary: schema, then backend auth, then
the public join surface, then the frontend that consumes it.

---

### T-1.1 — Add the identity and class migrations
**Depends on:** T-0.15
**Touches:** `migrations/`
**Size:** M
**Migrations:** `00004_create_users.sql`, `00005_create_refresh_tokens.sql`,
`00006_create_classes.sql`, `00007_create_class_members.sql`,
`00008_create_audit_log.sql`, `00009_grant_app_role.sql`
**Done when:**
- [ ] Tables match `20-data-model.md` §3–§5 exactly, including deviations D-08,
      D-09, D-10 and D-16
- [ ] `seed/` creates one admin user and one class; never in a migration (§13.7)
- [ ] Test: `migrate_test.go` up/down/up still clean, and **mutation-tested** —
      removing one `DROP TABLE` from a `Down` must fail it
- [ ] That round trip is **opt-in** (`TEST_DESTRUCTIVE=1`, set in CI). It drops
      schema `app`, so leaving it on by default meant `go test ./...` silently
      wiped a developer's seeded database with no hint as to why
- [ ] Constraint tests build their fixtures with **generated** ids, never
      hard-coded ones. The first version used fixed uuids and every case broke
      the moment `make seed` ran — a suite that assumes an empty database only
      passes on a machine nobody has used
- [ ] Test: `server/internal/db/constraints_test.go` — inserting a user with
      `must_change_password = true` and `password_hash = NULL` is rejected (D-16)
- [ ] Test: `constraints_test.go` — two users differing only in email case
      collide on `users_email_lower_key`
- [ ] `00009` grants `quizzivy_app` DML **and** sets `ALTER DEFAULT PRIVILEGES`
      so every later table is covered automatically. It was planned for Phase 3;
      that left the app role unable to read anything until then
- [ ] Test: connected as `quizzivy_app`, `UPDATE app.audit_log` is refused —
      §13.4's append-only claim as a privilege, not a promise
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-1.2 — Implement password login with Argon2id
**Depends on:** T-1.1, T-0.14
**Touches:** `server/internal/auth/`
**Size:** M
**Done when:**
- [ ] `POST /auth/login` verifies against Argon2id (§13.5)
- [ ] A disabled user (`disabled_at IS NOT NULL`) is rejected with the same
      generic message and the same timing as a wrong password
- [ ] Response is `{ accessToken, user }` per §15; the JWT is ~15 minutes (§5.2)
- [ ] Failed logins are rate-limited per IP and per email
- [ ] Test: `auth/password_test.go` — correct password succeeds, wrong password
      fails, and both take comparable wall time
- [ ] Test: `auth/login_test.go` — a disabled account cannot log in
- [ ] Public endpoint: rate-limited and leak-reviewed — the response
      distinguishes neither "no such user" nor "wrong password" (§6.5, §14)

---

### T-1.3 — Implement refresh token rotation and reuse detection
**Depends on:** T-1.2
**Touches:** `server/internal/auth/`
**Size:** M
**Done when:**
- [ ] `POST /auth/refresh` reads the opaque token from the cookie, rotates it,
      and sets the replacement (§5.2)
- [ ] Cookie is `httpOnly; Secure; SameSite=Lax; Path=/auth`, **host-only** —
      no `Domain` attribute (`00-overview.md` §4.1)
- [ ] Tokens are stored SHA-256 hashed, and looked up BY that hash (§13.5)
      — this checkbox previously said "compared in constant time", which
      misquotes §13.5: that phrase belongs to the JOIN CODE bullet. A join code
      is short and human-typed; a refresh token is 256 bits from a CSPRNG, and
      the lookup is a b-tree probe an attacker cannot aim without already
      holding a candidate. No Go-side compare is added, because comparing a
      value against the row it was used to find would assert nothing
- [ ] Presenting an already-rotated token revokes the **whole family** and
      forces re-login (§5.2)
- [ ] `POST /auth/logout` revokes the presented token's family
- [ ] An expired-token cleanup path exists, runs at startup and daily, and
      prunes by **family** — never individual rows. `replaced_by` is
      `ON DELETE SET NULL`, and rotation reads it to tell a replayed token from
      a wholesale-revoked one; deleting a successor nulls its predecessor's link
      and silently downgrades reuse detection on it. Latent today (a successor
      outlives its predecessor), live the moment `REFRESH_TOKEN_TTL` is reduced.
      This means the scan does not use `refresh_tokens_expiry_idx` — the index
      the original checkbox named. Correctness over the index, at this size
- [ ] Test: `auth/rotation_test.go` — "replaying a rotated token revokes every
      token in the family"
- [ ] Test: `auth/rotation_test.go` — "rotation issues a token in the same
      family with `replaced_by` set on the predecessor"
- [ ] Test: `auth/cookie_test.go` — asserts the exact `Set-Cookie` attributes,
      including the absence of `Domain`
- [ ] `POST /auth/logout` is authenticated by the refresh COOKIE, not the
      access token: an expired access token must not be able to strand a live
      refresh family
- [ ] Contract additions: `Set-Cookie` on `/auth/refresh` 200 (rotation cannot
      deliver its replacement without it) and on `/auth/logout` 204
- [ ] Test: `auth/rotation_test.go` — concurrent refreshes of one token elect
      exactly one winner. The row lock is the guarantee; nothing else asserts it
- [ ] Public endpoint: rate-limited and leak-reviewed (§14)

---

### T-1.4 — Implement `/auth/me`, change-password, and the session shape
**Depends on:** T-1.3
**Touches:** `server/internal/auth/`
**Size:** S
**Done when:**
- [ ] `GET /auth/me` returns §7's `User`, including `hasPassword`,
      `linkedProviders` and `mustChangePassword`
- [ ] `POST /auth/change-password` requires the current password, clears
      `must_change_password`, and **revokes all other refresh families** for that
      user
- [ ] Test: `auth/me_test.go` — a Google-only user reports
      `hasPassword: false`, `linkedProviders: ['google']`
- [ ] Test: `auth/change_password_test.go` — a second device's refresh token is
      dead after a password change
- [ ] **Bearer authentication middleware.** T-1.4 is where access tokens start
      being CHECKED; before it, `/auth/login` minted them and nothing verified
      one. Driven from the contract, not a list in Go: an operation inherits the
      document's top-level `security` unless it overrides it, so a new endpoint
      is protected by default. Fail-CLOSED — anything not explicitly open needs
      a token, because a route missing from a protected-list serves data
      silently while one missing from an open-list returns a visible 401
- [ ] Test: exactly five operations are reachable without an access token
      (`login`, `google`, `refresh`, `logout`, `join/preview`). The list is
      pinned so opening a sixth takes an argument, not an unreviewed diff
- [ ] A wrong `currentPassword` returns **400, not 401**. `client.ts` treats 401
      as a dead session: refresh once, retry, sign out on the second. A 401 here
      means mistyping your own password silently logs you out
- [ ] Contract: `UNAUTHORIZED` added to the `ErrorCode` enum. It had none, so
      401s were being sent with code `FORBIDDEN` — the client could not tell
      "log in" from "you may not" except by reading the status line

---

### T-1.4b — Enforce the contract on incoming requests
**Depends on:** T-1.4
**Touches:** `server/internal/httpx/`, `server/internal/api/router.go`
**Size:** S
**Added during T-1.4**, not in the original plan. `oapi-codegen` generates types
and binds JSON; it enforces **none** of the contract's constraints. `minLength`,
`format`, `enum`, `required` and `additionalProperties` were all decorative on
the server side, so `POST /auth/login` accepted an empty password and a
`{"email": "banana"}`. Every one of the 60 remaining endpoints would have had to
restate its own rules in Go, or silently have none.

**Done when:**
- [ ] `nethttp-middleware`'s `OapiRequestValidator` runs on every generated
      route, driven by the same embedded spec the rest of the router uses
- [ ] Failures return the §7 envelope with `VALIDATION_FAILED`, not
      `kin-openapi`'s default — which is English, unreadable, and quotes the
      failing schema back at an anonymous caller
- [ ] Authentication runs **before** validation: an anonymous caller is told to
      log in, not handed a critique of their request body
- [ ] `spec.Servers` is cleared for the validator. Left set, it validates the
      `Host` header against the production URLs and rejects every request in
      development and in tests
- [ ] Authentication is **not** delegated to the validator. It would enforce the
      security requirements it can see, knows nothing about our tokens, and
      would need a second copy of `RequireAuth`
- [ ] Middleware is written in execution order. `oapi-codegen` applies the slice
      so the LAST entry runs FIRST; the chain had been reading backwards, with
      two comments describing an order that was not the real one. `NewRouter`
      reverses explicitly and a test pins the direction
- [ ] Test: contract constraints rejected — short password, bad email, unknown
      field, wrong type, unparseable body, malformed uuid path parameter
- [ ] Test: a well-formed request still reaches its handler
- [ ] Test: validation messages do not echo the schema

### T-1.5 — Implement the Google OAuth exchange
**Depends on:** T-1.4, T-0.2
**Touches:** `server/internal/auth/google/`
**Size:** L
**Done when:**
- [ ] **No GIS SDK.** Per O-13 the frontend builds the authorization request
      itself: CSPRNG `code_verifier`, S256 `code_challenge`, a `state` value
      checked on return, popup to Google's `authorization_endpoint`. Approved
      deviation from §2 — do not reintroduce `gsi/client`
- [ ] `state` is single-use and compared in constant time; a mismatch aborts
      before any network call to our backend
- [ ] `POST /auth/google` exchanges `{ code, codeVerifier, redirectUri }` with
      Google server-side; the client secret never leaves the backend (§5.3)
- [ ] ID token verified for `iss`, `aud`, `exp` and signature via JWKS, with the
      JWKS response cached and refreshed on unknown `kid`
- [ ] **`email_verified: false` is rejected outright — no link, no create**
      (§5.1). This closes an account-takeover path and is not negotiable
- [ ] Resolution order is exactly §5.3 step 4: identity exists → log in;
      verified email matches a user → link identity, log in; no match with a
      valid `joinCode` → create + enrol (T-1.7); no match, no join code →
      `403 ACCOUNT_NOT_PROVISIONED`
- [ ] Test: `google/verify_test.go` — a token with a wrong `aud` is rejected
- [ ] Test: `google/verify_test.go` — `email_verified: false` is rejected even
      when the email matches an existing user
- [ ] Test: `google/resolve_test.go` — one case per branch of the §5.3 order
- [ ] Public endpoint: rate-limited and leak-reviewed (§6.5, §14). Both buckets:
      per-IP, and per-`joinCode` — one code must not be usable to create
      accounts from a hundred addresses. A request without a join code is a
      plain sign-in and uses the per-IP bucket alone
- [ ] Contract: `Set-Cookie` on the 200. It was missing, exactly as it was on
      `/auth/refresh`; §5.3 step 5 requires the sign-in to set the refresh
      cookie, and without the header the generated response cannot carry it
- [ ] Contract: the 403 also documents `ACCOUNT_DISABLED` and
      `IDENTITY_ALREADY_LINKED`. Both are named explicitly rather than hidden:
      the caller has just proved to Google that they control the address, so
      neither discloses anything they could not confirm themselves

**Not done in this task — branch 3.** "No match and a valid `joinCode` → create
account + enrol" needs join codes, which are T-1.6/T-1.7, and enrolment, which
is T-1.8. It is implemented as a `SelfEnroller` seam: a join code reaches it and
gets `ErrSelfEnrolNotAvailable` → 501, rather than falling through to
`ACCOUNT_NOT_PROVISIONED`, which would be a wrong answer rather than a missing
one. **T-1.8 must wire the enroller and flip the branch-3 test from asserting
the seam to asserting an enrolment.**

---

### T-1.6 — Implement join-code generation, rotation and revocation
**Depends on:** T-1.1
**Touches:** `server/internal/join/`, `server/internal/classes/`
**Size:** M
**Done when:**
- [ ] Codes are 8 characters from `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`, drawn from
      a CSPRNG, never sequential and never derived from the class ID (§6.1)
- [ ] Normalization accepts the code with or without the dash and in any case,
      before hashing
- [ ] Only `code_hash` and a 4-character `code_hint` are stored; the plaintext is
      returned exactly once, from `POST /admin/classes/:id/join-code` (§13.3)
- [ ] Rotation revokes the old code and issues a new one **in one transaction**,
      so the `class_join_codes_one_active` partial unique cannot be violated
- [ ] `DELETE /admin/classes/:id/join-code` revokes and sets
      `self_join_enabled = false` (§6.4)
- [ ] Default expiry 30 days (§6.1); default `max_uses` per `40-open-items.md`
- [ ] Generation, rotation and revocation are written to `audit_log` (§13.4)
- [ ] Test: `join/code_test.go` — "normalize" is idempotent across
      `abcd-1234`, `ABCD1234`, `abcd 1234`
- [ ] Test: `join/code_test.go` — 100k generated codes contain no character from
      the ambiguous set `0O1I`. **Not `L`** — this checkbox said `0O1IL`, which
      contradicts §6.1's own alphabet: `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`
      contains `L`. The spec's "(no 0/O, 1/I/L)" names the two confusion
      GROUPS, not a blocklist; from {1, I, L} it keeps one member, and with `1`
      and `I` gone there is nothing left for `L` to be mistaken for. Dropping it
      too would make the alphabet 31 characters — no longer a power of two, so
      uniform selection needs rejection sampling for no gain in legibility
- [ ] Test: `join/rotate_test.go` — after rotation the old code fails and
      existing members are untouched (§6.1)
- [ ] Test: `join/rotate_test.go` — six concurrent rotations of one class leave
      exactly one active code. `class_join_codes_one_active` is a partial unique
      index: insert-then-revoke violates it, revoke-then-insert leaves a window
      with no code. The transaction plus a row lock on the class is what makes
      neither state observable
- [ ] **Role authorization.** T-1.6 adds the first real `/admin` endpoint, so
      `/admin/*` must stop being reachable by any authenticated student. Driven
      by the path prefix, because the path IS §3's route-tree structure — a
      per-operation annotation is a second thing to remember, and the cost of
      forgetting is a student reading every attempt in the school. The contract's
      `Forbidden` response already documented this rule; nothing enforced it
- [ ] Contract: `maxUses` is **not nullable on the request**, unlike the stored
      column. An omitted field and an explicit `null` both arrive as "no value",
      so `null = unlimited` was indistinguishable from "use the default 40" and
      one of the two would silently never happen. Capped at 1000 instead

---

### T-1.7 — Implement `POST /join/preview` (public)
**Depends on:** T-1.6
**Touches:** `server/internal/join/`
**Size:** M
**Done when:**
- [ ] Returns **only** `classId`, `className` and `teacherName` — never student
      names, never counts, never other IDs (§6.5)
- [ ] Lookup is by hash of the normalized code with a constant-time comparison
      (§6.5); no plaintext equality anywhere
- [ ] Invalid, expired, exhausted and revoked each return a distinct error
      `code` with a plain message that reveals nothing about which classes exist
      (§9)
- [ ] Rate limited per IP (10/min, 60/hour) **and** per code (30/hour), returning
      `429` with `Retry-After` (§6.5)
- [ ] `self_join_enabled = false` behaves identically to an invalid code
- [ ] Test: `join/preview_test.go` — the response body contains no key other
      than the three permitted ones, asserted structurally
- [ ] Test: `join/preview_test.go` — the four failure modes produce four codes
      and none echoes the class name
- [ ] Test: the 11th request in a minute from one IP is 429; the 31st for one
      code is 429 even across different IPs. Lives in
      `api/join_preview_test.go`, not `join/ratelimit_test.go` — the limiter is
      router middleware, and testing it needs the router
- [ ] **The per-code bucket keys on the NORMALIZED code.** §6.1 accepts a code
      with or without the dash and in any case, so `K7M3-P9QR` and `k7m3p9qr`
      are one code — and keyed on the raw body value they are two buckets, which
      hands an attacker a fresh allowance for every spelling of the same secret.
      Same fix applies to the `joinCode` bucket on `POST /auth/google` added in
      T-1.5. A test walks five spellings and requires the 31st to be 429
- [ ] Note: because revoke ALSO closes self-join, and self-join is checked
      first, a revoked code reports `JOIN_CODE_INVALID`. The path that surfaces
      `JOIN_CODE_REVOKED` is a **rotation**, which re-opens the class — which is
      the case that matters: a student holding last month's code is told to ask
      for the new one instead of being told they mistyped it
- [ ] **Public endpoint: rate-limited and leak-reviewed (§6.5, §14)**

---

### T-1.8 — Implement self-join account creation and enrolment
**Depends on:** T-1.5, T-1.7
**Touches:** `server/internal/auth/google/`, `server/internal/join/`
**Size:** M
**Done when:**
- [ ] `POST /auth/google` with a valid `joinCode` and no matching identity
      creates the user, creates the identity, and enrols in one transaction
      (§5.3, §6.3)
- [ ] `class_members` records `joined_via = 'join_code'` and the `join_code_id`
      (D-10)
- [ ] `uses_count` increments in the same transaction; the `uses_count <=
      max_uses` constraint (D-09) makes an over-limit enrolment impossible
- [ ] Every enrolment writes `class_id`, `user_id`, `ip`, `user_agent`, `at` to
      `audit_log` (§6.5)
- [ ] `POST /app/classes/join` handles the already-authenticated path (§6.2) and
      is idempotent when the student is already a member
- [ ] Test: `join/enrol_test.go` — concurrent enrolments against a code with
      `max_uses = 1` produce exactly one member
- [ ] Test: `join/enrol_test.go` — an expired code creates no user
      (E2E 4's backend half)
- [ ] Test: `join/audit_test.go` — an enrolment writes exactly one audit row
      with a non-null ip
- [ ] **Public endpoint: rate-limited and leak-reviewed (§6.5, §14)**

---

### T-1.9 — Implement Google link and unlink
**Depends on:** T-1.5
**Touches:** `server/internal/auth/`
**Size:** S
**Done when:**
- [ ] `POST /auth/google/link` links an identity to the current account, subject
      to the same `email_verified` rule (§5.1)
- [ ] `DELETE /auth/google/link` is **rejected if it would leave no login
      method** — a Google-only account cannot unlink (§15)
- [ ] Test: `auth/link_test.go` — unlinking from a passwordless account returns
      a 409 with a specific error code
- [ ] Test: `auth/link_test.go` — linking a Google identity already bound to
      another user is rejected (D-08's `UNIQUE (provider, provider_user_id)`)

---

### T-1.10 — Build the auth store and route guards
**Depends on:** T-0.12, T-1.4
**Touches:** `web/src/features/auth/`, `web/src/app/guards/`
**Size:** M
**Done when:**
- [ ] Zustand store holds the access token **in memory only** (§5.2)
- [ ] App load calls `GET /auth/me`; 401 → `/login` (§5.4)
- [ ] `RequireAuth` redirects to `/login?next=<path>`
- [ ] `RequireRole`: an `admin` on `/app/*` redirects to `/admin`; a `student` on
      `/admin/*` gets a **403 page, not a redirect** — a redirect hides the
      misconfiguration (§5.4)
- [ ] `mustChangePassword` forces `/change-password` from every route; Google-only
      users never reach it (§5.4)
- [ ] Logout calls `POST /auth/logout`, clears the store, calls
      `queryClient.clear()`, and navigates to `/login` (§5.4)
- [ ] Test: `guards.test.tsx` — one case per rule above, including that the
      student-on-admin case renders 403 rather than navigating
- [ ] Test: `auth/store.test.ts` — the token is absent from `localStorage` and
      `sessionStorage` after login
- [ ] All strings via `t()` in both locales; keyboard-operable (§14)

---

### T-1.11 — Build `/login`
**Depends on:** T-1.10
**Touches:** `web/src/features/auth/pages/`
**Size:** M
**Done when:**
- [ ] Password form with react-hook-form + zod, and a "Tiếp tục với Google"
      button (§9)
- [ ] GIS loaded lazily and requests an **authorization code with PKCE**, not an
      implicit ID token (§5.3)
- [ ] Errors map from the envelope `code` to localized copy; the account-not-
      provisioned case explains that a class code is needed (§5.3)
- [ ] Design follows §12: single centered card, `zinc-900` primary button, no
      gradient, no glow
- [ ] Test: `login.test.tsx` — invalid email blocks submit; server error renders
      the localized message; the Google button is keyboard-reachable
- [ ] Loading / error / empty states present; all strings via `t()` (§14)

---

### T-1.12 — Build the `/join` flow
**Depends on:** T-1.11, T-1.7
**Touches:** `web/src/features/join/`
**Size:** L
**Done when:**
- [ ] `/join` (enter code), `/join/:code` (deep link, prefilled), and
      `/join/:code/confirm` (class name + teacher name, one primary button) —
      §6.2's three steps
- [ ] **The confirm step is mandatory**: no path creates an account and enrols in
      one blind tap (§6.2)
- [ ] Code input normalizes as the user types, accepts with or without the dash,
      and is case-insensitive (§6.1)
- [ ] Invalid / expired / exhausted render distinct plain messages that hint
      nothing about which classes exist (§9)
- [ ] An already-authenticated student on `/join/:code` skips to
      `POST /app/classes/join` (§6.2)
- [ ] Design per §12's join-screen rule: single centered card, class name large,
      calm and legitimate — this is the first thing a new student sees
- [ ] Test: `join/normalize.test.ts` — the unit cases from §14
- [ ] Test: `join/confirm.test.tsx` — the confirm step renders the class name
      before any auth call is made
- [ ] **E2E 3** passes: anonymous visitor opens `/join/:code` → sees class name →
      mocked GIS sign-in → account created, enrolled, lands on `/app`
- [ ] **E2E 4** passes: expired code → plain error, no account created, nothing
      leaked about the class
- [ ] Loading / error / empty states; keyboard-operable; both locales (§14)

---

### T-1.13 — Build the admin join-code panel
**Depends on:** T-1.12, T-1.6
**Touches:** `web/src/features/classes/`
**Size:** M
**Done when:**
- [ ] `/admin/classes/:id` shows the active code with a copy button, QR code,
      expiry and uses count (§6.4)
- [ ] The plaintext code is displayed **only** in the response to a rotation;
      thereafter the panel shows `code_hint` and a "rotate to get a new code"
      affordance (§13.3)
- [ ] Rotate has the §6.4 confirm dialog: "Mã cũ sẽ ngừng hoạt động ngay"
- [ ] A "Disable self-join" toggle revokes without issuing a new code (§6.4)
- [ ] Member list shows `joined_via` and `joined_at`; remove-member revokes
      access and **retains attempts** (§6.4)
- [ ] The QR library is either avoided (canvas-drawn) or justified in the PR
      description as a new dependency (§14)
- [ ] Test: `classes/join-code-panel.test.tsx` — after mount without a rotation,
      no full code appears in the DOM
- [ ] Test: `classes/rotate.test.tsx` — the confirm dialog blocks rotation until
      accepted
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-1.14 — Build the settings pages and close the phase
**Depends on:** T-1.13, T-1.9
**Touches:** `web/src/features/auth/`, `web/src/app/`
**Size:** M
**Done when:**
- [ ] `/admin/settings` — profile, password, link/unlink Google, language (§8)
- [ ] `/app/settings` — password, link/unlink Google, language (§9)
- [ ] `/change-password` handles the `mustChangePassword` redirect target
- [ ] Unlink is disabled with an explanation when it would leave no login method
- [ ] **E2E 2a** passes: student logs in with password → lands on `/app` → the
      three §9 sections render their empty states
- [ ] E2E 3 and E2E 4 still pass — **Phase 1 exit criteria met**
- [ ] `release/phase-1` merges to `main` and back to `develop`; the phase is
      deployable (§16)
