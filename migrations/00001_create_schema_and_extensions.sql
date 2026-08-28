-- +goose Up

CREATE SCHEMA app;

-- Both are TRUSTED extensions in PG13+, so quizzivy_migrate installs them
-- without superuser. Verified against 18.6.
--
-- They land in `public` (the default), which is why the app role keeps USAGE on
-- that schema: `gin_trgm_ops` must be resolvable when indexes are created in
-- `app`.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- unaccent() is STABLE in PG18 -- BOTH the one-argument and the two-argument
-- form, contrary to the widespread claim that the two-argument one is
-- IMMUTABLE. Verified on 18.6; pg18_test.go pins it. Postgres therefore refuses
-- a bare unaccent() in an index expression:
--
--   ERROR:  functions in index expression must be marked IMMUTABLE
--
-- This wrapper pins the dictionary and asserts immutability, which is the
-- standard workaround. The assertion is a deliberate white lie: if the unaccent
-- dictionary file ever changes, every index built on this function must be
-- REINDEXed. That is why the dictionary is named explicitly rather than left to
-- the search path.
--
-- Needed because pg_trgm is case-insensitive but NOT accent-insensitive --
-- 'nghé' ILIKE '%nghe%' is false -- and this is a Vietnamese-first product
-- (docs/plan/20-data-model.md D-11).
-- +goose StatementBegin
CREATE FUNCTION app.immutable_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS
  $fn$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $fn$;
-- +goose StatementEnd

-- Documentation only: this has been the default since PG15. Kept so the intent
-- in spec §13.2 is visible in the schema history rather than assumed.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- +goose Down

DROP FUNCTION IF EXISTS app.immutable_unaccent(text);
DROP SCHEMA IF EXISTS app;
DROP EXTENSION IF EXISTS unaccent;
DROP EXTENSION IF EXISTS pg_trgm;
