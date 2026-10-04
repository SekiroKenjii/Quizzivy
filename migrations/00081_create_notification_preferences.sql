-- +goose Up

-- One row per switch a user has saved (T-R4.10a). A missing row means the
-- default: in the app, and not by email. email is stored now so that the
-- release that sends email adds no column.
CREATE TABLE app.notification_preferences (
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  event      text NOT NULL,
  in_app     boolean NOT NULL,
  email      boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (user_id, event),
  -- Later releases replace this constraint under this name to add switches.
  CONSTRAINT notification_preferences_event_check CHECK (event IN (
    'attempt.submitted', 'attempt.flagged', 'assignment.closing', 'assignment.due_soon', 'result.ready'))
);

CREATE TRIGGER notification_preferences_set_updated_at BEFORE UPDATE ON app.notification_preferences
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- +goose Down
DROP TABLE app.notification_preferences;
