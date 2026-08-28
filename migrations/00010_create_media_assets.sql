-- +goose Up

-- §11.1's limits live here as CHECKs, not only in the handler. The handler
-- validates first and says so in Vietnamese; these are what make "we validate
-- server-side" true when a code path is added later that forgets.
CREATE TABLE app.media_assets (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  kind              app.media_kind NOT NULL,
  storage_key       text NOT NULL UNIQUE,

  -- An allowlist, matching §11.1's mp3/m4a decision exactly. Adding ogg/wav
  -- later is a one-line migration, which is the right shape for something
  -- §17.3 flags as revisitable.
  mime_type         text NOT NULL CHECK (mime_type IN
                      ('audio/mpeg','audio/mp4','audio/aac',
                       'image/png','image/jpeg','image/webp')),

  -- 10 MB and 5 minutes, spelled out (§11.1).
  bytes             bigint NOT NULL CHECK (bytes > 0 AND bytes <= 10485760),
  duration_ms       integer CHECK (duration_ms IS NULL
                                   OR (duration_ms > 0 AND duration_ms <= 300000)),

  original_filename text NOT NULL CHECK (original_filename <> ''),

  -- [D-06] Plain index, never UNIQUE. §13.3 calls the checksum a dedupe key and
  -- §11.1 says a re-upload never overwrites an existing object; both cannot
  -- hold. Immutability wins, because it is what lets a frozen test version
  -- reference an asset without copying the file. The checksum stays so the
  -- upload UI can WARN and so integrity can be re-verified against R2 -- it
  -- never blocks a write.
  checksum_sha256   bytea NOT NULL CHECK (length(checksum_sha256) = 32),

  -- RESTRICT: an asset referenced by a frozen version must not lose its
  -- provenance because a user row was deleted.
  uploaded_by       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),

  -- Soft delete (§13.2): a published version may still reference this row.
  deleted_at        timestamptz,

  -- [D-05] The composite-FK target. `id` is already the primary key, so this
  -- index is redundant for lookups -- it exists so `questions` and
  -- `test_version_questions` can declare
  --   FOREIGN KEY (media_asset_id, media_asset_kind)
  --     REFERENCES media_assets (id, kind)
  -- which is what lets a CHECK on those tables enforce §7's "audio policy is
  -- present iff media.kind === 'audio'" RELATIONALLY rather than in application
  -- code. Postgres requires a UNIQUE on exactly the referenced columns.
  CONSTRAINT media_assets_id_kind_key UNIQUE (id, kind),

  -- An audio asset has a duration and a non-audio asset does not. Equality of
  -- the two predicates says that in one line, in both directions.
  CONSTRAINT media_assets_audio_has_duration
    CHECK ((kind = 'audio') = (duration_ms IS NOT NULL)),
  CONSTRAINT media_assets_kind_matches_mime
    CHECK ((kind = 'audio') = (mime_type LIKE 'audio/%'))
);

-- The library screen: newest first, within a kind (§11.2).
CREATE INDEX media_assets_kind_created_idx
  ON app.media_assets (kind, created_at DESC) WHERE deleted_at IS NULL;

-- [D-06] Non-unique, deliberately. Powers the "you already uploaded this"
-- warning and integrity re-verification.
CREATE INDEX media_assets_checksum_idx
  ON app.media_assets (checksum_sha256) WHERE deleted_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS app.media_assets;
