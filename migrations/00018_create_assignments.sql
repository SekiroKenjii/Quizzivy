-- +goose Up

-- §7's assignment. No `status` column [D-18]: status is a pure function of
-- opens_at, closes_at and closed_at, and storing it would need a scheduler to
-- flip rows at two timestamps each -- a cron job, a missed tick, and a whole
-- class of stale-status bugs, for a value the projection can compute.
-- `closed_at` exists because closing early is the one thing the window cannot
-- express.
CREATE TABLE app.assignments (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  test_id          uuid NOT NULL,
  test_version_id  uuid NOT NULL,
  opens_at         timestamptz NOT NULL,
  closes_at        timestamptz NOT NULL,
  closed_at        timestamptz,
  duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 1 AND 600),
  max_attempts     smallint NOT NULL DEFAULT 1 CHECK (max_attempts > 0),
  shuffle_questions boolean NOT NULL DEFAULT false,
  shuffle_options   boolean NOT NULL DEFAULT false,

  review_show_score            boolean NOT NULL DEFAULT true,
  review_show_correct_answers  boolean NOT NULL DEFAULT false,
  review_show_explanations     boolean NOT NULL DEFAULT false,

  -- §10.3 verbatim, in the DDL so a new assignment is conservative even if a
  -- handler forgets to set them.
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

  -- [D-17] Proves test_version_id belongs to test_id. Without it an assignment
  -- could reference version 3 of test A while claiming test B, and every join
  -- downstream would quietly produce the wrong paper.
  FOREIGN KEY (test_version_id, test_id)
    REFERENCES app.test_versions (id, test_id) ON DELETE RESTRICT,
  CHECK (closes_at > opens_at),
  CHECK (closed_at IS NULL OR closed_at >= opens_at)
);

CREATE INDEX assignments_window_idx ON app.assignments (opens_at, closes_at);
CREATE INDEX assignments_version_idx ON app.assignments (test_version_id);

-- +goose StatementBegin
CREATE TRIGGER assignments_set_updated_at BEFORE UPDATE ON app.assignments
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE app.assignments;
