-- +goose Up

-- A user import's row that would take the centre past its seat or teacher
-- limit (D-060) is settled as 'skipped_limit', not created, and the rest of the
-- job goes on; over_limit_count counts those rows, and a job is done once
-- created, skipped and over-limit rows reach total_rows. Both tables are small
-- and the swaps run in one transaction.
ALTER TABLE app.user_import_rows
  DROP CONSTRAINT user_import_rows_outcome_check,
  ADD CONSTRAINT user_import_rows_outcome_check
    CHECK (outcome IN ('pending', 'created', 'skipped_exists', 'skipped_limit'));

ALTER TABLE app.user_import_jobs
  ADD COLUMN over_limit_count smallint NOT NULL DEFAULT 0,
  DROP CONSTRAINT user_import_jobs_counts_check,
  ADD CONSTRAINT user_import_jobs_counts_check
    CHECK (created_count >= 0 AND skipped_count >= 0 AND over_limit_count >= 0
           AND created_count + skipped_count + over_limit_count <= total_rows),
  DROP CONSTRAINT user_import_jobs_done_settled,
  ADD CONSTRAINT user_import_jobs_done_settled
    CHECK (status <> 'done' OR created_count + skipped_count + over_limit_count = total_rows);

-- +goose Down
UPDATE app.user_import_rows SET outcome = 'skipped_exists' WHERE outcome = 'skipped_limit';
UPDATE app.user_import_jobs SET skipped_count = skipped_count + over_limit_count, over_limit_count = 0 WHERE over_limit_count > 0;
ALTER TABLE app.user_import_jobs
  DROP CONSTRAINT user_import_jobs_done_settled,
  DROP CONSTRAINT user_import_jobs_counts_check,
  DROP COLUMN over_limit_count,
  ADD CONSTRAINT user_import_jobs_counts_check
    CHECK (created_count >= 0 AND skipped_count >= 0 AND created_count + skipped_count <= total_rows),
  ADD CONSTRAINT user_import_jobs_done_settled
    CHECK (status <> 'done' OR created_count + skipped_count = total_rows);
ALTER TABLE app.user_import_rows
  DROP CONSTRAINT user_import_rows_outcome_check,
  ADD CONSTRAINT user_import_rows_outcome_check
    CHECK (outcome IN ('pending', 'created', 'skipped_exists'));
