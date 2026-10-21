-- +goose Up

-- The centre's usage limits (CR-002, D-060), edited on Settings > Plan & usage.
-- A NULL column takes its default: the deploy's variable when one is set, else
-- the hosting plan's figure, and for seats and teachers no limit. Each CHECK is
-- the range shared/limits accepts. Adding nullable columns without a default
-- rewrites nothing, and the table holds one row.
ALTER TABLE app.organization_settings
  ADD COLUMN limit_seats                    integer,
  ADD COLUMN limit_teachers                 integer,
  ADD COLUMN limit_storage_bytes            bigint,
  ADD COLUMN limit_media_per_teacher_bytes  bigint,
  ADD COLUMN limit_import_bytes             bigint,
  ADD COLUMN limit_import_per_teacher_bytes bigint,
  ADD COLUMN limit_emails_per_day           integer,
  ADD COLUMN limit_emails_per_month         integer,
  ADD COLUMN limit_database_bytes           bigint,
  ADD COLUMN warn_at_percent                smallint,
  ADD CONSTRAINT organization_settings_limit_seats_check
    CHECK (limit_seats BETWEEN 1 AND 1000000),
  ADD CONSTRAINT organization_settings_limit_teachers_check
    CHECK (limit_teachers BETWEEN 1 AND 1000000),
  ADD CONSTRAINT organization_settings_limit_storage_bytes_check
    CHECK (limit_storage_bytes BETWEEN 1048576 AND 1125899906842624),
  ADD CONSTRAINT organization_settings_limit_media_per_teacher_bytes_check
    CHECK (limit_media_per_teacher_bytes BETWEEN 1048576 AND 1125899906842624),
  ADD CONSTRAINT organization_settings_limit_import_bytes_check
    CHECK (limit_import_bytes BETWEEN 1048576 AND 1125899906842624),
  ADD CONSTRAINT organization_settings_limit_import_per_teacher_bytes_check
    CHECK (limit_import_per_teacher_bytes BETWEEN 1048576 AND 1125899906842624),
  ADD CONSTRAINT organization_settings_limit_emails_per_day_check
    CHECK (limit_emails_per_day BETWEEN 1 AND 100000),
  ADD CONSTRAINT organization_settings_limit_emails_per_month_check
    CHECK (limit_emails_per_month BETWEEN 1 AND 10000000),
  ADD CONSTRAINT organization_settings_limit_database_bytes_check
    CHECK (limit_database_bytes BETWEEN 1048576 AND 1125899906842624),
  ADD CONSTRAINT organization_settings_warn_at_percent_check
    CHECK (warn_at_percent BETWEEN 50 AND 100);

-- +goose Down
ALTER TABLE app.organization_settings
  DROP COLUMN limit_seats,
  DROP COLUMN limit_teachers,
  DROP COLUMN limit_storage_bytes,
  DROP COLUMN limit_media_per_teacher_bytes,
  DROP COLUMN limit_import_bytes,
  DROP COLUMN limit_import_per_teacher_bytes,
  DROP COLUMN limit_emails_per_day,
  DROP COLUMN limit_emails_per_month,
  DROP COLUMN limit_database_bytes,
  DROP COLUMN warn_at_percent;
