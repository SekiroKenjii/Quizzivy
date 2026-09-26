-- +goose Up
GRANT DELETE ON app.word_import_drafts TO quizzivy_app;

-- +goose Down
REVOKE DELETE ON app.word_import_drafts FROM quizzivy_app;
