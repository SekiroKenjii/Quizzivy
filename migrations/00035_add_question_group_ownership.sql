-- +goose Up
CREATE TABLE app.question_groups (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  owner_section_id uuid REFERENCES app.test_sections(id) ON DELETE RESTRICT,
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200 AND btrim(title) <> ''),
  instructions jsonb CHECK (instructions IS NULL OR
    (jsonb_typeof(instructions) = 'object' AND (instructions->>'format' = 'semantic_v1') IS TRUE)),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  archived_at timestamptz,
  created_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT question_groups_owner_key UNIQUE (id, owner_section_id),
  CONSTRAINT question_groups_archive_bank_only CHECK (owner_section_id IS NULL OR archived_at IS NULL)
);

CREATE INDEX question_groups_section_idx ON app.question_groups (owner_section_id)
  WHERE owner_section_id IS NOT NULL;
CREATE INDEX question_groups_bank_idx ON app.question_groups (id DESC)
  WHERE owner_section_id IS NULL AND archived_at IS NULL;
CREATE TRIGGER question_groups_set_updated_at BEFORE UPDATE ON app.question_groups
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

ALTER TABLE app.questions
  ADD COLUMN context_group_id uuid REFERENCES app.question_groups(id) ON DELETE RESTRICT,
  ADD COLUMN context_ordinal smallint,
  ADD COLUMN context_option_order text,
  ADD CONSTRAINT questions_context_complete CHECK (
    (context_group_id IS NULL AND context_ordinal IS NULL AND context_option_order IS NULL)
    OR (context_group_id IS NOT NULL AND context_ordinal IS NOT NULL
      AND context_ordinal BETWEEN 0 AND 199 AND context_option_order IS NOT NULL
      AND context_option_order IN ('shuffle', 'fixed') AND deleted_at IS NULL)),
  ADD CONSTRAINT questions_context_option_order CHECK (
    context_option_order IS DISTINCT FROM 'fixed'
    OR type IN ('single_choice', 'multiple_choice', 'true_false'));

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.question_groups) THEN
    RAISE EXCEPTION 'question group ownership cannot be removed after group writes';
  END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE app.questions
  DROP COLUMN context_group_id,
  DROP COLUMN context_ordinal,
  DROP COLUMN context_option_order;
DROP TABLE app.question_groups;
