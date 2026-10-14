-- +goose Up

-- T-R5.15: the full export of the centre's data. An Admin asks for one, the
-- worker builds a ZIP of CSV files into the private import bucket, and the
-- requester fetches it once from an app page. A row is the whole
-- conversation: queued, running, then ready (the file is at storage_key until
-- expires_at) or failed.
--
-- At most one export is queued or running for the whole centre, which the
-- partial unique index below enforces, so two Admins asking at once cannot
-- both build a ZIP of everyone's data. The same index is the worker's queue:
-- it holds one row at most, so the claim and the next-due read never scan.
--
-- attempts counts the claims. A worker that dies mid-build leaves the row
-- running; once its lease (30 minutes, in Go) has lapsed the next claim takes
-- it again, and after the third the row is failed rather than retried for
-- ever. error_code is a short code, never a message: nothing about the data
-- reaches this table.
--
-- storage_key is the object's key in the import bucket (exports/{id}.zip),
-- cleared when the housekeeping deletes the object and marks the row expired.
-- downloaded_at is set by the statement that hands out the URL, so a second
-- request finds it set and is refused.
CREATE TABLE app.data_exports (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  requested_by    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  include_answers boolean NOT NULL,
  status          text NOT NULL DEFAULT 'queued',
  attempts        smallint NOT NULL DEFAULT 0,
  storage_key     text,
  bytes           bigint,
  error_code      text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  started_at      timestamptz,
  completed_at    timestamptz,
  expires_at      timestamptz,
  downloaded_at   timestamptz,

  CONSTRAINT data_exports_status_check
    CHECK (status IN ('queued', 'running', 'ready', 'failed', 'expired')),
  CONSTRAINT data_exports_attempts_check
    CHECK (attempts BETWEEN 0 AND 3),
  CONSTRAINT data_exports_bytes_check
    CHECK (bytes > 0),
  CONSTRAINT data_exports_storage_key_check
    CHECK (char_length(storage_key) BETWEEN 1 AND 200),
  CONSTRAINT data_exports_error_code_check
    CHECK (char_length(error_code) BETWEEN 1 AND 64),
  CONSTRAINT data_exports_ready_has_file
    CHECK (status <> 'ready' OR (storage_key IS NOT NULL AND bytes IS NOT NULL AND expires_at IS NOT NULL)),
  CONSTRAINT data_exports_failed_has_code
    CHECK (status <> 'failed' OR error_code IS NOT NULL)
);

CREATE UNIQUE INDEX data_exports_active_idx ON app.data_exports ((true))
  WHERE status IN ('queued', 'running');
-- The cascade from app.users and the requester's own reads.
CREATE INDEX data_exports_requested_by_idx ON app.data_exports (requested_by, created_at DESC);

-- +goose Down
DROP TABLE app.data_exports;
