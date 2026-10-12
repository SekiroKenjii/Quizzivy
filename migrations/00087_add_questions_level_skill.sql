-- +goose Up
ALTER TABLE app.questions
    ADD COLUMN level text,
    ADD COLUMN skill text,
    ADD CONSTRAINT questions_level_check CHECK (level IN ('pre_a1','a1','a2','b1','b2','c1','c2')),
    ADD CONSTRAINT questions_skill_check CHECK (skill IN ('grammar','vocabulary','reading','listening','writing','speaking'));

-- +goose Down
ALTER TABLE app.questions
    DROP CONSTRAINT questions_level_check,
    DROP CONSTRAINT questions_skill_check,
    DROP COLUMN level,
    DROP COLUMN skill;
