-- +goose Up

-- A short answer's word limit (T-R6.1a). It is advice, never truncation: the
-- engine counts words and flags "too many", the server stores whatever was
-- typed, and the grader sees the over-limit badge. NULL is no limit. The frozen
-- copy carries it, as 00146 does select_count, and in the same pattern.
ALTER TABLE app.questions ADD COLUMN max_words smallint;
ALTER TABLE app.questions
  ADD CONSTRAINT questions_max_words_check
    CHECK (max_words IS NULL OR (type = 'short_answer' AND max_words BETWEEN 1 AND 100))
    NOT VALID;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_max_words_check;

ALTER TABLE app.test_version_questions ADD COLUMN max_words smallint;
ALTER TABLE app.test_version_questions
  ADD CONSTRAINT test_version_questions_max_words_check
    CHECK (max_words IS NULL OR (type = 'short_answer' AND max_words BETWEEN 1 AND 100))
    NOT VALID;
ALTER TABLE app.test_version_questions VALIDATE CONSTRAINT test_version_questions_max_words_check;

-- +goose Down

ALTER TABLE app.test_version_questions DROP COLUMN max_words;
ALTER TABLE app.questions DROP COLUMN max_words;
