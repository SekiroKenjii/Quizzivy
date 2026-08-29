-- +goose Up

-- A draft assignment: written, targeted maybe, but not yet given to anyone.
--
-- This does NOT reintroduce the stored status D-18 rejects. Status is still
-- derived, and still needs no scheduler: draft -> scheduled is an explicit act
-- by the teacher, not a timestamp arriving. What is stored is the act itself,
-- and the window rule reads exactly as before once it has happened.
--
-- Nullable rather than a boolean because "when was this given out" is a
-- question the teacher will eventually ask, and a boolean cannot answer it.
ALTER TABLE app.assignments
  ADD COLUMN published_at timestamptz;

-- Every assignment that exists today was created by the assign action, which is
-- the only path there has ever been -- so none of them is a draft.
UPDATE app.assignments SET published_at = created_at WHERE published_at IS NULL;

-- A draft cannot have been closed early, and cannot have been attempted; both
-- are states that only exist downstream of handing it out.
ALTER TABLE app.assignments
  ADD CONSTRAINT assignments_draft_not_closed
  CHECK (published_at IS NOT NULL OR closed_at IS NULL);

-- The list filters on it and the dashboard's open-count excludes it, so the
-- partial index is the one that matters: drafts are the rarer row.
CREATE INDEX assignments_draft_idx ON app.assignments (created_by)
  WHERE published_at IS NULL;

-- +goose Down
DROP INDEX app.assignments_draft_idx;
ALTER TABLE app.assignments DROP CONSTRAINT assignments_draft_not_closed;
ALTER TABLE app.assignments DROP COLUMN published_at;
