-- +goose Up

-- T-R5.14: an Admin's import of accounts from a CSV or XLSX file. The API reads
-- the file, judges every row, and stores the job and its accepted rows in one
-- transaction; the worker then creates the accounts, a few at a time, and
-- moves the counters. A refused row is counted in invalid_count and never
-- stored, so the file's text does not outlive the request.
--
-- total_rows is the rows stored (1 to 2,000), the accepted ones: an address
-- that already belongs to an account is stored and skipped, and created_count
-- plus skipped_count reach it when the job is done. A file with nothing to
-- create is refused before it gets here, so a job always has a row to do.
--
-- request_id is chosen by the caller and held for the dialog's life, so a retry
-- of the same request returns the same job and creates no second one;
-- (created_by, request_id) is unique. default_role_id is the role a row with
-- no role takes, kept because the worker re-checks the importer's right to
-- every role of the job at the moment it runs.
--
-- attempts counts the claims. A worker that dies mid-job leaves the row
-- running; once its lease (30 minutes, in Go) has lapsed the next claim takes it
-- again and resumes from the pending rows, and after the third the job is failed
-- rather than retried for ever. error_code is a short code, never a message:
-- nothing about the file reaches this table.
--
-- For a job delivered by sheet, sheet_expires_at is set when the job is done
-- (24 hours later) and sheet_downloaded_at is the first and only download, which
-- erases the temporary passwords sealed in user_import_rows.
CREATE TABLE app.user_import_jobs (
  id                  uuid PRIMARY KEY DEFAULT uuidv7(),
  created_by          uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  request_id          uuid NOT NULL,
  filename            text NOT NULL,
  format              text NOT NULL,
  default_role_id     uuid NOT NULL REFERENCES app.roles(id) ON DELETE RESTRICT,
  delivery            text NOT NULL,
  status              text NOT NULL DEFAULT 'queued',
  total_rows          smallint NOT NULL,
  created_count       smallint NOT NULL DEFAULT 0,
  skipped_count       smallint NOT NULL DEFAULT 0,
  invalid_count       smallint NOT NULL DEFAULT 0,
  attempts            smallint NOT NULL DEFAULT 0,
  error_code          text,
  sheet_expires_at    timestamptz,
  sheet_downloaded_at timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  started_at          timestamptz,
  completed_at        timestamptz,

  CONSTRAINT user_import_jobs_request_key
    UNIQUE (created_by, request_id),
  CONSTRAINT user_import_jobs_filename_check
    CHECK (char_length(filename) BETWEEN 1 AND 255),
  CONSTRAINT user_import_jobs_format_check
    CHECK (format IN ('csv', 'xlsx')),
  CONSTRAINT user_import_jobs_delivery_check
    CHECK (delivery IN ('email_link', 'sheet')),
  CONSTRAINT user_import_jobs_status_check
    CHECK (status IN ('queued', 'running', 'done', 'failed')),
  CONSTRAINT user_import_jobs_total_rows_check
    CHECK (total_rows BETWEEN 1 AND 2000),
  CONSTRAINT user_import_jobs_counts_check
    CHECK (created_count >= 0 AND skipped_count >= 0 AND created_count + skipped_count <= total_rows),
  CONSTRAINT user_import_jobs_invalid_count_check
    CHECK (invalid_count BETWEEN 0 AND 2000),
  CONSTRAINT user_import_jobs_attempts_check
    CHECK (attempts BETWEEN 0 AND 3),
  CONSTRAINT user_import_jobs_error_code_check
    CHECK (char_length(error_code) BETWEEN 1 AND 64),
  CONSTRAINT user_import_jobs_done_settled
    CHECK (status <> 'done' OR created_count + skipped_count = total_rows),
  CONSTRAINT user_import_jobs_failed_has_code
    CHECK ((status = 'failed') = (error_code IS NOT NULL)),
  CONSTRAINT user_import_jobs_completed_has_end
    CHECK ((status IN ('done', 'failed')) = (completed_at IS NOT NULL)),
  CONSTRAINT user_import_jobs_started_once_claimed
    CHECK (status = 'queued' OR started_at IS NOT NULL),
  CONSTRAINT user_import_jobs_sheet_only_by_sheet
    CHECK (delivery = 'sheet' OR (sheet_expires_at IS NULL AND sheet_downloaded_at IS NULL))
);

-- The worker's queue: the jobs that are not finished, oldest first.
CREATE INDEX user_import_jobs_open_idx ON app.user_import_jobs (created_at)
  WHERE status IN ('queued', 'running');

-- +goose Down
DROP TABLE app.user_import_jobs;
