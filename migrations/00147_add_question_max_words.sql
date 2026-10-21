-- +goose Up

-- A short answer's word limit (T-R6.1a). It is advice, never truncation: the
-- engine counts words and flags "too many", the server stores whatever was
-- typed, and the grader sees the over-limit badge. NULL is no limit. The frozen
-- copy carries it, as 00146 does select_count, and in the same pattern.
SET LOCAL lock_timeout = '5s';

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

-- Refuses while a question or a frozen version question has a max_words, as
-- 00145 refuses for the types: the limit cannot be rebuilt once the column goes.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.questions WHERE max_words IS NOT NULL)
     OR EXISTS (SELECT 1 FROM app.test_version_questions WHERE max_words IS NOT NULL) THEN
    RAISE EXCEPTION 'a short answer with a word limit exists; dropping max_words would lose its rule';
  END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE app.test_version_questions DROP COLUMN max_words;
ALTER TABLE app.questions DROP COLUMN max_words;
