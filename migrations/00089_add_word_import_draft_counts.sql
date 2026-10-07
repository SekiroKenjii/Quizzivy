-- +goose Up
ALTER TABLE app.word_import_drafts
    ADD COLUMN open_action_count integer CHECK (open_action_count >= 0),
    ADD COLUMN open_confirm_count integer CHECK (open_confirm_count >= 0);

-- +goose Down
ALTER TABLE app.word_import_drafts
    DROP COLUMN open_action_count,
    DROP COLUMN open_confirm_count;
