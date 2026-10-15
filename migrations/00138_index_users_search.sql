-- +goose NO TRANSACTION
-- +goose Up

-- The index behind the Users search of the Admin console (listUsers, `q`). The
-- query compares app.immutable_unaccent(lower(full_name || ' ' || email)) with
-- the same call over the search text, LIKE '%…%', which a b-tree cannot serve:
-- without this index the count, the facets and a sparse page each read the whole
-- table, 99 ms at 100,000 accounts with no filter, 400 ms for a search that finds
-- something and about 1 s for one that finds nothing (QC-N-5). A gin index with
-- gin_trgm_ops took the first to 71 ms and the last to 4 ms. The expression must
-- be the query's, character for character in effect: repositories.UserSearchText
-- holds it, and a test fails when the planner stops choosing the index for it.
--
-- Two characters or fewer give no trigram to look up, so a search that short
-- still reads the table, as it does today; a centre with few accounts reads a
-- small table either way. Changing the unaccent dictionary needs a REINDEX of
-- this index, like every index built on app.immutable_unaccent (00001).
--
-- The index reads full_name and email only, so an update of any other column of a
-- user (a password, a role, a disable) stays a heap-only update. A write to
-- either column, and every insert, adds to the gin pending list.
--
-- CONCURRENTLY, because users is written to all the time. A build that is
-- interrupted leaves the index INVALID, which the planner ignores, and goose has
-- not recorded this file. The drop comes first so the rerun removes that
-- leftover and builds the index again: with IF NOT EXISTS alone the rerun finds
-- the name taken, does nothing, and records the file as applied over an index
-- that is never used (00133 does the same).
--
-- Grants are untouched: an index needs none.
DROP INDEX CONCURRENTLY IF EXISTS app.users_search_trgm_idx;

CREATE INDEX CONCURRENTLY users_search_trgm_idx
  ON app.users USING gin (app.immutable_unaccent(lower(full_name || ' ' || email)) gin_trgm_ops);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.users_search_trgm_idx;
