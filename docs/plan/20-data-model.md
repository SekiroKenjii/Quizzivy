# 20 — Data Model

Full DDL for every table in §13, expanded from the sketches. Every foreign key
has a decided `ON DELETE`, every index has a reason, and every deviation from the
§13.3 sketch is flagged with `[D-nn]` and justified in §12 of this document.

Governed by the Neon `postgres-best-practices` skill (schema-design, indexing)
and the PostgreSQL 18 docs (§13.1). PG18-specific constructs cite their docs
section. Verified behaviour is recorded in `00-overview.md` §1.

**Target: PostgreSQL 18.** `uuidv7()` and virtual generated columns have no
fallback on 16/17; this schema does not degrade, it fails to create.

---

## 1. Provisioning (outside migrations)

Roles cannot be created by the migration role, so they are provisioned by the
compose `initdb` script locally and by a documented runbook step in production:

```sql
CREATE ROLE quizzivy_migrate LOGIN PASSWORD :'migrate_pw';
CREATE ROLE quizzivy_app     LOGIN PASSWORD :'app_pw';
CREATE DATABASE quizzivy OWNER quizzivy_migrate;
GRANT CONNECT ON DATABASE quizzivy TO quizzivy_app;
```

`quizzivy_migrate` owns the schema and runs `goose`. `quizzivy_app` is what the
API connects as and never owns anything (§13.5). `pg_trgm` is a trusted extension
in PG13+, so `quizzivy_migrate` can install it without superuser.

---

## 2. Foundations

### `00001_create_schema_and_extensions.sql`

```sql
CREATE SCHEMA app;

CREATE EXTENSION IF NOT EXISTS pg_trgm;    -- questions_prompt_trgm_idx [D-11]
CREATE EXTENSION IF NOT EXISTS unaccent;   -- Vietnamese search        [D-11]

-- unaccent() is STABLE in PG18 -- BOTH the one-argument and the two-argument
-- form; verified on 18.6. Postgres therefore refuses it in an index expression
-- ("functions in index expression must be marked IMMUTABLE"). This wrapper pins
-- the dictionary and asserts immutability, which is the standard workaround.
--
-- The assertion is a deliberate white lie: if the unaccent dictionary file ever
-- changes, every index built on this function must be REINDEXed. That is an
-- accepted trade-off and is why the dictionary is named explicitly.
CREATE FUNCTION app.immutable_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS
  $fn$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $fn$;

-- Documentation only: this has been the default since PG15. Kept so the
-- intent in spec §13.2 is visible in the schema history.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
```

Both extensions are *trusted* in PG13+, so `quizzivy_migrate` can install them
without superuser.

### `00002_create_enums.sql`

Enums where the set is genuinely closed (§13.2). Adding a value is easy;
removing one is not, so anything that might grow is `text` with a `CHECK`, or a
lookup table.

```sql
CREATE TYPE app.user_role        AS ENUM ('admin','student');
CREATE TYPE app.test_status      AS ENUM ('draft','published','archived');
CREATE TYPE app.question_type    AS ENUM ('single_choice','multiple_choice',
                                          'true_false','fill_blank','short_answer');
CREATE TYPE app.attempt_status   AS ENUM ('in_progress','submitted','timed_out',
                                          'graded','voided');
CREATE TYPE app.media_kind       AS ENUM ('image','audio');
CREATE TYPE app.join_source      AS ENUM ('admin','join_code');
CREATE TYPE app.integrity_action AS ENUM ('warn','flag','auto_submit');   -- [D-15]
```

`integrity_action` is new — §7's `IntegrityPolicy.onLimitExceeded` is a closed
three-value set and belongs in the type system, not a `CHECK`.

**`attempt_events.kind` is deliberately not an enum.** §10.1's list looks closed,
but an append-only telemetry log reliably gains kinds, and each new one would be
a migration plus a deploy-ordering problem for a value that is never joined on.
Constrained by length only.

### `00003_create_updated_at_trigger.sql`

`updated_at` via trigger, never application code (§13.2). One function, reused.

```sql
CREATE FUNCTION app.set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$fn$;
```

Applied to `users`, `classes`, `questions`, `tests`, `assignments`,
`attempt_answers` — every table with an `updated_at` column. Version and event
tables are immutable or append-only and have none.

---

## 3. Identity

### `00004_create_users.sql`

```sql
CREATE TABLE app.users (
  id                   uuid PRIMARY KEY DEFAULT uuidv7(),
  email                text NOT NULL CHECK (length(email) BETWEEN 3 AND 254),
  full_name            text NOT NULL CHECK (length(full_name) BETWEEN 1 AND 200),
  role                 app.user_role NOT NULL DEFAULT 'student',
  password_hash        text,                  -- NULL = Google-only account
  must_change_password boolean NOT NULL DEFAULT false,
  disabled_at          timestamptz,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT users_must_change_needs_password
    CHECK (NOT must_change_password OR password_hash IS NOT NULL)   -- [D-16]
);

CREATE UNIQUE INDEX users_email_lower_key ON app.users (lower(email));
CREATE INDEX users_role_active_idx ON app.users (role) WHERE disabled_at IS NULL;

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON app.users
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
```

- `text` with a length `CHECK` over `varchar(n)`, per the schema-design reference:
  `varchar(n)` gives no performance benefit and makes future changes harder. 254
  is the RFC 5321 address limit.
- `users_email_lower_key` is the §13.3 sketch, unchanged. Case-insensitive
  uniqueness must be an expression index — a plain `UNIQUE (email)` would let
  `A@x.com` and `a@x.com` coexist and break §5.1's linking rule.
- `users_must_change_needs_password` encodes §5.4's "Google-only users never hit
  `/change-password`" as a constraint rather than a convention.
- `users_role_active_idx` serves §8's active-student count. Tiny table; it exists
  so the dashboard query is not a repeated seq scan, not because it is hot.

### `00004` (cont.) — `user_identities`

```sql
CREATE TABLE app.user_identities (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id          uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  provider         text NOT NULL CHECK (provider IN ('google')),
  provider_user_id text NOT NULL,             -- Google `sub`, immutable
  email_at_link    text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),

  UNIQUE (provider, provider_user_id),
  UNIQUE (user_id, provider)                  -- [D-08]
);
```

- `ON DELETE CASCADE`: an identity is a true owned child of a user — meaningless
  without it, and never referenced elsewhere.
- `UNIQUE (provider, provider_user_id)` is §5.3 step 4's "identity exists → log
  in" lookup and prevents one Google account reaching two users.
- `UNIQUE (user_id, provider)` is new. §15's `DELETE /auth/google/link` and §7's
  `linkedProviders` both assume at most one Google identity per user; without it
  "unlink Google" is ambiguous.
- **The §13.3 sketch's `CREATE INDEX ON (user_id)` is dropped** — it is now the
  redundant prefix of `UNIQUE (user_id, provider)`. The indexing reference is
  explicit: avoid redundant single-column indexes when a composite exists.
- `provider` stays `text + CHECK` rather than an enum: a second provider is a
  plausible v2 change and `ALTER TYPE` in a deploy is worse than `ALTER TABLE`.

### `00005_create_refresh_tokens.sql`

§13.5 specifies the columns in prose only. Full DDL:

```sql
CREATE TABLE app.refresh_tokens (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id     uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  family_id   uuid NOT NULL,
  token_hash  bytea NOT NULL UNIQUE CHECK (length(token_hash) = 32),
  issued_at   timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  replaced_by uuid REFERENCES app.refresh_tokens(id) ON DELETE SET NULL,
  user_agent  text,
  ip          inet,                                                 -- [D-10]

  CHECK (expires_at > issued_at)
);

CREATE INDEX refresh_tokens_family_idx ON app.refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_live_idx
  ON app.refresh_tokens (user_id) WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_expiry_idx
  ON app.refresh_tokens (expires_at) WHERE revoked_at IS NULL;
```

- `token_hash bytea` with a length check — SHA-256 is exactly 32 bytes, so a
  wrong-length value is a bug caught at write time. Lookup is by hash and the
  comparison is constant-time in Go, per §13.5.
- `family_id` is the rotation chain. §5.2's reuse detection is
  `UPDATE … SET revoked_at = now() WHERE family_id = $1`, which is why
  `refresh_tokens_family_idx` exists and is not optional.
- `replaced_by` `ON DELETE SET NULL`: rows are pruned by expiry, and a pruned
  predecessor must not cascade-delete a live successor.
- `refresh_tokens_expiry_idx` is the cleanup job's index. Partial on
  `revoked_at IS NULL` because revoked rows are deleted by family, not by expiry.
- `ip inet` not `text`: `inet` validates, normalizes IPv6, and supports subnet
  containment if abuse investigation ever needs it. Same for `audit_log.ip`.

---

## 4. Classes and join codes (§6)

### `00006_create_classes.sql`

```sql
CREATE TABLE app.classes (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  name              text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  description       text,
  self_join_enabled boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER classes_set_updated_at BEFORE UPDATE ON app.classes
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
```

No index beyond the PK. There will be single-digit classes; §8's list is a seq
scan and that is the correct plan.

### `00006` (cont.) — `class_join_codes`

```sql
CREATE TABLE app.class_join_codes (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  class_id   uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  code_hash  bytea NOT NULL UNIQUE CHECK (length(code_hash) = 32),
  code_hint  text NOT NULL CHECK (length(code_hint) = 4),
  expires_at timestamptz NOT NULL,
  max_uses   integer CHECK (max_uses IS NULL OR max_uses > 0),
  uses_count integer NOT NULL DEFAULT 0 CHECK (uses_count >= 0),
  revoked_at timestamptz,
  created_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),

  CHECK (expires_at > created_at),                                  -- [D-09]
  CHECK (max_uses IS NULL OR uses_count <= max_uses)                -- [D-09]
);

CREATE UNIQUE INDEX class_join_codes_one_active
  ON app.class_join_codes (class_id) WHERE revoked_at IS NULL;
```

- `code_hash` is the only lookup path. `POST /join/preview` is
  `WHERE code_hash = $1 AND revoked_at IS NULL AND expires_at > now()`, which the
  `UNIQUE` serves. The plaintext code is never stored (§13.3), so a database dump
  does not hand over class access.
- **The exhaustion check is in the database**, not only in the handler. §6.5
  treats the code as a bearer secret; a `uses_count > max_uses` row is a silent
  policy failure and this makes it impossible.
- `class_join_codes_one_active` is §13.3's, unchanged. Note what it does *not*
  say: an **expired** code still satisfies `revoked_at IS NULL` and so still
  occupies the active slot. That matches §6.1 — only rotation revokes — and means
  rotation must revoke before inserting, inside one transaction.
- `created_by ON DELETE RESTRICT`: the audit trail for who issued a bearer secret
  must not be erasable by deleting a user.
- No separate `(class_id)` index. The partial unique covers the only hot lookup
  (the class's active code) and per §13.6 B-tree skip scan is left passive rather
  than adding prefix indexes preemptively.

### `00007_create_class_members.sql`

```sql
CREATE TABLE app.class_members (
  class_id     uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  joined_via   app.join_source NOT NULL,
  joined_at    timestamptz NOT NULL DEFAULT now(),
  added_by     uuid REFERENCES app.users(id) ON DELETE SET NULL,     -- [D-10]
  join_code_id uuid REFERENCES app.class_join_codes(id) ON DELETE RESTRICT,  -- [D-10]

  PRIMARY KEY (class_id, user_id),
  CONSTRAINT class_members_source_consistent
    CHECK ((joined_via = 'admin') = (join_code_id IS NULL))
);

CREATE INDEX class_members_user_idx ON app.class_members (user_id);
```

- `join_code_id` is new and cheap. §6.4 shows `joined_via` so the teacher can
  spot unexpected enrolments; recording *which* code someone came through means
  that after a rotation the teacher can tell "joined via the code I leaked" from
  "joined via the current one". `ON DELETE RESTRICT` because codes are revoked,
  never deleted — and `SET NULL` here would violate the consistency `CHECK`.
- `class_members_user_idx` is required, not optional: §9's `/app/classes` and the
  assignment target resolution both query by `user_id`, which the PK cannot serve.

---

## 5. Audit (§13.4)

### `00008_create_audit_log.sql`

```sql
CREATE TABLE app.audit_log (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  actor_user_id uuid REFERENCES app.users(id) ON DELETE SET NULL,
  action        text NOT NULL CHECK (action <> ''),
  entity        text NOT NULL CHECK (entity <> ''),
  entity_id     uuid,
  occurred_at   timestamptz NOT NULL DEFAULT now(),
  ip            inet,
  user_agent    text,
  diff          jsonb
);

CREATE INDEX audit_log_entity_idx
  ON app.audit_log (entity, entity_id, occurred_at DESC);
CREATE INDEX audit_log_actor_idx
  ON app.audit_log (actor_user_id, occurred_at DESC);
```

- `bigint IDENTITY` not uuid, for the same reason §13.3 gives for
  `attempt_events`: a high-volume append-only log read by a narrow key is better
  served by a narrow sequential key. `GENERATED ALWAYS AS IDENTITY` over `serial`
  per the schema-design reference.
- `actor_user_id ON DELETE SET NULL` — an audit row must survive the actor. The
  only nullable-actor case besides deletion is a system action.
- No `(occurred_at DESC)` index yet. §8 has no global audit screen, and §13.6
  says to leave skip scan passive rather than adding indexes preemptively. Add it
  with the screen that needs it.

Required entries per §13.4: class enrolment, join-code generation and rotation,
attempt reset/void/extend, password reset, media deletion, test publish. Each is
written with the `OLD`/`NEW` CTE pattern in `00-overview.md` §4.4.

---

## 6. Media (§11)

### `00010_create_media_assets.sql`

```sql
CREATE TABLE app.media_assets (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  kind              app.media_kind NOT NULL,
  storage_key       text NOT NULL UNIQUE,
  mime_type         text NOT NULL CHECK (mime_type IN
                      ('audio/mpeg','audio/mp4','audio/aac',
                       'image/png','image/jpeg','image/webp')),
  bytes             bigint NOT NULL CHECK (bytes > 0 AND bytes <= 10485760),
  duration_ms       integer CHECK (duration_ms IS NULL
                                   OR (duration_ms > 0 AND duration_ms <= 300000)),
  original_filename text NOT NULL,
  checksum_sha256   bytea NOT NULL CHECK (length(checksum_sha256) = 32),
  uploaded_by       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,

  CONSTRAINT media_assets_id_kind_key UNIQUE (id, kind),            -- [D-05]
  CONSTRAINT media_assets_audio_has_duration
    CHECK ((kind = 'audio') = (duration_ms IS NOT NULL)),
  CONSTRAINT media_assets_kind_matches_mime
    CHECK ((kind = 'audio') = (mime_type LIKE 'audio/%'))
);

CREATE INDEX media_assets_kind_created_idx
  ON app.media_assets (kind, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX media_assets_checksum_idx
  ON app.media_assets (checksum_sha256) WHERE deleted_at IS NULL;   -- [D-06]
```

- **§11.1's limits are constraints, not just handler code.** 10 MB = 10485760
  bytes, 5 minutes = 300000 ms. The handler validates first with a friendly
  Vietnamese message; the `CHECK` is what makes "we validate server-side" true
  even if a code path is added later that forgets.
- `mime_type` is an allowlist, matching §11.1's mp3/m4a decision exactly. Adding
  `.ogg`/`.wav` later is a one-line migration — which is the right shape for a
  decision §17.3 flags as revisitable.
- **`UNIQUE (id, kind)` is the composite-FK target [D-05].** `id` is already the
  PK, so this adds an index that is redundant for lookups. It exists purely so
  `questions` and `test_version_questions` can declare
  `FOREIGN KEY (media_asset_id, media_asset_kind) REFERENCES media_assets (id, kind)`,
  which is what lets a `CHECK` on those tables enforce §7's "audio policy present
  iff `media.kind === 'audio'`" *relationally* instead of in application code.
  Postgres requires a `UNIQUE` on exactly the referenced columns.
- **`checksum_sha256` has a plain index, never `UNIQUE` [D-06].** §13.3's comment
  says the checksum is for "dedupe identical re-uploads", but §11.1 says
  "Re-uploading creates a new `media_assets` row; it never overwrites an existing
  key". Those cannot both hold. Immutability wins because it is what lets a
  version snapshot reference an asset without copying the file. The checksum
  keeps its column and gets a non-unique index so the upload UI can *warn*
  ("bạn đã tải tệp này lên rồi") and so integrity can be re-verified against R2 —
  but it never blocks a write.
- `uploaded_by ON DELETE RESTRICT`: an asset referenced by a frozen test version
  must not lose its provenance.
- `deleted_at` soft delete per §13.2 — history matters here because a published
  version may reference the row. `DELETE /admin/media/:id` (§15) returns `409`
  when `test_version_questions` still references it; only unreferenced assets
  get a `deleted_at`. **R2 object lifecycle after soft delete is deliberately
  out of scope for v1** — the object stays. See `40-open-items.md`.

---

## 7. Question bank

Normalized, not `jsonb` (§13.3). Options and blanks are reordered by the builder,
edited individually, and graded against.

### `00011_create_questions.sql`

```sql
CREATE TABLE app.questions (
  id                          uuid PRIMARY KEY DEFAULT uuidv7(),
  type                        app.question_type NOT NULL,
  prompt                      text NOT NULL CHECK (prompt <> ''),
  media_asset_id              uuid,
  media_asset_kind            app.media_kind,                       -- [D-05]
  audio_max_plays             integer CHECK (audio_max_plays IS NULL
                                             OR audio_max_plays > 0),
  audio_allow_seek            boolean,                              -- [D-04]
  audio_show_transcript_after boolean,                              -- [D-04]
  transcript                  text,
  points                      numeric(8,2) NOT NULL
                                CHECK (points > 0 AND points <= 999999.99),
  explanation                 text,
  sample_answer               text,
  tags                        text[] NOT NULL DEFAULT '{}',
  created_by                  uuid NOT NULL
                                REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at                  timestamptz NOT NULL DEFAULT now(),
  updated_at                  timestamptz NOT NULL DEFAULT now(),
  deleted_at                  timestamptz,

  FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets (id, kind) ON DELETE RESTRICT,

  CONSTRAINT questions_media_pair_complete
    CHECK ((media_asset_id IS NULL) = (media_asset_kind IS NULL)),
  CONSTRAINT questions_audio_policy_iff_audio
    CHECK ((media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind)
           = (audio_allow_seek IS NOT NULL
              AND audio_show_transcript_after IS NOT NULL)),
  CONSTRAINT questions_transcript_iff_audio
    CHECK (transcript IS NULL
           OR media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind),
  CONSTRAINT questions_sample_answer_only_short_answer
    CHECK (sample_answer IS NULL OR type = 'short_answer')
);

CREATE TRIGGER questions_set_updated_at BEFORE UPDATE ON app.questions
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
```

Indexes:

```sql
CREATE INDEX questions_tags_idx
  ON app.questions USING gin (tags) WHERE deleted_at IS NULL;
CREATE INDEX questions_prompt_fts_idx
  ON app.questions USING gin (to_tsvector('simple', prompt)) WHERE deleted_at IS NULL;
CREATE INDEX questions_prompt_trgm_idx
  ON app.questions USING gin (app.immutable_unaccent(lower(prompt)) gin_trgm_ops)
  WHERE deleted_at IS NULL;
CREATE INDEX questions_media_idx
  ON app.questions (media_asset_id) WHERE media_asset_id IS NOT NULL;
CREATE INDEX questions_type_id_idx
  ON app.questions (type, id DESC) WHERE deleted_at IS NULL;
```

- **Audio policy columns are nullable [D-04].** §13.3 declares them
  `NOT NULL DEFAULT`, which cannot express §7's `audio?: AudioPolicy` — "present
  iff `media.kind === 'audio'`". With `NOT NULL DEFAULT false`, every text
  question carries a meaningless `audio_allow_seek`, and the API layer has to
  decide when to suppress it. Nullable plus a biconditional `CHECK` makes the
  database the one that knows. Defaults from §11.1 (`maxPlays` 2, `allowSeek`
  false, `showTranscriptAfterSubmit` true) move to the application, which is
  where they belong — they are authoring defaults, not storage defaults.
- `questions_sample_answer_only_short_answer` encodes §7's "`short_answer`;
  ADMIN ONLY". The column-list discipline in §11 of this document is what keeps
  it out of student payloads; this constraint keeps it from being *set* on a
  question type where the grading UI would never show it.
- **All bank indexes are partial on `deleted_at IS NULL` [D-12].** Every bank
  query filters deleted rows, so the predicate is free and the index stays
  smaller — directly from the indexing reference's partial-index guidance.
- **`questions_prompt_trgm_idx` is an addition [D-11].** §13.3 gives only
  `to_tsvector('simple', prompt)`. `'simple'` does no stemming and no diacritic
  folding, so in a Vietnamese-first product a search for `nghe` will not match
  `nghé`, and `phat am` will not match `phát âm`.

  **`pg_trgm` alone does not fix this** — an earlier draft of this document
  claimed it did, and that was wrong. Verified on 18.6: `'nghé' ILIKE '%nghe%'`
  is **false**, and `similarity('nghé','nghe')` is only 0.43. Trigram matching is
  case-insensitive but not accent-insensitive.

  So the index folds accents explicitly, through `app.immutable_unaccent`.
  Queries must use the **identical** expression or the planner will not match the
  index:

  ```sql
  WHERE app.immutable_unaccent(lower(prompt))
        LIKE '%' || app.immutable_unaccent(lower($1)) || '%'
  ```

  Verified that the wrapper folds Vietnamese correctly, including the `Đ`/`đ`
  stroke that plain Unicode decomposition misses: `Đường` → `duong`,
  `tiếng Việt` → `tieng viet`.

  Both indexes are kept: `@@` against the tsvector for word queries, trigram for
  substring and accent-insensitive matching. `to_tsvector(regconfig, text)` with
  a literal config is `IMMUTABLE`, so that expression index is legal as written;
  the one-argument form is `STABLE` and would be rejected.
- A **stored** `tsvector` generated column would be the indexing reference's
  preferred shape, but it buys little here (prompts are short, the table is
  small) and it would foreclose adding `unaccent`, which is not `IMMUTABLE`
  without a wrapper. Expression index for now; revisit only if search gets slow.
- `questions_type_id_idx` serves §8's type filter plus keyset pagination in one
  index (`uuidv7()` PKs make `id DESC` a valid time order, §13.8). Tag filtering
  goes through the GIN index; combining the two is a bitmap AND, which is correct.

### `00012_create_question_options.sql`

```sql
CREATE TABLE app.question_options (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal     smallint NOT NULL CHECK (ordinal >= 0),
  text        text NOT NULL CHECK (text <> ''),
  is_correct  boolean NOT NULL DEFAULT false,

  CONSTRAINT question_options_ordinal_key UNIQUE (question_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE                                   -- [D-13]
);
```

- `ON DELETE CASCADE`: an option is a true owned child (§13.2).
- No separate `(question_id)` index — the `UNIQUE (question_id, ordinal)` prefix
  serves every lookup, and adding one would be the redundancy the indexing
  reference warns about.
- **`DEFERRABLE INITIALLY IMMEDIATE` [D-13].** The builder reorders options by
  drag-and-drop (§8). Rewriting ordinals 0,1,2 → 1,2,0 in one `UPDATE` transiently
  violates uniqueness. Deferrable lets the reorder transaction issue
  `SET CONSTRAINTS app.question_options_ordinal_key DEFERRED` and write the new
  ordinals directly, instead of the two-phase negative-offset trick. Applied to
  every ordinal-unique on a draft-editable table: `question_options`,
  `question_blanks`, `test_sections`, `test_section_questions`. **Not** applied to
  the `test_version_*` tables, which are written once and never reordered.

### `00013_create_question_blanks.sql`

§13.3 says only "`question_blanks` and `question_blank_answers` mirror this".

```sql
CREATE TABLE app.question_blanks (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id    uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal        smallint NOT NULL CHECK (ordinal >= 1),
  case_sensitive boolean NOT NULL DEFAULT false,

  CONSTRAINT question_blanks_ordinal_key UNIQUE (question_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE
);

CREATE TABLE app.question_blank_answers (
  id       uuid PRIMARY KEY DEFAULT uuidv7(),
  blank_id uuid NOT NULL REFERENCES app.question_blanks(id) ON DELETE CASCADE,
  answer   text NOT NULL CHECK (answer <> ''),

  UNIQUE (blank_id, answer)
);
```

- **`ordinal >= 1`, not `>= 0`.** Blanks are addressed from the prompt Markdown by
  a 1-indexed placeholder (`{{1}}`, `{{2}}` — the default in `40-open-items.md`),
  so a 0-ordinal blank would be unreachable. §7's `blanks[].ordinal` is silent on
  the base; this pins it.
- `UNIQUE (blank_id, answer)` is exact, not `lower(answer)` — `case_sensitive`
  blanks legitimately want `Cat` and `cat` as distinct accepted answers.
  Case-insensitive matching happens at grading time against `case_sensitive`.
- Publish validation (§8) enforces "≥1 accepted answer per blank" and that the
  prompt's placeholder set equals the blank ordinal set. Neither is expressible
  as a `CHECK` — both span tables — so both live in T-2.10 with tests.

---

## 8. Tests, drafts, and versions

§13.3 jumps from `tests` straight to the version tables, leaving the **draft**
structure the builder autosaves into (§8, debounced 1.5s) undefined. Three
draft tables are added [D-14].

### `00014_create_tests.sql`

```sql
CREATE TABLE app.tests (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  title           text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
  description     text,
  status          app.test_status NOT NULL DEFAULT 'draft',
  current_version integer NOT NULL DEFAULT 0 CHECK (current_version >= 0),
  created_by      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,

  CONSTRAINT tests_published_has_version
    CHECK (status = 'draft' OR current_version > 0)
);

CREATE INDEX tests_status_id_idx
  ON app.tests (status, id DESC) WHERE deleted_at IS NULL;

CREATE TRIGGER tests_set_updated_at BEFORE UPDATE ON app.tests
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
```

`tests_published_has_version` makes §7's `currentVersion` honest: a test cannot
be `published` or `archived` with no snapshot behind it.

### `00013` (cont.) — draft structure [D-14]

```sql
CREATE TABLE app.test_sections (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  test_id      uuid NOT NULL REFERENCES app.tests(id) ON DELETE CASCADE,
  ordinal      smallint NOT NULL CHECK (ordinal >= 0),
  title        text NOT NULL CHECK (title <> ''),
  instructions text,

  CONSTRAINT test_sections_ordinal_key UNIQUE (test_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE
);

CREATE TABLE app.test_section_questions (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  test_section_id uuid NOT NULL REFERENCES app.test_sections(id) ON DELETE CASCADE,
  ordinal         smallint NOT NULL CHECK (ordinal >= 0),
  question_id     uuid NOT NULL REFERENCES app.questions(id) ON DELETE RESTRICT,

  CONSTRAINT test_section_questions_ordinal_key UNIQUE (test_section_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE,
  CONSTRAINT test_section_questions_no_dupes UNIQUE (test_section_id, question_id)
);

CREATE INDEX test_section_questions_question_idx
  ON app.test_section_questions (question_id);
```

- `question_id ON DELETE RESTRICT`, not `CASCADE`: a draft references a bank
  question, it does not own it. Questions are soft-deleted anyway (§13.2), so the
  restriction fires only on a hard purge — which is exactly when someone should
  be told a draft still uses it.
- `test_section_questions_question_idx` exists because of that `RESTRICT`. The
  schema-design reference is explicit: Postgres does not index FK columns, and
  without one, a parent delete scans and locks the child table. It also serves
  §8's "where used" on the question bank.

### `00015_create_test_versions.sql`

Immutable snapshot root. No `updated_at`, no soft delete — a published version
is a historical fact.

```sql
CREATE TABLE app.test_versions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  test_id      uuid NOT NULL REFERENCES app.tests(id) ON DELETE RESTRICT,
  version      integer NOT NULL CHECK (version > 0),
  total_points numeric(8,2) NOT NULL CHECK (total_points > 0),
  published_at timestamptz NOT NULL DEFAULT now(),
  published_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,

  UNIQUE (test_id, version),
  CONSTRAINT test_versions_id_test_key UNIQUE (id, test_id)          -- [D-17]
);

CREATE TABLE app.test_version_sections (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_id uuid NOT NULL
                    REFERENCES app.test_versions(id) ON DELETE CASCADE,
  ordinal         smallint NOT NULL CHECK (ordinal >= 0),
  title           text NOT NULL,
  instructions    text,

  UNIQUE (test_version_id, ordinal)
);
```

- **`test_id ON DELETE RESTRICT`** on `test_versions`, against §13.2's
  "`CASCADE` only for true owned children". A version *is* owned by a test, but
  attempts reference versions with `RESTRICT`, so cascading here would only ever
  produce a constraint violation two levels down with a confusing message.
  `RESTRICT` fails at the right place. Tests are soft-deleted regardless.
- **`UNIQUE (id, test_id)` [D-17]** is the composite-FK target that lets
  `assignments` prove its `test_version_id` actually belongs to its `test_id`
  (§9 below). Same technique as `media_assets`.
- `total_points` is stored, not derived. §7 calls it "server-computed"; computing
  it once at publish and freezing it means the score denominator on a two-year-old
  attempt cannot drift.

### `00016_create_test_version_content.sql`

```sql
CREATE TABLE app.test_version_questions (
  id                          uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_section_id     uuid NOT NULL
    REFERENCES app.test_version_sections(id) ON DELETE CASCADE,
  ordinal                     smallint NOT NULL CHECK (ordinal >= 0),
  source_question_id          uuid
    REFERENCES app.questions(id) ON DELETE SET NULL,                 -- [D-07]
  type                        app.question_type NOT NULL,
  prompt                      text NOT NULL,
  media_asset_id              uuid,
  media_asset_kind            app.media_kind,
  audio_max_plays             integer CHECK (audio_max_plays IS NULL
                                             OR audio_max_plays > 0),
  audio_allow_seek            boolean,
  audio_show_transcript_after boolean,
  transcript                  text,
  points                      numeric(8,2) NOT NULL CHECK (points > 0),
  explanation                 text,
  sample_answer               text,

  UNIQUE (test_version_section_id, ordinal),
  FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets (id, kind) ON DELETE RESTRICT,
  CONSTRAINT tvq_media_pair_complete
    CHECK ((media_asset_id IS NULL) = (media_asset_kind IS NULL)),
  CONSTRAINT tvq_audio_policy_iff_audio
    CHECK ((media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind)
           = (audio_allow_seek IS NOT NULL
              AND audio_show_transcript_after IS NOT NULL))
);

CREATE INDEX tvq_media_idx
  ON app.test_version_questions (media_asset_id) WHERE media_asset_id IS NOT NULL;
CREATE INDEX tvq_source_idx
  ON app.test_version_questions (source_question_id)
  WHERE source_question_id IS NOT NULL;
```

```sql
CREATE TABLE app.test_version_options (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_question_id uuid NOT NULL
    REFERENCES app.test_version_questions(id) ON DELETE CASCADE,
  ordinal                  smallint NOT NULL CHECK (ordinal >= 0),
  text                     text NOT NULL,
  is_correct               boolean NOT NULL DEFAULT false,
  UNIQUE (test_version_question_id, ordinal)
);

CREATE TABLE app.test_version_blanks (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_question_id uuid NOT NULL
    REFERENCES app.test_version_questions(id) ON DELETE CASCADE,
  ordinal                  smallint NOT NULL CHECK (ordinal >= 1),
  case_sensitive           boolean NOT NULL DEFAULT false,
  UNIQUE (test_version_question_id, ordinal)
);

CREATE TABLE app.test_version_blank_answers (
  id                     uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_blank_id  uuid NOT NULL
    REFERENCES app.test_version_blanks(id) ON DELETE CASCADE,
  answer                 text NOT NULL,
  UNIQUE (test_version_blank_id, answer)
);
```

- **`tvq_media_idx` is load-bearing, not decorative.** It is the index behind
  §15's `DELETE /admin/media/:id → 409 if referenced by a published version` and
  behind §8's "Delete blocked if referenced". Without it that check is a full
  scan of every version question on every delete attempt.
- **`source_question_id ON DELETE SET NULL` [D-07]**, deviating from the
  `RESTRICT` default in §13.2. §13.3 wants `source_question_id` to "preserve the
  bank link without coupling to it" — `RESTRICT` is coupling: it would let a
  three-year-old frozen version veto a bank cleanup forever. The column is
  nullable and purely informational (it powers "which bank question did this come
  from"); losing it degrades a link, not the attempt.
- No `is_correct` or `accepted_answers` ever leaves these tables toward a student.
  See §11.
- The snapshot is normalized rather than `jsonb`, per §13.3 and Thuong's
  decision. `00-overview.md` §5 records the alternative that was considered.

---

## 9. Assignments

§13.3 references `app.assignments(id)` but never defines the table. Full DDL,
with §7's nested objects flattened — three booleans and four policy fields do
not earn separate tables, and flattening keeps the monitor query a single row
read.

### `00017_create_assignments.sql`

```sql
CREATE TABLE app.assignments (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  test_id          uuid NOT NULL,
  test_version_id  uuid NOT NULL,
  opens_at         timestamptz NOT NULL,
  closes_at        timestamptz NOT NULL,
  closed_at        timestamptz,                                      -- [D-18]
  duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 1 AND 600),
  max_attempts     smallint NOT NULL DEFAULT 1 CHECK (max_attempts > 0),
  shuffle_questions boolean NOT NULL DEFAULT false,
  shuffle_options   boolean NOT NULL DEFAULT false,

  review_show_score            boolean NOT NULL DEFAULT true,
  review_show_correct_answers  boolean NOT NULL DEFAULT false,
  review_show_explanations     boolean NOT NULL DEFAULT false,

  integrity_require_fullscreen boolean NOT NULL DEFAULT false,
  integrity_block_copy_paste   boolean NOT NULL DEFAULT true,
  integrity_max_focus_loss     integer NOT NULL DEFAULT 0
                                 CHECK (integrity_max_focus_loss >= 0),
  integrity_on_limit_exceeded  app.integrity_action NOT NULL DEFAULT 'flag',
  integrity_min_away_ms        integer NOT NULL DEFAULT 3000
                                 CHECK (integrity_min_away_ms >= 0),

  created_by       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),

  FOREIGN KEY (test_version_id, test_id)
    REFERENCES app.test_versions (id, test_id) ON DELETE RESTRICT,   -- [D-17]
  CHECK (closes_at > opens_at),
  CHECK (closed_at IS NULL OR closed_at >= opens_at)
);

CREATE INDEX assignments_window_idx ON app.assignments (opens_at, closes_at);
CREATE INDEX assignments_version_idx ON app.assignments (test_version_id);

CREATE TRIGGER assignments_set_updated_at BEFORE UPDATE ON app.assignments
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
```

- **The integrity defaults are §10.3 verbatim** — `requireFullscreen: false`,
  `blockCopyPaste: true`, `maxFocusLoss: 0`, `onLimitExceeded: 'flag'`. They live
  in the DDL so a new assignment is conservative even if a handler forgets.
  `integrity_min_away_ms` defaults to §10.1's 3000.
- **There is no `status` column [D-18].** §7 types it, but it is a pure function
  of `opens_at`, `closes_at` and `closed_at`, and storing it would require a
  scheduler to flip rows at two timestamps per assignment — a cron job, a missed
  tick, and a stale-status bug class, all for a value the API can compute in the
  projection. `closed_at` exists so the admin can close early, which is the one
  thing the window cannot express.
- **The composite FK [D-17]** proves `test_version_id` belongs to `test_id`.
  Without it, an assignment could reference version 3 of test A while claiming
  test B, and every downstream join would silently produce the wrong paper.
- **`test_version_id` is immutable once an attempt exists.** This is enforced in
  the application (T-3.2), not the schema: re-pointing an assignment with zero
  attempts to a corrected version is a legitimate workflow, and a constraint
  strong enough to stop the dangerous case would also forbid the safe one.
  Existing attempts always carry their own `test_version_id` regardless (§10).
- `assignments_window_idx` serves §9's due/upcoming/completed partitioning.

### `00018_create_assignment_targets.sql`

§7's `targets: { classIds, studentIds }` — two link tables, not an array column.
Arrays would make "which assignments is this student eligible for" a GIN
containment query against a growing list instead of an index scan, and would
lose referential integrity to a deleted class.

```sql
CREATE TABLE app.assignment_classes (
  assignment_id uuid NOT NULL REFERENCES app.assignments(id) ON DELETE CASCADE,
  class_id      uuid NOT NULL REFERENCES app.classes(id) ON DELETE RESTRICT,
  PRIMARY KEY (assignment_id, class_id)
);
CREATE INDEX assignment_classes_class_idx ON app.assignment_classes (class_id);

CREATE TABLE app.assignment_students (
  assignment_id uuid NOT NULL REFERENCES app.assignments(id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  PRIMARY KEY (assignment_id, user_id)
);
CREATE INDEX assignment_students_user_idx ON app.assignment_students (user_id);
```

- `assignment_id CASCADE` (owned child), target `RESTRICT` (referenced entity).
  Deleting a class that is still targeted should fail loudly, not silently
  un-assign a test.
- Both reverse indexes are required: `/app/assignments` (§9) resolves
  "assignments for this student" from both directions in one query.
- **Removing a student from a class does not revoke an in-flight attempt.** §6.4
  says attempts are retained. Eligibility is evaluated at attempt *creation*;
  after that the attempt stands on its own.

---

## 10. Attempts

The hot path. Everything here is read on every autosave.

### `00019_create_attempts.sql`

```sql
CREATE TABLE app.attempts (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  assignment_id     uuid NOT NULL REFERENCES app.assignments(id) ON DELETE RESTRICT,
  test_version_id   uuid NOT NULL REFERENCES app.test_versions(id) ON DELETE RESTRICT,
  student_id        uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  attempt_no        smallint NOT NULL CHECK (attempt_no > 0),
  status            app.attempt_status NOT NULL DEFAULT 'in_progress',
  session_id        uuid NOT NULL,                     -- takeover detection
  shuffle_seed      bigint NOT NULL,                                 -- [D-02]
  beacon_token_hash bytea NOT NULL CHECK (length(beacon_token_hash) = 32),  -- [D-03]
  started_at        timestamptz NOT NULL DEFAULT now(),
  deadline_at       timestamptz NOT NULL,              -- authoritative
  submitted_at      timestamptz,
  graded_at         timestamptz,
  score_earned      numeric(8,2) CHECK (score_earned IS NULL OR score_earned >= 0),
  score_total       numeric(8,2) CHECK (score_total  IS NULL OR score_total  > 0),
  focus_loss_count  integer NOT NULL DEFAULT 0 CHECK (focus_loss_count >= 0),
  flagged           boolean NOT NULL DEFAULT false,
  void_reason       text,

  UNIQUE (assignment_id, student_id, attempt_no),
  CHECK (deadline_at > started_at),
  CONSTRAINT attempts_live_not_submitted
    CHECK (status <> 'in_progress' OR submitted_at IS NULL),
  CONSTRAINT attempts_graded_has_timestamp
    CHECK (status <> 'graded' OR graded_at IS NOT NULL),
  CONSTRAINT attempts_void_has_reason
    CHECK ((status = 'voided') = (void_reason IS NOT NULL))
);
```

Indexes:

```sql
CREATE UNIQUE INDEX attempts_one_live
  ON app.attempts (assignment_id, student_id) WHERE status = 'in_progress';
CREATE INDEX attempts_assignment_status_idx ON app.attempts (assignment_id, status);
CREATE INDEX attempts_student_started_idx   ON app.attempts (student_id, started_at DESC);
CREATE INDEX attempts_version_idx           ON app.attempts (test_version_id);
CREATE INDEX attempts_grading_queue_idx
  ON app.attempts (submitted_at) WHERE status = 'submitted';
CREATE INDEX attempts_flagged_idx
  ON app.attempts (id) WHERE flagged;
```

- **`shuffle_seed` is new and non-optional [D-02].** §7 has
  `shuffleQuestions`/`shuffleOptions` on the assignment, and §14's E2E 2 requires
  answers to survive a mid-test reload. If the order is regenerated per request,
  a reload reshuffles and every stored answer binds to the wrong option — silent,
  and it corrupts the grade rather than crashing. The seed is drawn once at
  attempt creation and the ordering is a pure function of `(seed, question_id)`,
  so client, server, and the grading pass all derive the same order.
- **`beacon_token_hash` is new [D-03].** `navigator.sendBeacon` (§10.6) cannot set
  an `Authorization` header, and the 15-minute access token (§5.2) is normally
  expired by the `pagehide` of a 60-minute test. See `00-overview.md` §4.3.
  Stored hashed for the same reason refresh tokens are.
- **All three FKs are `RESTRICT`.** An attempt is evidence. Deleting an
  assignment, a version, or a student must fail rather than erase a graded
  record. §6.4 already says removing a class member retains attempts.
- `attempts_one_live` is §13.3's, unchanged, and is what makes
  `POST /app/assignments/:id/attempts` (§15) safely idempotent under a
  double-tap: the second insert loses the race and the handler resumes instead.
- `attempts_grading_queue_idx` and `attempts_flagged_idx` are the two dashboard
  counts §8 needs and §15 has no endpoint for. Both are partial and therefore
  near-empty most of the time — the indexing reference's exact use case.
- No `extended_by` column: deadline extensions rewrite `deadline_at` and the
  history lives in `audit_log` (§13.4), which is where §8's "confirm + reason"
  already goes.

### `00020_create_attempt_answers.sql`

```sql
CREATE TABLE app.attempt_answers (
  attempt_id      uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id     uuid NOT NULL
                    REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  payload         jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
  requires_manual boolean NOT NULL DEFAULT false,                    -- [D-19]
  auto_score      numeric(8,2) CHECK (auto_score   IS NULL OR auto_score   >= 0),
  manual_score    numeric(8,2) CHECK (manual_score IS NULL OR manual_score >= 0),
  final_score     numeric(8,2)
                    GENERATED ALWAYS AS (coalesce(manual_score, auto_score)) VIRTUAL,
  grader_comment  text,
  graded_by       uuid REFERENCES app.users(id) ON DELETE SET NULL,  -- [D-19]
  graded_at       timestamptz,                                       -- [D-19]
  updated_at      timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (attempt_id, question_id),
  CONSTRAINT attempt_answers_manual_grade_paired
    CHECK ((manual_score IS NULL) = (graded_at IS NULL))
);

CREATE INDEX attempt_answers_question_idx ON app.attempt_answers (question_id);
CREATE INDEX attempt_answers_pending_idx  ON app.attempt_answers (attempt_id)
  WHERE requires_manual AND manual_score IS NULL;

CREATE TRIGGER attempt_answers_set_updated_at BEFORE UPDATE ON app.attempt_answers
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
```

- **`final_score` is a PG18 virtual generated column**, exactly as §13.3
  specifies — computed on read, no storage, never stale
  ([ddl-generated-columns](https://www.postgresql.org/docs/18/ddl-generated-columns.html)).
  VIRTUAL is the PG18 default; it is written explicitly anyway so the intent
  survives a reader who does not know that.

  **The constraints this imposes**, verified against 18.6 rather than recalled:

  | Operation on a virtual generated column | PG18.6 |
  |---|---|
  | `CREATE INDEX` | ❌ "indexes on virtual generated columns are not supported" |
  | `UNIQUE` | ❌ "unique constraints on virtual generated columns are not supported" |
  | `PRIMARY KEY` | ❌ "primary keys on virtual generated columns are not supported" |
  | `CREATE STATISTICS` | ❌ "statistics creation on virtual generated columns is not supported" |
  | `SET NOT NULL` | ✅ allowed |
  | `CHECK` referencing it | ✅ allowed |
  | `sum()` / aggregates | ✅ allowed |
  | Logical replication | ❌ stored only |

  None of the rejections hurt here — `final_score` is only ever summed within one
  attempt (≤ ~100 rows). But the missing index is why `pendingManual` cannot be
  derived from it, and why `requires_manual` exists. Do not add
  `ORDER BY final_score` to a cross-attempt query without reading this table.
- **`requires_manual` is new [D-19].** §7's `score.pendingManual` needs an
  indexable predicate, and `final_score IS NULL` is not one. Set at answer
  creation from the question type (`short_answer` ⇒ true) so the grading queue is
  a partial-index scan rather than a join back to `test_version_questions`.
- **`graded_by` / `graded_at` are new [D-19].** §8's grading screen records points
  and a comment; who graded and when is the minimum for a two-person future
  (§1.1's third role) and costs two columns.
- `payload jsonb` is correct here and not a violation of §13.3's "do not stuff
  options into jsonb". A student answer is a closed variant type (§7's `Answer`)
  written and read as a unit, never joined, never filtered. The zod schema is the
  contract; the `CHECK` only ensures it is an object, not a bare scalar.
- **`question_id ON DELETE RESTRICT`, referencing `test_version_questions`** — an
  answer belongs to the frozen version, never to the mutable bank. This is the
  structural half of §7's versioning invariant.
- `attempt_answers_question_idx` supports the `RESTRICT` and the per-question
  analytics the normalized snapshot exists to enable.

### `00021_create_attempt_audio_plays.sql`

```sql
CREATE TABLE app.attempt_audio_plays (
  attempt_id     uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id    uuid NOT NULL
                   REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  plays          integer NOT NULL DEFAULT 0 CHECK (plays >= 0),
  last_played_at timestamptz,

  PRIMARY KEY (attempt_id, question_id)
);

CREATE INDEX attempt_audio_plays_question_idx
  ON app.attempt_audio_plays (question_id);
```

Unchanged from §13.3 apart from the FK index. `plays` has **no upper bound
`CHECK` against `audio_max_plays`** — deliberately. §11.4 is explicit that
over-limit plays are reported to the teacher, never enforced retroactively, so a
constraint that rejected the write would turn a reporting signal into a 500 on a
fire-and-forget endpoint.

The increment is `INSERT … ON CONFLICT (attempt_id, question_id) DO UPDATE SET
plays = attempt_audio_plays.plays + 1, last_played_at = now() RETURNING plays` —
one statement, atomic, returns the authoritative count §11.4 requires.

### `00022_create_attempt_events.sql`

```sql
CREATE TABLE app.attempt_events (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  session_id  uuid NOT NULL,                                         -- [D-01]
  kind        text NOT NULL CHECK (kind <> '' AND length(kind) <= 40),
  occurred_at timestamptz NOT NULL,       -- client time, offset-corrected
  received_at timestamptz NOT NULL DEFAULT now(),
  client_seq  integer NOT NULL CHECK (client_seq >= 0),
  question_id uuid REFERENCES app.test_version_questions(id) ON DELETE SET NULL,
  meta        jsonb,

  UNIQUE (attempt_id, session_id, client_seq)                        -- [D-01]
);

CREATE INDEX attempt_events_timeline_idx
  ON app.attempt_events (attempt_id, occurred_at);
CREATE INDEX attempt_events_question_idx
  ON app.attempt_events (question_id) WHERE question_id IS NOT NULL;
```

**[D-01] is the most important deviation in this document.**

§13.3 declares `UNIQUE (attempt_id, client_seq)`. §10.6 says `clientSeq` is a
monotonic counter buffered in memory and `sessionStorage`. `sessionStorage` does
not survive a tab close, a crash, or a device change — all three of which §10.1
explicitly expects and names (`resume`, `session_takeover`). On any of them the
counter restarts at 1, every event of the resumed session collides with an
existing row, and the insert fails.

The failure is silent — §10.6 mandates fire-and-forget — and it destroys exactly
the portion of the timeline the teacher most wants: what happened *after* the
student came back. Adding `session_id` to the key scopes the sequence to the
session that generated it, which is what `clientSeq` always meant.

Inserts additionally use `ON CONFLICT (attempt_id, session_id, client_seq) DO
NOTHING`, so a retried `sendBeacon` (which cannot report success, and so is
retried on the next flush) cannot fail a whole batch on one duplicate row.

- `kind` is `text`, not an enum — see §2. Length-bounded so a malformed client
  cannot write an essay into an indexed-adjacent column.
- `question_id` FK with `ON DELETE SET NULL`: version questions are never
  deleted, so this never fires; it is there so "the question on screen" (§10.4)
  is a real reference rather than a loose uuid.
- `bigint IDENTITY` PK per §13.3 — narrow sequential key for an append-only log
  read only by `attempt_id`. No `UPDATE` or `DELETE` is ever issued; §11 enforces
  that with grants rather than convention.

---

## 11. Privileges and the student-payload rule

### `00009_grant_app_role.sql`

```sql
GRANT USAGE ON SCHEMA app TO quizzivy_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA app TO quizzivy_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA app TO quizzivy_app;

-- Custom types need their own USAGE; schema USAGE is not enough.
GRANT USAGE ON TYPE app.user_role TO quizzivy_app;   -- and the other six

-- Append-only (§13.4). audit_log exists by Phase 1; attempt_events gets the
-- same REVOKE in the migration that creates it (Phase 3).
REVOKE UPDATE, DELETE ON app.audit_log FROM quizzivy_app;

ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO quizzivy_app;
ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  GRANT USAGE, SELECT ON SEQUENCES TO quizzivy_app;
```

Note the ordering: the blanket `GRANT … ON ALL TABLES` runs first and the two
`REVOKE`s narrow it afterwards. Reversing them silently re-grants.

§13.3 asserts that no `UPDATE` or `DELETE` is ever issued against
`attempt_events`. This makes that a privilege rather than a promise. Same for
`audit_log`: an audit trail the application can rewrite is not an audit trail.

The app role never owns anything and cannot run DDL, per §13.5.

### The student-payload rule (§13.5)

`sample_answer`, `transcript`, `is_correct`, and accepted blank answers must
never reach a student response. Three layers, because one is not enough:

1. **No `SELECT *` in application code** (§13.8). Every student-facing query
   names its columns. The `test_version_options` projection for a student is
   `id, ordinal, text` — `is_correct` is not in the list and cannot be
   accidentally serialized by adding a struct field.
2. **Separate Go types.** `StudentQuestion` and `AdminQuestion` are distinct
   structs in distinct packages, not one struct with `json:"-"` tags. A forgotten
   tag is invisible; a missing field is a compile error.
3. **A test that asserts the keys are absent** from `GET /app/attempts/:id`,
   required by §13.5 and listed as E2E 9 in §14. It walks the response JSON
   recursively and fails on `isCorrect`, `sampleAnswer`, `acceptedAnswers`, or
   `transcript` at any depth — not just at the top level, which is where a
   nested-object regression would hide.

`transcript` is the one exception and only on `GET /app/attempts/:id/result`,
gated on `audio_show_transcript_after` (§11.3). It is served from the result
endpoint and never from the attempt endpoint, so the take-test payload has no
code path that could leak it.

---

## 12. Deviation register

Every departure from the §13.3 sketch, with the reason in one line. Anything not
listed here matches the spec.

| # | Deviation | Why |
|---|---|---|
| D-01 | `attempt_events`: add `session_id`, key becomes `UNIQUE (attempt_id, session_id, client_seq)`, insert `ON CONFLICT DO NOTHING` | `sessionStorage`-scoped `clientSeq` restarts at 1 on resume/takeover, silently discarding the resumed session's whole timeline (§10) |
| D-02 | `attempts`: add `shuffle_seed bigint NOT NULL` | Shuffle must be reproducible across reload and grading; §7 stores the flags but no seed, so a reload rebinds answers to the wrong options |
| D-03 | `attempts`: add `beacon_token_hash bytea NOT NULL` | `sendBeacon` cannot send `Authorization`, and the access token has usually expired by `pagehide` (§10.6) |
| D-04 | `questions` / `test_version_questions`: audio policy columns nullable + biconditional `CHECK` | §13.3's `NOT NULL DEFAULT` cannot express §7's `audio?: AudioPolicy` ("present iff `media.kind === 'audio'`") |
| D-05 | `media_assets`: add `UNIQUE (id, kind)`; referencing tables carry `media_asset_kind` and a composite FK | Makes "audio policy ⇒ audio asset" a database constraint instead of an application convention |
| D-06 | `media_assets.checksum_sha256`: plain index, never `UNIQUE` | §13.3's "dedupe" comment contradicts §11.1's immutability rule; immutability wins, checksum becomes a warning + integrity check |
| D-07 | `test_version_questions.source_question_id`: `ON DELETE SET NULL`, nullable | `RESTRICT` would let a frozen version veto bank cleanup forever; §13.3 wants the link "without coupling to it" |
| D-08 | `user_identities`: add `UNIQUE (user_id, provider)`, drop the standalone `(user_id)` index | §15's unlink endpoint assumes one Google identity per user; the composite makes the single-column index redundant |
| D-09 | `class_join_codes`: add `CHECK (expires_at > created_at)` and `CHECK (uses_count <= max_uses)` | §6.5 treats the code as a bearer secret; exhaustion must not be enforceable only in a handler |
| D-10 | Add `class_members.added_by` + `join_code_id`; `ip` columns are `inet` not `text` | §6.4 wants unexpected enrolments spottable — after a rotation, *which* code someone used is the useful fact |
| D-11 | `questions`: add `gin (app.immutable_unaccent(lower(prompt)) gin_trgm_ops)` alongside the tsvector index; requires `pg_trgm` + `unaccent` + an IMMUTABLE wrapper | `'simple'` does no diacritic folding, and `pg_trgm` alone does **not** either — `'nghé' ILIKE '%nghe%'` is false. Vietnamese-first search needs explicit accent folding (§12) |
| D-12 | All bank/test/media indexes partial on `deleted_at IS NULL` | Every query filters deleted rows, so the predicate is free and the index stays smaller |
| D-13 | Ordinal uniques on draft-editable tables are `DEFERRABLE INITIALLY IMMEDIATE` | Drag-to-reorder (§8) transiently violates uniqueness mid-transaction |
| D-14 | Add `test_sections` + `test_section_questions` | §13.3 defines no draft structure, but §8's builder autosaves one every 1.5s |
| D-15 | Add `app.integrity_action` enum | §7's `onLimitExceeded` is a closed three-value set (§13.2) |
| D-16 | `users`: `CHECK (NOT must_change_password OR password_hash IS NOT NULL)` | Encodes §5.4's "Google-only users never hit this" |
| D-17 | `test_versions`: add `UNIQUE (id, test_id)`; `assignments` uses a composite FK | Prevents an assignment referencing a version of a different test |
| D-18 | `assignments`: no `status` column; add `closed_at` | §7's status is a pure function of the window; storing it needs a scheduler and invents a stale-state bug class |
| D-19 | `attempt_answers`: add `requires_manual`, `graded_by`, `graded_at` | `final_score` is VIRTUAL and unindexable, so `pendingManual` needs a real-column predicate |

---

## 13. Migration inventory

goose, SQL, forward-only, one concern per file (§13.7). Every phase task names
the file it adds.

| File | Creates | Phase |
|---|---|---|
| `00001_create_schema_and_extensions.sql` | schema `app`, `pg_trgm`, `unaccent`, `app.immutable_unaccent()`, public revoke | 0 |
| `00002_create_enums.sql` | seven enum types | 0 |
| `00003_create_updated_at_trigger.sql` | `app.set_updated_at()` | 0 |
| `00004_create_users.sql` | `users`, `user_identities` | 1 |
| `00005_create_refresh_tokens.sql` | `refresh_tokens` | 1 |
| `00006_create_classes.sql` | `classes`, `class_join_codes` | 1 |
| `00007_create_class_members.sql` | `class_members` | 1 |
| `00008_create_audit_log.sql` | `audit_log` | 1 |
| `00009_grant_app_role.sql` | grants + default privileges for `quizzivy_app` | **1** |
| `00010_create_media_assets.sql` | `media_assets` | 2 |
| `00011_create_questions.sql` | `questions` | 2 |
| `00012_create_question_options.sql` | `question_options` | 2 |
| `00013_create_question_blanks.sql` | `question_blanks`, `question_blank_answers` | 2 |
| `00014_create_tests.sql` | `tests`, `test_sections`, `test_section_questions` | 2 |
| `00015_create_test_versions.sql` | `test_versions`, `test_version_sections` | 2 |
| `00016_create_test_version_content.sql` | `test_version_questions`, `_options`, `_blanks`, `_blank_answers` | 2 |
| `00017_create_assignments.sql` | `assignments` | 3 |
| `00018_create_assignment_targets.sql` | `assignment_classes`, `assignment_students` | 3 |
| `00019_create_attempts.sql` | `attempts` | 3 |
| `00020_create_attempt_answers.sql` | `attempt_answers` | 3 |
| `00021_create_attempt_audio_plays.sql` | `attempt_audio_plays` | 3 |
| `00022_create_attempt_events.sql` | `attempt_events` | 3 |

Notes on migration mechanics (§13.7):

- Every file has a `-- +goose Down` that is actually correct. CI runs `up` then
  `down` then `up` against a clean `postgres:18` (T-0.17), so an unreversible
  migration is a red build on the day it is written.
- **No `CREATE INDEX CONCURRENTLY` in any of these.** All 22 create empty tables,
  where a plain `CREATE INDEX` is instant and transactional. `CONCURRENTLY`
  becomes mandatory the first time an index is added to a populated table —
  and that file gets `-- +goose NO TRANSACTION`, because
  [CREATE INDEX CONCURRENTLY cannot run inside a transaction block](https://www.postgresql.org/docs/18/sql-createindex.html)
  and leaves an `INVALID` index if it fails.
- **`NOT NULL … NOT VALID` does not appear here.** Every column that should be
  `NOT NULL` says so inline at `CREATE TABLE`, where it is free. The PG18
  construct (§13.6) applies only to tightening an existing nullable column
  against existing rows — realistically Phase 5 or later. T-0.16 verifies it
  works so it is proven when needed rather than assumed.
- **`00009` runs in Phase 1, not last.** It was originally scheduled for Phase 3
  on the reasoning that `GRANT … ON ALL TABLES` must follow the tables. That is
  backwards: it left `quizzivy_app` unable to read anything from the first table
  until the end of Phase 3, and the first integration test to connect as the app
  role failed with `permission denied for schema app`.
  `ALTER DEFAULT PRIVILEGES` resolves it — applying to objects created *after*
  it, so one early migration covers every table Phases 2–5 add. Custom types
  need their own `USAGE`; schema `USAGE` is not enough to reference
  `app.user_role` in a query.
- Seed data lives in `seed/`, never in a migration (§13.7).

## 14. Query discipline (§13.8)

- No `SELECT *` anywhere. Enforced by review and by the separate student/admin
  Go types described in §11.
- Keyset pagination on every list endpoint. Because PKs are `uuidv7()`, `id DESC`
  is a valid recency order and most lists need no `created_at` index at all. The
  three that filter first — `questions` (by type), `tests` (by status),
  `media_assets` (by kind) — have `(filter, id DESC)` composite indexes above.
- `EXPLAIN (ANALYZE, BUFFERS)` before merging, on the three screens where N+1 is
  the default failure mode: the assignment monitor, attempt grading, and the
  integrity timeline. Each has one query for the parent and one for the children,
  never one per row.
- **A sequential scan on a small table is the correct plan.** At ~50 students and
  a few thousand attempts, most of these tables fit in a page or two and the
  planner will rightly ignore the indexes. The indexes above exist for the shapes
  that stay selective as data grows, not to force index scans today.
  `15-phase-5.md` measures latency at seeded volume rather than asserting plan
  shapes.
