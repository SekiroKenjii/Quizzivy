-- +goose Up

-- The permission catalogue, the roles and their grants (T-R2.1, plan 70 §4.1).
-- Reference data the app cannot run without, so the rows are written here and
-- not in seed/ (D-21). Code and seeds find a built-in role by builtin_key.
CREATE TABLE app.permissions (
  key       text PRIMARY KEY CHECK (key ~ '^[a-z]+(\.[a-z_]+)+$'),
  group_key text NOT NULL CHECK (group_key IN ('content', 'teaching', 'people', 'system')),
  ordinal   smallint NOT NULL CHECK (ordinal > 0),
  in_matrix boolean NOT NULL,
  UNIQUE (group_key, ordinal)
);

-- The deck's matrix rows in its order (Quizzivy Admin.dc.html, PERMS), with
-- D12's "Create student accounts" second in People (DG-03). The four hidden
-- keys follow; only the Admin wildcard holds them, and they are never granted.
INSERT INTO app.permissions (key, group_key, ordinal, in_matrix) VALUES
  ('content.tests.write',            'content',  1, true),
  ('content.tests.publish',          'content',  2, true),
  ('content.questions.write',        'content',  3, true),
  ('content.media.write',            'content',  4, true),
  ('content.share',                  'content',  5, true),
  ('teaching.classes.write',         'teaching', 1, true),
  ('teaching.assignments.write',     'teaching', 2, true),
  ('teaching.grading',               'teaching', 3, true),
  ('teaching.attempts.intervene',    'teaching', 4, true),
  ('teaching.attendance',            'teaching', 5, true),
  ('people.students.read',           'people',   1, true),
  ('people.students.create',         'people',   2, true),
  ('people.students.reset_password', 'people',   3, true),
  ('people.users.manage',            'people',   4, true),
  ('people.roles.manage',            'people',   5, true),
  ('system.audit.read',              'system',   1, true),
  ('system.settings.write',          'system',   2, true),
  ('learning.take_tests',            'system',   3, true),
  ('scope.all',                      'system',   4, false),
  ('system.api_reference',           'system',   5, false),
  ('system.data_export',             'system',   6, false),
  ('system.leads',                   'system',   7, false);

CREATE TABLE app.roles (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  builtin_key text UNIQUE CHECK (builtin_key IN ('admin', 'teacher', 'assistant', 'student')),
  name        text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 60),
  description text NOT NULL DEFAULT '' CHECK (length(description) <= 300),
  -- The Edit role dialog's twelve icons and seven card colours.
  icon        text NOT NULL CHECK (icon IN ('shield', 'graduation-cap', 'hand-helping', 'user',
                'key-round', 'crown', 'briefcase', 'book-open', 'users', 'clipboard-check',
                'headset', 'star')),
  color       text NOT NULL CHECK (color IN ('dark', 'lime', 'blue', 'green', 'amber', 'rose', 'gray')),
  copied_from uuid REFERENCES app.roles ON DELETE SET NULL,
  -- Bumped by every grant change, so a cache keyed by (id, revision) is never stale.
  revision    bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_by  uuid REFERENCES app.users ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX roles_name_lower_key ON app.roles (lower(name));

CREATE TRIGGER roles_set_updated_at BEFORE UPDATE ON app.roles
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

CREATE TABLE app.role_permissions (
  role_id        uuid NOT NULL REFERENCES app.roles ON DELETE CASCADE,
  permission_key text NOT NULL REFERENCES app.permissions ON DELETE RESTRICT,
  PRIMARY KEY (role_id, permission_key)
);

CREATE INDEX role_permissions_permission_key_idx ON app.role_permissions (permission_key);

-- Names and descriptions are data in the organisation's language (D2, D11).
INSERT INTO app.roles (builtin_key, name, description, icon, color) VALUES
  ('admin',     'Quản trị viên', 'Điều hành trung tâm. Quản lý người dùng và cài đặt.', 'shield',         'dark'),
  ('teacher',   'Giáo viên',     'Soạn đề, quản lý lớp, chấm bài.',                     'graduation-cap', 'lime'),
  ('assistant', 'Trợ giảng',     'Hỗ trợ giáo viên chấm bài và điểm danh.',             'hand-helping',   'blue'),
  ('student',   'Học viên',      'Làm bài và xem kết quả của mình.',                    'user',           'gray');

-- The Admin stores no grant: it is the wildcard, with "Take tests" off.
INSERT INTO app.role_permissions (role_id, permission_key)
SELECT r.id, g.permission_key
  FROM app.roles r
  JOIN (VALUES
    ('teacher',   'content.tests.write'),
    ('teacher',   'content.tests.publish'),
    ('teacher',   'content.questions.write'),
    ('teacher',   'content.media.write'),
    ('teacher',   'content.share'),
    ('teacher',   'teaching.classes.write'),
    ('teacher',   'teaching.assignments.write'),
    ('teacher',   'teaching.grading'),
    ('teacher',   'teaching.attempts.intervene'),
    ('teacher',   'teaching.attendance'),
    ('teacher',   'people.students.read'),
    ('teacher',   'people.students.create'),
    ('teacher',   'people.students.reset_password'),
    ('assistant', 'content.tests.write'),
    ('assistant', 'content.questions.write'),
    ('assistant', 'teaching.assignments.write'),
    ('assistant', 'teaching.grading'),
    ('assistant', 'teaching.attendance'),
    ('assistant', 'people.students.read'),
    ('student',   'learning.take_tests')
  ) AS g (builtin_key, permission_key) ON g.builtin_key = r.builtin_key;

-- +goose StatementBegin
CREATE FUNCTION app.roles_builtin_key_immutable() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF NEW.builtin_key IS DISTINCT FROM OLD.builtin_key THEN
    RAISE EXCEPTION 'a role''s builtin_key cannot change'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'roles_builtin_key_immutable';
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER roles_builtin_key_immutable BEFORE UPDATE OF builtin_key ON app.roles
  FOR EACH ROW EXECUTE FUNCTION app.roles_builtin_key_immutable();

-- A hidden key is never granted, and the Admin, being the wildcard, stores only
-- its one toggle. UPDATE is covered too, although no role is ever granted it on
-- this table: an update is a delete and an insert in one statement.
-- +goose StatementBegin
CREATE FUNCTION app.role_permissions_guard() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF EXISTS (SELECT 1 FROM app.permissions p
              WHERE p.key = NEW.permission_key AND NOT p.in_matrix) THEN
    RAISE EXCEPTION 'permission % is never granted to a role', NEW.permission_key
      USING ERRCODE = 'check_violation', CONSTRAINT = 'role_permissions_guard';
  END IF;
  IF NEW.permission_key <> 'learning.take_tests'
     AND EXISTS (SELECT 1 FROM app.roles r WHERE r.id = NEW.role_id AND r.builtin_key = 'admin') THEN
    RAISE EXCEPTION 'the Admin role holds every permission; only learning.take_tests is stored for it'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'role_permissions_guard';
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER role_permissions_guard BEFORE INSERT OR UPDATE ON app.role_permissions
  FOR EACH ROW EXECUTE FUNCTION app.role_permissions_guard();

-- +goose StatementBegin
CREATE FUNCTION app.role_permissions_keep_student_take_tests() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF OLD.permission_key = 'learning.take_tests'
     AND (TG_OP = 'DELETE' OR NEW.permission_key <> OLD.permission_key OR NEW.role_id <> OLD.role_id)
     AND EXISTS (SELECT 1 FROM app.roles r WHERE r.id = OLD.role_id AND r.builtin_key = 'student') THEN
    RAISE EXCEPTION 'the built-in Student role cannot lose learning.take_tests'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'role_permissions_keep_student_take_tests';
  END IF;
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER role_permissions_keep_student_take_tests BEFORE DELETE OR UPDATE ON app.role_permissions
  FOR EACH ROW EXECUTE FUNCTION app.role_permissions_keep_student_take_tests();

-- Row-level on purpose: the table is tiny, and a statement-level trigger with
-- transition tables would need one trigger per event. It runs as the invoker;
-- in R2 only quizzivy_migrate writes grants, and R5 grants the app role UPDATE
-- on revision along with the role commands' writes (T-R5.8).
-- +goose StatementBegin
CREATE FUNCTION app.role_permissions_bump_revision() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF TG_OP <> 'DELETE' THEN
    UPDATE app.roles SET revision = revision + 1 WHERE id = NEW.role_id;
  END IF;
  IF TG_OP = 'DELETE' OR NEW.role_id <> OLD.role_id THEN
    UPDATE app.roles SET revision = revision + 1 WHERE id = OLD.role_id;
  END IF;
  RETURN NULL;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER role_permissions_bump_revision AFTER INSERT OR UPDATE OR DELETE ON app.role_permissions
  FOR EACH ROW EXECUTE FUNCTION app.role_permissions_bump_revision();

-- 00009's default privileges give the app role full DML on every new table.
-- It reads these; the role commands' writes arrive in R5 (T-R5.8), and neither
-- DELETE on roles (DG-52) nor any write on permissions is ever granted.
REVOKE INSERT, UPDATE, DELETE ON app.permissions, app.roles, app.role_permissions FROM quizzivy_app;

-- +goose Down

DROP TABLE app.role_permissions;
DROP TABLE app.roles;
DROP TABLE app.permissions;
DROP FUNCTION app.role_permissions_bump_revision();
DROP FUNCTION app.role_permissions_keep_student_take_tests();
DROP FUNCTION app.role_permissions_guard();
DROP FUNCTION app.roles_builtin_key_immutable();
