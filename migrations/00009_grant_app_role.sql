-- +goose Up

-- §13.5: the app connects with DML on `app` only, never as owner.
--
-- ORDERING NOTE. The plan originally put this last (as 00022, in Phase 3), on
-- the reasoning that a blanket GRANT ON ALL TABLES has to run after the tables
-- exist. That was backwards: it left quizzivy_app unable to read anything from
-- the moment the first table appeared until the end of Phase 3, and the first
-- integration test to connect as the app role failed with "permission denied
-- for schema app".
--
-- ALTER DEFAULT PRIVILEGES is the right tool. It applies to objects created
-- AFTER it, so running it here covers every table Phases 2-5 add without any
-- of them having to remember. The explicit grants below cover 00004-00008,
-- which already exist.

GRANT USAGE ON SCHEMA app TO quizzivy_app;

-- Custom types need their own USAGE; schema USAGE is not enough to reference
-- app.user_role in a query.
GRANT USAGE ON TYPE app.user_role      TO quizzivy_app;
GRANT USAGE ON TYPE app.test_status    TO quizzivy_app;
GRANT USAGE ON TYPE app.question_type  TO quizzivy_app;
GRANT USAGE ON TYPE app.attempt_status TO quizzivy_app;
GRANT USAGE ON TYPE app.media_kind     TO quizzivy_app;
GRANT USAGE ON TYPE app.join_source    TO quizzivy_app;
GRANT USAGE ON TYPE app.integrity_action TO quizzivy_app;

-- Everything that exists today.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA app TO quizzivy_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA app TO quizzivy_app;

-- Everything added later, automatically.
ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO quizzivy_app;
ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  GRANT USAGE, SELECT ON SEQUENCES TO quizzivy_app;
ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  GRANT USAGE ON TYPES TO quizzivy_app;

-- §13.4 asserts audit_log is append-only. This makes it a privilege rather
-- than a promise: an audit trail the application can rewrite is not one.
-- Ordering matters -- the blanket GRANT above runs first, and this narrows it.
REVOKE UPDATE, DELETE ON app.audit_log FROM quizzivy_app;

-- attempt_events gets the same treatment when Phase 3 creates it.

-- +goose Down

REVOKE ALL ON ALL TABLES IN SCHEMA app FROM quizzivy_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA app FROM quizzivy_app;
ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM quizzivy_app;
ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  REVOKE USAGE, SELECT ON SEQUENCES FROM quizzivy_app;
ALTER DEFAULT PRIVILEGES FOR ROLE quizzivy_migrate IN SCHEMA app
  REVOKE USAGE ON TYPES FROM quizzivy_app;
REVOKE USAGE ON SCHEMA app FROM quizzivy_app;
