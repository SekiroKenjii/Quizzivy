-- +goose Up

-- PUBLIC may create temporary objects in any new database, and pg_temp is
-- where a SECURITY DEFINER function with a loose search_path is attacked
-- from. The application creates none, so the privilege goes; the database's
-- owner keeps it. Only the owner may revoke it: where the migration role
-- does not own the database, this says so and changes nothing.
-- +goose StatementBegin
DO $$
BEGIN
  IF pg_has_role(current_user,
                 (SELECT datdba FROM pg_database WHERE datname = current_database()),
                 'USAGE') THEN
    EXECUTE format('REVOKE TEMPORARY ON DATABASE %I FROM PUBLIC', current_database());
  ELSE
    RAISE NOTICE 'TEMPORARY on database % stays granted to PUBLIC: % does not own it, its owner must revoke it',
      current_database(), current_user;
  END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DO $$
BEGIN
  IF pg_has_role(current_user,
                 (SELECT datdba FROM pg_database WHERE datname = current_database()),
                 'USAGE') THEN
    EXECUTE format('GRANT TEMPORARY ON DATABASE %I TO PUBLIC', current_database());
  END IF;
END;
$$;
-- +goose StatementEnd
