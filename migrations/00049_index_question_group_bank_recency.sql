-- +goose Up
CREATE INDEX question_groups_bank_recent_idx ON app.question_groups (updated_at DESC, id DESC)
  WHERE owner_section_id IS NULL AND archived_at IS NULL;
DROP INDEX app.question_groups_bank_idx;

-- +goose Down
CREATE INDEX question_groups_bank_idx ON app.question_groups (id DESC)
  WHERE owner_section_id IS NULL AND archived_at IS NULL;
DROP INDEX app.question_groups_bank_recent_idx;
