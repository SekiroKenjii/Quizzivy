-- +goose Up

-- Enums where the set is genuinely closed (§13.2). Adding a value is easy;
-- removing one is not, so anything that might grow is `text` with a CHECK or a
-- lookup table instead.
--
-- Deliberately NOT an enum: attempt_events.kind. §10.1's list looks closed, but
-- an append-only telemetry log reliably gains kinds, and each new one would be
-- a migration plus a deploy-ordering problem for a value nothing joins on.

CREATE TYPE app.user_role AS ENUM ('admin', 'student');

CREATE TYPE app.test_status AS ENUM ('draft', 'published', 'archived');

CREATE TYPE app.question_type AS ENUM (
  'single_choice', 'multiple_choice', 'true_false', 'fill_blank', 'short_answer'
);

CREATE TYPE app.attempt_status AS ENUM (
  'in_progress', 'submitted', 'timed_out', 'graded', 'voided'
);

CREATE TYPE app.media_kind AS ENUM ('image', 'audio');

CREATE TYPE app.join_source AS ENUM ('admin', 'join_code');

-- D-15: §7's IntegrityPolicy.onLimitExceeded is a closed three-value set and
-- belongs in the type system rather than a CHECK.
CREATE TYPE app.integrity_action AS ENUM ('warn', 'flag', 'auto_submit');

-- +goose Down

DROP TYPE IF EXISTS app.integrity_action;
DROP TYPE IF EXISTS app.join_source;
DROP TYPE IF EXISTS app.media_kind;
DROP TYPE IF EXISTS app.attempt_status;
DROP TYPE IF EXISTS app.question_type;
DROP TYPE IF EXISTS app.test_status;
DROP TYPE IF EXISTS app.user_role;
