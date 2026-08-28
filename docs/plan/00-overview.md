# 00 — Architecture Overview

Implementation plan for `docs/quizzivy-spec-v0.3.md`. This file records the
decisions the spec left open, the ones I made against it, and the mechanics of
local development. Phase task lists are in `10-` … `15-`; the schema is in
`20-data-model.md`.

Spec sections are referenced by number and never restated.

---

## 1. Verified platform facts

The spec commits to four PostgreSQL 18 constructs (§13.6). All four were checked
against <https://www.postgresql.org/docs/18/> before any DDL was designed. All
four work, but three carry caveats §13 does not mention.

| Construct | Docs | Verdict |
|---|---|---|
| `uuidv7()` | [functions-uuid](https://www.postgresql.org/docs/18/functions-uuid.html) | Works as §13.2 assumes. Built-in, no extension. Signature is `uuidv7([shift interval])`. |
| Virtual generated columns | [ddl-generated-columns](https://www.postgresql.org/docs/18/ddl-generated-columns.html) | VIRTUAL is the PG18 default — "A generated column is by default of the virtual kind." `final_score` works. **Caveat:** virtual columns cannot be indexed and cannot carry UNIQUE, FK, NOT NULL, or extended statistics; they are excluded from logical replication ("only supported for stored generated columns"). |
| `OLD` / `NEW` in `RETURNING` | [dml-returning](https://www.postgresql.org/docs/18/dml-returning.html) | Works in INSERT/UPDATE/DELETE/MERGE; aliases renameable. **Caveat:** §13.4's "one statement instead of read-then-write" only holds inside a data-modifying CTE (§4.4 below). A bare `UPDATE … RETURNING` followed by an `INSERT` is still two round trips. |
| `NOT NULL … NOT VALID` | [sql-altertable](https://www.postgresql.org/docs/18/sql-altertable.html), [release-18](https://www.postgresql.org/docs/18/release-18.html) | Real in PG18 — release note: "Allow `ALTER TABLE` to set the `NOT VALID` attribute of `NOT NULL` constraints." **Caveat:** only via the table-constraint form `ADD CONSTRAINT c NOT NULL col NOT VALID`. It is not in the `CREATE TABLE` grammar (which shows `NOT NULL [ NO INHERIT ]` only) and not available via `SET NOT NULL`. |

Consequence for this project: `NOT NULL NOT VALID` is **unreachable on a
greenfield schema**. Every table in `20-data-model.md` declares `NOT NULL`
inline at `CREATE TABLE` time, where it is free. The construct becomes relevant
only when a nullable column is tightened against existing production rows —
realistically Phase 5 or post-v1. T-0.16 pins the behaviour with a test so it is
proven rather than remembered.

Where the Neon `postgres-best-practices` skill conflicts with the PG18 docs, the
docs win (§13.1). The one live conflict: the skill teaches the pre-18
`ADD CHECK NOT VALID → VALIDATE → SET NOT NULL` dance. On PG18 use the native
not-null form above.

---

## 2. Frontend vs backend sequencing

**Decision: (a) — contract-first, both sides generated from one OpenAPI document.**

The load-bearing risks in v1 are all *server* semantics: the authoritative
`deadline_at`, server-side `maxPlays` (§11.4), the publish snapshot (§13.3), the
shuffle seed, session takeover, and refresh rotation (§5.2). A frontend built
against hand-written MSW mocks would encode a plausible fiction of each, and
Phase 3 would be a rewrite rather than an integration. Writing the contract first
costs one Phase 0 task and makes the mocks *derived* rather than invented.

With one developer, "in parallel" in practice means vertical slices: a backend
endpoint and the screen that consumes it as a PR pair within the same phase. The
generated contract is what makes that safe.

---

## 3. How §15 stays in sync

`api/openapi.yaml` — hand-authored OpenAPI 3.1 — is the single source of truth.
It supersedes §15, which becomes documentation. Everything else is generated.

| Target | Tool | Output |
|---|---|---|
| Go server | `oapi-codegen` (strict server mode) | `server/gen/openapi/` — interfaces, request/response types, param binding |
| TS types | `openapi-typescript` | `web/src/lib/api/schema.d.ts` — types only, zero runtime |
| MSW fixtures | `ajv` at test time | fixtures validated against the OpenAPI schema so mocks cannot drift |

**Zod stays hand-written.** §2 makes zod the source of validation and TS types;
in practice its real job is react-hook-form input validation, which is a
*narrower* shape than the wire type (a password field has a min length the API
schema does not express). So each feature keeps its `schemas.ts`, and each
request schema carries a type-level assertion:

```ts
// web/src/features/auth/schemas.ts
type Expect<T extends true> = T;
type Equal<A, B> = (<G>() => G extends A ? 1 : 2) extends
  (<G>() => G extends B ? 1 : 2) ? true : false;

export const loginSchema = z.object({ email: z.string().email(), password: z.string().min(8) });

type _check = Expect<Equal<
  z.infer<typeof loginSchema>,
  paths['/auth/login']['post']['requestBody']['content']['application/json']
>>;
```

Drift fails `tsc`, not review. No third code generator.

**CI enforcement:** regenerate all outputs, then `git diff --exit-code`. A stale
generated file is a red build. Generated files are committed so the repo is
buildable without the toolchain.

New dependencies introduced here, with reasons for the PR description
(AGENTS.md rule): `openapi-typescript` (dev — type generation), `ajv` (dev —
fixture validation), `oapi-codegen` (Go tool — server generation).

---

## 4. Architecture decisions

### 4.1 Deployment topology and the refresh cookie

`app.quizzivy.x` serves the SPA; `api.quizzivy.x` serves the Go API. Both are
subdomains of one registrable domain, which makes every request between them
**same-site** even though it is cross-*origin*.

This matters because §5.2 specifies `SameSite=Lax`, and Lax cookies are never
sent on cross-**site** `fetch`. On this topology they are sent, so §5.2 works
verbatim. On genuinely cross-site origins (a Vercel frontend against a Fly.io
backend, say) `/auth/refresh` would silently never authenticate. The topology is
load-bearing; it is not a deployment detail to revisit casually.

- Refresh cookie: `httpOnly; Secure; SameSite=Lax; Path=/auth`, and **host-only**
  — no `Domain` attribute, so `app.quizzivy.x` never receives it. The API is at
  the root of its host, so `Path=/auth` matches `/auth/refresh` exactly.
- CORS: exact-origin allowlist (never `*`, which is illegal with credentials),
  `Access-Control-Allow-Credentials: true`, `Vary: Origin`, and preflight
  handling for `PATCH`/`DELETE` plus the `Authorization` and JSON content-type
  headers.

### 4.2 Single-flight refresh

§5.4 says the app calls `GET /auth/me` on load and §5.2 says a 401 triggers one
refresh-and-retry. With TanStack Query mounting several queries at once, a cold
load fires N parallel requests, all 401, all call `POST /auth/refresh`. Rotation
plus §5.2's reuse detection then revokes the whole family and logs the user out —
on every page refresh.

`src/lib/api/client.ts` therefore holds **one shared in-flight refresh promise**.
Concurrent 401s await the same promise; only one rotation happens. This is
unit-tested (T-0.12) because it is invisible until it isn't.

### 4.3 Beacon authentication

§10.6 flushes integrity events on `pagehide` via `navigator.sendBeacon`.
`sendBeacon` cannot set request headers, so it cannot carry
`Authorization: Bearer`. And the access token is ~15 minutes (§5.2) while a test
runs 45–90, so at `pagehide` the token is usually expired anyway — exactly when
the flush matters most.

So `POST /app/assignments/:id/attempts` returns an opaque **beacon token**,
valid until `deadline_at`, stored as `attempts.beacon_token_hash`. The beacon
body carries it:

```ts
navigator.sendBeacon(
  `${API}/app/attempts/${id}/events`,
  new Blob([JSON.stringify({ beaconToken, sessionId, events })], { type: 'text/plain' })
);
```

`text/plain` is a CORS-safelisted content type, so the beacon skips preflight —
which is necessary, because a preflight on unload is not reliably delivered. The
endpoint accepts *only* event appends; it grants no read access and cannot
mutate answers, so the weaker credential buys no additional authority.

### 4.4 Audit via data-modifying CTE

§13.4 wants the audit diff captured in the same statement as the mutation. That
requires the `RETURNING` to feed an `INSERT`, which means a CTE:

```sql
WITH updated AS (
  UPDATE app.attempts SET deadline_at = $2
   WHERE id = $1
  RETURNING id, old.deadline_at AS prev, new.deadline_at AS next
)
INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, ip, user_agent, diff)
SELECT $3, 'attempt.extend', 'attempt', id, $4, $5,
       jsonb_build_object('deadline_at', jsonb_build_object('old', prev, 'new', next))
  FROM updated;
```

Written as two statements it is a read-then-write with a race. T-4.2 uses this
form and T-0.16 proves `old`/`new` survive inside a CTE.

### 4.5 Assignment status is derived, not stored

§7 types `Assignment.status` as `'scheduled' | 'open' | 'closed'`. Storing it
requires a scheduler to flip rows at `opens_at` and `closes_at`. Deriving it from
`opens_at`, `closes_at` and a nullable `closed_at` (manual early close) needs
nothing, cannot go stale, and produces the same frontend type. Derived.

### 4.6 Rate limiting

In-process: `golang.org/x/time/rate` limiters in a bounded LRU, keyed
independently by client IP and by join-code hash, per §6.5's two limits. One API
instance at this scale, so no Redis and no shared state. The limiter interface is
narrow enough that swapping in a distributed backend later is one implementation,
not a refactor. `429` responses carry `Retry-After`.

Note on §6.5's stated rationale: an 8-character code from a 32-symbol alphabet is
2^40 ≈ 1.1 × 10^12, and only a handful of codes are live at a time, so random
probing is not a realistic attack even unthrottled. The rate limit is still
correct — it bounds a cheap nuisance — but the threat that actually matters is a
**forwarded** code. The mitigations that address that are expiry, `max_uses`, and
the §6.4 member list. See `40-open-items.md` on changing the `max_uses` default.

### 4.7 Other reversible calls

- **`attempt_events.kind` stays `text`**, not an enum. §13.2 prefers enums for
  closed sets, and §10.1's list looks closed — but an append-only telemetry log
  reliably gains kinds, and each one would be a migration plus a deploy ordering
  problem. Lookup tables and enums both cost more than they return here.
- **Local object storage is MinIO**, S3-compatible, so `aws-sdk-go-v2/s3` targets
  it and R2 with only endpoint config differing. This decouples Phases 0–2 from
  the R2 credential blocker.
- **Migrations are sequential and zero-padded** (`00001_…`), one concern each
  (§13.7). Sequential over timestamped because there is one developer and one
  branch line, so ordering conflicts cannot arise and the numbers stay readable.
- **`REVOKE CREATE ON SCHEMA public FROM PUBLIC`** (§13.2) is kept in migration
  00001 as documentation, annotated as a no-op — it has been the default since
  PG15.

---

## 5. Where this plan disagrees with the spec

Both of these were raised in the pre-plan critique and decided by Thuong. They
are recorded here so the reasoning is not relitigated in a later session.

**Version snapshot shape.** I proposed keeping `test_version_questions` as a real
table (it is the FK target for `attempt_answers` and holds ordinal/points/type)
while collapsing `test_version_options`, `test_version_blanks` and
`test_version_blank_answers` into one `jsonb` column, on the grounds that snapshot
leaves are written once, always read whole, never joined, and never filtered —
and that §13.3's justification for normalizing them ("per-question analytics
later") is explicitly deferred by §1.3 and §16. **Thuong chose full
normalization**, which preserves per-option SQL analytics as a plain query at the
cost of four extra tables and a longer publish routine. `20-data-model.md`
implements the normalized form.

**Audio duration probing.** I proposed adding `ffprobe` to the backend image,
since VBR MP3 without a Xing header and unusual MP4 atom layouts are exactly the
cases hand-rolled parsers get wrong. **Thuong chose pure-Go**, avoiding a non-Go
binary in the container at the cost of roughly a day of work and residual
edge-case risk, tracked as R-08 in `30-risks.md` and mitigated by the fixture
corpus in T-2.2.

Two further disagreements were accepted and are implemented as described:
`30-risks.md` R-05 (the `clientSeq` collision, §13.3 deviation D-01) and the
§16 exit-criteria corrections in `11-phase-1.md` and `12-phase-2.md`.

---

## 6. Repository layout

Monorepo. §3's frontend structure is preserved verbatim under `web/`.

```
Quizzivy/
├─ AGENTS.md                     standing rules for every session
├─ docs/
│  ├─ quizzivy-spec-v0.3.md      source of truth (moved into the repo)
│  └─ plan/                      this plan
├─ api/
│  └─ openapi.yaml               THE API contract; §15 is documentation
├─ web/                          quizzivy-web (§4)
│  └─ src/                       exactly §3 — app/ features/ components/
│                                layouts/ lib/ hooks/ stores/ styles/
├─ server/                       Go module `quizzivy` (§4)
│  ├─ cmd/api/main.go
│  ├─ internal/
│  │  ├─ auth/  join/  classes/  media/  questions/  tests/
│  │  ├─ assignments/  attempts/  grading/  integrity/  audit/
│  │  ├─ storage/               S3/R2 client
│  │  ├─ media/probe/           pure-Go mp3 + mp4 duration (T-2.2)
│  │  ├─ httpx/                 error envelope, cursor codec, middleware
│  │  └─ ratelimit/
│  ├─ gen/openapi/              generated, committed, never hand-edited
│  └─ go.mod
├─ migrations/                   goose SQL, forward-only, 00001…
├─ seed/                         never in migrations (§13.7)
├─ docker-compose.yml            postgres:18 + minio
└─ .github/workflows/ci.yml
```

`web/src/features/<name>/` follows §3's convention unchanged: `api.ts`,
`schemas.ts`, `components/`, `pages/`, optional `store.ts`, `index.ts` for public
exports only. Features import each other only through `index.ts`;
`components/shared` never imports from `features/`.

Backend packages mirror the feature folders deliberately, so a vertical slice is
one frontend folder plus one backend package plus one section of
`api/openapi.yaml`.

---

## 7. Error envelope

§15 implies one (`403 ACCOUNT_NOT_PROVISIONED`) but never defines it. Every
non-2xx response body is:

```json
{
  "error": {
    "code": "ACCOUNT_NOT_PROVISIONED",
    "message": "Tài khoản chưa được cấp quyền.",
    "details": { "field": "joinCode" },
    "requestId": "01931f4e-..."
  }
}
```

- `code` — `SCREAMING_SNAKE`, stable, the only thing clients branch on. Enumerated
  in `api/openapi.yaml`.
- `message` — already localized server-side via `Accept-Language`, `vi` default.
  Clients display it; they do not construct copy from `code`.
- `details` — optional, shape depends on `code`. Field-level validation errors
  land here for react-hook-form.
- `requestId` — the copyable error ID §9's global error boundary shows.

`429` additionally carries `Retry-After` (§6.5).

## 8. Pagination

Keyset per §13.8, uniform across list endpoints: `{ items, nextCursor }`, with
`nextCursor` an opaque base64 of the sort key. Clients never construct or parse
it. Because PKs are `uuidv7()` and therefore time-ordered, most lists sort by
`id DESC` alone and need no separate `created_at` index; `20-data-model.md`
records which tables deviate.

---

## 9. Local development

```
docker compose up -d          # postgres:18 on 5432, minio on 9000/9001
make migrate                  # goose up, as quizzivy_migrate
make seed                     # seed/ — admin user, one class, sample questions
make gen                      # oapi-codegen + openapi-typescript
cd server && go run ./cmd/api # :8080
cd web && pnpm dev            # :5173, proxying /api to :8080
```

`make dev` runs the last two together.

**Hosts.** Local dev uses `localhost:5173` and `localhost:8080`, which are
same-site (both `localhost`), so cookie behaviour matches production. Do not
introduce a `127.0.0.1` vs `localhost` split — that *is* cross-site and will
produce a bug that does not reproduce in staging.

**Environment.** `.env.example` is committed and stays current; `.env` is
ignored. `VITE_GOOGLE_CLIENT_ID` and `VITE_API_BASE_URL` are the only public
values (§5.3). The Google client secret, R2 credentials, and the JWT signing key
are backend-only.

**Two database roles** (§13.5): `quizzivy_migrate` owns the schema and runs
`goose`; `quizzivy_app` has DML plus `USAGE` and is what the API connects as.
Compose provisions both, so a missing grant fails locally rather than in
production.

## 10. Git workflow

Gitflow, as instructed.

| Branch | Purpose |
|---|---|
| `main` | Released only. Tagged `v0.x`. Never committed to directly. |
| `develop` | Integration. Every feature PR targets this. |
| `feature/<task-id>-<slug>` | One task from a phase file = one branch = one PR. |
| `release/phase-<n>` | Cut when a phase's tasks are all merged; stabilization only. |
| `hotfix/<slug>` | From `main`, merged to both `main` and `develop`. |

One task is one reviewable PR (§18), so `feature/t-1-04-google-oauth` maps to
T-1.4. Phase completion is a release branch merged to `main` and back to
`develop`, which is what makes §16's "each phase ends deployable" a branch state
rather than an assertion.

The PR template carries the §14 definition of done, plus the two conditional
blocks (DDL review; public-endpoint rate limit and leak review) so they cannot be
skipped silently.

One honest note: for a single developer deploying to one environment, `develop`
plus `main` doubles merge overhead for no isolation benefit, and `release/*`
branches are ceremony. It is being followed because it was specified, and it does
buy a clean phase boundary. If it becomes friction, see `40-open-items.md`.
