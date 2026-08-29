-- +goose Up

-- The immutable snapshot root. No updated_at and no soft delete: a published
-- version is a historical fact, and an attempt from two years ago must still
-- resolve to exactly what the student sat.
CREATE TABLE app.test_versions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),

  -- RESTRICT rather than CASCADE, against §13.2's default for owned children.
  -- A version IS owned by a test, but attempts reference versions with
  -- RESTRICT, so cascading here would only ever surface as a constraint
  -- violation two levels down with a confusing message. Tests soft-delete
  -- anyway, so this fires on a hard purge and fails at the right place.
  test_id      uuid NOT NULL REFERENCES app.tests(id) ON DELETE RESTRICT,
  version      integer NOT NULL CHECK (version > 0),

  -- Stored rather than derived. §7 calls it server-computed; freezing it at
  -- publish means the score denominator on an old attempt cannot drift when
  -- the bank question's points are edited.
  total_points numeric(8,2) NOT NULL CHECK (total_points > 0),

  published_at timestamptz NOT NULL DEFAULT now(),
  published_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,

  UNIQUE (test_id, version),

  -- [D-17] The composite-FK target that lets assignments prove their
  -- test_version_id belongs to their test_id. Same technique as media_assets.
  CONSTRAINT test_versions_id_test_key UNIQUE (id, test_id)
);

CREATE TABLE app.test_version_sections (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_id uuid NOT NULL
                    REFERENCES app.test_versions(id) ON DELETE CASCADE,
  ordinal         smallint NOT NULL CHECK (ordinal >= 0),
  title           text NOT NULL,
  instructions    text,

  -- Not deferrable, unlike the draft tables: a version is written once and
  -- never reordered.
  UNIQUE (test_version_id, ordinal)
);

-- +goose Down

DROP TABLE IF EXISTS app.test_version_sections;
DROP TABLE IF EXISTS app.test_versions;
