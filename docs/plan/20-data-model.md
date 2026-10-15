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

R4 adds `schedule_label` and `room` (T-R4.18, D-39, §41).

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

- `code_hash` is the only lookup path, through its `UNIQUE` index. A typed code
  is looked up as `code_hash = ANY(candidates)`, and revocation, expiry and
  uses are judged on the row found (§33). The plaintext code is never stored
  (§13.3). From `00078` a new code is also sealed under a key the database
  never holds (D-27), so a dump does not hand it over; a legacy row's SHA-256
  still does, in 2^20 guesses, until the start-up rotation of v0.10.0 replaces it
  (§33); a revoked or expired one stays.
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

`00133_index_audit_log_recency.sql` (R5, T-R5.12) adds the order the Admin audit log is read in:

```sql
DROP INDEX CONCURRENTLY IF EXISTS app.audit_log_recent_idx;
CREATE INDEX CONCURRENTLY audit_log_recent_idx
  ON app.audit_log (occurred_at DESC, id DESC);
```

The drop is there for a rerun: a build that is interrupted leaves the index INVALID and the file
unrecorded, and `CREATE INDEX CONCURRENTLY IF NOT EXISTS` would find the name taken, build nothing
and let goose record the file.

Neither index above leads with the time, so without it every page sorts the whole range and the
export's keyset batches `(occurred_at, id) < ($1, $2)` have no index to walk. `entity` is read from
the heap; the facet statement at `range=7d` reads a week of rows through the index and `range=all`
reads the table once (100,000 rows: 18 ms). Grants are unchanged: the app role still has `INSERT` and
`SELECT` only.

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
  version may reference the row. `DELETE /teacher/media/:id` (§15) returns `409`
  when `test_version_questions` still references it; only unreferenced assets
  get a `deleted_at`. **R2 object lifecycle after soft delete is deliberately
  out of scope for v1** — the object stays. See `40-open-items.md`.

### `00082`–`00084` — the library's fields (T-R4.17a, D-29)

Three files on a populated table. Every new column is nullable with no
default, so the v0.9.0 binary's insert, which names none of them, keeps
working during the rolling deploy. None adds an index.

```sql
-- 00082_add_media_assets_library_fields.sql
ALTER TABLE app.media_assets
  ADD COLUMN display_name text,
  ADD COLUMN default_max_plays smallint,
  ADD COLUMN replaced_by uuid,
  ADD CONSTRAINT media_assets_display_name_check
    CHECK (char_length(display_name) BETWEEN 1 AND 200 AND display_name ~ '\S'),
  ADD CONSTRAINT media_assets_default_max_plays_check
    CHECK (default_max_plays IS NULL
           OR (default_max_plays BETWEEN 0 AND 3 AND kind = 'audio')),
  ADD CONSTRAINT media_assets_replaced_by_fkey
    FOREIGN KEY (replaced_by) REFERENCES app.media_assets(id) ON DELETE SET NULL,
  ADD CONSTRAINT media_assets_replaced_by_check CHECK (replaced_by <> id);

-- 00083_add_media_assets_dimensions.sql
ALTER TABLE app.media_assets
  ADD COLUMN width integer,
  ADD COLUMN height integer,
  ADD CONSTRAINT media_assets_dimensions_paired CHECK ((width IS NULL) = (height IS NULL)),
  ADD CONSTRAINT media_assets_dimensions_positive CHECK (width > 0 AND height > 0),
  ADD CONSTRAINT media_assets_dimensions_image_only CHECK (width IS NULL OR kind = 'image');

-- 00084_widen_media_assets_audio_bytes.sql
ALTER TABLE app.media_assets
  ADD CONSTRAINT media_assets_bytes_by_kind
    CHECK (bytes > 0 AND bytes <= CASE WHEN kind = 'audio' THEN 52428800 ELSE 10485760 END)
    NOT VALID;
ALTER TABLE app.media_assets VALIDATE CONSTRAINT media_assets_bytes_by_kind;
ALTER TABLE app.media_assets DROP CONSTRAINT media_assets_bytes_check;
```

- **`display_name`** is the name the library shows; NULL means the original
  filename, which never changes. The server stores it trimmed, so
  `media_assets_display_name_check` asks for 1 to 200 characters and one that
  is not white space. Readers select
  `coalesce(display_name, original_filename)`.
- **`default_max_plays`** is the play limit a new question starts from: NULL
  is "not set" and leaves the editor's own default, 0 is unlimited, 1 to 3 a
  limit. `media_assets_default_max_plays_check` keeps it off an image, as
  `questions_audio_policy_iff_audio` keeps an audio policy off a question
  without audio. Nothing in the server applies it to a question: the editor
  reads it (T-R4.65).
- **`replaced_by`** names the file that took this one's place. T-R4.17b
  writes it; T-R4.17a only reads it. **The library is the rows with
  `deleted_at IS NULL AND replaced_by IS NULL`**, one predicate
  (`media/repositories/list.go`, `library`) under the list, its total, the
  facets, the usage and the quota sum. `Get`, `Readable`, `ReferencesFor`,
  `LockForVersionUse` and `SoftDelete` do not ask it, so a replaced file keeps
  serving the published versions and the other owners' rows that still point
  at it. `ON DELETE SET NULL`: removing the newer row returns the older one to
  the library. `media_assets_replaced_by_check` refuses a file replaced by
  itself. No index on the column: a row is hard-deleted only by a test, and
  the release adds no index to a populated table.
  Replacement uses the existing 00082–00084 fields and constraints; T-R4.17b
  adds no migration or index. It locks owner-eligible groups and current
  questions in sorted order, then the old asset and its owner's existing quota
  advisory lock. After waiting it rechecks owner, library eligibility, kind,
  name and play limit; an owner change aborts rather than acquiring another
  owner's locks. It inserts with explicit locked owner and acting uploader,
  repoints only the retained current graph and audits the counts. Published
  versions, import review JSON, foreign bindings and test timestamps stay
  unchanged. Its repository entry accepts a concrete pool and owns the outer
  transaction; raw/acquired connections, wrappers and nested transactions are
  refused before mutation.
- **`width` and `height`** are an image's pixels, read from its header at
  upload (`platform/probe/image.go`). Both are NULL for audio, for an image
  whose header could not be read, and for every row stored before 00083:
  there is no backfill. Three checks, so a violation names its rule: both or
  neither, positive, images only.
- **The size limit depends on the kind.** 00010's unnamed column CHECK, which
  PostgreSQL named `media_assets_bytes_check`, held every file to 10 MiB.
  `media_assets_bytes_by_kind` allows audio 52428800 bytes (50 MiB) and an
  image 10485760 (DG-63). It is added `NOT VALID` and validated before the
  old constraint is dropped, so no moment has neither rule. Down re-adds the
  old CHECK `NOT VALID`: audio above 10 MiB may exist by then, and the
  constraint still binds every row written afterwards.
- **`questionCount` is computed, not stored.** A file is used by a question
  that is not deleted and attaches it (`questions.media_asset_id`), and by
  every member of a group whose `group_recordings` or `group_stimulus_assets`
  name it, each question once and whoever owns it. A member cannot be
  soft-deleted (`questions_context_complete`, 00035), so every member is live.
  The `unused` filter and the facet of that name are `NOT EXISTS` over the
  same subquery (`questionsUsing`), so a tab and a card cannot disagree. A
  published version and a group without members add nothing: such a file is
  "not used" and its delete is still refused.
- **The search** is
  `app.immutable_unaccent(lower(coalesce(display_name, '') || ' ' || original_filename)) LIKE '%' || app.immutable_unaccent(lower($n)) || '%' ESCAPE '\'`,
  the term through `db.EscapeLike`. No index: the rows are one owner's,
  behind `media_assets_owner_idx`.
- **The quota** is checked inside the insert's transaction, under an
  advisory lock keyed by the owner (§28).

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
-- Answers "which BANK questions use this asset". The delete-blocking check in
-- §8 is a different query, against `test_version_questions`, served by
-- `tvq_media_idx` in 00017 -- both indexes are wanted, for different questions.
CREATE INDEX questions_type_id_idx
  ON app.questions (type, id DESC) WHERE deleted_at IS NULL;
```

- **`audio_max_plays` needed a second constraint, added in `00014`.** The
  biconditional below covers `audio_allow_seek` and `audio_show_transcript_after`
  and does not mention `audio_max_plays`. Leaving it out of the right-hand side
  is right — `maxPlays` is `[integer, 'null']` with null meaning unlimited, so a
  NULL on an audio question is a value, not an absence, and requiring it would
  forbid "unlimited". But the other direction was missing: on a question with no
  media the left side is false, which the biconditional satisfies as soon as one
  of the two booleans is NULL, leaving `audio_max_plays` unconstrained. A
  `short_answer` question with `audio_max_plays = 3` satisfied every CHECK on
  the table. `questions_max_plays_only_with_audio` closes it, one-directional
  like `questions_transcript_iff_audio` beside it.

- **Audio policy columns are nullable [D-04].** §13.3 declares them
  `NOT NULL DEFAULT`, which cannot express §7's `audio?: AudioPolicy` — "present
  iff `media.kind === 'audio'`". With `NOT NULL DEFAULT false`, every text
  question carries a meaningless `audio_allow_seek`, and the API layer has to
  decide when to suppress it. Nullable plus a biconditional `CHECK` makes the
  database the one that knows. Defaults from §11.1 (`maxPlays` 2, `allowSeek`
  false, `showTranscriptAfterSubmit` true, and `allowPause` true, the fourth
  control, added by `00114` in §44) move to the application, which is
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
  drag-and-drop (§8).

  **Corrected against 18.6 in T-2.5.** This paragraph used to say that rewriting
  ordinals 0,1,2 → 1,2,0 in one `UPDATE` transiently violates uniqueness. It does
  not: Postgres checks a unique constraint at **end of statement**, so a
  set-based permutation is accepted with no deferral at all. What genuinely needs
  deferral is the **multi-statement** reorder — the shape a real one takes, since
  the client sends a list of `(id, ordinal)` pairs and the server writes them one
  row at a time, colliding with rows that have not moved yet. D-13 is right; this
  justification for it was not. `server/internal/db/reorder_test.go` pins both
  halves so the claim cannot rot again.

  Deferrable lets the reorder transaction issue
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

### `00015_create_tests.sql`

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

### `00015` (cont.) — draft structure [D-14]

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

### `00016_create_test_versions.sql`

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

### `00017_create_test_version_content.sql`

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
  §15's `DELETE /teacher/media/:id → 409 if referenced by a published version` and
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

### `00018_create_assignments.sql`

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

### `00019_create_assignment_targets.sql`

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

### `00020_create_attempts.sql`

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

### `00021_create_attempt_answers.sql`

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

### `00022_create_attempt_audio_plays.sql`

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

### `00023_create_attempt_events.sql`

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

### `00079_revoke_temporary_from_public.sql`

It revokes `TEMPORARY` on the database from `PUBLIC`, so `quizzivy_app` can
create nothing in `pg_temp`, the schema a `SECURITY DEFINER` function with a
loose `search_path` is attacked from (PR #193 closed one such hole); the
application creates no temporary object. The database's owner keeps the
privilege, so the migrate role's own temporary tables (`pg18_test.go`) still
work.

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
| D-20 | Add `maintenance_windows`, read-only for the app role; written only through `cmd/maintenance`, whose extensions are audited as System | §13 has no maintenance state; the API must be able to answer 503 without being able to schedule, move or cancel a window itself (T-R1.12) |
| D-21 | Reference data the app cannot run without is written by a migration, not `seed/`: the permission catalogue (`permissions`), the four built-in `roles` and their grants (`role_permissions`) | §13.7 keeps seed data out of migrations, but a production database with no roles cannot sign anyone in; plan 70 §3 (T-R2.1) |
| D-22 | `tests`, `questions`, `question_groups` and `media_assets` gain `owner_id` beside `created_by` / `uploaded_by`; `users` gains a nullable `created_by` | Ownership is who a row belongs to and provenance is who made it. R5's ownership transfer and R7's co-editing move the first and must not rewrite the second (T-R2.9). Expand half (00060 to 00063): each owner column is `NOT NULL NOT VALID`, with a fill trigger for the v0.7.0 binary. Contract half (00106, T-R4.49, §43): the constraints validated and the triggers and their functions dropped |
| D-23 | `classes.teacher_id` is backfilled from the oldest active Admin, with no `classes.created_by` | `app.classes` never recorded a creator, and the oldest active Admin is who v0.7.0 shows as every class's teacher, so no teacher changes; a class's creator from R2 on is its `class.created` audit row (Thuong, 2026-09-28; T-R2.9). Expand half (00065): `classes_teacher_id_not_null` `NOT VALID` and the trigger `classes_fill_teacher`. Contract half (00106, §43): validated, and trigger and function dropped |
| D-24 | Add the view `app.student_like_roles`, the one strict-student predicate: the built-in Student, or a custom role holding nothing but `learning.take_tests`; the other built-in roles are excluded by key | The Admin stores only its "Take tests" cell, so a predicate on grants alone would make it a student target; every student read and write, the legacy-role sync and the anonymiser use this one definition (plan 70 §4.3, T-R2.1) |
| D-25 | `users.role_id` references `app.roles` and replaces the `app.user_role` enum. Expand half (00056): added nullable, backfilled and `NOT NULL NOT VALID`, with `users_sync_legacy_role` keeping `role` in step. Contract half (00105, T-R4.49, §42): the constraint validated, and the trigger, `role`, its index and the enum dropped | §13.3's two-value enum holds no Teacher, Assistant or custom role, and v0.7.0 keeps writing `role` while both binaries run (plan 70 §3, T-R2.2) |
| D-26 | `users_last_admin`, an AFTER UPDATE OR DELETE trigger, refuses any change that leaves no active Admin; its function is `SECURITY DEFINER`, owned by `quizzivy_migrate`, with a pinned `search_path` and EXECUTE revoked from PUBLIC | The guard must hold on every path, a hand-written `UPDATE` included, and its row lock on `app.roles` needs the UPDATE privilege `00054` withholds from the app role (§30, T-R2.2) |
| D-27 | `class_join_codes` gains `code_ciphertext`, `key_id` and `lookup_scheme`: a new code is sealed with AES-256-GCM under `JOIN_CODE_KEY`, and its `code_hash` is an HMAC-SHA256 under that key's lookup key; legacy rows keep the SHA-256 | D5 has a teacher read a code back, which a hash cannot give; the key never reaches the database, so a dump can neither open a sealed code nor search the code space against its keyed hash (§33, T-R2.14a) |
| D-28 | Add `notifications` and `notification_preferences`. `notifications.params` and `target` are `jsonb`, bounded objects rather than columns, and `kind` is a checked dotted `text`, not an enum | §13.3 has no notification. Each kind carries its own few fields and every release adds kinds, so columns would be mostly NULL and an enum a migration per kind; the contract's closed `NotificationParams` and the Go type per kind are the schema. A name in `params` is a copy, not a reference: R5's anonymisation (T-R5.17a) deletes or scrubs the notifications that name an anonymised user (§34, T-R4.10a) |
| D-29 | `media_assets` gains `display_name`, `default_max_plays`, `replaced_by`, `width` and `height`, and its size limit depends on the kind: audio 50 MiB, an image 10 MiB | §13.3's row is a stored file and nothing else, and §11.1 allowed 10 MB for either kind. The deck's Media page names a file, gives it a play limit, shows an image's size and replaces a file in place (DG-63, DG-09); a replaced row stays, because published versions still point at it (§6, T-R4.17a, T-R4.17b) |
| D-30 | `users` gains nullable profile fields and bounded object `preferences` | The caller's private profile and account settings are required by T-R4.7; student-facing teacher names use the chosen display name without exposing private account fields |
| D-31 | Questions and frozen version questions gain nullable level and skill; the choice cap applies to authoring only | DG-62/DG-63 and T-R4.15 preserve historical question content while adding teacher metadata and filters |
| D-32 | `test_versions` gains a nullable `change_note`, 1 to 200 characters | The test detail's version history shows what the teacher said changed when publishing (DG-66, T-R4.16a). Nullable with no default and no backfill, so the previous release's insert keeps working and a version published before it reads NULL; the command stores NULL for a blank note, so the check never meets an empty string |
| D-33 | `questions` and `test_version_questions` gain a nullable `media_alt`, 1 to 1000 characters | The deck's builder draws "Alt text: describe the image for screen readers" under a question's image and the model had no field for it (DG-111, T-R4.62a). The bound is `ContentImage.alt`'s. It lives on the question and is frozen at publish like every other part of the question, so a later edit of the bank question never reaches a version. Nullable with no default and no backfill, so the previous release's insert keeps working and a row written before it reads NULL. No check ties it to an image in R4: the write path refuses it for any other media (a field error on `mediaAlt`) and the bank store keeps it only beside an image, and a database check would make an R3 write that swaps an image for audio fail during the rolling deploy. The rule becomes a database invariant one release later, as a contract step (§37) |
| D-34 | `assignments` gains `review_release` (`on_submit` or `after_close`, default `on_submit`) and `review_show_class_average` (default false) | The deck's Review step releases results after submitting or after the window closes, and shows the class average only on request (DG-65, T-R4.11). Constant defaults, so the previous release's insert keeps its meaning (results at once, no average) and nothing is rewritten; no fill trigger |
| D-35 | `assignments` gains a nullable `student_note`, 1 to 500 characters once trimmed of the whitespace JavaScript's `trim()` removes (the set of `shared/answered`, not `btrim`'s space alone) | The Student deck's Test intro draws a note from the teacher and the Teacher deck had no field for it (DG-71, T-R4.11). Nullable with no default, so the previous release's insert keeps working; the command trims the same set and stores NULL for a note that is blank once trimmed, so the check never meets an empty string, and a note of a tab, a newline or a no-break space is refused by the check as it is dropped by the command |
| D-36 | Add `assignment_student_overrides (assignment_id, student_id, closes_at, duration_minutes, extra_attempts, reason, created_by, created_at, updated_at)` | §13.3 has no per-student accommodation, and R4 lets a teacher give one student more time, more attempts or a reopened window without touching the assignment everyone else has (T-R4.12). A separate table, not columns on `assignments`, because the rows are per student and optional: no row is the assignment as it is. A new table adds nothing the previous release reads or writes, so the rolling deploy needs no expand step (§38) |
| D-37 | `refresh_tokens` gains a nullable `geo_label`, 1 to 80 characters | The Signed-in devices list names where a device last signed in or refreshed (T-R4.9, DG-58). A label is the edge's city and country at the request, so it is data about the request and lives beside `ip` and `user_agent` on the token row, which each rotation rewrites; a column on `users` or a table of devices would need a second writer and a second delete path. Nullable with no default, so the previous release's insert keeps working; the command stores NULL for no label, never an empty string, so the check never meets one (§39) |
| D-38 | `00103_compose_text_nfc.sql` has a Down that does nothing, where every other file's Down undoes its Up (§13, AGENTS "Migrations") | The file changes data and no schema, and nothing records which rows were decomposed before it ran, so there is nothing to restore: NFC and NFD are the same text, a rolled-back binary reads composed text exactly as it read decomposed text, and the one row it deletes (a decomposed accepted answer whose composed twin exists) was equal to the grader's eyes, so it is not brought back. A Down that decomposed every row would be wrong, since it would also decompose what was composed before. The file says so in the Down, and up/down/up still passes because the Down leaves the schema and every row as they are (§40) |
| D-39 | `classes` gains a nullable `schedule_label`, 1 to 120 characters, and a nullable `room`, 1 to 60 | The deck's class card and the student's class say when and where a class meets, and the model had no field for either (T-R4.18). Free text the teacher words, not a structured timetable, so one column each. Nullable with no default, so the previous release's insert keeps working; the commands trim and store NULL for a blank value, so the checks never meet an empty string (§41) |
| D-40 | `word_import_drafts` gains nullable `open_action_count` and `open_confirm_count`, `integer` and not negative | The history lists each import with "{n} need action" and "{n} to confirm" (T-R4.21), which are derived from the review body, and a body can be 8 MiB, so the page cannot parse one per row. The counts are written with the body in the same statement (processing result, review save, adoption), from `domain.Assess(body).Summary`. `integer` and not `smallint`, because a valid draft can exceed 32,767 derived findings and a count must not become a new authoring limit. Nullable with no default and no backfill: NULL means a draft this binary has not assessed, which is not zero, and the previous release's insert keeps working (§26) |
| D-41 | `word_import_sources.format` admits `text`, and `characters integer` (1 to 100000) is non-null exactly for a text source | A pasted test is a fourth source format, stored as a private `.txt` like any original (T-R4.55, `17-word-import.md` §1.41). The character count is the NFC code-point count the intake measured, so the history can say "{n} characters" without reading the object; bytes and hash cannot rebuild it. Nullable with no default, so a file insert from the previous release stays valid. `00090`'s Down restores the file-only CHECK `NOT VALID`, so retained text rows do not block it. `00091`'s Down drops the column, and its Up then refuses, atomically, while any text row is retained, because `word_import_sources_characters_format` is validated at once and no count can be rebuilt: nothing invents one (§23) |
| D-42 | `questions` and `test_version_questions` gain a nullable `audio_allow_pause`, present only with audio; `group_recordings` and `test_version_group_recordings` gain `allow_pause boolean NOT NULL DEFAULT true` | "Students can pause" is a fourth audio control, on a question's audio and on a group's recording (DG-111, DG-183, T-D4.13). The question columns are nullable and read as `COALESCE(…, true)` because the previous binary inserts those rows during the rolling deploy and writes no value, and because `questions_audio_policy_iff_audio` is a biconditional over the other two policy columns, which this one is kept out of; `questions_allow_pause_only_with_audio` and `tvq_allow_pause_only_with_audio` hold the one direction the way `questions_max_plays_only_with_audio` does. A recording always has its whole policy, so its column is NOT NULL with a constant default, which PostgreSQL stores without a rewrite. A write that leaves the field out keeps the stored value (§44), so a client that strips the key cannot switch pausing back on |
| D-43 | `media_assets` gains nullable `unreferenced_at` and `purged_at`, and the six tables that name a file get a trigger that refuses a purged one and clears the wait of a retired one | A replaced or deleted file keeps its object until seven days after nothing names it, then the sweep removes the object and marks the row; the row stays for the audit trail and the `replaced_by` chain, and the trigger is what makes "nothing can name a removed file" the database's rule and not a convention every writer must keep (F-68, §45) |
| D-44 | Add `ai_settings`, one row (a boolean primary key that only `true` satisfies), written by `00118` with the plan: a 200,000-credit pool, a 20,000-credit default allowance, a warning at 80% and requests on | The AI assistant's rules are the centre's, not a teacher's, and the app cannot run the budget without them, so the migration writes the row (D-21, D21 in plan 70 §1). The pool has one rule, pause everyone (DG-196), so there is no column for what happens when it runs out. The allowance bounds (1,000 to 10,000,000) and the warning share (1 to 99) are checks; the narrower ranges the Admin may pick (50 to 99) are the command's (§46, T-D4.31) |
| D-45 | Add `ai_credit_periods (month, pool_credits, added_credits, used_credits, reserved_credits)` with `ai_credit_periods_within_pool CHECK (used_credits + reserved_credits <= pool_credits + added_credits)` | D21: the pool is never overspent without an Admin's approval, so the hard stop is a constraint and not a check in a handler. `month` is the first day of the month in the centre's zone, enforced as a date on day 1; `pool_credits` is copied from `ai_settings` when the month's first use creates the row, so a later change of the plan never rewrites a month that has begun; `added_credits` is the Admin's approval and does not carry over (§46, T-D4.31) |
| D-46 | Add `ai_teacher_credits (month, user_id, allowance_credits, granted_credits, custom, paused, used_credits, reserved_credits, warned_at)` with `ai_teacher_credits_within_allowance CHECK (used_credits + reserved_credits <= allowance_credits)` and `ai_teacher_credits_user_idx (user_id, month DESC)` | The same constraint for a teacher's allowance. Three columns go beyond the task's list because the model needs them: `granted_credits` keeps what approved requests added to this month apart from the teacher's own allowance, so a change of the default rule and a new month treat it right; `warned_at` makes the warning at `warn_percent` once a month a fact of the row and not a read-then-write; and `custom` and `paused` belong to the person, so a month's first row copies them from the teacher's latest earlier row, which the index serves (§46, T-D4.31) |
| D-47 | Add `ai_reservations (id, month, user_id, feature, credits, created_at)` with a composite foreign key to `ai_teacher_credits (month, user_id)` and a `feature` slug checked by pattern, not an enum | A call holds its ceiling until it settles or is released, and a row that outlives its call is returned by the sweep after ten minutes. The foreign key is `RESTRICT` on purpose: deleting a teacher's month while a call holds credits would strand them in the pool for ever. `feature` is a slug for the reason `notifications.kind` is (D-28): a feature added by a release adds no migration |
| D-48 | Add `ai_usage_ledger`, append-only: `UPDATE` and `DELETE` are revoked from the app role in the file that creates it | One row per settled call with the actor, the feature, the tier, the four token counts the provider reported, the credits charged and the provider's request id, so a month's invoice reconciles with the pool. A ledger the application can rewrite proves nothing, which is why `audit_log` and `attempt_events` are the same (D-01, §11). `user_id` is `ON DELETE SET NULL`, run by the table's owner, so a row outlives its actor |
| D-49 | Add `ai_credit_requests`, with a partial unique index `ai_credit_requests_one_pending_idx (user_id, month) WHERE status = 'pending'` | At most one request per teacher per month is waiting (DG-200), which the index makes a fact of the data and not a race between two clicks. `decided_by` is `ON DELETE SET NULL` and is NULL when the console decided, which the audit row says. It is the one foreign key left unindexed on purpose: the table gains a row per request, and removing an Admin scans a few hundred |
| D-50 | `questions` gains a nullable `copied_from_id`, a self-reference `ON DELETE SET NULL`, and a partial index on the copies | A member copy of a bank question names the question it was made from, so that dragging an unedited copy out of its group (E1, `74e-e1-group-move.md`, "After release") puts the original back and no second bank question is left. The link is informational, like `test_version_questions.source_question_id` (D-07): it is cleared when the copy leaves its group and becomes an independent question, and `SET NULL` means a deleted origin never waits on a copy. Nullable with no default and no backfill, so the v0.11.6 binary's insert, which names no such column, keeps working; a copy it wrote reads NULL and is never restored. The REFERENCES is inline in the `ADD COLUMN`, so PostgreSQL skips the validation scan of a column it is adding. The index is partial because almost every question is not a copy, and `ON DELETE SET NULL` updates the copies by equality, which implies its predicate. See §48 |
| D-51 | Add `email_outbox`, one row per message to send, with a partial index on the pending rows | A message is written in the transaction that decided to send it, so a rolled-back decision sends nothing, and the API and the worker both drain the table (T-R5.2). `template` is a checked slug and not an enum, as `notifications.kind` is (D-28), because the template list is Go's and R7 adds templates without a migration. `to_address` is a copy, since `to_user_id` is `ON DELETE SET NULL` and an unsent message must outlive its user's row. `secret_ciphertext` holds a set-password token sealed under `SECRETS_KEY` with the row id as associated data, and is cleared in the statement that marks the row sent or failed. `dedupe_key` is a plain unique column: a second message with the same key is not written. `attempts` is bounded at 10 to match the dispatcher. See §49 |
| D-52 | `users` gains four nullable columns, `last_active_at`, `last_sign_in_at`, `last_sign_in_method` (CHECK in `password`, `google`, `set_password_link`) and `password_changed_at`; a new `user_activity_days (user_id, day)` records the local days a user was active | DG-54: the Students list's "Last active" and "Not signed in", the Settings "Changed …" line and R5's daily chart need facts the schema did not keep (the Students list used the last attempt). All four are nullable with no default, so the v0.10.0 binary's insert, which names none, keeps working; NULL `last_sign_in_at` is "never signed in". The migration backfills `last_sign_in_at` and `last_active_at` with the newest of the user's refresh token `issued_at`, attempt `started_at` and audit entry as actor, with the `updated_at` trigger off, so an account used before R5 does not read "Not signed in"; the method and `password_changed_at` stay NULL because nothing records them. The day table is keyed by the pair, indexed on `day` for range reads, and cascades with the user; a day is the local date in `APP_TIMEZONE`. See §50 |
| D-53 | `organization_settings` is a singleton keyed by `id boolean PRIMARY KEY DEFAULT true CHECK (id)`, written by the migration, and the app role has neither `INSERT` nor `DELETE` on it | The centre has exactly one settings document, and the front door and sign-in cannot run without it, so it is reference data the migration owns (D-21) and not a seed. A boolean key that only `true` satisfies makes a second row impossible without a trigger, and revoking the two statements means the application can only change the row, never lose it or add a rival. Every optional text is nullable and every bound is a CHECK, so the database refuses what the application would have refused. `CHECK (google_sign_in OR password_sign_in)` is the last line of the rule that keeps a way in. See §51 |
| D-54 | Add `data_exports`, one row for each request, with a partial unique index `data_exports_active_idx ON ((true)) WHERE status IN ('queued','running')` | At most one export is queued or running in the whole centre (T-R5.15): a full export of personal data is heavy and two Admins asking at once must not both build a ZIP, so the index makes "one at a time" a fact of the data and `409 EXPORT_IN_PROGRESS` its report. It is also the worker's queue, which holds one row at most. `status` is a checked text, not an enum, as `notifications.kind` is (D-28). `attempts` counts claims so an export a dying worker keeps re-claiming is failed after the third. `error_code` is a short code and never a message, so nothing about the data reaches the table. `requested_by` is `ON DELETE CASCADE`: an export of a deleted account has no reader. `data_exports_ready_has_file` ties `ready` to its key, size and expiry. See §52 |
| D-55 | Add `set_password_tokens`, one row per set-password link, with a full index on `user_id` and the 24-hour bound as a `CHECK` | A link is the credential of an account that has none yet (T-R5.3, D15), so the database holds only the SHA-256 of a 256-bit token (`token_hash bytea`, 32 bytes, unique), never the token, which waits sealed in `email_outbox` until the message is sent (D-51). `expires_at <= created_at + interval '24 hours'` is the database's own last line against a longer-lived link; the application writes `created_at` itself, from the clock it computes `expires_at` with, truncated to the microsecond, so the bound holds exactly. `used_at` and `revoked_at` are separate because the claim is one `UPDATE … SET used_at … WHERE used_at IS NULL AND revoked_at IS NULL AND expires_at > $now`, which two submits of one link race for and one wins, while a newer link, a password set and a disable revoke by `revoked_at`; the lookup reads all three alike. `user_id` cascades, since a link without its account is meaningless; `created_by` is provenance (an Admin, the account itself, or NULL for a job) and `SET NULL`s, so a deleted account's issuer leaves rows nobody can use. **Deviation from the plan:** T-R5.3 asked for a *partial* index on `(user_id) WHERE used_at IS NULL AND revoked_at IS NULL`. The index is full instead, because `ON DELETE CASCADE` from `users` must find rows of every state and a partial index would leave a delete of a user scanning the table; the unused-link reads (revoke-and-replace) still use it, over a handful of rows per account. See §53 |
| D-56 | Add `user_import_jobs`, one row for each Admin's import of accounts from a file, with `UNIQUE (created_by, request_id)` and a partial index on the open jobs | The API reads and judges the file and the worker creates the accounts (T-R5.14), so a job is both the request and the worker's queue: `user_import_jobs_open_idx (created_at) WHERE status IN ('queued','running')` holds the rows the worker may take and nothing else. `(created_by, request_id)` is unique so a retried request returns the job it stored the first time, and the caller's `requestId` is held for as long as the dialog is open. `total_rows` is the rows stored (1 to 2,000): the rows that passed every rule, including those whose address already belonged to an account, which are stored settled as skipped; a refused row is only counted in `invalid_count`, so the file's text does not outlive the request. `created_count + skipped_count` may not pass `total_rows`, and a `done` job has settled every row (`user_import_jobs_done_settled`). `attempts` (0 to 3) counts claims, and `error_code` is a short code and never a message, so nothing about the file reaches the table (`failed` has one and no other status does). `default_role_id` is kept because the worker re-checks the importer's right to every role of the job when it runs; it is `RESTRICT` and roles are never deleted. `created_by` is `ON DELETE CASCADE`: the job of a deleted account has no reader. `sheet_expires_at` and `sheet_downloaded_at` exist only for `delivery = 'sheet'`. See §54 |
| D-57 | Add `user_import_rows`, the rows of a job that passed, with `PRIMARY KEY (job_id, ordinal)` and a sealed password that only a created row may hold | `ordinal` counts the stored rows from 0 (not the file's lines) and is also a row's place in the hand-out sheet. `full_name` and `email` are kept as the file spelled them, trimmed and composed (NFC), and the account takes the same. `outcome` is `pending` until the worker settles the row as `created` (`user_id` is the account) or `skipped_exists`. `secret_ciphertext` holds the temporary password of a row created for a sheet, sealed under `SECRETS_KEY` with the job id and ordinal as associated data; `user_import_rows_secret_only_when_created` makes a password exist only beside the account it opens, and the transaction that hands the sheet out erases it. `user_id` and `class_id` are `ON DELETE SET NULL`, so a row outlives the account and the class it created or named, each with a partial index for the cascade. The rows keep names and addresses and nothing removes them yet; a purge by T-R5.16's housekeeping is open. See §54 |
| D-58 | Add `users_search_trgm_idx`, a gin `gin_trgm_ops` index on `app.immutable_unaccent(lower(full_name \|\| ' ' \|\| email))`, built `CONCURRENTLY` | The Users search of the Admin console (`listUsers`, `q`, T-R5.9a) is a `LIKE '%…%'` over that expression, which no b-tree serves, and it runs up to three times a request (the count, the facets, a sparse page). §13.6 held an index back until there was evidence; QC measured 99 ms with no filter, 400 ms for `q=nghe` and about 1 s for a search that finds nothing at 100,000 accounts (1.3 to 5 s at 500,000), and an index over this expression took them to 71 ms and 4 ms (QC-N-5). The expression is `repositories.UserSearchText`, shared by the query and the migration's test, so a change to one without the other fails `users_search_index_test.go`. The index reads `full_name` and `email` only, so an update of any other column stays heap-only; it adds a gin pending-list entry for each insert and each rename. A query of one or two characters has no trigram and still scans. Changing the `unaccent` dictionary needs a `REINDEX`, as for every index on `app.immutable_unaccent` (00001). The file is `NO TRANSACTION` and drops any invalid leftover first, as 00133 does |
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
| `00014_restrict_audio_max_plays_to_audio.sql` | CHECK on `questions` [D-04] | 2 |
| `00015_create_tests.sql` | `tests`, `test_sections`, `test_section_questions` | 2 |
| `00016_create_test_versions.sql` | `test_versions`, `test_version_sections` | 2 |
| `00017_create_test_version_content.sql` | `test_version_questions`, `_options`, `_blanks`, `_blank_answers` | 2 |
| `00018_create_assignments.sql` | `assignments` | 3 |
| `00019_create_assignment_targets.sql` | `assignment_classes`, `assignment_students` | 3 |
| `00020_create_attempts.sql` | `attempts` | 3 |
| `00021_create_attempt_answers.sql` | `attempt_answers` | 3 |
| `00022_add_assignment_draft_state.sql` | assignment publication state | 3 |
| `00023_create_attempt_events.sql` | `attempt_events` | 3 |
| `00024_create_attempt_audio_plays.sql` | `attempt_audio_plays` | 3 |
| `00025_widen_grading_queue_to_timed_out.sql` | grading queue index | 4 |
| `00026_add_classes_archived_at.sql` | class archive state | 4 |
| `00027_let_a_draft_be_archived.sql` | archive unpublished tests | 4 |
| `00028_add_attempts_teacher_note.sql` | attempt review notes | 4 |
| `00029_index_integrity_retention.sql` | integrity retention index | 5 |
| `00030_test_version_sequence.sql` | monotonic publication counter | Admin change request |
| `00031_allow_no_focus_loss.sql` | no-departure integrity policy | Admin change request |
| `00032_add_option_content.sql` | bounded inline documents on normalized bank/snapshot options | Word W-05a |
| `00033_add_question_content.sql` | bounded prompt/explanation documents on bank/snapshot questions | Word W-05b |
| `00034_add_question_gap_bindings.sql` | stable gap bindings on bank/snapshot blanks | Word W-06b |
| `00035_add_question_group_ownership.sql` | Independent bank/test group ownership and ordered member columns | Word W-07 |
| `00036_index_question_group_members.sql` | Concurrent group-member identity and ordinal indexes | Word W-07 |
| `00037_create_question_group_graph.sql` | Ordered units, materials, cloze targets, protected media/recording bindings | Word W-07 |
| `00038_index_version_question_sections.sql` | Concurrent section identity index for frozen questions | Word W-08 |
| `00039_create_version_group_graph.sql` | Independent immutable group/unit/material/recording snapshots | Word W-08 |
| `00040_create_group_audio_plays.sql` | Shared recording counters and append-only gesture receipts | Word W-09 |
| `00041_version_test_delivery.sql` | Immutable delivery algorithm marker | Word W-09d |
| `00042_create_word_import_sources.sql` | Private source intake, upload journal and immutable source sets | Word W-10a |
| `00043_create_word_import_runs.sql` | Fenced processing runs and append-only run events | Word W-11a |
| `00044_create_word_import_artifacts.sql` | Private stage artifact sets and object journal | Word W-13b |
| `00045_create_word_import_drafts.sql` | One revisioned review draft per import, plus a kept-aside reprocessed candidate | Word W-18 |
| `00046_create_word_import_commits.sql` | One append-only commit record per import linking the approved revision to its test | Word W-20 |
| `00047_add_word_import_run_profile.sql` | Recognition profile a run was scheduled with | Word W-14 |
| `00048_allow_legacy_word_sources.sql` | Legacy `.doc` sources | Word W-13 |
| `00049_index_question_group_bank_recency.sql` | Bank group listing ordered by recency | Word W-07 |
| `00050_add_word_import_files_removed_at.sql` | When retention removed an import's files, and the index its sweep reads | Word D-08 |
| `00051_grant_word_import_draft_delete.sql` | Lets the retention sweep delete a review draft when it removes an import's files | Word D-08 |
| `00052_allow_pdf_import_sources.sql` | PDF sources | Word D-10 |
| `00053_create_maintenance_windows.sql` | `maintenance_windows`, read-only for the app role | R1 (T-R1.12) |
| `00054_create_roles_and_permissions.sql` | `permissions`, `roles`, `role_permissions`, the built-in rows and their guard triggers; read-only for the app role | R2 (T-R2.1), D-21 |
| `00055_create_student_like_roles_view.sql` | `student_like_roles`, the strict student predicate | R2 (T-R2.1), D-24 |
| `00056_add_users_role_id.sql` | `users.role_id`, backfilled, `NOT NULL NOT VALID`, with the legacy-role sync trigger | R2 (T-R2.2), D-25 |
| `00057_index_users_role_id.sql` | `users_role_id_active_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.2) |
| `00058_add_users_session_epoch.sql` | `users.session_epoch` | R2 (T-R2.2) |
| `00059_add_users_last_admin_guard.sql` | the `users_last_admin` trigger, the schema's first `SECURITY DEFINER` function | R2 (T-R2.2), D-26 |
| `00060_add_tests_owner.sql` | `tests.owner_id`, backfilled from `created_by`, `NOT NULL NOT VALID`, with `app.fill_owner_from_created_by()` and its fill trigger | R2 (T-R2.9), D-22 |
| `00061_add_questions_owner.sql` | `questions.owner_id`, the same way | R2 (T-R2.9), D-22 |
| `00062_add_question_groups_owner.sql` | `question_groups.owner_id`, the same way | R2 (T-R2.9), D-22 |
| `00063_add_media_assets_owner.sql` | `media_assets.owner_id` from `uploaded_by`, with `app.fill_owner_from_uploaded_by()` | R2 (T-R2.9), D-22 |
| `00064_add_users_created_by.sql` | `users.created_by`, nullable, `ON DELETE SET NULL` | R2 (T-R2.9), D-22 |
| `00065_add_classes_teacher.sql` | `classes.teacher_id` from the oldest active Admin, `NOT NULL NOT VALID`, with `classes_fill_teacher` | R2 (T-R2.9), D-23 |
| `00066_add_tests_owner_fkey.sql` | `tests_owner_id_fkey`, `ON DELETE RESTRICT`, apart from its column | R2 (T-R2.9) |
| `00067_add_questions_owner_fkey.sql` | `questions_owner_id_fkey`, `ON DELETE RESTRICT`, apart from its column | R2 (T-R2.9) |
| `00068_add_question_groups_owner_fkey.sql` | `question_groups_owner_id_fkey`, `ON DELETE RESTRICT`, apart from its column | R2 (T-R2.9) |
| `00069_add_media_assets_owner_fkey.sql` | `media_assets_owner_id_fkey`, `ON DELETE RESTRICT`, apart from its column | R2 (T-R2.9) |
| `00070_add_classes_teacher_fkey.sql` | `classes_teacher_id_fkey`, `ON DELETE RESTRICT`, apart from its column | R2 (T-R2.9) |
| `00071_index_classes_teacher.sql` | `classes_teacher_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.10) |
| `00072_index_tests_owner.sql` | `tests_owner_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.10) |
| `00073_index_questions_owner.sql` | `questions_owner_bank_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.10) |
| `00074_index_question_groups_owner.sql` | `question_groups_owner_bank_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.10) |
| `00075_index_media_assets_owner.sql` | `media_assets_owner_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.10) |
| `00076_index_users_created_by.sql` | `users_created_by_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.10) |
| `00077_index_assignments_creator.sql` | `assignments_creator_idx`, built `CONCURRENTLY` (no transaction) | R2 (T-R2.10) |
| `00078_add_join_code_encryption.sql` | `class_join_codes.code_ciphertext`, `key_id`, `lookup_scheme` and their four checks; Down refuses a live scheme-2 code | R2 (T-R2.14a), D5, D-27 |
| `00079_revoke_temporary_from_public.sql` | revokes `TEMPORARY` on the database from `PUBLIC`; a no-op with a notice where the migration role does not own the database | fix for #194 |
| `00080_create_notifications.sql` | `notifications`, its four checks, `UNIQUE (user_id, dedupe_key)`, three indexes and the `updated_at` trigger | R4 (T-R4.10a), D-28 |
| `00081_create_notification_preferences.sql` | `notification_preferences`, `notification_preferences_event_check` and the `updated_at` trigger | R4 (T-R4.10a), D-28 |
| `00082_add_media_assets_library_fields.sql` | `media_assets.display_name`, `default_max_plays`, `replaced_by` and their four constraints | R4 (T-R4.17a), D-29 |
| `00083_add_media_assets_dimensions.sql` | `media_assets.width`, `height` and their three checks | R4 (T-R4.17a), D-29 |
| `00084_widen_media_assets_audio_bytes.sql` | `media_assets_bytes_by_kind` in place of `media_assets_bytes_check`; Down re-adds the old rule `NOT VALID` | R4 (T-R4.17a), D-29, DG-63 |
| `00085_add_users_profile_fields.sql` | `users.display_name`, `phone`, `avatar_key`, `locale`, `time_zone` and four checks | R4 (T-R4.7), D-30 |
| `00086_add_users_preferences.sql` | `users.preferences` and object/8192-byte checks | R4 (T-R4.7), D-30 |
| `00087_add_questions_level_skill.sql` | Nullable bank `level` and `skill`, with closed CHECK sets | R4 (T-R4.15), D-31 |
| `00088_add_test_version_questions_level_skill.sql` | Nullable frozen `level` and `skill`, with closed CHECK sets | R4 (T-R4.15), D-31 |
| `00089_add_word_import_draft_counts.sql` | `word_import_drafts.open_action_count` and `open_confirm_count`, each nullable and checked not negative, no default | R4 (T-R4.21), D-40 |
| `00090_allow_text_import_sources.sql` | `word_import_sources_format_check` admits `text`; Down restores the file-only check `NOT VALID` | R4 (T-R4.55), D-41 |
| `00091_add_word_import_sources_characters.sql` | `word_import_sources.characters`, `word_import_sources_characters_range` (1 to 100000) and `word_import_sources_characters_format` (non-null exactly when `format = 'text'`) | R4 (T-R4.55), D-41 |
| `00092_add_test_versions_change_note.sql` | `test_versions.change_note` and `test_versions_change_note_check` (1 to 200 characters), added with the column | R4 (T-R4.16a), D-32 |
| `00097_add_questions_media_alt.sql` | `questions.media_alt` and `questions_media_alt_check` (1 to 1000 characters), added with the column | R4 (T-R4.62a), D-33 |
| `00098_add_test_version_questions_media_alt.sql` | `test_version_questions.media_alt` and `test_version_questions_media_alt_check` (1 to 1000 characters), added with the column | R4 (T-R4.62a), D-33 |
| `00099_add_assignment_review_options.sql` | `assignments.review_release`, `assignments_review_release_check` and `review_show_class_average`, constant defaults | R4 (T-R4.11), D-34 |
| `00100_add_assignments_student_note.sql` | `assignments.student_note` and `assignments_student_note_check` (1 to 500 characters once trimmed of the `shared/answered` whitespace set), added with the column | R4 (T-R4.11), D-35 |
| `00101_create_assignment_student_overrides.sql` | `assignment_student_overrides`, its four checks, `assignment_student_overrides_student_idx` and the `updated_at` trigger | R4 (T-R4.12), D-36 |
| `00102_add_refresh_tokens_geo_label.sql` | `refresh_tokens.geo_label` and `refresh_tokens_geo_label_check` (1 to 80 characters), added with the column | R4 (T-R4.9), D-37 |
| `00103_compose_text_nfc.sql` | Data only: composes to NFC the plain columns a person types, deletes a decomposed accepted answer whose composed twin exists, and keeps `updated_at` by disabling the eight named `set_updated_at` triggers for the transaction. The Down does nothing | R4 (F-37), D-38 |
| `00104_add_classes_schedule_room.sql` | `classes.schedule_label` and `classes.room`, each with its check (1 to 120 and 1 to 60 characters), added with the column; 00103 is reserved for F-37, which merges first, and the second of the two renumbers if the order inverts | R4 (T-R4.18), D-39 |
| `00105_drop_users_legacy_role.sql` | Validates `users_role_id_not_null`; drops `users_sync_legacy_role`, `users_role_active_idx`, `users.role` and `app.user_role`; `lock_timeout` 5 s | R4 (T-R4.49, T-R3.1), D-25, §42 |
| `00106_validate_ownership.sql` | Validates the five owner constraints; drops the five fill triggers and their three functions; `lock_timeout` 5 s | R4 (T-R4.49, T-R3.2), D-22, D-23, §43 |
| `00111_add_media_assets_retention.sql` | `media_assets.unreferenced_at`, `media_assets.purged_at` and `media_assets_purged_check` (a purged file was unreferenced first), nullable with no default | R4 (F-68), D-43, §45 |
| `00112_index_media_assets_retired.sql` | `media_assets_retired_idx (owner_id) WHERE purged_at IS NULL AND (deleted_at IS NOT NULL OR replaced_by IS NOT NULL)`, `CONCURRENTLY` | R4 (F-68), D-43, §45 |
| `00113_guard_purged_media_bindings.sql` | `app.media_asset_binding_guard()` and its `BEFORE INSERT OR UPDATE OF media_asset_id` trigger on `questions`, `group_recordings`, `group_stimulus_assets`, `test_version_questions`, `test_version_group_recordings` and `test_version_group_assets` | R4 (F-68), D-43, §45 |
| `00114_add_questions_audio_allow_pause.sql` | `questions.audio_allow_pause` (nullable, no default) and `questions_allow_pause_only_with_audio`, added with the column | D4 (T-D4.13), D-42 |
| `00115_add_test_version_questions_audio_allow_pause.sql` | `test_version_questions.audio_allow_pause` (nullable, no default) and `tvq_allow_pause_only_with_audio`, added with the column | D4 (T-D4.13), D-42 |
| `00116_add_group_recordings_allow_pause.sql` | `group_recordings.allow_pause boolean NOT NULL DEFAULT true` | D4 (T-D4.13), D-42 |
| `00117_add_test_version_group_recordings_allow_pause.sql` | `test_version_group_recordings.allow_pause boolean NOT NULL DEFAULT true` | D4 (T-D4.13), D-42 |
| `00118_create_ai_settings.sql` | `ai_settings`, its five checks and the `updated_at` trigger, and the one row the plan sets | D4 (T-D4.31), D-44, §46 |
| `00119_create_ai_credit_periods.sql` | `ai_credit_periods` with `ai_credit_periods_within_pool` | D4 (T-D4.31), D-45, §46 |
| `00120_create_ai_teacher_credits.sql` | `ai_teacher_credits` with `ai_teacher_credits_within_allowance`, `ai_teacher_credits_user_idx` and the `updated_at` trigger | D4 (T-D4.31), D-46, §46 |
| `00121_create_ai_reservations.sql` | `ai_reservations`, its composite foreign key and its two indexes | D4 (T-D4.31), D-47, §46 |
| `00122_create_ai_usage_ledger.sql` | `ai_usage_ledger`, its indexes, and `REVOKE UPDATE, DELETE` from the app role | D4 (T-D4.31), D-48, §46 |
| `00123_create_ai_credit_requests.sql` | `ai_credit_requests`, `ai_credit_requests_one_pending_idx` and two more indexes | D4 (T-D4.31), D-49, §46 |
| `00124_add_ai_permissions.sql` | `ai.use` (Content, ordinal 6) and `ai.credits.manage` (System, ordinal 4) in `app.permissions`, both `in_matrix`; the System group's four hidden ordinals move up one, in two statements because `(group_key, ordinal)` is unique and not deferrable; `ai.use` granted to the built-in Teacher role | D4 (T-D4.32), DG-199, §46 |
| `00125_add_word_imports_read_with_ai.sql` | `word_imports.read_with_ai boolean NOT NULL DEFAULT false`; a constant default, so the v0.10.x binary keeps inserting | D4 (T-D4.33), §47 |
| `00126_add_questions_copied_from.sql` | `questions.copied_from_id` and `questions_copied_from_id_fkey` (`REFERENCES questions (id) ON DELETE SET NULL`), added with the column, nullable with no default | E1 hotfix v0.11.7, D-50, §48 |
| `00127_index_questions_copied_from.sql` | `questions_copied_from_idx (copied_from_id) WHERE copied_from_id IS NOT NULL`, `CONCURRENTLY` | E1 hotfix v0.11.7, D-50, §48 |
| `00128_create_email_outbox.sql` | `email_outbox`, its checks, `email_outbox_dedupe_key`, the pending index `email_outbox_pending_idx`, `email_outbox_sent_idx` and the `to_user_id` index | R5 (T-R5.2), D-51, §49 |
| `00129_grant_role_writes.sql` | `INSERT` and `UPDATE (name, description, icon, color, revision, updated_at)` on `roles`, `INSERT` and `DELETE` on `role_permissions`, for the app role; `DELETE` on `roles` and any write on `permissions` stay revoked | R5 (T-R5.8), DG-52, §29 |
| `00130_create_organization_settings.sql` | `organization_settings`, the singleton, its checks and `updated_at` trigger, the one row the migration writes, and `REVOKE INSERT, DELETE` from the app role | R5 (T-R5.5), D-53, §51 |
| `00131_add_users_activity.sql` | `users.last_active_at`, `last_sign_in_at`, `last_sign_in_method` (`users_last_sign_in_method_check`), `password_changed_at`, all nullable; backfills the first two | R5 (T-R5.4), D-52, §50 |
| `00132_create_user_activity_days.sql` | `user_activity_days (user_id REFERENCES users ON DELETE CASCADE, day date, PRIMARY KEY (user_id, day))` and `user_activity_days_day_idx (day)` | R5 (T-R5.4), D-52, §50 |
| `00134_create_data_exports.sql` | `data_exports`, its checks (`data_exports_ready_has_file`, `data_exports_failed_has_code`), the active index `data_exports_active_idx` and `data_exports_requested_by_idx` | R5 (T-R5.15), D-54, §52 |
| `00135_create_set_password_tokens.sql` | `set_password_tokens`, its checks (`token_hash` length, `purpose`, the 24-hour bound), `set_password_tokens_token_hash_key` and `set_password_tokens_user_idx` | R5 (T-R5.3), D-55, §53 |
| `00136_create_user_import_jobs.sql` | `user_import_jobs`, its checks (counts, `attempts`, `user_import_jobs_done_settled`, `user_import_jobs_failed_has_code`, `user_import_jobs_completed_has_end`, `user_import_jobs_started_once_claimed`, `user_import_jobs_sheet_only_by_sheet`), `user_import_jobs_request_key` and the open-jobs index `user_import_jobs_open_idx` | R5 (T-R5.14), D-56, §54 |
| `00137_create_user_import_rows.sql` | `user_import_rows`, its checks (`user_import_rows_outcome_check`, `user_import_rows_secret_only_when_created`) and the `user_id` and `class_id` partial indexes | R5 (T-R5.14), D-57, §54 |
| `00138_index_users_search.sql` | `users_search_trgm_idx`, built `CONCURRENTLY` (no transaction) | R5 (T-R5.9a), D-58 |

Notes on migration mechanics (§13.7):

- **D4's four files were built as `00107`–`00110` and are `00114`–`00117`.** Production holds
  `00111`–`00113` (F-68) before D4 ships, and goose refuses a migration numbered below the
  database's version, so the develop sync of 2026-10-12 renumbered them after `00113`, in order.
  The next D4 migration (T-D4.31) takes `00118`.

- Every file has a `-- +goose Down` that is actually correct. CI runs `up` then
  `down` then `up` against a clean `postgres:18` (T-0.17), so an unreversible
  migration is a red build on the day it is written.
- **No `CREATE INDEX CONCURRENTLY` in `00001`–`00022`.** They create empty
  tables, where a plain `CREATE INDEX` is instant and transactional.
  `CONCURRENTLY` becomes mandatory once an index is added to a populated table
  (`00029`, `00036`, `00038`, `00057`, `00071`–`00077`), and each such file
  gets `-- +goose NO TRANSACTION`, because
  [CREATE INDEX CONCURRENTLY cannot run inside a transaction block](https://www.postgresql.org/docs/18/sql-createindex.html)
  and leaves an `INVALID` index if it fails.
- **`NOT NULL … NOT VALID` only where a populated table gains a column.**
  Every column of a new table says `NOT NULL` inline at `CREATE TABLE`, where
  it is free. R2 is the first to need the PG18 construct (§13.6): `00056`,
  `00060`–`00063` and `00065` backfill a new column, add `NOT NULL … NOT
  VALID`, and leave validation to v0.9.1 (§30, §31). T-0.16 proved it before it
  was needed.
- **`00009` runs in Phase 1, not last.** It was originally scheduled for Phase 3
  on the reasoning that `GRANT … ON ALL TABLES` must follow the tables. That is
  backwards: it left `quizzivy_app` unable to read anything from the first table
  until the end of Phase 3, and the first integration test to connect as the app
  role failed with `permission denied for schema app`.
  `ALTER DEFAULT PRIVILEGES` resolves it — applying to objects created *after*
  it, so one early migration covers every table Phases 2–5 add. Custom types
  need their own `USAGE`; schema `USAGE` is not enough to reference
  `app.user_role` in a query.
- Seed data lives in `seed/`, never in a migration (§13.7). The exception is
  reference data the app cannot run without (D-21).

## 14. Query discipline (§13.8)

- No `SELECT *` anywhere. Enforced by review and by the separate student/admin
  Go types described in §11.
- `ORDER BY id DESC` on every list endpoint, paged by OFFSET since O-20 (the
  keyset rule §13.8 states was overridden for the admin lists on 2026-09-03;
  the indexes below serve the order either way). Because PKs are `uuidv7()`,
  `id DESC` is a valid recency order and most lists need no `created_at` index at all. The
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

## 15. Approved retention maintenance (2026-09-22)

`00029_index_integrity_retention.sql` adds `(received_at, id)` concurrently to
support bounded owner-role deletion strictly before the UTC thirteen-month
cutoff, with assignment closure also older than that cutoff. Down drops the index concurrently. No application privileges change.
`audit_log` remains append-only and retained. Manual anonymization keeps user
IDs and historical relationships while removing structured identity and login
credentials. See `../setup/operations.md` for execution and privacy limits.


## 14. Approved admin lifecycle changes (spec v0.6)

`00030_test_version_sequence.sql` adds `tests.last_published_version`. Existing
rows are backfilled from both the default and the greatest historical version.
A before-insert/update trigger keeps the counter at least the default and never
allows it to decrease; `tests_version_sequence_check` pins that relationship.
Publication increments this counter; choosing an older default or deleting an
unused version never reuses its number. Down removes the trigger, function and
column without deleting snapshots.

`00031_allow_no_focus_loss.sql` expands the assignment CHECK to accept `-1`
(no departures allowed), preserving `0` as unlimited and positive counts.
Down maps `-1` to one allowed departure because the old schema cannot express
none, then reinstates the original constraint. This rollback changes that policy
and must be coordinated with the old application.

Permanent deletion locks the parent and relies on real foreign keys to serialize
against new references. Archived tests may remove only unused versions; archived
classes may remove owned membership/join-code rows but not referenced assignments.
Draft/closed assignments require no attempts. Disabled student accounts require
no assigned work, attempts or audit actor references. Audit entries are appended
in the same transaction; existing audit and attempt events are never updated or
deleted by the application. Restoring a version creates independent bank rows,
including frozen grading keys, and takes the draft-use question locks before
inserting the replacement outline.


## 16. Inline option content (W-05a, spec v0.10)

`00032_add_option_content.sql` adds nullable `content jsonb` to
`app.question_options` and `app.test_version_options`. Each stores one bounded
`OptionContent` document alongside the existing literal/plain-projection `text`.
The CHECK accepts SQL NULL or an object with explicit `semantic_v1` format;
the shared domain validator checks closed shape, marked text/break vocabulary,
Unicode/budgets and exact text projection before a write and again at publish.
There are no asset or gap nodes, so this slice adds no media bindings or FK locks.
Options, ordinals and correctness stay normalized; there is no whole-question or
whole-version JSON blob. No index is needed: content is loaded with its existing
parent/ordinal queries and existing search behavior is unchanged.

Question UPDATE takes the parent row lock before replacing children. Under that
lock, a legacy write omitting content retains it only for a matching current
option ID and unchanged text; a mismatch aborts the entire write. Explicit JSON
null removes formatting and is stored as SQL NULL. Bank duplication copies the
validated document; publication freezes it, restoration copies the snapshot into
fresh bank option identities. Student/teacher reads use the snapshot document,
never a live bank join. Existing rows remain SQL NULL without backfill.

Down drops the two columns and preserves relational text/keys but loses rich
formatting. Do not use it after rollout; retain this schema and compatible readers
when disabling the pilot authoring flag. App-role privileges and append-only
logs are unchanged. Up/down/up is tested only on a disposable database.


## 17. Question prose (W-05b, spec v0.11)

`00033_add_question_content.sql` adds nullable `prompt_content` and
`explanation_content` JSONB to `app.questions` and `app.test_version_questions`.
CHECK constraints accept SQL NULL or an object with explicit semantic_v1 format;
rich prompts cannot be attached to fill_blank questions before gap binding exists.
The domain checks raw JSON, the reference-free prose profile, aggregate budgets
and exact text projection at write and publication. Relational media, options,
blanks and grading keys remain on their current tables. No references, new
privileges or indexes are introduced. Existing prompt indexes search the exact
plain projection for new prose and the historical Markdown for legacy rows.

Question updates lock the parent before comparing omitted content's companion
text. If a legacy write changes a rich field's text without supplying a document
or explicit null, the whole update rolls back. This prevents formatting loss; it
is not a replacement for the later authoring revision protocol. Student paper
and preview queries select only prompt content. Result extras select explanation
content inside the same review-policy CASE as explanation text.

Down removes the four columns and loses formatting; use it only for disposable
or pre-rollout data. After rich writes, retain this migration and capable readers
when disabling new authoring. No historical snapshots are backfilled.


## 18. Stable gap bindings (W-06b, spec v0.13)

`00034_add_question_gap_bindings.sql` adds nullable `gap_id text` on
`app.question_blanks` and `app.test_version_blanks`. Each CHECK uses the content
identity alphabet (ASCII alphanumeric then alphanumeric/underscore/hyphen, 1–64).
UNIQUE `(question_id, gap_id)` and `(test_version_question_id, gap_id)` protect
non-null bindings while permitting existing null legacy rows. The leading parent
key also supports per-question graph reads; no global gap lookup/index is needed.
The parent prompt CHECK retains object/semantic format validation and permits
fill-blank; exact AST/metadata bijections are checked by the shared domain validator.

No new grants, roles, background job or historical backfill. The parent question
lock still serializes updates; snapshot writes and graph copies are transactional.
Copies allocate fresh gap IDs and remap AST and blank metadata together. Down
restores the old parent restriction before dropping the columns, so rich fill-blank
rows block rollback atomically. After these writes, keep readers and migration 34.


## 19. Independent group ownership and graph storage (W-07b)

This additive storage foundation remains unavailable through application writes
until group lifecycle commands and W-08/09 readers/snapshots are integrated.
Existing sections and bank questions retain their interpretation; no backfill
changes old versions or attempts.

`00035_add_question_group_ownership.sql` introduces `question_groups`. A null
`owner_section_id` identifies an independent bank group; otherwise the group
belongs exclusively to that draft section. The section FK uses RESTRICT so old
section deletion cannot strand its owned questions. Bank groups can be archived;
test-owned groups follow their enclosing draft lifecycle. Revision and updated
at are held on the aggregate. New nullable question columns carry group ownership,
member ordinal and option order as one complete tuple; owned questions cannot be
soft-deleted separately. The group FK is RESTRICT: lifecycle commands must remove
the complete graph explicitly, never orphan children into the standalone bank.

`00036_index_question_group_members.sql` builds two unique indexes CONCURRENTLY
outside a transaction on the existing questions table. The identity pair supports
same-group foreign keys; the ordinal index supports ordered group reads and later
becomes a deferrable uniqueness constraint. Legacy null ownership is not unique.
`00037_create_question_group_graph.sql` attaches these indexes as constraints and
adds the owned graph. Failed concurrent index creation must be inspected for an
invalid index and repaired before retry; migration re-runs must not silently
accept an invalid existing index with IF NOT EXISTS. See PostgreSQL 18
[CREATE INDEX](https://www.postgresql.org/docs/18/sql-createindex.html) and
[ALTER TABLE](https://www.postgresql.org/docs/18/sql-altertable.html).

| Table | Ownership, references and indexes |
| --- | --- |
| `question_groups` | Section RESTRICT; creator RESTRICT; section lookup and active bank recency indexes; `(id, owner_section_id)` prevents mounting bank/foreign groups into a section |
| `test_section_units` | Ordered standalone-question/group union, exclusive via CHECK; section CASCADE; referenced question/group RESTRICT; deferrable section ordinal; unique group mounting and no repeated standalone question in a section |
| `group_stimuli` | Group CASCADE; bounded ordered materials; JSONB content with explicit known format; `(id, group_id)` supports same-group links |
| `group_gap_bindings` | Material CASCADE; member composite FK prevents cross-group targets; stable `(question_id, gap_id)` blank FK; choice/blank shape CHECK; scoped material-gap primary key; unique response targets; reverse question/blank index |
| `group_recordings` | Group CASCADE; composite media kind FK RESTRICT; one audio asset binding per group; immutable binding identity and playback/transcript policy; reverse media index |
| `group_stimulus_assets` | Material CASCADE; media kind FK RESTRICT; audio iff explicit recording; composite recording FK prevents using another group's allowance or another file; reverse media/recording indexes |

Gap-to-member/blank and material-to-recording foreign keys are deferred NO ACTION:
an atomic replacement may replace normalized answer rows while preserving stable
gap identities, but commit cannot leave dangling bindings. AST/mirror equality,
question interaction compatibility, complete member/material/recording coverage,
aggregate budgets and media authorization still require the domain validator and
repository checks; JSONB format checks alone do not validate semantic content.

A group update must lock the containing test when present, then the group and
owned questions, before its media reference locks; lists of question/media IDs
are locked in stable order. Creation/copy/deletion and bank insertion must use the
same locks on both sides. Existing bank and outline paths must explicitly reject
or hide owned children until they can carry complete context. No partial group
insert is allowed. Legacy flat section membership remains readable; new ordered
units are enabled only with group-aware draft/snapshot readers.

Down refuses to remove group storage after any group/unit write. With no such
writes, the graph Down restores the pre-attachment indexes before returning to
migration 36, so down-one/up works as well as a full reset. That rollback rebuild
holds the ordinary DDL lock inside the transaction; it is a pre-enablement path,
not a supported downgrade after group content exists. After enabling writes, keep
a reader-capable binary and schema floor; disable new writes instead of dropping
context. The guarded rollback and fresh up/down/up are tested in isolated Docker
databases. No privilege changes touch append-only audit or attempt events.

The W-07c internal repository materializes graphs atomically and resolves owned
questions through a transaction-bound adapter. Legacy standalone APIs scope reads
and writes to null ownership. First group mounting locks the test revision, locks
legacy standalone members by ID, preserves their section ordinals and appends the
group. Creation then locks all media IDs in stable order before writing members
and bindings. Readback holds the group shared lock and verifies AST/relational
asset equality. Group-aware update/delete must take its exclusive counterpart.

Media soft deletion includes material, recording and owned-member references,
including archived bank groups, under the existing asset row lock. The race tests
observe actual PostgreSQL blocking and exercise both winner orders. These internal
operations remain unexposed until complete draft/snapshot readers and lifecycle
commands are present; new-unit readers must not fall back to legacy flat membership.

W-07d extends the internal repository with revision-checked updates and lifecycle
operations. Update locks the enclosing test first, the group FOR UPDATE, then
owned questions by ID and incoming media by ID. It clears old material bindings,
replaces members under deferred ordinal uniqueness, reconstructs validated
bindings and increments the group revision in the same audited transaction.
Question type and fixed/shuffle policy change in one statement so neither
intermediate state violates the compatibility CHECK. Readback revalidates the
complete graph before commit; any failure rolls everything back.

Bank archival/restoration advances the same revision. Permanent deletion requires
an archived bank group and removes materials, recordings and owned questions
before the root; audit rows remain. Section removal instead requires both current
revisions, removes its unit and compacts remaining unit ordinals under their
deferrable constraint. Copies resolve one coherent source revision, remap all
editable identities and create a new graph; no editable source FK is retained.


## 20. Frozen group graph (W-08)

Migrations 38–39 add snapshot-owned group context without altering historical
question content or assigning old versions a group interpretation. The existing
frozen question table retains grading keys, response identities and section-wide
ordinals. A separate membership table binds those frozen questions to a group,
with member order and option-order policy. This avoids nullable group ownership
columns on the populated snapshot table and keeps the legacy read path intact.

Migration 38 builds `(id,test_version_section_id)` uniquely and concurrently on
`test_version_questions`. Migration 39 attaches that index and creates the graph.
Composite foreign keys require every group member and ordered unit to belong to
the same frozen section; material gap targets must be members of the same frozen
group. Stable blank-gap targets reference the existing frozen blank identity pair.
Question-type compatibility, complete unit/member coverage and AST binding
bijections remain domain/repository invariants checked before freezing.

| Table | Contract and reverse-reference access |
| --- | --- |
| `test_version_groups` | Snapshot section owns the root; title and semantic instructions; section index; no live source-group FK |
| `test_version_group_members` | Same-section group/question links; unique question membership and ordered members; option-order policy |
| `test_version_units` | Ordered standalone/group union; same-section links; unique unit position, question and group |
| `test_version_group_stimuli` | Immutable ordered material content; group identity pair for scoped child links |
| `test_version_group_gap_bindings` | Scoped stable gaps, same-group question targets and frozen blank-gap targets; unique response targets and reverse question index |
| `test_version_group_recordings` | Independent recording identity, immutable policy/transcript and protected audio kind; reverse asset index |
| `test_version_group_assets` | AST reference mirror, same-group recording and asset linkage; reverse media and recording indexes |

Deleting an unused version cascades through its owned graph. Frozen blank targets
use deferred NO ACTION to permit that whole-version cleanup while rejecting
partial dangling targets. Physical media deletion is restricted by frozen graph
references; soft deletion and the media library must include them under the same
asset locks as publication. New snapshot graph tables revoke UPDATE from the app
role; correcting published content creates another version. Audit and attempt
event grants remain untouched.

Down is for disposable/pre-enablement data only and refuses any frozen group or
ordered-unit rows. The graph Down restores the index owned by migration 38 so
an intermediate down/up has identical schema. Once group snapshots exist, keep
these migrations and compatible readers even when authoring is disabled. No old
version, attempt or shuffle seed is backfilled.

Publication holds the parent test lock, then shared locks on all context roots
and referenced question rows in stable ID order before loading prompts and child
keys. This prevents a concurrent bank edit from mixing parent text from one
revision with options/blanks from another. Group writes already require that
parent lock. After validation, every member/material asset is locked once in
stable ID order across the full test; section-local lock ordering is insufficient
when tests share assets in different authored orders.

The flattened frozen question ordinals remain the response/grading order, while
`test_version_units` preserves grouping and `test_version_group_members` preserves
member policy/order. Material and recording rows receive fresh UUIDv7 identities;
gap targets are remapped to the frozen questions, with their stable prompt gap IDs
retained. Version listening counts count each question once if it has own audio
or any shared recording in its group. Frozen group references participate in
media soft deletion and batched library usage lookups. W-09b extends student
reachability only through protected material bindings on a version they have an
attempt on; preview/attempt safe readers never select keys or transcripts.

W-08b reads the frozen graph under its parent version lock and validates content,
member ordering, grading inputs and AST/relational asset mirrors. Restoration
locks the target test/version, clears its old owned draft graph, locks all snapshot
assets in stable order and creates independent copies in one transaction. New
gap identities are remapped through material and question documents together.
Standalone snapshot questions remain ordinary independent bank copies, matching
the previous restore behavior. Repeated restores delete replaced owned children;
failed restores roll back that cleanup as well as every new row.

Duplication holds the source test lock and takes referenced question locks in
stable order before copying; it invokes the existing draft-use guard before every
standalone reference insertion. These locks are exclusive from the start to avoid
shared-to-exclusive lock upgrades between concurrent copies. All copied group
assets are locked together before insertion. Source group graphs receive new
editable IDs; standalone bank references keep their original duplication meaning.
Whole-test deletion clears owned context only after version reference checks,
with audit history preserved. Grouped draft/preview and learner read paths must
still be integrated before authoring is enabled.

## 21. Shared recording accounting (W-09c)

`00040_create_group_audio_plays.sql` keeps the counter identity separate from the
immutable asset. `attempt_group_audio_plays` has primary key `(attempt_id,
recording_id)`, a positive integer count and last-play timestamp. The attempt owns
the count through a cascading FK; the frozen recording is protected by RESTRICT.
A recording-first index supports reverse references and teacher aggregation.

`attempt_group_audio_receipts` has primary key `(attempt_id,play_id)`, the recording,
reporting session and received timestamp. Its composite FK names the corresponding
counter and cascades with that owned counter. An `(attempt_id,recording_id)` index
supports that reverse reference. The application can insert/read receipts but
cannot update/delete them; existing audit/event privileges are untouched.

The shared-play transaction first takes the same attempt lock as autosave, resume
and submission, checking owner, writable session, status and deadline. A relational
join verifies the requested recording is bound to a material on that attempt's
version. It then looks up the gesture receipt. A matching retry reads the current
counter without another event; a different recording is refused. A new gesture
increments via UPSERT, inserts its receipt and appends one `audio_play` event with
group/recording/gesture identity and count/limit metadata. Server events retain a
NULL client sequence, so they never collide with browser event sequences.

READ COMMITTED is sufficient: the attempt row serializes shared-play writers with
each other and with session/closure changes. The whole transaction rolls back if
any of its three writes fails. Counters may exceed the frozen policy; monitoring
reports that excess and never changes grading. No historical ledger is backfilled.
Down restores the prior schema only while the new ledger is empty; populated
listening evidence must survive disabling the feature.

## 22. Frozen delivery algorithm (W-09d)

`00041_version_test_delivery.sql` adds `test_versions.delivery_version`: non-null
text defaulting to `section_v1`, checked against `section_v1` and `group_v1`.
An enum would make future algorithm expansion/rollback harder; no index is needed
because readers resolve by the existing version primary key. Legacy snapshots
keep their marker, IDs and seeds. Prerelease snapshots with frozen group rows are
classified by an indexed section/group existence join. This is metadata only;
no questions, content, options, attempts or answers are rewritten.

The publisher explicitly writes `group_v1` for new versions. Its standalone
ordering is identical to the historical section algorithm. Attempt and result
readers use `attempts.test_version_id`, never the test's selected default. The
marker remains immutable through the same API boundary as all snapshot fields.
Down refuses while any group-aware snapshot exists; otherwise it drops only the
new column. Tests exercise both reversible empty schema and refusal with data.


## 23. Private import sources (W-10a)

Migration: `00042_create_word_import_sources.sql`. The only PG18-specific construct
introduced is built-in [`uuidv7()`](https://www.postgresql.org/docs/18/functions-uuid.html).
The Neon schema/isolation guidance was applied: text checks for evolving states,
`bigint` revisions/byte counts, `timestamptz`, indexed foreign keys and explicit
`ON DELETE RESTRICT`. No partitioning, learner-media link or audit privilege change.

- `word_imports` holds actor-scoped create idempotency, title, lifecycle, optimistic
  revision and nullable current source revision (wire zero before the first upload).
  Its composite head FK points to a set owned by that same import. Recency/status
  indexes serve history; immutable-unaccent trigram indexes serve Vietnamese title
  and source-filename search. The creator/request unique also indexes the user FK.
- `word_import_sources` is the durable upload journal and immutable original metadata.
  Its import/upload unique pins replay identity; SHA-256 is not globally unique.
  Every storage key has a row before Put. Readiness and completion revision agree via
  CHECK, and the completion FK cannot name another import's set. Actor/import/revision
  and pending-age indexes support quota, history and future cleanup inventory.
- `word_import_source_sets` and `word_import_source_set_items` preserve every completed
  source revision. The role primary key allows at most one exam and one answer key;
  a composite source FK pins import, role and `ready=true`, rejecting pending bytes
  and cross-import sources even if application validation is bypassed. The app role
  cannot UPDATE/DELETE these histories, rewrite original metadata, or delete originals;
  only source readiness/completion columns receive UPDATE. Reference-safe retention
  will need its own explicitly reviewed maintenance operation.

Create/reserve acquire advisory transaction lock `(73819,10)` before any parent row
lock. This serializes global and actor quota reservations across API processes.
Completion locks the import row and checks status/revision before atomically attaching
one new source set and auditing the actor. Exact completed replay succeeds without
advancing the head. A stale upload retains its tracked object reservation and quota.
No storage call or package inspection runs inside a transaction. History/get reads
use short repeatable-read transactions so a head and its source list agree.

Local Docker verification uses the migration owner for fixture setup and the
`quizzivy_app` role for intake, plus a separate private MinIO bucket. Up/down/up is
verified on a disposable database. Automatic source retention remains undecided;
this migration does not infer it from the integrity-event retention policy.

### R4 pasted-text source extension (T-R4.55)

`00090_allow_text_import_sources.sql` extends the existing format CHECK to
`docx`, `doc`, `pdf` and `text`. Its Down restores the file-only CHECK `NOT VALID`,
so existing text rows remain retained while new text writes are refused.
`00091_add_word_import_sources_characters.sql` adds nullable integer `characters`
with named range 1–100000 and format constraints: it is non-null exactly for text
sources and null for files. Existing file inserts using the old column list remain
valid. No owner, grant, index, source-set or append-only audit policy changes.

Intake writes the NFC Unicode code-point count; bytes, hash and private-original
metadata keep their existing meaning. Migration 91 Down drops the character
column. Empty-purpose 90/91 up/down/up is reversible. After retained text loses
its counts through 91 Down, 91 Up must refuse atomically: bytes and hash cannot
reconstruct truthful character counts. There is no invented count, backfill or
retained-row repair. A separate disposable purpose verifies this refusal and is
ordinarily disposed only under its exact lifecycle authorization.


## 24. Durable import runs (W-11a)

Migration: `00043_create_word_import_runs.sql`. PG18
[`SELECT … FOR UPDATE … SKIP LOCKED`](https://www.postgresql.org/docs/18/sql-select.html)
is used only for queue allocation. [Transaction advisory locks](https://www.postgresql.org/docs/18/explicit-locking.html)
serialize capacity accounting; durable state is in tables, never the lock itself.

`word_import_runs` references an immutable source set belonging to its import.
Request identity pins the source revision, expected import revision, pipeline and
attempt budget. A partial unique index permits one active run per import; queue
and expiry indexes support allocation/recovery, and actor/source indexes cover FKs.
CHECKs couple lease/worker presence, terminal completion, ready stage and private
result presence to lifecycle state. Results are bounded JSON objects, not arbitrary
SQL or learner payloads. Application column grants protect request/source identity;
a terminal-state trigger prevents later result/history mutation.

`word_import_run_events` references the run and its same import through a composite
FK. It contains ordered operational identifiers/codes only. The app role can insert
and read but cannot update or delete events. Actor and run indexes cover lookups and
references. Existing `audit_log`/`attempt_events` grants are unchanged.

Lock order is advisory `(73819,11)` when allocating/renewing capacity, then parent
import, then run. Other transitions start at the parent and never acquire the queue
advisory lock later. Eligibility is re-read after locking the parent; snapshots from
joined candidate selection do not authorize a stale lease. Processing and storage
calls run after commit. Cancellation increments the fence and competes with success
on the same parent, preventing a cancelled worker from completing the import.

There are at most 50 requests per import and ten attempts per request. Stage writes
are monotonic within an attempt and repeated stages are no-ops, bounding run-event
history without truncating it. Down removes events before runs and their guard
function. Docker up/down/up checks run on a disposable database.


## 25. Private stage artifacts (W-13b)

Migration: `00044_create_word_import_artifacts.sql`. `word_import_artifact_sets`
pins one bounded output manifest to its producing run/claim and immutable source
set item. Composite FKs enforce same-import run/source revision, source role and
source identity. Component identity includes processor configuration and upstream
artifact lineage; reuse additionally requires the same pipeline version.

`word_import_artifacts` journals each private object before storage. Generated
keys include import/set/file identity, and every takeover receives new writable
keys. Unique ordinal/name constraints prevent ambiguous manifests. The manifest
has at most 512 files, 64 MiB per file and 256 MiB total; compact application
metadata is limited to 60 KiB (128 KiB JSONB allowance for representation spacing).
These are internal development bounds, not the approved production envelope.

Reservations take quota advisory lock `(73819,10)`, then parent import and run.
Pending and ready set bytes count towards separate actor/global artifact limits;
sets per import are also bounded. Storage and checksum IO happen after commit.
Every acknowledgement/completion requires the live source/worker/fencing claim.
Completed set reads need no mutable snapshot because completed files and metadata
cannot change. Only complete sets are reusable, including across explicit retries;
old incomplete sets remain tracked and charged. No retention decision is inferred.

Column grants prevent metadata rewrites and deletion. Triggers reject completed
row mutation, file additions to completed sets, out-of-plan ordinals and incomplete
set completion. Parent/run/source and lookup indexes support reverse references,
reuse and pending inventory. Existing append-only audit/event permissions remain
unchanged. Down drops owned files before sets and removes the new FK target keys.

Object persistence uses [conditional S3 creation](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html)
with a SHA-256 checksum. Exact lost-response replay verifies the stored checksum,
size and MIME type using [HeadObject](https://docs.aws.amazon.com/AmazonS3/latest/API/API_HeadObject.html).
Incompatible storage implementations fail closed; production provider compatibility
is a W-21 gate. Trusted processor reads stage on disk and verify size/digest before
parsing. A stage manifest grants no learner media authorization.


## 26. Review drafts and commits (W-18, W-20)

Migrations: `00045_create_word_import_drafts.sql`, `00046_create_word_import_commits.sql`,
`00047_add_word_import_run_profile.sql`, `00048_allow_legacy_word_sources.sql`,
`00089_add_word_import_draft_counts.sql`.

`word_import_drafts` holds exactly one review copy per import (the primary key is the
import). `revision` is the optimistic-concurrency token for teacher saves: an update
names the revision it read and matches zero rows otherwise. `run_id` is the run whose
extraction the draft's source references point into, so the source view and the draft
always describe the same evidence. A run that completes while `edited_by` is null
replaces the body and bumps the revision; once a teacher has saved, a newer machine
draft is kept in `candidate`/`candidate_run_id` and never overwrites the edits. The
body is bounded to 8 MiB as JSONB. The application role may update drafts but not
delete them; draft saves are not audited individually because autosave would flood
the log.

`00089` adds nullable `open_action_count integer` and
`open_confirm_count integer`, each checked nonnegative, without a default or
backfill. They store `domain.Assess(current body).Summary` (`Blocking` and
`NeedsDecision`) in the same write as the body. A pending candidate preserves
the current counters; adoption computes them from the locked candidate. NULL
means a historical current draft has not been assessed by this binary, not zero.
Valid existing draft shapes can exceed32767 findings, so SMALLINT would impose
an unrelated new authoring limit; the existing8MiB body limit remains unchanged.
History reads these counters with its facets and rows in one repeatable-read
snapshot. Down drops only the two reporting columns.

`word_import_commits` is append-only (no UPDATE or DELETE for the application role)
and has one row per import. It records the request identity, the committed draft
revision and its SHA-256 digest. The commit runs in one transaction with the test,
section, bank question and group writes it describes, after locking the import row
`FOR UPDATE`; a second concurrent commit waits on that lock, finds the import already
committed and rolls back its own writes, and a replay with the same request identity
returns the recorded test. `test_id` is `ON DELETE SET NULL` so deleting the created
test later is not blocked by import history.

`word_import_runs.profile` stores the recognition profile (for example the paper to
read from a multi-paper answer key) so a replayed schedule request must match it.
Adding the column with a constant default is metadata-only on PG18. The source
`format` check now admits `doc`; its Down restores the `docx`-only check `NOT VALID`
so rows written meanwhile do not block the rollback.

`00049` replaces the bank group index on `id DESC` with `(updated_at DESC, id DESC)`
under the same predicate, which is the order the listing actually uses.

Retention (D-08), in `00050` and `00051`, works in three steps. The API runs
the sweep daily with the application role, so nothing waits for a manual job:

- **Close idle imports.** An import left in `awaiting_sources`, `failed` or
  `needs_review` with `updated_at` older than 60 days becomes `cancelled` with
  `closed_idle`. Reservations, draft saves, adoptions and commits all touch
  `updated_at` under the row lock, so the idle test needs no join.
- **Remove the files.** A terminal import's objects are removed 30 days after
  `word_import_commits.committed_at`, 7 days after cancellation (`updated_at`),
  or at once when `closed_idle` is set. That covers every source and artifact
  `storage_key` the import owns.
- **Delete the draft, which holds the exam content, and set `files_removed_at`.**
  This happens only after the objects are gone.

What stays:
- The source and artifact rows stay. They hold no content, and the history shows
  their filenames.
- `runs`, `run_events`, `commits` and `audit_log` stay untouched and append-only.

Enforcement:
- `word_imports_files_removed_terminal` allows `files_removed_at` only on
  committed or cancelled imports, which can never be processed again, so stage
  reuse never meets a missing object.
- `word_imports_closed_idle_cancelled` allows `closed_idle` only on cancelled
  imports. The column has a constant default, so adding it is metadata-only on
  PG18.
- The partial index `word_imports_retention (status, updated_at, id) WHERE
  files_removed_at IS NULL` serves both queries. Both name the predicate and
  filter by status, and the removal sweep's keyset order is `(updated_at, id)`.
- `00051` grants DELETE on `word_import_drafts` only. No other table gains a
  delete grant.

`00052` widens the source `format` check to `docx`, `doc` and `pdf`. Like
`00048`, its Down restores the narrower check `NOT VALID`, so PDF rows written
meanwhile do not block the rollback.

## 27. Maintenance windows (T-R1.12)

`00053_create_maintenance_windows.sql` adds `app.maintenance_windows (id uuid PK
DEFAULT uuidv7(), starts_at, ends_at, created_at, created_by text DEFAULT
current_user, cancelled_at)`.

- `ends_at > starts_at` and `ends_at - starts_at <= interval '12 hours'` are
  CHECKs.
- `EXCLUDE USING gist (tstzrange(starts_at, ends_at) WITH &&) WHERE
  (cancelled_at IS NULL)` keeps live windows from overlapping. It excludes on the
  range alone, so `btree_gist` is not needed; `[)` bounds let one window start as
  another ends. The same gist index serves the start guard's overlap query.
- `created_by` is the database role that scheduled the window. No app user does.
- The file revokes INSERT, UPDATE and DELETE from `quizzivy_app`, narrowing
  `00009`'s default privileges: the API reads windows (T-R1.13), and only the
  operator's role writes them (D-20). `privileges_test.go` pins it.

`window-schedule` writes in one statement of data-modifying CTEs, under the
advisory lock below: the window, the moved `attempts.deadline_at` and
`assignments.closes_at` (PG18 `RETURNING old.…, new.…`, so each audit row
carries only the column that moved), and their audit rows with a NULL actor.

## 28. Advisory-lock registry

Every transaction advisory lock this codebase takes is in one of two
namespaces. `pg_advisory_xact_lock(73819, <key>)` holds the fixed keys of the
table below: add a key here before it is used. `73820` holds one lock per
media owner, described under the table.

| Key | Holder | Serialises |
|---|---|---|
| 10 | Word imports (W-10a, W-13b) | Upload quota reservation |
| 11 | Word imports (W-11a) | Run capacity allocation and renewal |
| 40 | Maintenance windows (T-R1.12, T-R1.13, T-R4.12b) | `window-schedule`, `window-cancel` and `window-end` take it exclusively; the attempt start guard takes it shared, so no attempt starts between a window's scheduling and its extensions. `extendAssignment`, `updateAssignment`, `reopenAssignment`, `setStudentOverrides` and `deleteStudentOverride` take it shared too, before they lock the assignment row (`schedule.LockWindows`), so none of them runs between a window's scheduling and its extensions, and none can wait on a row that `window-schedule` holds while it waits on theirs |
| 41 | Legacy join-code rotation (T-R4.22) | Each class's rotation across API machines |
| 42 | Reserved: R7's notification scheduler | Confirmed by R7 |
| 43 | Email dispatch (T-R5.2) | `pg_try_advisory_xact_lock(73819, 43)` in the API's and the worker's dispatcher: one batch is claimed and sent at a time across both processes, so the daily budget and the provider's request rate hold. A dispatcher that does not get it waits and tries again |
| 44 | Role creation (T-R5.8) | `pg_advisory_xact_lock(73819, 44)` in `createRole`'s insert, then a count of `app.roles`: two creations cannot both take the last of the 200 roles the contract's lists hold (`ROLE_LIMIT_REACHED`). Nothing else takes it, and it is taken after the copy source's `FOR SHARE`, never before a role row lock. |

**Namespace 73820: one media library's quota (T-R4.17a).**
`pg_advisory_xact_lock(73820, hashtext(<owner id>::uuid::text))` serialises
the writers of one owner's media library. `media/repositories.RequireQuota`
takes it first, then sums the owner's library bytes (§6: not deleted, not
replaced), and the insert follows in the same transaction, so two uploads
that each fit alone cannot both land. The second key is a hash of the owner,
which is why it has a namespace of its own: in 73819 a hash could land on
one of the fixed keys. Two owners whose hashes collide wait for
each other and nothing else. The uuid is cast before it is hashed, so the
same owner written in another letter case takes the same lock. T-R4.17b's
replace takes this lock for the old file's owner, with the old file left out
of the sum. An upload holds no other lock when it takes this one, and its
only row is the one it inserts.

## 29. Roles and permissions (T-R2.1)

`00054_create_roles_and_permissions.sql` stores the access model of plan 70 §4.1
as data. Its rows are reference data the app cannot run without, so the file
writes them (D-21).

- `app.permissions (key text PK, group_key, ordinal, in_matrix)` holds the
  catalogue: the deck's eighteen matrix rows in the matrix's order, with
  "Create student accounts" second in People (DG-03), then the four hidden keys
  (`scope.all`, `system.api_reference`, `system.data_export`, `system.leads`)
  with `in_matrix = false`, ordinals 4–7 of `system`. `key` is checked against
  `^[a-z]+(\.[a-z_]+)+$`, and `(group_key, ordinal)` is unique.
  `core/tests/catalogue_test.go` asserts that the table, `access.All()` and the
  contract's `PermissionKey` enum are the same list in the same order.
- `app.roles (id uuid PK DEFAULT uuidv7(), builtin_key UNIQUE, name, description,
  icon, color, copied_from, revision, created_by, created_at, updated_at)`.
  `builtin_key` is one of `admin`, `teacher`, `assistant`, `student`, or NULL for
  a custom role, and `roles_builtin_key_immutable` refuses any change to it.
  `icon` and `color` are checked against the Edit role dialog's twelve icons and
  seven colours. Names are unique case-insensitively (`roles_name_lower_key`).
  `revision` starts at 1 and moves with every grant change, so a cache keyed by
  `(id, revision)` is never stale. `copied_from` and `created_by` are not
  indexed: the table holds a handful of rows.
- `app.role_permissions (role_id, permission_key)` is the grant table, PK on the
  pair, with `role_permissions_permission_key_idx` for the foreign key to
  `permissions` (`ON DELETE RESTRICT`). A role's grants go with it
  (`ON DELETE CASCADE`), although nothing may delete a role (DG-52).
- The built-in rows: Quản trị viên (shield, dark), Giáo viên (graduation-cap,
  lime), Trợ giảng (hand-helping, blue), Học viên (user, gray), with the deck's
  descriptions in Vietnamese (D2, D11). The Teacher holds 13 keys, the Assistant
  6 and the Student `learning.take_tests`. The Admin stores nothing: it is the
  wildcard, with "Take tests" off (DG-51).
- Triggers on `role_permissions`, all row-level:
  - `role_permissions_guard` (BEFORE INSERT OR UPDATE) refuses a hidden key for
    any role, and any key but `learning.take_tests` for the Admin.
  - `role_permissions_keep_student_take_tests` (BEFORE DELETE OR UPDATE) keeps
    `learning.take_tests` on the built-in Student.
  - `role_permissions_bump_revision` (AFTER INSERT OR UPDATE OR DELETE) moves
    `roles.revision` for every role a grant leaves or joins. It runs as the
    invoker; R5's role commands get UPDATE on `revision` with their writes.
  UPDATE is covered although no role may update a grant: an update is a delete
  and an insert in one statement.
- All three tables are read-only for `quizzivy_app`, narrowing `00009`'s
  default privileges; `00129_grant_role_writes.sql` (R5, T-R5.8) grants the role
  commands' writes: `INSERT` and `UPDATE (name, description, icon, color,
  revision, updated_at)` on `roles`, `INSERT` and `DELETE` on `role_permissions`.
  Never `DELETE` on `roles` and never any write on `permissions`.

`00055_create_student_like_roles_view.sql` adds `app.student_like_roles (id)`,
the one definition of a strict student target (plan 70 §4.3, D-24): the built-in
Student, or a custom role holding nothing but `learning.take_tests`. Built-in
roles other than Student are excluded by key, because the Admin stores only its
"Take tests" cell and would otherwise match. A single-table view is
automatically updatable, so the file revokes INSERT, UPDATE and DELETE on it
from `quizzivy_app`. `access.IsStudentLike` is the same predicate in Go.

## 30. A role per user, the session epoch and the last Admin (T-R2.2)

The expand half of moving users from `users.role` to `users.role_id` (plan 70
§3, D-25). 00105 (73-r3.md T-R3.1, §42) validated the constraint and dropped the
sync trigger, `users.role`, `users_role_active_idx` and `app.user_role`.

- `00056_add_users_role_id.sql` adds `role_id uuid REFERENCES app.roles ON
  DELETE RESTRICT`, nullable, and backfills it: `admin` → the Admin role,
  `student` → the Student role. `users_set_updated_at` is disabled for the
  backfill, so `updated_at` keeps meaning "the user changed". The file raises if
  a row is still NULL, then adds `users_role_id_not_null NOT NULL role_id NOT
  VALID` (PG18), which holds for every new and updated row.
- `users_sync_legacy_role` (BEFORE INSERT OR UPDATE OF `role`, `role_id`) keeps
  the two columns in step while v0.7.0 and v0.8.0 machines overlap:
  - an insert without `role_id` is the old binary's and takes it from `role`;
  - an insert or update with `role_id` derives `role`: `student` for a role in
    `student_like_roles`, `admin` for any other, over the column default;
  - an update of `role` alone re-derives `role_id`.
  The new binary inserts `role_id` (the built-in Student, found by
  `builtin_key` in the statement) and reads the legacy value through
  `student_like_roles`.
- `00057_index_users_role_id.sql` builds `users_role_id_active_idx ON app.users
  (role_id) WHERE disabled_at IS NULL` `CONCURRENTLY`, in a `NO TRANSACTION`
  file, because the table is populated.
- `00058_add_users_session_epoch.sql` adds `session_epoch integer NOT NULL
  DEFAULT 0` with `users_session_epoch_check (session_epoch >= 0)`. The
  constant default is metadata-only. A command that ends a user's access bumps
  it; in R2, a disable and a staff password reset.
- `00059_add_users_last_admin_guard.sql` adds `users_last_admin`, AFTER UPDATE
  OR DELETE, row-level, with no column list. When the old row was an active
  Admin and the new one is not, it takes `FOR NO KEY UPDATE` on the Admin row of
  `app.roles`, then raises `check_violation` with constraint `users_last_admin`
  if no active Admin remains. The count is its own statement, so under READ
  COMMITTED it sees a concurrent departure the lock waited for: of two
  transactions disabling the only two Admins, exactly one commits. Any other
  change returns before locking, so disabling a student takes no lock on
  `roles`.
  - The guarantee holds under READ COMMITTED, which every writer in the app
    uses, and under SERIALIZABLE. Two REPEATABLE READ transactions count on
    their own snapshots and could both pass; only a hand-written transaction
    runs that way. Locking the remaining Admins `FOR SHARE` would close it, but
    two departures that leave a third Admin would then deadlock, and that is
    the likelier case.
  - `FOR NO KEY UPDATE`, not `FOR UPDATE`: it serialises the guard against
    itself without conflicting with the `FOR KEY SHARE` a foreign-key check on
    `users.role_id` takes, so promoting one user to Admin does not hold up
    another Admin's departure (and two transfers cannot deadlock).
  - No column list: the v0.7.0 binary demotes by writing `role`, and a column
    list matches the columns an UPDATE names, not those a BEFORE trigger sets.
    Naming `role` would also have made 00105's `DROP COLUMN role` depend on the trigger.
- **The schema's first `SECURITY DEFINER` function (D-26).** The row lock
  needs UPDATE privilege on `app.roles`, which `00054` revokes from
  `quizzivy_app`. Running as the invoker, R5's demote and disable would fail
  with 42501 instead of the guard. It is owned by `quizzivy_migrate` and
  hardened as the PostgreSQL docs require for definer functions:
  - `search_path = pg_catalog, app, pg_temp`. An unlisted `pg_temp` is searched
    first for relations and types, so a session that can create temporary
    objects could otherwise plant a `pg_temp.uuid` domain whose CHECK runs as
    the owner.
  - Every type and relation inside is schema-qualified.
  - EXECUTE is revoked from PUBLIC, so no role can attach it to a table of its
    own. A trigger fires regardless of EXECUTE.

`platform/db/tests/expand_r2_test.go` proves all of this on a scratch database.
It migrates to the version before `create_roles_and_permissions`, writes users
the v0.7.0 way, migrates the rest of the way up, then checks the backfill and
`updated_at`. As the app role, it checks both insert paths, the last-Admin
refusals and the two-transaction race, and it checks the definer's
attributes and that a foreign-key check on the Admin role does not hold the
guard up. It carries the `integration` tag and runs where `TEST_DESTRUCTIVE=1`
is set, as CI does.

## 31. Ownership (T-R2.9)

The expand half of per-teacher ownership (plan 70 §3, D-22, D-23). 00106
(73-r3.md T-R3.2, §43) validated every constraint below and dropped the fill
triggers and their functions.

- `00060`–`00063` add `owner_id uuid` to `tests`, `questions`,
  `question_groups` and `media_assets`, beside the provenance column each
  already has. Each file:
  - backfills `owner_id` from `created_by` (`uploaded_by` for media) with the
    table's `*_set_updated_at` trigger disabled, so no `updated_at` moves. An
    open builder tab sends a test's `updated_at` back as `expectedUpdatedAt`,
    and a moved value would fail its next autosave with `STALE_WRITE` during
    the deploy. `media_assets` has no `updated_at`;
  - raises if a row is still NULL;
  - adds `<table>_owner_id_not_null NOT NULL owner_id NOT VALID` (PG18);
  - adds a BEFORE INSERT trigger, `<table>_fill_owner`, that sets `owner_id`
    from the provenance column when the v0.7.0 binary inserts without it.
    `00060` creates `app.fill_owner_from_created_by()`, which `00061` and
    `00062` reuse; `00063` creates `app.fill_owner_from_uploaded_by()`.
- `00064` adds `users.created_by uuid REFERENCES app.users ON DELETE SET NULL`,
  nullable and not backfilled. NULL means the creator is unknown: every account
  that predates R2, and a Google self-join, which has none. Deleting the
  creator clears it rather than taking the account with it.
- `00065` adds `classes.teacher_id uuid`. `app.classes` has never had a
  `created_by`, so the backfill takes
  the oldest active Admin (by `created_at`, then `id`), the teacher v0.7.0
  shows for every class. It raises if no active Admin exists and a class does.
  `classes_fill_teacher` (BEFORE INSERT) applies the same rule to the old
  binary's inserts; it and its function `app.classes_fill_teacher()` share the
  name. It reads `users.role_id`, not the legacy `role`, so 00105's `DROP COLUMN
  role` could not break it.
- `00066`–`00070` add the foreign keys, `<table>_owner_id_fkey` and
  `classes_teacher_id_fkey`, each `REFERENCES app.users ON DELETE RESTRICT`
  and each in its own file after the columns. A column file holds ACCESS
  EXCLUSIVE on its table until it commits; a reference inline would then ask
  for SHARE ROW EXCLUSIVE on `users` while holding it. A v0.7.0 transaction
  that has written `users` and next needs a lock on that table would then wait
  on the migration while the migration waits on it: the student creation that
  enrols into classes through `class_members`' foreign key, or a user delete
  whose foreign-key checks read the referencing rows. The deadlock detector
  would abort one side, most likely the release. Apart, a column file never
  locks `users` beyond ACCESS SHARE, and a foreign-key file takes SHARE ROW
  EXCLUSIVE on the referencing table, which reads and foreign-key checks do
  not conflict with, so it simply waits for such a transaction to commit.
  `00064` keeps its inline reference: it references its own table.
- The new binary names every owner itself; the triggers existed only for the
  deploy overlap and 00106 dropped them:

  | Row | Owner written |
  |---|---|
  | a class | `teacher_id` = the acting user |
  | a new test, a bank question, a bank group | the acting user |
  | a duplicated test | the source test's owner; `created_by` stays the actor |
  | a committed Word import's test, its section groups and bank questions | the import's creator; `created_by` stays the committer (T-R2.12f) |
  | a section-owned group, a restored draft's question | the test's owner |
  | a group member | the group's owner |
  | a media asset | the uploader |
  | a student created by staff | `created_by` = the actor |
  | a Google self-join | `created_by` = NULL |

- Test fixtures and `seed/01-dev.sql` that insert a class name `teacher_id`.
  The class fill is the one that reads other rows: a fixture relying on it
  would take whichever fixture Admin is oldest at that moment, and a parallel
  package's delete of that user would then fail one side on the foreign key.
  The shared fixture in `constraints_test.go` also runs on scratch databases
  migrated to versions before `00065`, so it names `teacher_id` only when the
  column exists. Since 00106 every fixture and seed insert into `tests`,
  `questions`, `question_groups` and `media_assets` names `owner_id` too: a
  fixture that omits it fails with `23502`.

`platform/db/tests/expand_r2_test.go` (`TestTheOwnershipExpandKeepsTheOldBinaryWorking`)
writes a legacy dataset at v0.7.0's schema by two authors, with an older,
disabled Admin, and migrates to the version before 00106, the end of the expand
half. It checks that every row went to its author and
the class to the oldest active Admin, with `updated_at` unmoved. It also
checks the old binary's inserts are filled, an explicit owner is kept, an
account's creator is optional and cleared when the creator goes, and each
constraint stays NOT VALID.
`TestAddingTheClassTeacherWaitsForTheOldStudentCreateInsteadOfDeadlocking`
holds a v0.7.0-style student insert open across `00065`, enrols the student
into a class after it, and checks the foreign-key files wait for the commit
instead of deadlocking.

## 32. Ownership indexes (T-R2.10)

`00071`–`00077` each hold one `CREATE INDEX CONCURRENTLY` in a `NO
TRANSACTION` file, because every table is populated and in use; Down is `DROP
INDEX CONCURRENTLY IF EXISTS`. `docs/setup/deploy.md` lists the drops to run
after an interrupted build.

| Index | Definition | Serves |
|---|---|---|
| `classes_teacher_idx` | `(teacher_id)` | a teacher's classes; the RESTRICT check on a user delete |
| `tests_owner_idx` | `(owner_id, id DESC) WHERE deleted_at IS NULL` | a teacher's tests list |
| `questions_owner_bank_idx` | `(owner_id, id DESC) WHERE deleted_at IS NULL AND context_group_id IS NULL` | a teacher's bank, without group members |
| `question_groups_owner_bank_idx` | `(owner_id, updated_at DESC, id DESC) WHERE owner_section_id IS NULL AND archived_at IS NULL` | a teacher's bank groups, in `00049`'s order |
| `media_assets_owner_idx` | `(owner_id, kind, created_at DESC) WHERE deleted_at IS NULL` | a teacher's media library, as `media_assets_kind_created_idx` serves everyone's |
| `users_created_by_idx` | `(created_by) WHERE created_by IS NOT NULL` | the accounts a staff member made; the SET NULL scan when they go |
| `assignments_creator_idx` | `(created_by, id DESC)` | a teacher's assignments |

The four partial indexes (tests, questions, question groups, media) cannot
serve the `owner_id` RESTRICT checks from `00066`–`00069`, which run on every
user delete (today only a student's) and do not filter on the partial
predicate. Each check scans its table, as the unindexed `created_by` and
`uploaded_by` checks already did, which is cheap at today's sizes. Only
`classes_teacher_idx` covers its check.

No plan is forced. At today's volumes a sequential scan is often the right
plan (§14), and the indexes earn their place as teachers and rows grow.

## 33. Encrypted join codes (T-R2.14a)

`00078` lets a join code be read back (D5, D-27) without letting a database dump
redeem one. Three columns join `app.class_join_codes`:

| Column | Type | Meaning |
|---|---|---|
| `code_ciphertext` | `bytea`, 36 bytes when set | a 12-byte nonce, the 8-byte code and a 16-byte AES-256-GCM tag; the additional data is the class id's 16 bytes, then the code id's |
| `key_id` | `smallint`, never 0 | the 16-bit HKDF fingerprint of the key that sealed the code and keyed its hash |
| `lookup_scheme` | `smallint NOT NULL DEFAULT 1`, 1 or 2 | 1: `code_hash` is the SHA-256 of the code (v0.7.0); 2: it is the HMAC-SHA256 under `key_id`'s lookup key |

- **`class_join_codes_scheme_consistent`** makes a row either legacy (scheme
  1, no ciphertext, no key) or fully sealed (scheme 2 with both). The v0.7.0
  binary's insert names none of the columns and is a legacy row.
- **`code_hash` keeps its `UNIQUE` index**, and it is still the only lookup
  path: a typed code is searched as `code_hash = ANY(candidates)`, the keyed
  hash under the current and the previous key and the legacy SHA-256. The
  newest matching row is taken, and it counts only against the candidate of
  its own scheme and key.
- **The keys never reach the database.** A dump yields ciphertexts and keyed
  hashes; without `JOIN_CODE_KEY` neither opens nor can be hashed through the
  code space. A legacy row's SHA-256 still can, and quickly: beside the
  stored four-character hint only 2^20 codes remain. That is why R4 replaced the
  live legacy codes at start-up (D5, below).
- **Down refuses** while an unrevoked, unexpired scheme-2 code exists,
  because the previous binary looks codes up by SHA-256 alone.
- **The API replaces the legacy rows at start-up from v0.10.0 (T-R4.22):**
  each unrevoked, unexpired scheme-1 row is revoked and a scheme-2 row is
  inserted for its class with the same `expires_at`, `max_uses` and
  `created_by` and `uses_count` (F-67), in one transaction per class under
  advisory key 41 (§28), audited with a NULL actor; a revoked or expired
  scheme-1 row is left as it is.
- **Lock order for a class and its join codes (F-23).** The transactions that
  lock a class row and one of its code rows take them in these modes and orders:
  - The start-up rotation takes advisory `(73819, 41)` (§28), then the class row
    `FOR NO KEY UPDATE`, then the class's active scheme-1 code row `FOR UPDATE`.
    `Rotate` and `Revoke` take the class row `FOR NO KEY UPDATE`, then update the
    code row, and take no advisory key.
  - `Rotate` reads `archived_at` in that same locking `SELECT` (F-64), so the read
    and the lock are one statement: an `Archive`, which holds the same row lock for
    its plain `UPDATE`, either committed first, and the locked read returns the
    archived row (READ COMMITTED re-reads the latest version after the wait), or
    waits for the `Rotate`. An archived class answers `ErrClassArchived` before any
    code row is touched, after the scope predicate has already answered
    `ErrClassNotFound` for a class the caller does not reach. `Revoke` does not read
    `archived_at` and stays allowed on an archived class.
  - A redemption (`EnrolNewMember`, `EnrolExisting`) takes the code row `FOR UPDATE`
    first, found by its lookup hash, and its member insert then takes `KEY SHARE`
    on the class row through the foreign key.
  - `NO KEY UPDATE` conflicts with itself, with `SHARE` and with `UPDATE`, and not
    with `KEY SHARE`. A redemption and the three writers above therefore never wait
    for each other in a cycle: the writer waits for the code row and goes on when the
    redemption commits. `Update` and `Archive` (a plain `UPDATE` of the class) and
    `AddMember` (`FOR SHARE`) still wait for those writers, and the writers for them.
  - `Delete` takes the class row `FOR UPDATE`, as every `DELETE` does, and removes the
    code rows by the cascade: the inverse of a redemption's order. It meets one only
    when the class was archived and deleted between the redemption's claim and its
    member insert, because a redemption of an archived class is refused before it
    takes a class lock. That window exists because the claim reads `archived_at` and
    `self_join_enabled` from the class row without a lock (`repositories/enrol.go`,
    `claimCode`), so an `Archive` can commit between a claim and its insert; this
    predates F-23 and is unchanged. `Delete` stays as it is: taking the code rows first
    would invert it against `Rotate` and `Revoke`.
  - A contender outside this list can still abort the rotation as a deadlock or
    serialization victim; the repository answers `domain.ErrRotationContended` and
    the command tries that class once more (T-R4.22).

## 34. Notifications (T-R4.10a)

Two tables, both new and empty at release, so every column is `NOT NULL`
inline where it must be and no index is built `CONCURRENTLY`. The app role
gets SELECT, INSERT, UPDATE and DELETE on both through `00009`'s default
privileges; neither file writes a grant.

```sql
CREATE TABLE app.notifications (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  kind       text NOT NULL,                       -- dotted key, at most 64 characters
  params     jsonb NOT NULL DEFAULT '{}'::jsonb,  -- an object, at most 4096 bytes as text
  target     jsonb,                               -- NULL, or an object of at most 1024 bytes
  dedupe_key text NOT NULL,                       -- 1 to 200 characters
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  read_at    timestamptz,
  CONSTRAINT notifications_user_dedupe_key UNIQUE (user_id, dedupe_key)
);

CREATE TABLE app.notification_preferences (
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  event      text NOT NULL,
  in_app     boolean NOT NULL,
  email      boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, event),
  CONSTRAINT notification_preferences_event_check CHECK (event IN (
    'attempt.submitted', 'attempt.flagged', 'assignment.closing', 'assignment.due_soon', 'result.ready'))
);
```

- **No sentence is stored.** The reader's console words a row from `kind` and
  `params` in the reader's language. `kind` is checked for its shape only
  (`notifications_kind_check`), so a release adds a kind in the contract and
  the domain without a migration.
- **`params` and `target` are bounded objects** (D-28): `notifications_params_shape`
  and `notifications_target_shape` require `jsonb_typeof(…) = 'object'` and
  bound `octet_length(…::text)`, as `00047` does. The text length is what
  the constraint measures because it is the same for a value whatever its
  storage: `pg_column_size` moves with compression and TOAST. A NULL
  `target` passes its check; the JSON value `null` does not.
- **`UNIQUE (user_id, dedupe_key)` is what a merge conflicts on.** `Notify`
  is one `INSERT … ON CONFLICT (user_id, dedupe_key) DO UPDATE`: a second
  notice about the same thing updates the row, makes it unread again
  (`read_at = NULL`) and, when asked to add, sums `count` and `toGrade` with
  the stored values inside the statement. Nothing reads the row first, so a
  class handing in within the same seconds loses no increment. The same key
  for two users is two rows.
- **A switch that is off stops the write.** The same statement inserts only
  when no `notification_preferences` row for the user and the kind's event has
  `in_app = false`; a missing row means the default, which is on. `email` is
  stored and read by nothing until R7.
- **`updated_at` moves by the trigger** on both tables (§13.2), on a merge
  and on a mark-read alike. `created_at` never moves: a merged row keeps its
  id, its place in the list and its retention.
- **Retention is 180 days from `created_at`.** `PruneNotifications` runs at
  API start-up and then daily and deletes `WHERE created_at < now − 180
  days`; it is idempotent and takes no lock beyond its rows.
- **`ON DELETE CASCADE` from `users`** on both tables: a deleted account
  takes its notifications and switches with it. Names inside another user's
  `params` stay, which is the hand-off in D-28.
- **Later releases replace `notification_preferences_event_check` by name** to
  add switches (R7, R10). The constraint name is part of the interface.

Indexes, and the statement each exists for:

| Index | Serves |
|---|---|
| `notifications_user_recent_idx (user_id, id DESC)` | The reader's list: `WHERE user_id = $1 AND id < $2 ORDER BY id DESC LIMIT n`, a keyset on the time-ordered uuidv7 id. The statement orders by the column, not by the `id::text` it returns, or the index cannot give the order. Its leading column also serves the cascade from `users` |
| `notifications_unread_idx (user_id) WHERE read_at IS NULL` | The unread count the shell polls (`GET /me/summary`) and "mark all read". Partial, so it holds unread rows only |
| `notifications_created_at_idx (created_at)` | The daily prune |
| `notifications_user_dedupe_key (user_id, dedupe_key)`, unique | The merge's conflict target |
| `notification_preferences_pkey (user_id, event)` | The five switches of one user, and the switch lookup inside `Notify` |

`repositories/tests/plan_test.go` seeds 50,000 rows for 200 users inside a
transaction it rolls back and asserts that the list reads
`notifications_user_recent_idx` alone, with the cursor as an index condition
and no sort, and that the count uses `notifications_unread_idx`.

A bell that names an audience (DG-170, spec v0.80) adds `kind = ANY($n::text[])` to the list, the count
and both markings, where the array holds that console's kinds (`notifications/domain/audience.go`). It
adds no column, index or migration: the list reads the same index in the same order and applies the kind
as a filter, and the count finds the user's unread rows through `notifications_unread_idx` and reads
their kind from the few rows it returns. The markings add the same filter to statements that already
select by `user_id`. The plan test asserts the audience list and count as it asserts the others.

## 35. Caller profile and preferences (T-R4.7)

`00085_add_users_profile_fields.sql` adds five nullable text columns to
`app.users`. The display-name check accepts 1–80 characters after `btrim`;
the application stores the trimmed value. Phone accepts 6–20 digits, spaces
and plus signs. Locale is `vi` or `en`. A non-null time zone has 1–64
characters; the application additionally validates it with `time.LoadLocation`.
`avatar_key` holds the storage key of the photo (`avatars/{userId}/{uuid}.png`),
written and cleared by T-R4.8 (`PUT` and `DELETE /me/avatar`) with an audit row each;
this migration adds no photo operation and T-R4.8 adds no column. Down drops the five columns and their checks.

`00086_add_users_preferences.sql` adds `preferences jsonb NOT NULL DEFAULT
'{}'`. Named checks require an object and at most 8192 bytes of normalized
`preferences::text`, measured by `octet_length`. This is the stored UTF8
representation limit, separate from the HTTP body limit. Down drops the
column and both checks. No new index or privilege is needed. Existing inserts
that omit these columns retain nullable fields and an empty object.

The contract restricts preference keys and nested assignment-default fields.
A PATCH merges top-level keys; a supplied nested object replaces that key's
previous object. An empty object is a no-op. Read defaults do not materialize
locale, zone or preference keys in the row. The effective display zone is
`Asia/Ho_Chi_Minh` until the caller explicitly stores another valid zone.

Both profile and preference writes lock and read the current row before
merging. Mutation and audit insertion use one data-modifying CTE. Only actual
changed fields are audited, with sorted private field names; no-op writes
produce no audit. Invalid audit metadata rolls back the mutation as well.
Concurrent disjoint writes therefore retain both changes, while the same key
follows lock acquisition order. Disabled callers are rechecked against the
locked row. The legacy full-name rename audit remains compatible.

Private fields appear only in the caller's `CurrentUser`. Public `User`
projections exclude phone, locale, zone and preferences. Student class, intro
and join-preview teacher names use `coalesce(display_name, full_name)` and
add no private field. `/join/preview` still exposes only the teacher's chosen
student-facing name through its existing field. Contract and hosted API/isolation
checks cover these public/private projection boundaries.

The independently verified fixture helpers register cleanup before insert,
use only newly allocated ordinary Student IDs, drain repository writers before
releasing locks, and check exact removal. Audit rows are retained; deleting
the owned user leaves the audit actor null through the existing foreign key.
Failure coverage requires real reached-stage markers and canceled-Commit
proof; successful cleanup cannot stand in for an intended failure branch.
No legacy user, role, owner trigger or compatibility alias is removed here.

## 36. Question level, skill and legacy choice compatibility (T-R4.15)

Migrations `00087_add_questions_level_skill.sql` and
`00088_add_test_version_questions_level_skill.sql` add nullable text columns
`level` and `skill` to the bank and frozen question tables. Named CHECK sets
accept the seven CEFR levels and six skills recorded in spec §7.1; null remains
valid. There is no backfill, index, new privilege or trigger. Each Down removes
only its two columns and matching checks.

New teacher authoring rejects a choice array above eight. Storage and frozen
read contracts remain uncapped so historical nine-option content can be read,
bodylessly copied, published and restored. Included import questions undergo
the same authoring refusal; a late refusal rolls back earlier writes in the
commit transaction. These restrictions do not delete or rewrite old content.

Bank writes, standalone and grouped snapshots, structural copies and restored
drafts carry both fields. Snapshot values are independent of subsequent bank
edits; existing question/media row-lock ordering is unchanged. Draft list skills
combine both standalone and group-member arms, drop nulls and return a distinct
sorted array. Metadata facets retain the other dimensions and owner scope while
omitting their own dimension. The existing accent-folded search expression is
unchanged. No representative query-plan or runtime performance claim is made.

The combined candidate contains the full migration inventory 00001–00091.
Its purpose-database app/migrate tests, the isolated 87/88 down/up rehearsal and
the separate historical oversized-choice Neon inventory remain acceptance gates.
A synthetic nine-option fixture is compatibility evidence, not a count of real
historical rows.

## 37. Alt text for a question's image (T-R4.62a)

Migrations `00097_add_questions_media_alt.sql` and
`00098_add_test_version_questions_media_alt.sql` add one nullable `text` column,
`media_alt`, to the bank and frozen question tables. Each carries a named CHECK,
`questions_media_alt_check` and `test_version_questions_media_alt_check`, of
`char_length(media_alt) BETWEEN 1 AND 1000`, added with the column. There is no
default, backfill, index, trigger or new privilege, and each Down drops its
constraint and column. 00093 to 00096 were reserved for other R4 tasks and are
unused: those tasks took the next number above the integration branch's highest when
their pull requests opened (T-R4.11 took 00099 and 00100, T-R4.12 00101, T-R4.9
00102), so the inventory skips from 00092 to 00097 and goose applies 00097 across
the gap.

Only an image has alt text. `Input.Validate` refuses a value unless the
resolved media kind is `image`, with a field error on `mediaAlt` (400
`VALIDATION_FAILED`, `details.mediaAlt`), and also refuses a blank one and one
over 1000 characters; publish validates again. The bank store writes the value
only beside an image, so clearing the media, or changing it to audio, leaves
NULL. A write replaces the stored value, as it does for `level` and `skill`, so
an update that leaves `mediaAlt` out clears it. A file replacement keeps the
question's alt text, because it never changes the kind.

Publish copies the bank value into `test_version_questions.media_alt` in the
statement that freezes the question; the draft read, the lock order and the
`media.LockForVersionUse` and `questions.LockForDraftUse` calls are unchanged.
Restore as draft copies it back from the version, a duplicated group copies the
draft's, and the version preview, the student's paper and the result read the
frozen value. The version diff puts it in the `media` part of a question, so a
changed alt text is one `changed` entry and raises `unpublishedChanges`.

### Contract step for the image rule (R5, after v0.10.0)

R4 enforces "alt text only beside an image" in the write path and nowhere in the
schema, because an R3 binary can still change an image to audio without touching
`media_alt`, and a check would turn that write into an error during the rolling
deploy. Once v0.10.0 is the only binary, the rule moves into the schema, in three
files that follow expand then contract:

1. **Clear what the rule would refuse, on `questions` only.**
   `UPDATE app.questions SET media_alt = NULL WHERE media_alt IS NOT NULL AND
   media_asset_kind IS DISTINCT FROM 'image'::app.media_kind`. A bank row is
   editable, and the only rows it can touch are ones an R3 binary left behind. On
   `test_version_questions` there is no UPDATE: a published version does not change,
   and none can hold the pair, because publish validation refuses alt text beside
   other media and an R3 publish writes NULL. The step runs `SELECT count(*) FROM
   app.test_version_questions WHERE media_alt IS NOT NULL AND media_asset_kind IS
   DISTINCT FROM 'image'::app.media_kind` first and stops for a decision on any
   other answer than 0, instead of rewriting a version.
2. **Add the checks without validating them.**
   `ALTER TABLE app.questions ADD CONSTRAINT questions_media_alt_image_check CHECK
   (media_alt IS NULL OR media_asset_kind IS NOT DISTINCT FROM 'image'::app.media_kind)
   NOT VALID`, and the same on `app.test_version_questions` as
   `test_version_questions_media_alt_image_check`. New and changed rows are checked
   from here on.
3. **Validate.** `ALTER TABLE ... VALIDATE CONSTRAINT` for each, which scans the table
   under a `SHARE UPDATE EXCLUSIVE` lock and blocks no write.

The kind needs no trigger and no join: both tables already hold it as
`media_asset_kind`, tied to the asset by the composite foreign key
`(media_asset_id, media_asset_kind)` to `media_assets(id, kind)` and to
`media_asset_id` by `questions_media_pair_complete` and `tvq_media_pair_complete`.
The predicate must say `IS NOT DISTINCT FROM`, as `00011` does for audio: with a plain
`= 'image'` a NULL kind makes the whole check NULL, and a CHECK passes on NULL, so alt
text with no media at all would be accepted. Each Down drops its constraint, and the
three files are numbered when R5 plans them.

## 38. Per-student overrides (T-R4.12)

`00101_create_assignment_student_overrides.sql` adds one table. Down drops it.

```sql
CREATE TABLE app.assignment_student_overrides (
  assignment_id    uuid NOT NULL REFERENCES app.assignments(id) ON DELETE CASCADE,
  student_id       uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  closes_at        timestamptz,
  duration_minutes integer,
  extra_attempts   smallint NOT NULL DEFAULT 0,
  reason           text NOT NULL,
  created_by       uuid REFERENCES app.users(id) ON DELETE SET NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (assignment_id, student_id)
  -- and the four named checks in the table below
);
```

| Constraint or index | What it holds |
|---|---|
| `assignment_student_overrides_pkey (assignment_id, student_id)` | One override per student and assignment; its leading column serves the cascade from `app.assignments` and every read of one assignment's overrides |
| `assignment_student_overrides_student_idx (student_id)` | The cascade from `app.users` |
| `..._duration_check` | `duration_minutes BETWEEN 1 AND 600`, the assignment's own bound |
| `..._extra_attempts_check` | `extra_attempts BETWEEN 0 AND 10` |
| `..._reason_check` | `char_length(reason) BETWEEN 1 AND 500`; the command stores the trimmed reason, so a reason of only whitespace never reaches the check |
| `..._changes_check` | `closes_at IS NOT NULL OR duration_minutes IS NOT NULL OR extra_attempts > 0`: a row that changes nothing is a DELETE |

`created_by` is `ON DELETE SET NULL`, so removing a teacher's account keeps the
accommodation they gave. The previous release neither reads nor writes the table, so
the table needs no expand step, and a rolled-back binary leaves its rows alone. The
app role holds the usual `SELECT, INSERT, UPDATE, DELETE` through the default
privileges. R5's `grant_maintenance_role` grants `quizzivy_maintenance` `SELECT,
UPDATE (closes_at)` on it for the maintenance extension (T-R4.12b).

**The merge.** `setStudentOverrides` computes the merged row in its `wanted`
CTE, from the existing override, and inserts that row: PostgreSQL checks the
CHECKs on the proposed row before it looks for the conflict, so a candidate
that carried only the request's fields would fail `..._changes_check` on a
student whose stored row holds the rest. `ON CONFLICT DO UPDATE` then takes the
merged values from `EXCLUDED`. The assignment row is held `FOR NO KEY UPDATE`
for the transaction, which serialises writers of one assignment's window and
overrides and does not block an attempt's insert (its foreign-key check takes
`FOR KEY SHARE`).

**Reading the student's window.** `schedule.OverrideJoin` is the `LEFT JOIN` on the
student, `schedule.CloseOf("o")` the close it yields, and `schedule.Window.WithOverride`
its Go twin; `attempts.Rules`, `attempts.RulesFor`, `LoadResult` and the student's card
and intro all read through them. An override's `closes_at` never closes a student
sooner: the close is `greatest(least(closed_at, closes_at), o.closes_at)`.

**Moving the deadlines in progress (12b).** `extendAssignment`, `updateAssignment`,
`reopenAssignment` and `setStudentOverrides` end by calling `schedule.RecomputeDeadlines` in the
transaction that changed the window; `deleteStudentOverride` takes the same two locks and
recomputes nothing, because removing an override can only shorten a student's window and a
deadline never moves earlier. An update or a reopening recomputes the whole assignment
unconditionally, and a shorter close or a Close now moves nothing for the same reason. An
attempt whose `deadline_at` is not after the database's `now()` is over, swept or not, and is
skipped, so no writer revives it. The recompute sets `deadline_at` of each `in_progress` attempt to `least(started_at +
coalesce(o.duration_minutes, a.duration_minutes), CloseOf("o"))` where that is later than the
deadline the attempt has, and writes one `attempt.extended` audit entry per attempt moved
(`deadline_at` old and new, the cause, the assignment) through a data-modifying CTE. A deadline
never moves earlier: removing an override, or a time limit shorter than the one an attempt
started under, leaves a running attempt alone, and a deadline that a maintenance window pushed
past the rule stays where it is. The recompute is limited to `studentIds` for an override and
runs over every attempt of the assignment for an extension.

*Lock order, always:* the maintenance-windows advisory lock `(73819, 40)` shared, then the
assignment row, then the attempt rows in id order. An attempt start takes the lock and the
assignment row `FOR SHARE` before it inserts; the five writers above take the assignment row
`FOR NO KEY UPDATE` (`updateAssignment` its long-standing `FOR UPDATE`), both of which conflict
with `FOR SHARE`, so a start waits for them; a plain `NO KEY` lock does not conflict with the
`FOR KEY SHARE` an attempt's foreign key takes. A start therefore either commits before the writer reads the
attempts, and the recompute moves the attempt it inserted, or reads the rules after the writer
committed, and computes its deadline from the new window. `window-schedule` takes the lock
exclusively and updates assignments and attempts in one statement, so it never runs among them.

*The start race (R12a-5).* `attempts.Store.Create` re-reads the rules inside its transaction,
after the maintenance guard, with `FOR SHARE OF a`, and stores the deadline it computes from
that read. The service's earlier read, outside the transaction, decides what the first
maintenance check covers and nothing else: the transaction checks again that the student is
targeted and that the committed window is open, and when the deadline it computes is later than
the one the first check covered, it runs the maintenance check a second time over the longer span. `FOR SHARE OF o` is not taken: an override is written under the assignment row
lock, so the assignment lock already orders a start against every override write, and a lock on
the override row would only add a second, unordered path. The override join is a `LEFT JOIN`,
and `FOR SHARE` on its nullable side is refused by PostgreSQL in any case.

## 39. Where a device last refreshed (T-R4.9)

`00102_add_refresh_tokens_geo_label.sql` adds one nullable column. Down drops the constraint,
then the column.

```sql
ALTER TABLE app.refresh_tokens
  ADD COLUMN geo_label text
    CONSTRAINT refresh_tokens_geo_label_check
    CHECK (char_length(geo_label) BETWEEN 1 AND 80);
```

The server builds the label as "City, CC" from `CF-IPCity` and `CF-IPCountry`, only when
`CLIENT_IP_HEADER` names `CF-Connecting-IP` and the request carries it. A sign-in stores it, and a
rotation stores the label of its own request when it has one and copies the predecessor's when it
has none, so a family's head row says where the device last signed in or refreshed. The check
scans the table once while the column is added; `refresh_tokens` is pruned by expiry, so the scan is
cheap, and a nullable column with no default rewrites nothing. The label reaches no log and no
`audit_log` row, and leaves the server only in `GET /auth/sessions`, to the owner of the family.

A session is a family with an unrevoked, unexpired row. The list reads it through
`refresh_tokens_user_live_idx (user_id) WHERE revoked_at IS NULL`, and the current session's
lookup by `token_hash` and `refresh_tokens_family_idx`; neither needed a new index.

## 40. Composed text (F-37)

`00103_compose_text_nfc.sql` changes data and no schema. Be Vietnam Pro draws the precomposed
Vietnamese letters, so text a person typed in a decomposed form (Unikey's "Unicode tổ hợp", a file
name from a Mac, a paste from a PDF) fell back to another font in a title or a name. From this
release the server composes the text it stores (spec §12), and this file brings the rows written
before to the same form.

**What it rewrites.** Each plain column below is set to `normalize(col, NFC)` where it
`IS NOT NFC NORMALIZED`, one `UPDATE` per column from a single list in a `DO` block:
`tests` (`title`, `description`), `test_sections` (`title`, `instructions`), `questions`
(`sample_answer`, `transcript`, `media_alt`, `tags`, and `prompt` and `explanation` only where
`prompt_content` / `explanation_content` is null), `question_options.text` where `content` is null,
`question_blank_answers.answer`, `question_groups.title`, `group_stimuli.title`,
`group_recordings.transcript`, `classes` (`name`, `description`), `users` (`full_name`,
`display_name`), `media_assets` (`display_name`, `original_filename`), `word_imports.title`,
`word_import_sources.filename`, `assignments.student_note`, `assignment_student_overrides.reason`
and `attempts` (`teacher_note`, `void_reason`). A tag array is composed element by element, a
repeat that composing makes of two spellings is dropped, and the order of first appearance is kept.
A value is left as it is when composing would take it past the `CHECK` that bounds its column
(`tests.title` 200, `classes.name` 120, `attempts.teacher_note` 2000 and the rest of the list), which
only a few scripts can do (U+0958 grows to two characters): no row can stop the deploy.

**What it leaves, and why.** The published versions (`test_version_*`, `test_versions.change_note`)
are frozen. `attempt_answers` is a student's saved work, and the server composes new saves.
`attempt_events` and `audit_log` are append-only. Every prose document (`prompt_content`,
`explanation_content`, `question_options.content`, `question_groups.instructions`,
`group_stimuli.content`) and the plain text kept beside one are left together: a write checks that
the text equals the document's projection, SQL cannot derive that projection, and composing the two
apart breaks the equality where a combining mark opens a text node. The next save of such a row
composes the pair, and the web composes it where it draws. Emails, passwords, tokens, join codes, ids,
keys, kinds, statuses and URLs are never composed.

**The accepted answers.** `UNIQUE (blank_id, answer)` would refuse composing a decomposed answer
whose composed twin exists, so the file first deletes, per blank and per composed form, every row but
one, preferring the row that is already composed. The grader folds the two spellings into one
(`normalise` composes both sides), so the answer the deleted row gave is still accepted. The write
path de-duplicates the same way.

**`updated_at`.** `app.set_updated_at()` sets `updated_at := now()` on every `UPDATE`, unconditionally.
`tests.updated_at` is the token an open editor sends back with its autosave (a moved one is a 409),
the lists sort on it, and `word_imports.updated_at` is the clock idle retention reads. So for each of the eight
tables that has such a trigger (`tests`, `questions`, `question_groups`, `classes`, `assignments`,
`assignment_student_overrides`, `users`, `word_imports`) the file runs `ALTER TABLE … DISABLE TRIGGER
<name>` before the rewrite and `ENABLE TRIGGER <name>` after it, in the same transaction. It names the
trigger: never `ALL`, never `session_replication_role`. The other tables the file touches have no such
trigger.

**Lock and duration.** One transaction, no batching, no table rewrite, no index build. `DISABLE
TRIGGER` takes `SHARE ROW EXCLUSIVE` on its table until the transaction commits: a write to one of
those eight tables (the autosave of a test, a profile edit) waits, and a read does not. The
`UPDATE`s take `ROW EXCLUSIVE` and block no reader and no writer of another row. Production holds tens
of questions and a handful of accounts (T-R4.15), so the transaction lasts milliseconds to seconds.
`SET LOCAL lock_timeout = '5s'` makes it fail instead of wait: a transaction that holds a conflicting
lock for longer than five seconds makes the migration fail at its first `ALTER TABLE`, rather than queue
every writer behind it. A failure rolls the whole transaction back, the disabled triggers included
(`DISABLE TRIGGER` is transactional), so nothing is left half done or disabled, and the deploy is
retried; the file is idempotent, so a retry, or a second run after the old binary has written
decomposed text, changes only the rows that are not composed.

**The deploy log.** The `DO` block raises `NOTICE`s: how many decomposed duplicate accepted answers
were folded, the rows per column that the `CHECK` guard left (with the table and column, nonzero
counts only), or that every listed column is composed. `TestTheComposeMigration*` captures them
through pgx's `OnNotice`.

**Down.** A no-op with a comment, a deliberate deviation (D-38). The file is data only and
forward-safe.
## 41. When and where a class meets (T-R4.18)

`00104_add_classes_schedule_room.sql` adds two nullable columns. Down drops the two constraints,
then the two columns. The number follows 00102 because 00103 is reserved for F-37, and the order
of merging is the rule: F-37 (00103) merges before T-R4.18 (00104). If the order ever inverts, the
second pull request renumbers its migration before it merges. goose refuses a database that has
applied 00104 when 00103 is missing ("found 1 missing migrations"), and a deploy never passes
`-allow-missing` to get past it.

```sql
ALTER TABLE app.classes
  ADD COLUMN schedule_label text
    CONSTRAINT classes_schedule_label_check
    CHECK (char_length(schedule_label) BETWEEN 1 AND 120),
  ADD COLUMN room text
    CONSTRAINT classes_room_check
    CHECK (char_length(room) BETWEEN 1 AND 60);
```

The commands trim both and store NULL for a value that is blank once trimmed, so a check never
meets an empty string. The checks scan `classes` once while the columns are added; a nullable column
with no default rewrites nothing. Both columns are read by the teacher's class reads, by the class a
student joins and by the student's own classes; neither reaches an index, a log or an audit row.

The class average (`Class.averageScore`) is derived, not stored. It reads the best graded attempt of
each live member on each assignment that targets the class, through
`attempts_student_started_idx (student_id, started_at DESC)`, `assignment_classes_class_idx` and
`attempt_answers_pending_idx`; on 25 classes with 3,000 attempts each (75,000 attempts) the plan is an
index scan of `attempts` with no sequential scan and takes 173 ms for the page and 6.8 ms for one class,
so no index was added.

## 42. The legacy role goes (T-R4.49, T-R3.1)

The contract half of §30 (D-25). `00105_drop_users_legacy_role.sql` takes the legacy role out
once nothing older than v0.8.0 can still be serving. v0.8.0 and the previous binary, v0.9.0,
never name `users.role`, insert `role_id` themselves, derive what they need from
`student_like_roles`, and accept a token with or without a `role` claim.

One transaction, `SET LOCAL lock_timeout = '5s'`, in this order:

1. `VALIDATE CONSTRAINT users_role_id_not_null` under SHARE UPDATE EXCLUSIVE, so reads and writes
   continue. A row without a role stops the migration here with `23502`, before anything is
   dropped. The constraint has refused every new and updated row since 00056, so a row inserted
   meanwhile cannot make the validation fail.
2. `DROP TRIGGER users_sync_legacy_role` and its function. The trigger goes first: its
   `UPDATE OF role` list depends on the column, and dropping the column first fails.
3. `DROP INDEX users_role_active_idx`, `DROP COLUMN role`, `DROP TYPE app.user_role`. There is no
   `IF EXISTS` anywhere, so a wrong name fails the migration rather than leaving an object.

Steps 2 and 3 hold ACCESS EXCLUSIVE on `app.users` to commit. Nothing is rewritten, because a
dropped column is only marked, so the window is milliseconds at any size. What waits is a read or
write of `app.users` in those milliseconds. A query already running on the table delays the lock
request, and every later query queues behind the request, so `lock_timeout` turns a lock that does
not arrive within five seconds into a failed migration: the release stops with the schema as it
was (`55P03`), instead of a sign-in queue. The app role's grants on `app.users` are table level, so
a column added back by Down inherits them.

A deploy that fails after the migration leaves v0.9.0 serving the contracted schema, which it
reads. A rollback as far as v0.8.0 works for this migration too; v0.7.0 names the column
(`docs/setup/deploy.md` says roll forward).

Down restores the enum (and `USAGE` for the app role, granted explicitly rather than left to default
privileges), the column, backfilled with `users_set_updated_at` disabled so no `updated_at` moves,
then `NOT NULL DEFAULT 'student'`, the index, the function and the trigger. The backfill reads
`app.student_like_roles`: a rule over grants alone would call an Admin with "Take tests" on a
student. The constraint stays validated, because Down never weakens a valid `NOT NULL`. Nothing
runs it in production.

Tests: `contract_r3_test.go` (what is gone and what is kept, the refusal of a user without
`role_id`, the stop at a user without a role, the give-up under a held lock, and the Down), the
two expand tests in `expand_r2_test.go`, which stop at the migration before this one because they
prove the old binary on the expand schema, and `core/tests/legacy_role_test.go`, which fails on
the legacy role anywhere in `server/`, `seed/` and `docker/` except the directory of migration tests.

## 43. Ownership is validated and the fill triggers go (T-R4.49, T-R3.2)

The contract half of §31 (D-22, D-23). `00106_validate_ownership.sql` takes the expand half's
scaffolding out once nothing older than v0.8.0 can still be serving. v0.8.0 and the previous
binary, v0.9.0, name the owner in every insert: a test, a question, a group and a media asset
carry `owner_id`, a class `teacher_id`. The `BEFORE INSERT` triggers filled it only for v0.7.0.
The five constraints have refused a NULL owner for every new and updated row since 00060 to
00065, so a row written meanwhile cannot make the validation fail.

One transaction, `SET LOCAL lock_timeout = '5s'`, in this order:

1. A `DO` block counts, for `classes.teacher_id` and `owner_id` of `tests`, `questions`,
   `question_groups` and `media_assets`, the rows still NULL. The first table that has any stops
   the migration with `23502`, whose message, `TABLE` and `COLUMN` name it, before anything
   changes. Without it `VALIDATE` would stop too, naming a relation and no more.
2. `VALIDATE CONSTRAINT` on `classes_teacher_id_not_null`, `tests_owner_id_not_null`,
   `questions_owner_id_not_null`, `question_groups_owner_id_not_null` and
   `media_assets_owner_id_not_null`. PG18 validates a `NOT NULL` this way, under SHARE UPDATE
   EXCLUSIVE with one scan of the table, so reads and writes continue.
3. `DROP TRIGGER` on `tests`, `questions` and `question_groups`, then `app.fill_owner_from_created_by()`;
   on `media_assets`, then `app.fill_owner_from_uploaded_by()`; on `classes`, then
   `app.classes_fill_teacher()`. A trigger goes before its function, and there is no `IF EXISTS`
   anywhere, so a wrong name fails the migration instead of leaving an object.

The locks, read from `pg_locks` after each phase on a migrated scratch database: the check holds
ACCESS SHARE on the five tables, the validations add SHARE UPDATE EXCLUSIVE, and the drops add
ACCESS EXCLUSIVE, all released at commit. The drops come last and touch the catalog only, so the
window in which a table refuses a read or a write is milliseconds at any size. What can wait in
it is a statement that arrives, or a transaction that is open, at that moment: a query already
running on a table delays the lock request, and every later query on the table queues behind the
request.

The migration stops in three ways, and each rolls everything back and leaves the schema, the
version row and the v0.9.0 binary as they were; the release stops, and a redeploy retries:

- `23502`, from the check in step 1, naming the table that holds a row without an owner.
- `55P03`, when `lock_timeout` turns a request that does not arrive within five seconds into a
  failed migration.
- `40P01`, when the deadlock detector ends the migration. A transaction of the previous binary that
  touches two of the five tables in the opposite order to the migration, and is open at that
  millisecond, deadlocks with it, and the detector ends one of the two after `deadlock_timeout`
  (1 s): the one whose wait the check reaches first, which the release does not choose. The other
  transaction can be the victim, and one v0.9.0 request then fails with a server error while the
  migration proceeds. The rollout notes
  (`docs/setup/deploy.md`, "Rolling out v0.10.0") say what to do in each case.

The previous binary is safe on both sides of the migration. Its seven `INSERT` statements name the owner
(`classes/repositories/postgres.go`, `media/repositories/store.go`, `questions/repositories/store.go`,
`tests/repositories/{store,duplicate,group_create,version_draft}.go`; a grep of the `v0.9.0` tag finds
no other `INSERT` into the five tables outside `seed/` and tests), so the triggers it never needed
going changes nothing for it, and a rollback to it after 00106 is safe. v0.7.0 does need them: it
inserts a test without `owner_id`, which now fails with `23502`. Roll forward, not back past v0.8.0.

Down recreates the three functions and the five triggers byte for byte as 00060, 00063 and 00065
created them (a test compares `pg_get_functiondef`, `pg_get_triggerdef`, owner and ACL before 00106
and after Down). The constraints stay validated, because Down never weakens a valid `NOT NULL`.
Nothing runs it in production.

Tests, in `platform/db/tests/contract_r3_ownership_test.go` and `constraints_test.go`: what is gone
and what is kept; a row without an owner, in each of the five tables, stops the migration naming
that table and leaves everything as it was; a held lock makes it give up with `55P03`; the previous
binary's inserts, reads and class inserts run through the migration without an error; Down restores
the objects and a bare insert is filled again; validating blocks neither a read nor a write; an
insert that omits an owner is refused; and the not-validated constraints in `app` equal an explicit
list, which is empty once 00105 and 00106 have both run. A later expand step adds its constraint
to that list and its contract step removes it, so a forgotten `NOT VALID` fails CI.

## 44. Students can pause (T-D4.13)

Four migrations add the pause rule of spec §11.1, one column and one concern each.
Every Down drops its column, and the two constraints with theirs.

```sql
ALTER TABLE app.questions
  ADD COLUMN audio_allow_pause boolean
    CONSTRAINT questions_allow_pause_only_with_audio
    CHECK (audio_allow_pause IS NULL OR media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind);
-- the same on app.test_version_questions as tvq_allow_pause_only_with_audio

ALTER TABLE app.group_recordings ADD COLUMN allow_pause boolean NOT NULL DEFAULT true;
ALTER TABLE app.test_version_group_recordings ADD COLUMN allow_pause boolean NOT NULL DEFAULT true;
```

**Why two are nullable.** The previous binary inserts into both question tables while a
release rolls out, and its insert names no such column. A nullable column with no default
takes that insert unchanged, and a question or a version from before the control reads
`COALESCE(audio_allow_pause, true)`, which is what every audio question did. The existing
biconditional `questions_audio_policy_iff_audio` is not widened to this column: the previous
binary writes no value, and a rule that required one beside every audio policy would refuse its
rows. The new CHECKs are the one-way rule `questions_max_plays_only_with_audio` already uses,
with `IS NOT DISTINCT FROM`, because `media_asset_kind` is NULL on a question with no media.
**Why two are not.** A recording is a whole policy (`allow_seek` and
`show_transcript_after_submit` are NOT NULL columns), so there is no absent case to model, and a
constant default is stored without a rewrite.

**A save that leaves the field out keeps the value.** `updateQuestion` and `updateQuestionGroup`
replace the whole object, and three clients send no `allowPause`: a tab loaded before the
release, whose `z.object` parsers strip a key they do not name; that tab's device-recovery
draft; and every tab until T-D4.15 teaches those parsers the field. So the store keeps the
stored value on an update, for a question by its id and for a group's member by its id in the
bundle, in one statement, `audio_allow_pause = CASE WHEN <audio present> THEN
coalesce($new, audio_allow_pause, true) END`; and the group update reads the group's recordings'
`allow_pause` by id before it clears and rewrites them, which is the same rule for a recording.
A policy new to a question (it had none, or an image) and a recording under a new id are
creates, and read true. A create stores true.

Publish copies the bank's `audio_allow_pause` as it is into `test_version_questions` (a NULL
stays NULL and reads true) and a recording's `allow_pause` into
`test_version_group_recordings`, in the statements that already freeze the question and the
recording. The draft read, the lock order and the `media.LockForVersionUse` and
`questions.LockForDraftUse` calls are unchanged. Restore as draft copies the frozen value back,
and duplicating a test or copying a group carries the draft's through the bundle. The version
preview, the student's paper and the teacher's review read `COALESCE(…, true)`; the version
diff puts the value in a question's `media` part and in a group's recording print, so a changed
rule is a `changed` entry, and an unset rule and true are the same rule there.

Tests: `tests/application/tests/allow_pause_test.go` (a freeze that later edits do not reach, a
version from before the column, a save that leaves the field out, a create that leaves it out, a
recording replaced by a new id, and copy, duplicate and restore), the attempts and HTTP tests of
the student and teacher answers, and the diff cases in `tests/domain/tests/diff_test.go`.

## 45. A file that left the library (F-68)

The owner's decision of 2026-10-12. A media file leaves the library when it is replaced
(`replaced_by`) or deleted (`deleted_at`). Until F-68 its object stayed in storage for ever and
the file stopped counting toward its owner's quota at once, even while a published version
still served it. Now the file counts while anything names it, and its object is removed seven
days after nothing does.

**Three states.** A file is in the library (`deleted_at` and `replaced_by` NULL), or it is
retired (one of them set, `purged_at` NULL), or it is purged (`purged_at` set: its object is
gone and the row stays). A retired file is held while a row names it and pending while none
does. `media/repositories/retention.go` holds the two predicates, `retiredFile` and
`namedFile`, and every query below is built on them.

**What names a file.** A row of any of the six tables whose foreign key to `media_assets` is
`ON DELETE RESTRICT`: a question that is not deleted (`questions.media_asset_id`), a group's
recording or material (`group_recordings`, `group_stimulus_assets`, archived groups and the
drafts of deleted tests included, as `GroupReferences` counts them for the delete check), and a
frozen version's question, recording or material (`test_version_questions`,
`test_version_group_recordings`, `test_version_group_assets`). Attempts, results and grading
read an asset only through those three version tables, so a version that holds a file holds it
for them. A deleted question names nothing: a deleted bank question would otherwise pin a file
for ever. An import review draft names a file in JSON, not in a row, so it does not hold one;
a draft that still names a file the sweep removed fails at its commit as one that is gone.

**Columns (00111).** `unreferenced_at` is when the file was last seen named by nothing.
`replaceMedia` and `deleteMedia` write it in their transaction after the write that retires the
file, when nothing names it (`markRetired`), and leave it NULL when something does. The sweep
writes it for a file whose last name went afterwards, which no write announces, so the wait
starts within one sweep period of the truth (an hour). A binding of the file clears it (00113),
and the sweep clears it for a file it finds named. `purged_at` is when the sweep removed the
object. `media_assets_purged_check` says a purged file was unreferenced first. 00112 is the
partial index of the retired files, which the usage figures and the sweep read. A file that
left the library before F-68 has no date: the first sweep dates it, and its object goes seven
days after that sweep.

**The quota (`repositories.CheckQuota`).** One statement sums the owner's files that are in the
library or retired and named, under the same advisory lock `(73820, owner)` (§28). An upload
checks before it inserts with `Add` the new bytes. A replacement takes the lock, inserts the
new file, repoints the editable references and marks the old one, and checks last with `Add`
zero, so the sum is the state after the replacement: the old file counts exactly when a row
still names it after the repoint, and the check needs no "leave this file out" argument
(`QuotaCheck.LeaveOut` is gone). A replacement that published versions hold needs room for
both files. A refusal rolls everything back, and the command deletes the fresh object as it
does for every other refusal.

**Usage.** `Usage` adds `HeldBytes` (retired and named, counted toward the quota),
`PendingBytes` (retired and unnamed, counted toward nothing) and `PendingDeleteAt` (the earliest
`unreferenced_at` of those plus seven days; a file the sweep has not dated yet counts as dated
now). The figures follow the scope as the library figures do, so the teacher workspace passes
`.Own()` and `scope.all` sums every owner.

**The sweep (`command.Sweep`, `jobs.SweepRetiredMedia`).** The API runs it at start-up and
every hour, five minutes at most, and ten minutes after a failure or a file it could not
remove, on every machine and only where media storage is configured. One pass:

1. `Observe` dates the retired files nothing names and clears the date of those named again,
   in two statements.
2. `Due` lists the retired files dated at or before now minus seven days, in id order, a
   hundred at a time.
3. `Purge` takes one file in its own transaction. `SELECT … FOR UPDATE SKIP LOCKED` takes the
   row, so a file that a second sweep or a writer holds is skipped. Under the lock it checks
   again that the file is due and unnamed (a file named again has its date cleared and is left),
   deletes the object, sets `purged_at` and writes the audit row `media.purged` with no actor.
   Once the delete has succeeded, the mark, the audit row and the commit run under their own ten
   seconds and not under the sweep's budget, so a sweep whose time ends at that moment still
   records the removal; if they still fail, `Purge` answers a `PurgeMarkError` and the job logs
   the file's id at ERROR.
   The object goes before the row is marked: a failure of the delete, of the mark or of the
   commit leaves the row for the next pass, whose delete of an object already gone succeeds
   (`storage.Client.Delete` treats `NoSuchKey` as success). The delete has thirty seconds, so a
   stalled storage cannot hold a row lock for long.

**Why it cannot race a writer.** A writer that freezes or binds a file takes the row first
(`LockForVersionUse`, which answers not found for a purged file, as it does for a deleted one),
then writes. Not every writer takes it: a bank question's write checks `Readable` and inserts,
and the foreign key's share lock does not conflict with the sweep's update of an unrelated
column. 00113 closes that for every writer at once. Each of the six tables has a
`BEFORE INSERT OR UPDATE OF media_asset_id` trigger that, for a changed non-null binding,
updates the file's row to clear `unreferenced_at` and then refuses the write with
`check_violation` (constraint `media_assets_purged_binding`) if the file is purged. The update
takes the row lock when the file is waiting for removal, so the writer waits for a sweep that
holds it and then sees `purged_at`; a writer that comes first holds the lock and clears the date,
and the sweep skips the row and finds it named. A file that is not waiting is not locked.
`Readable`, `Get`, `LockForVersionUse` and `SoftDelete` treat a purged file as gone, and the
library predicate excludes it, so a replaced file cannot reappear if its successor's row is
removed (`replaced_by` is `ON DELETE SET NULL`).

**The switch.** `MEDIA_RETENTION_SWEEP` (`on` by default, or `off`; anything else stops the
start-up) is read where `core.Serve` starts the sweep. Off starts nothing and logs a warning;
quota counting and the dating by replace and delete are not behind it. A bank question's write
that names a file the sweep removed in the instant between its `Readable` check and its insert
meets the trigger, which the questions repository maps to the missing-file refusal.

**Tests.** `media/application/tests/retention_test.go` (the quota with a version holding the old
file, the dating by a replace, a delete and the sweep, the seven days and not before, a publish
and a writer racing the sweep in both orders, two sweeps, a failed delete, a purged file read by
nobody), `retention_storage_test.go` (MinIO), `sweep_test.go`, `core/jobs/tests/media_retention_test.go`
and `tests/media_retention_flow_test.go` (the answers over HTTP). Each race test fails when the
lock, the skip or the trigger is taken out.

**Rollout.** Expand only. The columns are nullable and the trigger does nothing until a file is
purged, so v0.10.0 runs on the new schema. A file v0.10.0 replaced or deleted is dated by the
first sweep of the new binary and removed seven days later, so the first object is removed
seven days after the new binary starts. Down drops the triggers, the function, the index and the columns; nothing
restores an object a sweep removed.

## 46. The AI assistant's credits (T-D4.31)

Six migrations (`00118` to `00123`, one table each, none `-- +goose NO TRANSACTION`, all on new
empty tables, so the previous binary is unaffected) hold the budget of the AI assistant that D19
to D22 approved. A credit is $0.001 of the provider's list price; the centre has a pool of
200,000 a month ($200), every teacher an allowance under it (20,000 by default), and a call
reserves its ceiling before it reaches the provider and settles what the provider metered after.

**The hard stop is two constraints.** `ai_credit_periods_within_pool` and
`ai_teacher_credits_within_allowance` each say `used + reserved <= total`, and every credit that
moves does it in an `UPDATE` of those rows, so the database refuses the call that would overspend
and no code path has to remember to. `Reserve` takes the month's row and then the teacher's
(`FOR NO KEY UPDATE`, which does not block the foreign keys that point at them), adds the ceiling
to both `reserved_credits`, and inserts the reservation. A check violation is mapped from its
constraint's name: `ai_teacher_credits_within_allowance` is the teacher's refusal and
`ai_credit_periods_within_pool` the pool's, and the teacher's is tried first so a teacher who is
out of credits is told so when the pool is short too. A paused teacher is refused by an unlocked
read before the transaction begins, and again under the lock.

**One lock order, everywhere.** The month's pool row, then the teacher's row, then the
reservation. `Settle` and `Release` lock both parents before they delete the reservation, and the
sweep selects whose they are, locks the pools and then the teachers in key order, and only then
deletes and returns in one statement. The admin writes that touch one teacher's row take only
that row; `SetRules` locks `ai_settings` first, which a first use also takes `FOR SHARE` while it
creates the rows from it. No path takes a teacher's row and then the pool's, so no two can
deadlock. `TestTwoConcurrentReservationsAgainstAPoolWithRoomForOneLeaveExactlyOne` runs the race
twenty-five times.

**A settlement cannot fail on the pool's constraint.** `Settle` charges `min(metered, reserved)`.
Moving `reserved` down by the reservation and `used` up by no more than that can only lower what
the constraints add up, so a call the provider has already been paid for is never refused; a call
that cost more than its ceiling is logged, and the ledger keeps the provider's real token counts.
A settlement for a reservation the sweep already returned writes nothing and answers
`ErrReservationGone`: the sweep takes only reservations older than ten minutes and a call lasts at
most 120 seconds.

**Credits are integer arithmetic.** `credits = ceil((input x p_in + output x p_out + cache_read x
p_cache + cache_write x p_in) / 1e9)` with the prices in millionths of a dollar per million tokens
(`config.MicroUSD`), so no call is rounded across a credit by floating point. The configuration
has no price for a cache write; the client does not ask for caching today, and a write is charged
at the input price, which never undercounts by more than the provider's write premium.

**The allowance follows the person, the grant follows the month.** `custom` and `paused` are
copied from the teacher's latest earlier row when a month's first row is created (the lookup is
`ai_teacher_credits_user_idx`), so an Admin's pause or custom figure outlives the first of the
month. `granted_credits` is what approved requests added to this month's allowance; it is part of
`allowance_credits`, is not carried over, and survives a change of the default rule. A custom
allowance is stored as the figure given plus the month's grants. `SetRules` rewrites the month's
rows that are not custom to `greatest(default + granted, used + reserved)`, so lowering the
default never breaks the constraint or takes back a credit that was spent; `SetAllowance` for one
teacher refuses a figure below the use with `ErrAllowanceBelowUse` instead of raising it.

**The warning is raised once.** `warned_at` is written in the `UPDATE` that records the use, in
the same statement that tests `used x 100 >= allowance x warn_percent`, and `old.warned_at IS NULL
AND new.warned_at IS NOT NULL` in its `RETURNING` says this call crossed it. The notification is
sent after the settlement commits, through `cqrs.Announce`, so a failure to tell never fails the
settlement.

**Who is a teacher here.** The Admin's page lists every active user whose role is the Admin
(the wildcard) or holds `ai.use`, and every user with a row that month. An allowance, a pause or a
request needs an active user who is not in `app.student_like_roles`. The managers a request tells
are the active users of the Admin role and of any role that holds `ai.credits.manage`. Both keys
are inserted by T-D4.32's migration; until it runs only the Admin matches, which is what the
wildcard means.

**Every Admin write is one statement with its audit row.** `ai.rules.updated`,
`ai.credits.added`, `ai.allowance.updated`, `ai.credit_request.approved` and
`ai.credit_request.declined` are written by a data-modifying CTE that feeds `audit_log` from the
changed row's `old` and `new`, so the diff cannot disagree with the change. A command run from
`cmd/maintenance ai-credits` is System: `actor_user_id` is NULL and the diff carries `reason:
"maintenance"` and the operator. The console's dry run runs the same handlers inside a
transaction it rolls back, with no notifier, so nothing is written and nobody is told.

**Privileges.** The app role has full DML on the five tables that hold balances and requests
(the default privileges of `00009`) and only `INSERT` and `SELECT` on `ai_usage_ledger`;
`privileges_test.go` lists the ledger beside `audit_log` and `attempt_events`. `cmd/maintenance`
runs as the owner or maintenance credential `MAINTENANCE_DATABASE_URL` names. When R5's
`grant_maintenance_role` moves it to `quizzivy_maintenance`, `ai-credits` and
`ai-reservations-sweep` need `SELECT`, `INSERT` and `UPDATE` on `ai_credit_periods` and
`ai_teacher_credits`, `SELECT` and `UPDATE` on `ai_settings` and `ai_credit_requests`, `SELECT` and
`DELETE` on `ai_reservations`, `INSERT` and `UPDATE` on `notifications` (the upsert that tells a
teacher), `INSERT` on `audit_log`, and `SELECT` on the users, roles and grants they read; they
never write the ledger.

**Down.** Each file drops its table; the order is the reverse of the up. A Down loses the month's
balances, the ledger and the requests, and nothing else references them.

## 47. Reading a scan with AI (T-D4.33)

One migration, `00125_add_word_imports_read_with_ai.sql`, adds `word_imports.read_with_ai boolean NOT
NULL DEFAULT false`. The default is a constant, which PostgreSQL 18 records in the catalog without
rewriting the table, and the v0.10.x binary inserts imports without naming the column, so they read
as false and the old binary is unaffected. The column belongs to the import, not to a run: a teacher
chooses it when creating the import (`createWordImport.readWithAi`), it is fixed from then on, and a
replay of the create with a different value conflicts (`IMPORT_CONFLICT`) as a changed title does.
The worker reads it through `Sources`, which sets `Source.ReadWithAi` on every source of the run.
There is no index and no check: nothing filters on it. The Down drops the column and loses which
imports asked for AI, and nothing else.

## 48. Moving a question across the group boundary (E1)

`POST /teacher/tests/{id}/questions/move` (`moveTestQuestion`, spec §7.3 and §15) puts a
standalone question of a test's draft into a shared-context group, takes a member out of its group,
and moves a member to another group of the same test or to another place in its own group.

**No schema change in v0.11.6.** The release had no migration, no new constraint or index and no
grant (v0.11.7 added one column and its index, D-50, under "After release" at the end of this section).
Every table and column the move writes existed: `questions`, `test_section_units`, `test_section_questions`,
`question_groups`, `tests` and `audit_log`. The writes hit the primary keys, the three ordinal
constraints below and `questions_context_complete`, and the gap check hits
`group_gap_bindings_question_idx`. So nothing is expanded or contracted, the up/down/up round trip is
unchanged, and the §12 register needs no row: no table deviates. During a rolling deploy an old pod
answers the route 404, and every row the new binary writes is a row the old one already reads, because
a draft after a move is the same tables in the same shapes: standalone units and members of a group,
and a member that left its group is a row with the all-null tuple the check always allowed.

**The statements.** One transaction, `GroupsPostgres.MoveQuestion`, in three cases:

- **Into a group.** `DELETE` the section's `test_section_units` and `test_section_questions` rows for
  the bank question, renumber the section's units and recompute the standalone ordinals from them,
  then insert a new `questions` row with `context_group_id`, `context_ordinal` and
  `context_option_order = 'shuffle'` (through the questions module's `CreateGroupMember`, with new
  question, option and blank ids), and renumber the group's members so the copy sits at `index`. The
  bank row is read and never written.
- **Out of a group.** (From v0.11.7 an unedited copy of a bank question that is still in the bank
  takes the path under "After release" instead.) One `UPDATE app.questions SET context_group_id = NULL,
  context_ordinal = NULL, context_option_order = NULL, copied_from_id = NULL, owner_id = <the test's owner>
  WHERE id = $1 AND context_group_id = $2`,
  so the check sees the all-null tuple at once; renumber the source group's members; shift the
  section's units at and after `index` up by one and insert the question's `test_section_units` and
  `test_section_questions` rows, then recompute the standalone ordinals. The group stays when it
  empties.
- **Between groups, or within one.** One `UPDATE` of `context_group_id` and `context_ordinal` for a
  member that changes group; then a `row_number()`-style renumber of each touched group's members
  (`unnest` of the ids and their new ordinals), as `RemoveFromSection` renumbers a section.

Then, for every case: `UPDATE app.question_groups SET revision = revision + 1` on each touched group,
`UPDATE app.tests SET updated_at`, and the audit rows, `test.question_moved` (entity `test`, a diff of
the question, both places, the index and whether it was copied) and one `question_group.updated` per
touched group. The diff holds only facts the transaction already knows under its locks, so it needs no
data-modifying CTE (AGENTS.md). No `test_version_*` row is read or written.

**The lock order** is §19's, test, then group, then members, then media, with the question rows in
ascending id order:

1. `checkVersion`: `app.tests … FOR UPDATE`, the revision and the caller's scope; then the archived
   check and the permission. This is the parent lock publish, the outline autosave, every group write
   and "Remove group" take first, so the move serialises against all of them.
2. The source, the destination and every group the request lists are resolved inside this test, so a
   foreign or missing id is `ErrNotFound` before any revision is compared.
3. The touched groups `FOR UPDATE`, in id order, each revision compared with the request's.
4. The touched groups' members `FOR UPDATE`, in id order (`lockGroupMembers`).
5. `ensureSectionUnits` on the section whose units the move rewrites (the source section of a
   copy-in, the destination of a move-out). A section that has no `test_section_units` yet is seeded first, and
   seeding takes `questions.LockForDraftUse` on every question of the section in id order. It comes
   before the next lock so that a move never holds one question while it waits on an earlier one: the
   first build locked the moved question and then seeded, and deadlocked (`40P01`) against another
   test's autosave that shares both questions.
6. `lockBankQuestion`, which is `questions.LockForDraftUse` on the bank row. On a copy-in it comes
   before the row is read, and it is the lock `SoftDelete` contends on. On a move-out it comes after
   the `UPDATE`, before the insert into `test_section_questions` that AGENTS.md requires it for: the
   row lock is held already from step 4, and this re-asserts that the question is not deleted.
7. Copy-in only: `lockGroupAssets` on the copied question's media, the row locks of
   `media.LockForVersionUse` in id order and the caller's read check, as a group's create and update
   take them. A media soft delete counts owned-member references under the asset row lock (§19), so the
   member insert must hold it.
8. `SET CONSTRAINTS` on the three ordinal constraints, `DEFERRED`; then the writes above.
9. After the writes, the same constraints and `group_gap_bindings_member` are set `IMMEDIATE`, so a
   violation surfaces inside the move; `readGraph` reads every touched group back and runs
   `Bundle.Validate`, which is where the destination's 200 members, 4 MiB budget, audio scope and asset
   rules refuse a move; then the test is read back and the transaction commits. Any failure rolls
   everything back.

The gap check comes before any write: a move-out or a move to another group reads
`group_gap_bindings` for the question and refuses with `group_gap_bound`. A reorder within the same
group does not ask, since a gap binds a member to its group, not to a place in it. The member count
of a full destination is checked before the insert, so a full group answers `group_limits` before
`questions_context_complete` (`context_ordinal BETWEEN 0 AND 199`) can fire.

**The deferred constraints.** `questions_context_ordinal_key` (`context_group_id`, `context_ordinal`),
`test_section_units_ordinal_key` and `test_section_questions_ordinal_key` (`test_section_id`,
`ordinal`) are `DEFERRABLE INITIALLY IMMEDIATE`, and the move defers them because it renumbers a
whole list inside one statement sequence, when two rows briefly share an ordinal. The existing
writers defer the same three (`mixed_outline.go`, `group_update.go`). `group_gap_bindings_member`, the
foreign key from a gap to its member's `(context_group_id, id)`, is `DEFERRABLE INITIALLY DEFERRED`.
It would refuse the commit of a bound member's leaving anyway; the explicit check gives the teacher a
message, and the move sets it immediate before the read-back so the refusal belongs to the move.

**Why a copy on the way in, and the same id on the way out.**

- **In: a copy.** A bank question is referenced, not owned, by a draft (D-14). Other drafts may
  reference the same row (`DraftReferences`). Moving the same id into a group would turn their
  reference into a member: their next autosave would fail `LockForDraftUse`, which filters
  `context_group_id IS NULL`, with "a question in the test has been deleted", and a later "Remove
  group" would hard-delete a row their `test_section_questions` foreign key still restricts
  (`group_reference`). The bank list is defined by `context_group_id IS NULL`
  (`questions_owner_bank_idx` is partial on it), so the same id would also vanish from the bank
  without a delete, and the data could not tell a bank question from a test-only one. Published
  versions and grading history do not care: `attempt_answers` references `test_version_questions`,
  and `source_question_id` is `ON DELETE SET NULL` (D-07). The snapshot canary edits a bank question
  after publish, and a copy leaves that row alone. It is also the rule a group already follows: the
  builder inserts bank groups as complete independent copies, and "no editable source FK is retained"
  (§19).
- **Out: the same id** (v0.11.7: unless it is an unedited copy of a bank question that is still there,
  which goes back as that question). Nothing but its group can reference a member, so there is no second reader to
  protect, and a copy would only churn ids. The tuple goes to NULL in one statement. The row's
  `owner_id` is the test's owner by the group's ownership, but a member written before ownership
  existed carries an `owner_id` backfilled from `created_by` (00061, 00062), so the statement names
  the test's owner explicitly: the question comes out owned by the test's owner whoever created it,
  and then lists, opens, autosaves and soft-deletes like any bank question of that teacher.
- **Between groups: in place.** The same row changes `context_group_id`; only the group's ordinals and
  both revisions move.

**Tests of v0.11.6.** `tests/application/tests/group_move_*_test.go`, integration, on PostgreSQL 18: the three
cases and the clamp of `index` (`TestMoveInCopiesAndLeavesTheBankQuestion`,
`TestMoveOutKeepsTheIdAndReturnsItToTheBank`, `TestMoveBetweenGroupsBumpsBothRevisions`,
`TestMoveClampsIndexAndCompactsOrdinals`), the refusals writing nothing (`TestMoveRefusalsWriteNothing`
and the gap-bound case), the published versions untouched row by row across thirteen tables
(`TestMoveLeavesPublishedVersionsUntouched`), a publish after a move
(`TestPublishAfterMoveFreezesTheNewUnitsAndOrdinals`), the deal (`TestMovedQuestionsDealInsideAndOutsideTheirGroups`),
and the races, which pause one writer inside its lock and watch `pg_blocking_pids` for the other:
against a bank delete, a group save, the outline autosave, publish, a media delete, and another
test's autosave that shares questions (`TestMoveIntoGroupLocksQuestionsInIdOrderAgainstAnotherTestsAutosave`).
`server/tests/group_move_flow_test.go` drives the operation through the real stack.

### After release: the copy remembers its origin (v0.11.7)

The first release left a second bank question behind every round trip: a copy went into a group under
a new id and came out under that id as a bank question of its own, next to the original. v0.11.7
(decided 2026-10-14, `74e-e1-group-move.md` "After release") makes the copy remember where it came
from. It changes this section's consequence, not its decision: a copy is still a copy and the bank row
is still never written by a move into a group.

**The column** is `questions.copied_from_id uuid`, nullable, `REFERENCES questions (id) ON DELETE SET
NULL` (00126, D-50), with the partial index `questions_copied_from_idx` on the copies (00127). It
follows `test_version_questions.source_question_id` (D-07): informational, never a reason to refuse
anything. Nothing writes it but a copy into a group: `CreateGroupMember` takes it in the member's
`GroupOwnership` and inserts it with the row; a group save (`UpdateGroupMember`) never touches it; a
move between groups or within one leaves it; a member that leaves its group sets it NULL in the same
`UPDATE` that clears its tuple, because it is then an independent question. A copy the v0.11.6 binary
wrote has NULL and is never restored.

**What decides a restore.** On a move out of a group, after the gap check, all of these must hold, or the
member leaves as before:

1. the member has `copied_from_id`;
2. the origin is a live bank question: `questions.LockForDraftUse` finds it (not deleted, not a member),
   and the caller's scope reaches it (`BankQuestion`, as the copy-in reads its source);
3. the test's owner owns it (`NotOwnedBy`), the invariant the outline writer holds for every standalone
   question, so the next autosave accepts it;
4. no section of this test already holds it, so a restore never puts one question in a test twice (the
   unique key of `test_section_units` would refuse it within a section);
5. the member's content equals the origin's.

The answer says which: `restored` when all hold; `editedCopy` when 1 to 4 hold and 5 does not (the
builder tells the teacher the question was saved as a new one because it had been edited); neither when
1 to 4 fail, as there is no edit to report for a question that was never a copy or whose original is
gone. A concurrent delete of the origin is the second case of 2: `LockForDraftUse` waits for the
deleting transaction and then finds the row deleted, so the move leaves the member as a bank question
and does not restore a deleted row; if the move holds the origin's lock first, the delete finds the draft
referencing the question again and answers 409.

**Content** is `GroupQuestion.SameContent` in the tests domain, one function. It puts both questions
through `GroupQuestion.Copy` with the same fixed identities and compares what comes out, so what `Copy`
remaps (the question, option, blank and rich-prompt gap ids) cannot make an unedited copy differ from its
source, and the comparison is by position, not by id. It compares the type, level, skill, prompt and its
rich document, points, explanation and its document, sample answer, media asset and alt text, audio
policy (plays, seeking, transcript reveal, pausing) and transcript, tags (as a set), the options with
their rich content and correctness, in order, and the blanks with their ordinals, case sensitivity,
gap bindings and accepted answers (as a set). It leaves out ids, ordinals, `context_option_order` (a
group's choice, not the question's), the owner and the timestamps. Rich documents are compared as JSON
values, so whitespace and a stored `null` are not edits. There is no AI-draft flag on a question in
storage or in the contract, so there is nothing of that kind to decide. A test lists every field of the
input types and fails on a new one, so that a field added later is compared or declared an identity on
purpose, and a restore can never discard an edit to it. The diff's `questionParts` is a different
comparison for a different job: it works on frozen shapes and leaves out tags, level and skill, because
a student does not see them.

**Removing the copy** is a hard `DELETE FROM app.questions WHERE id = $1 AND context_group_id = $2`,
the statement a group save uses for a member it drops and "Remove group" for all of them. A member cannot
be soft-deleted: `questions_context_complete` requires `deleted_at IS NULL` while the tuple is set. What
can reference a member row, from the foreign keys on `questions (id)`: its options and blanks
(`ON DELETE CASCADE`, children); `test_version_questions.source_question_id` (`ON DELETE SET NULL`, D-07),
which is how a published version froze it; `test_section_questions` and `test_section_units`
(`RESTRICT`), which a member is never in; and `group_gap_bindings`, which the gap check refuses before
anything is written. Attempts and grading reference the version's frozen rows, so a published version
and an attempt on it keep working: they keep every frozen row, its options, blanks and answers, and
lose only the informational link of the copy's frozen question, which is NULL, as it already is when a
group save drops a member. A version diff against the draft then reads the original as added and the
frozen copy as removed, which is what it is.

**The lock order for a move out** is the list above through step 4, then:

1. the gap check and a read of the member's `copied_from_id`, under the member's lock from step 4;
2. `ensureSectionUnits` on the destination. When it has to seed, it takes `LockForDraftUse` in **one
   ascending sweep** over the section's questions and the origin together (an origin that is gone is
   skipped there and judged in the next step). The origin is outside the section, so locking it
   after the sweep would hold a later id while waiting on an earlier one, the order that deadlocked
   the first copy-in (`TestRestoreLocksQuestionsInIdOrderAgainstAnotherTestsAutosave` reproduces it with
   `40P01` against the unsorted order);
3. `LockForDraftUse` on the origin (a repeat when the sweep took it), then the reads that judge it:
   `BankQuestion`, `NotOwnedBy`, the origin's presence in the test, and the member's content;
4. a restore: `DELETE` the member, renumber the group, `LockForDraftUse` on the origin again directly
   before the inserts into `test_section_units` and `test_section_questions` (AGENTS.md), and insert it at
   `index`. A member that leaves as it is: the `UPDATE`, the renumber, `LockForDraftUse` on the member,
   the inserts.

A copy-in is unchanged: the source section's seeding already locks the moved question in the same
sweep. The audit entry `test.question_moved` carries `restored`, `editedCopy` and `removedMemberId`.

**Tests.** `tests/application/tests/group_move_origin_test.go`: the origin is recorded and kept across
groups; an unedited copy of each kind of question (choice with rich options, true or false, both blank
forms, short answer with an image, a listening question with a policy and a transcript) comes back as
its original, also after a group save that did not touch it; every kind of edit keeps the copy as a new
bank question; a deleted, foreign or already-present original does not fail the move; a gap-bound copy is
refused; a published version keeps every frozen row but the link; the foreign key and the index.
`group_move_origin_race_test.go` drives the races, both orders, against a delete of the origin and
another test's autosave. `tests/group_move_flow_test.go` drives the round trip over HTTP, and
`builder-groups.live.spec.ts` in a browser against the real API.

## 49. The email outbox (T-R5.2)

`app.email_outbox` (00128, D-51) holds every message the app has to send. A module enqueues in its own
transaction with `email.Outbox.Enqueue` and calls `Notify` after the commit; the API's and the worker's
dispatchers send what is due.

**What a row is.** `template` (a slug, the list is Go's), `locale` (`vi` or `en`; the recipient's
`users.locale`, else `vi`, unless the caller names one), `to_address` (one bare address, 3 to 254
characters, copied), `to_user_id` (`ON DELETE SET NULL`), `params` (a JSON object of at most 4096 bytes,
each template's own type, decoded strictly), `secret_ciphertext` (the AES-256-GCM seal of the one secret a
link carries, under the `email-outbox` key derived from `SECRETS_KEY`, with the row id as associated
data, so a ciphertext copied to another row does not open), `dedupe_key` (unique), `deliver_after`,
`attempts` (0 to 10), `last_error_code`, `provider_id`, and the two outcomes `sent_at` and `failed_at`,
which a check keeps from both being set. No sentence is stored: the subject, text and HTML are rendered
when the row is sent, from the template, the params, the organization's short name and the locale.

**Indexes.** `email_outbox_pending_idx (deliver_after) WHERE sent_at IS NULL AND failed_at IS NULL` is
the dispatcher's claim and the earliest-due read its timer waits on; it holds the queue only.
`email_outbox_sent_idx (sent_at) WHERE sent_at IS NOT NULL` serves the day's budget count.
`email_outbox_to_user_idx` serves the cascade from `users`.

**The dispatcher.** One batch at a time across both processes, under `pg_try_advisory_xact_lock(73819,
43)` (§28): it counts the sends since local midnight in `APP_TIMEZONE`; at or over `EMAIL_DAILY_LIMIT` it
moves every due row to the next local midnight, otherwise it claims up to 10 due rows `FOR UPDATE SKIP
LOCKED`, sends them `EMAIL_SEND_RATE` a second apart, and records each outcome in the same transaction. A
sent row and a failed row lose `secret_ciphertext` in that statement. A transient failure backs off 1, 5
and 15 minutes and then hourly; the tenth attempt, or any permanent failure, sets `failed_at`. A row whose
secret cannot be opened, whose params no longer decode (a wrong type; a field the binary does not know is
ignored) or which no longer renders fails closed with `secret_unreadable`, `params_unreadable` or
`render_failed`. A row of a template the binary does not send is not its row: the claim, the day's
deferral and the timer all skip it, so it stays pending, secret and attempts intact, for the newer binary
the API and the worker run side by side while a deploy rolls out. A release therefore never removes or
renames a template while rows of it can be pending. The timer is armed only for the earliest pending
`deliver_after` of a known template, so an empty outbox never wakes the database.

**Retention.** The daily housekeeping (T-R5.16, `core/jobs.Housekeep`) deletes, oldest first and 500 at a time,
the rows whose `sent_at` or `failed_at` is more than 30 days old (`email.Retention`). A pending row is never
touched, however old. The delete reads the table in order of the outcome's instant without an index: the table
holds a day's budget of mail a day for 30 days, a few thousand rows at most, and a partial index on `failed_at`
would cost every failure a write for a scan that runs once a day. A pruned row frees its `dedupe_key`: the only
producer today keys on the export id, which never comes round again, but a producer whose key can repeat after 30
days would send again. The app role has full DML on the table (it is not append-only: it is a queue).

## 50. Activity (T-R5.4, DG-54)

**What is recorded, and when.** A password or Google sign-in stamps `last_sign_in_at`, `last_sign_in_method` and
`last_active_at` in one statement after the session's refresh token is stored. It is not part of that
transaction, which holds the user `FOR SHARE`: two sign-ins of one account would each have to upgrade the
lock to write and would wait for each other. A failed stamp is logged and the sign-in goes on. `changePassword`
sets `password_changed_at` in the statement that sets the hash. A temporary password set by someone else does
not, so a student who has never chosen one reads no "Changed …" line.

**What "active" means.** An authenticated request that day. `adapters.Principals.Resolve`, which the permission
gate and the docs gate both call, tells the identity module's `StampActivity` of each user it resolved and did
not find disabled. The collector lets a user through once every five minutes per instance, and again at the
first request of a new local day, and writes what waits every 30 seconds or at 200 entries, only while any
waits, in one transaction: `last_active_at` raised with `greatest`, in user id order so two instances take row
locks in the same order, and the days with `ON CONFLICT DO NOTHING`. A user deleted in between drops out of both
statements. A failed write is logged and its entries dropped, since activity is a statistic and a queue behind a
database that is down only grows. `updated_at` moves with every `last_active_at` write, because the trigger has
no column list; nothing reads it for a user.

**Retention.** The daily housekeeping (T-R5.16) deletes the days older than 400 days, `domain.ActivityDaysKept`,
counted back from the start of the local day in `APP_TIMEZONE`, 500 rows a batch and oldest day first over
`user_activity_days_day_idx`. R5's readers look back 14 days at most, so nothing the product shows is lost;
`last_active_at` on the user is never pruned.

**Status.** `domain.StatusOf` applies DG-54's order: disabled, must change password, never signed in
(`last_sign_in_at IS NULL`), active. The contract's `AccountStatus` is that value, and `listStudents` filters by
it (`accountStatus`, any subset), because the order makes the Status facet's states disjoint and two flags
cannot express every pair of them. `neverSignedIn` is the plain predicate beside `mustChangePassword`.

## 51. The organization's settings (T-R5.5, T-R5.7)

`00130_create_organization_settings.sql` adds `app.organization_settings`, one row for the whole
installation, keyed `id boolean PRIMARY KEY DEFAULT true CHECK (id)` (D-53). The migration inserts
the row, named `Quizzivy`, and revokes `INSERT` and `DELETE` from `quizzivy_app`; the API only
`UPDATE`s it, under `SELECT … FOR UPDATE`, with the `set_updated_at` trigger.

- **Profile.** `name` (1 to 120 after `btrim`), and nullable `short_name` (1 to 40), `website`
  (≤ 200), `contact_email` (3 to 254), `front_desk_phone` (`^[0-9+ ().-]{6,30}$`),
  `front_desk_hours` (≤ 120) and `address` (≤ 300). An optional text that is empty after trimming
  is stored as `NULL`, never as `''`.
- **Logos.** `logo_sha256`, `logo_type` and the `_dark_` pair. A hash is 32 bytes, a type is
  `image/png` or `image/svg+xml`, and each pair is both NULL or both set. The application writes
  only `image/png`; `image/svg+xml` is a reserved value no code writes (D-019), and a row that held
  it would read as no logo. The bytes are not in
  the database: they are objects at `branding/{sha256}.png`, content-addressed, so a key's
  bytes never change and a new logo is a new URL.
- **Sign-in.** `google_sign_in`, `password_sign_in`, `force_password_change` and
  `join_codes_enabled` are `boolean NOT NULL DEFAULT true`; `allowed_google_domains text[] NOT
  NULL DEFAULT '{}'` holds at most 50 lower-case domains and no NULL element; an empty list allows
  every domain. `CHECK (google_sign_in OR password_sign_in)`.
- **Retention.** `keep_answers_months` (12 or 24, NULL for ever), `keep_focus_events_months` (6, 12,
  13 or 24, default 13) and `anonymise_disabled_after_months` (12 or 24, NULL never). T-R5.17a
  applies them; this table only holds the choice.
- **Revision.** `revision integer NOT NULL DEFAULT 1`. A save names the revision it read; the
  `UPDATE` runs only when it is still the stored one, and `revision` goes up by one. `updated_by`
  is `REFERENCES app.users ON DELETE SET NULL`.

**The lockout judgement** is a query over `app.users`, inside the save's transaction, and writes
nothing. An active account (`disabled_at IS NULL`) has a way in under a proposed policy when
password sign-in is on and `password_hash IS NOT NULL`, or when Google sign-in is on, the server
has Google configured and the account has a `user_identities` row for `google` whose
`email_at_link` is in an allowed domain (any domain when the list is empty). A policy is refused
when the actor is among the accounts left with none, or when every active built-in Admin is.
`email_at_link` is the address at the time of linking, not today's; an Admin whose Google address
changed domain since is counted by the domain they linked with, which is the one sign-in checks.

**Audit.** Every save writes `organization.updated` in the same transaction, with `fields`, the
contract paths of what changed, and the new `revision`. No value is in the diff.

**Cache.** Each server keeps the row in memory for at most 30 seconds, refreshed only when something
asks and single-flight, and replaced at once on the server that saved. Another server therefore
serves the old policy for up to 30 seconds after a change, and an idle server never queries.

**Tests.** `organization/domain/tests/settings_test.go` (the validation table),
`organization/application/tests` (revision conflict, the snapshot's reads and refresh, the
logo-in-store check), `organization/repositories/tests` (the constraints, the lockout outcomes and
counts, the concurrent saves, the audit row, the grants), `organization/http/tests` (the public
document's exact keys, the logo headers and caches, the error mapping).

## 52. Data exports (T-R5.15)

`app.data_exports` (00134, D-54) holds one row for each request for the centre's data. An Admin asks, the
worker builds a ZIP of CSV files into the private import bucket at `exports/{id}.zip`, and the requester
fetches it from an app page, up to three times.

**What a row is.** `requested_by` (`ON DELETE CASCADE`), `include_answers`, `status` (`queued`, `running`,
`ready`, `failed`, `expired`), `attempts` (0 to 3, the claims so far), `storage_key` (1 to 200 characters),
`bytes` (above 0), `error_code` (1 to 64 characters), and the instants `created_at`, `started_at` (the
current claim), `completed_at`, `expires_at` (24 hours after `ready`), `downloaded_at` and `downloads`. `ready` requires
`storage_key`, `bytes` and `expires_at`; `failed` requires `error_code`. A read reports a `ready` row past
`expires_at` as `expired`. The daily housekeeping (T-R5.16) makes it true in the table: it deletes the object,
then sets `status = 'expired'` and `storage_key = NULL` on the row, guarded by `status = 'ready' AND expires_at <=
now()` so a second run changes nothing. The row stays, with `bytes`, `expires_at` and `downloads`, for the audit
trail of who asked.

**One at a time.** `data_exports_active_idx` is a unique index on the constant `true` over the rows
`queued` or `running`. A second request inserts into it, fails with `23505` and answers
`EXPORT_IN_PROGRESS`; a finished or failed export leaves the index at once.

**One an hour (D-021).** In the same transaction, before the insert, the request reads the newest row of
`requested_by` with `status <> 'failed'` and `created_at > now() - 1 hour` on `data_exports_requested_by_idx`;
if there is one it answers `RATE_LIMITED` with the seconds until that row is an hour old. A `failed` row never
counts, and a refused request leaves no row, so neither a `409` nor a `429` spends the hour. Two requests of one
actor at once both pass the read, and the active index lets only one insert.

**The claim.** One transaction first fails the `running` row whose lease (30 minutes from `started_at`) has
lapsed and whose `attempts` has reached 3 (`WORKER_INTERRUPTED`), then takes the oldest `queued` row or
lapsed `running` row `FOR UPDATE SKIP LOCKED`, sets `started_at = now()` and adds one to `attempts`. Complete,
fail and release each name the attempt they hold and change the row only while it is `running` under that
attempt, so a worker whose lease was taken changes nothing. Complete also enqueues `data_export_ready` into
`email_outbox` (§49) in the same transaction.

**The fetch (D-024).** `UPDATE … SET downloaded_at = coalesce(downloaded_at, now()), downloads = downloads + 1
WHERE requested_by = $actor AND status = 'ready' AND expires_at > now() AND downloads < 3 RETURNING storage_key`
is the whole check; it signs the address and audits `data_export.downloaded` before it commits, so a failure to
sign counts nothing. `downloads smallint NOT NULL DEFAULT 0` is checked between 0 and 3, and `downloaded_at` keeps
the first fetch. The server cannot tell whether a transfer finished (the browser reads the object from storage),
so a failed transfer may be tried twice more without building another ZIP. Another admin's export finds no row
for them and answers as a missing one.

**Reading the tables.** The worker reads every file in one `REPEATABLE READ`, read-only transaction with a
20-minute statement timeout. Each file's columns are listed in `exports/domain/files.go` and the snapshot
fails a query whose columns differ, so a new column on a table never reaches a file unnoticed. A time is
written as ISO 8601 in UTC and a boolean as `true` or `false`; NULL is an empty cell.

**Tests.** `exports/domain/tests` (the catalogue, vi and en for every column, no secret column names, the
README), `exports/application/worker/tests` (the flow with fakes), `exports/repositories/tests` (the index,
the claim and its races, the lease, the three fetches, the audit rows, the notice, every query's columns),
`server/tests/data_export_flow_test.go` (API, worker and MinIO), `data_export_heap_test.go` (50,000 attempts).

## 53. Set-password links (T-R5.3, D15, DG-05)

`00135_create_set_password_tokens.sql` adds `app.set_password_tokens` (D-55). A link is the way an account
chooses a password without anyone typing one for it: an invite carries a new account's first password, a reset
any later one. The token is 32 random bytes in unpadded base64url; the row keeps `sha256(token)` and nothing
that could be presented. The raw token lives in two places only, the link in the email and the sealed secret
of the outbox row that carries it, which is cleared when the row is sent or fails (§49).

**States.** A link is *usable* while `used_at` and `revoked_at` are NULL, `now < expires_at` and its account is
not disabled. Every other state is one `domain.ErrSetPasswordLinkInvalid`, and the HTTP layer answers it as one
`410 SET_PASSWORD_LINK_INVALID` with one body, so nothing says which state a link is in. A link is revoked by
issuing another to the same account (`RevokeUnused`), using one (`Claim` revokes the account's other unused
links), or disabling the account: `Students.Update` runs the same `revokeUnusedLinks` in the transaction that
sets `disabled_at`, beside the revoke of the refresh families, and the anonymisation command deletes the rows.
Enabling the account again issues nothing, so a disable then an enable within 24 hours leaves the old link
revoked and an Admin sends a new one through Reset. The lookup's `AccountDisabled` check and the claim's
`disabled_at IS NULL` stay as the second line, for a link issued in the instant before a disable.

**Issuing.** `IssueSetPasswordLink` is a command other commands call with a `domain.LinkTx`, the writes of one
transaction the caller owns: `Recipient`, `RevokeUnused`, `Insert`, `EndSessions`, `Enqueue`, `Audit` and
`AfterCommit`. `repositories.SetPasswordLinks.InTx` begins that transaction for a caller that has none;
`Writes(tx)` returns the same writes over a transaction the caller already holds (T-R5.9a creates the account
in it), and the function that runs what was registered with `AfterCommit`, which the caller runs after its own
commit and never after a rollback. That is where `Principals.Forget` and the dispatcher's wake-up wait, so
neither fires for a transaction that rolled back. `Enqueue` maps the outbox's `ErrNoSecretsKey` to
`domain.ErrSetPasswordUnavailable`, which the HTTP layer answers `501`, as media does without storage. Order
inside the transaction is: read the recipient, revoke unused links, insert the link, (for a reset someone else
asked for) end sessions, enqueue the message, audit.

**Claiming.** `Claim` is one transaction: `UPDATE set_password_tokens SET used_at … WHERE token_hash = $1 AND
used_at IS NULL AND revoked_at IS NULL AND expires_at > $now RETURNING user_id, purpose`; then the account's
password hash, `must_change_password = false`, `password_changed_at` and `session_epoch + 1` (refused, and the
whole claim rolled back, for a disabled account, so a refused claim leaves the link unused); then the account's
other unused links revoked, its refresh tokens revoked, and `user.password_set` written as the account itself
with `{"purpose": …}`. The lock order is the link, then the user, then its refresh tokens, the order issuing
takes them in, so issuing and using links at once cannot deadlock. The caller forgets the principal, reloads the
account and issues the session (`SignInSetPasswordLink`, stamped like any sign-in, §50). A token that is
unknown, used, revoked, expired or for a disabled account is refused by `Find` and `Usable` before the password
is hashed, so an invalid token costs the server no argon2 work.

**Self-service.** `RequestSetPasswordLink` issues a `reset` to the signed-in account with `By` set to itself and
`KeepSessions`, writes `user.password_link_requested` in the same transaction, and ends nothing: the account
asked while signed in on purpose, and using the link ends the other sessions. A Google-only account keeps its
`user_identities` row, since setting a password adds a way in and removes none.

**Not here.** A reset does not clear the account's current password; T-R5.9b's "a link reset clears the
password" belongs to that task.

**Retention.** The daily housekeeping (T-R5.16) deletes a link 30 days after it ended, by being used, revoked or
expiring, whichever came first (`least(used_at, revoked_at, expires_at)`, `domain.SetPasswordLinksKept`), the
earliest first and 500 at a time. A link that can still be used has not ended and is never touched, and a token
whose row is gone is refused as any unknown one is. The batch locks its rows `FOR UPDATE SKIP LOCKED`: a link that
was never revoked is also a target of `RevokeUnused`, and a delete that waited on rows in its own order could
deadlock with it; a locked row is left for the next run. The scan has no index of its own, because the table holds the
links of 30 days; the cascade from `users` keeps using `set_password_tokens_user_idx`. The anonymisation command
deletes a user's links outright (see States), so this sweep is for the links of accounts that live on.

**Tests.** `identity/domain/tests/setpassword_test.go` (token shape, the purposes, the usable states),
`identity/application/tests/set_password_test.go` (every path against fakes: the order of the writes, forgets
after the commit, the policy, a disabled account, an unusable link costs no hash) and
`set_password_db_test.go` (the real repository, the outbox and the app role: hash-only storage, a sealed token,
a reissue revokes, eight concurrent submits set once, expiry at exactly 24 hours, a rolled-back issue leaves no
link, mail or wake-up, issuing and using at once do not deadlock), `identity/http/tests/set_password_test.go`
(byte-identical 410 bodies in both languages, the 200 with its cookie, the 400, 422 and 501 mappings),
`platform/db/tests/set_password_tokens_test.go` (each constraint fires, the cascade and the `SET NULL`) and
`core/router/tests/set_password_limits_test.go` (a class of 40 behind one address, the per-address and
per-token limits, a weak password spends the token's budget, the per-user limit of the request).
`identity/application/tests/prune_set_password_links_test.go` and `prune_set_password_links_db_test.go` (the cutoff
and the limit against a fake store; against Postgres, ten links in every state either side of 30 days, a live link
untouched, the earliest first from rows inserted newest first, the edge to the microsecond, a row another transaction
holds skipped without waiting, and a second run changing nothing).

## 54. User imports (T-R5.14)

`00136_create_user_import_jobs.sql` and `00137_create_user_import_rows.sql` add `app.user_import_jobs` (D-56) and
`app.user_import_rows` (D-57). An Admin uploads a CSV or XLSX file of people; the API reads it, judges every row
against what the Admin may give, and stores the job and the rows that passed in one transaction; the worker then
creates the accounts a few at a time. This section is the API half; the worker and the hand-out sheet are the
second half of the task.

**Judging a file.** The columns are `full_name` and `email` (required) and `role` and `class` (optional), found in
the header whatever their case or order; any other column is ignored. A header that lacks a required column, or
names one twice, is refused as a whole. Every other rule refuses one cell, so one pass shows everything to correct,
and a row with a refused cell is counted once in `invalidCount` and never stored. The cells are composed (NFC) and the
name's whitespace collapsed before they are judged.

| Code | Field | Meaning |
|---|---|---|
| `REQUIRED`, `TOO_LONG`, `INVALID` | `full_name`, `email` | empty; over 200 characters (254 for an address); a control or invisible character, or an address that is not a bare `local@domain` |
| `DUPLICATE_IN_FILE` | `email` | the address, in any letter case, appeared in an earlier row, whether that row was refused or not |
| `UNKNOWN_ROLE`, `AMBIGUOUS_ROLE` | `role` | no role has that name or key; two roles do (a custom role named like a built-in key) |
| `ROLE_NOT_ALLOWED` | `role` | the role holds a permission the importer lacks (the subset rule of §5) |
| `ROLE_NOT_GIVABLE` | `role` | the Assistant, which no one is given. The plan called this `ROLE_NOT_ASSIGNABLE`; the code generator prefixes a constant that two enums share, and the other is an `ErrorCode`, so the row code has its own name |
| `UNKNOWN_CLASS`, `AMBIGUOUS_CLASS` | `class` | no active class has that name; two do |
| `CLASS_NOT_ALLOWED` | `class` | the importer lacks `scope.all`, or the row's role is not a student target |

A role is named by its built-in key (`student`), its name (`Học viên`; the accents count) or a custom role's name,
compared without case or extra whitespace; empty takes the dialog's default, which is the built-in Student or
Teacher role and one the importer must be able to give, or the whole request is refused `403
TARGET_ROLE_NOT_ALLOWED`. An address that already belongs to an account, of any state, is not refused: its row is
stored settled `skipped_exists`, counted as `existingEmails`, and the account is never changed. A file in which no
row would create an account is refused `422 USER_IMPORT_NOTHING_TO_CREATE`.

**Reading a file.** The format is chosen by the file's first four bytes (`PK\x03\x04` is a workbook), never by its
name or the browser's type. The platform's reader (T-R5.13) takes it under its default limits, 5 MiB, 2,000 rows,
20 columns and 1,000 characters a cell; each of its errors has its own code and a vi/en message that says what to
do, and no message carries the reader's error text, since that can hold up to 64 KiB of the file. One file is held
at a time (a one-slot semaphore), a waiting upload that is cancelled leaves it.

**Creating the job.** `createUserImport` re-reads and re-judges the file; the preview is never trusted. A
`requestId` the caller has used returns that job without reading the file. Otherwise the job and the accepted rows
are inserted in one transaction with `ON CONFLICT (created_by, request_id) DO NOTHING`, so two requests at once
store one job, and `user_import.requested` is audited with the counts and the format, never with the file's names or
addresses. Both deliveries need `SECRETS_KEY` (a link by email and a sheet's passwords are sealed); without it, or
without the import worker, the three operations answer `501`. A `sheet` also needs password sign-in on
(`422 PASSWORD_SIGN_IN_DISABLED`).

**Reading a job.** `getUserImport` finds the job by `id` and `created_by`; another user's job is a missing one.
`sheet` is null for an email job and otherwise says whether the sheet can still be downloaded: the job is `done`,
created an account, has not been downloaded and has not passed `sheet_expires_at`.

**Open.** The rows keep full names and addresses for as long as the job exists. T-R5.16's housekeeping should delete
the rows of jobs older than a retention period it names; the job row keeps only counts.

**Tests.** `identity/domain/tests` (the rules table, the sniff, the header, the file name), `identity/application/tests/user_imports_db_test.go`
(preview counts, the stored rows and their order, the audit diff, replay, another user's job, nothing to create, the
subset rule, classes, deliveries, 2,000 rows, one file at a time), `identity/http/tests/user_imports_test.go` (the form,
each reader error's code and both languages, no echo of the file), `core/adapters/tests/user_import_tables_test.go`,
`platform/email/tests/budget_test.go`, `server/tests/user_import_api_flow_test.go` (the real router: 401, 403, 413,
422, 501, the rate limit).
