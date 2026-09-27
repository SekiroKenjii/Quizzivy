-- +goose Up

-- The periods the API answers 503 MAINTENANCE (T-R1.12). An operator writes
-- them through cmd/maintenance; the API reads them and nothing else.
CREATE TABLE app.maintenance_windows (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  starts_at    timestamptz NOT NULL,
  ends_at      timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  -- The database role that scheduled the window. No app user schedules one.
  created_by   text NOT NULL DEFAULT current_user,
  cancelled_at timestamptz,

  CONSTRAINT maintenance_windows_ordered CHECK (ends_at > starts_at),
  CONSTRAINT maintenance_windows_bounded CHECK (ends_at - starts_at <= interval '12 hours'),
  -- A range-only exclusion, so btree_gist is not needed: two live windows
  -- never overlap, and a cancelled one no longer counts.
  CONSTRAINT maintenance_windows_no_overlap
    EXCLUDE USING gist (tstzrange(starts_at, ends_at) WITH &&)
    WHERE (cancelled_at IS NULL)
);

-- 00009's default privileges give the app role full DML on every new table.
-- It reads windows; only the operator's role writes them.
REVOKE INSERT, UPDATE, DELETE ON app.maintenance_windows FROM quizzivy_app;

-- +goose Down
DROP TABLE app.maintenance_windows;
