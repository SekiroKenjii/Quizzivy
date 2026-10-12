-- +goose Up

-- No row can name a file whose object the sweep removed, and a row that names a
-- file the sweep is waiting to remove stops the wait (F-68).
--
-- The sweep takes the file's row FOR UPDATE and checks, under that lock, that
-- nothing names it. A writer that takes the same lock first (LockForVersionUse)
-- is serialised with it, but a bank question's write does not take it, and
-- neither does any future writer that forgets. The trigger makes the rule the
-- database's: the UPDATE below locks a file the sweep may remove, so the writer
-- waits for the sweep, and then it sees purged_at and is refused; if the writer
-- comes first it clears unreferenced_at and the sweep, which skips a locked
-- row and re-checks what it finds, leaves the file alone.
--
-- A write that leaves media_asset_id as it was does nothing, so only a binding
-- pays for the lookup. check_violation names the rule, as a table CHECK does.
-- +goose StatementBegin
CREATE FUNCTION app.media_asset_binding_guard() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF NEW.media_asset_id IS NULL
     OR (TG_OP = 'UPDATE' AND NEW.media_asset_id IS NOT DISTINCT FROM OLD.media_asset_id) THEN
    RETURN NEW;
  END IF;
  UPDATE app.media_assets
     SET unreferenced_at = NULL
   WHERE id = NEW.media_asset_id AND unreferenced_at IS NOT NULL AND purged_at IS NULL;
  IF EXISTS (SELECT 1 FROM app.media_assets WHERE id = NEW.media_asset_id AND purged_at IS NOT NULL) THEN
    RAISE EXCEPTION 'media asset % was removed from storage and cannot be bound', NEW.media_asset_id
      USING ERRCODE = 'check_violation', TABLE = TG_TABLE_NAME, CONSTRAINT = 'media_assets_purged_binding';
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER questions_media_binding_guard
  BEFORE INSERT OR UPDATE OF media_asset_id ON app.questions
  FOR EACH ROW EXECUTE FUNCTION app.media_asset_binding_guard();
CREATE TRIGGER group_recordings_media_binding_guard
  BEFORE INSERT OR UPDATE OF media_asset_id ON app.group_recordings
  FOR EACH ROW EXECUTE FUNCTION app.media_asset_binding_guard();
CREATE TRIGGER group_stimulus_assets_media_binding_guard
  BEFORE INSERT OR UPDATE OF media_asset_id ON app.group_stimulus_assets
  FOR EACH ROW EXECUTE FUNCTION app.media_asset_binding_guard();
CREATE TRIGGER test_version_questions_media_binding_guard
  BEFORE INSERT OR UPDATE OF media_asset_id ON app.test_version_questions
  FOR EACH ROW EXECUTE FUNCTION app.media_asset_binding_guard();
CREATE TRIGGER test_version_group_recordings_media_binding_guard
  BEFORE INSERT OR UPDATE OF media_asset_id ON app.test_version_group_recordings
  FOR EACH ROW EXECUTE FUNCTION app.media_asset_binding_guard();
CREATE TRIGGER test_version_group_assets_media_binding_guard
  BEFORE INSERT OR UPDATE OF media_asset_id ON app.test_version_group_assets
  FOR EACH ROW EXECUTE FUNCTION app.media_asset_binding_guard();

-- +goose Down

DROP TRIGGER test_version_group_assets_media_binding_guard ON app.test_version_group_assets;
DROP TRIGGER test_version_group_recordings_media_binding_guard ON app.test_version_group_recordings;
DROP TRIGGER test_version_questions_media_binding_guard ON app.test_version_questions;
DROP TRIGGER group_stimulus_assets_media_binding_guard ON app.group_stimulus_assets;
DROP TRIGGER group_recordings_media_binding_guard ON app.group_recordings;
DROP TRIGGER questions_media_binding_guard ON app.questions;
DROP FUNCTION app.media_asset_binding_guard();
