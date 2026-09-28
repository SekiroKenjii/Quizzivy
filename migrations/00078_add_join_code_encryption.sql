-- +goose Up

-- Join codes stored encrypted and found by a keyed hash (T-R2.14a, D5). A new
-- row seals the code with AES-256-GCM under the key key_id names, bound to its
-- class and code ids, and stores in code_hash the HMAC-SHA256 of the code under
-- that key's lookup key (lookup_scheme 2). A v0.7.0 row keeps its SHA-256
-- code_hash and no ciphertext (lookup_scheme 1), and the v0.7.0 binary's insert,
-- which names none of these columns, is such a row. Constant defaults and NULL
-- columns are metadata-only on PG18; the checks scan a table of a few hundred
-- rows.
ALTER TABLE app.class_join_codes
  ADD COLUMN code_ciphertext bytea,
  ADD COLUMN key_id smallint,
  ADD COLUMN lookup_scheme smallint NOT NULL DEFAULT 1,
  ADD CONSTRAINT class_join_codes_ciphertext_length
    CHECK (code_ciphertext IS NULL OR length(code_ciphertext) = 36),
  ADD CONSTRAINT class_join_codes_key_id_nonzero
    CHECK (key_id IS NULL OR key_id <> 0),
  ADD CONSTRAINT class_join_codes_lookup_scheme_known
    CHECK (lookup_scheme IN (1, 2)),
  ADD CONSTRAINT class_join_codes_scheme_consistent
    CHECK ((lookup_scheme = 2) = (code_ciphertext IS NOT NULL AND key_id IS NOT NULL));

-- +goose Down

-- The previous binary looks a code up by its SHA-256 alone, so a live keyed
-- code would stop redeeming without a word. Revoke or let them expire first.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.class_join_codes
              WHERE lookup_scheme = 2 AND revoked_at IS NULL AND expires_at > now()) THEN
    RAISE EXCEPTION 'class_join_codes holds % live encrypted join code(s) the previous binary cannot look up; revoke them first',
      (SELECT count(*) FROM app.class_join_codes
        WHERE lookup_scheme = 2 AND revoked_at IS NULL AND expires_at > now());
  END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE app.class_join_codes
  DROP CONSTRAINT class_join_codes_scheme_consistent,
  DROP CONSTRAINT class_join_codes_lookup_scheme_known,
  DROP CONSTRAINT class_join_codes_key_id_nonzero,
  DROP CONSTRAINT class_join_codes_ciphertext_length,
  DROP COLUMN lookup_scheme,
  DROP COLUMN key_id,
  DROP COLUMN code_ciphertext;
