-- +goose Up

-- §7's targets: { classIds, studentIds } as two link tables, not array columns.
-- An array would turn "which assignments is this student eligible for" into a
-- GIN containment query against a growing list, and would lose referential
-- integrity to a deleted class.
CREATE TABLE app.assignment_classes (
  assignment_id uuid NOT NULL REFERENCES app.assignments(id) ON DELETE CASCADE,
  class_id      uuid NOT NULL REFERENCES app.classes(id) ON DELETE RESTRICT,
  PRIMARY KEY (assignment_id, class_id)
);

-- Reverse index: §9 resolves "assignments for this student" from both
-- directions in one query.
CREATE INDEX assignment_classes_class_idx ON app.assignment_classes (class_id);

CREATE TABLE app.assignment_students (
  assignment_id uuid NOT NULL REFERENCES app.assignments(id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  PRIMARY KEY (assignment_id, user_id)
);

CREATE INDEX assignment_students_user_idx ON app.assignment_students (user_id);

-- +goose Down
DROP TABLE app.assignment_students;
DROP TABLE app.assignment_classes;
