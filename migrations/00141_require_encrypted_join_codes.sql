-- +goose Up

-- T-R5.1: a live join code is a sealed one. R4 (T-R4.22) replaced every
-- unrevoked, unexpired legacy (SHA-256) code at the v0.10.0 start-up, and from
-- v0.12.0 a code is found only through the keyed lookup, so a live legacy row
-- would be a code no student can redeem. The DO block refuses to go on while
-- one exists; a legacy code left unrevoked only because it expired is revoked
-- at its expiry, which is when it stopped working. The table holds a few
-- hundred rows: VALIDATE takes SHARE UPDATE EXCLUSIVE and blocks no read or
-- write.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.class_join_codes
              WHERE revoked_at IS NULL AND expires_at > now() AND code_ciphertext IS NULL) THEN
    RAISE EXCEPTION 'class_join_codes holds % live join code(s) without a ciphertext; start v0.10.0 or later once to rotate them first',
      (SELECT count(*) FROM app.class_join_codes
        WHERE revoked_at IS NULL AND expires_at > now() AND code_ciphertext IS NULL);
  END IF;
END;
$$;
-- +goose StatementEnd

UPDATE app.class_join_codes
   SET revoked_at = expires_at
 WHERE revoked_at IS NULL AND code_ciphertext IS NULL;

ALTER TABLE app.class_join_codes
  ADD CONSTRAINT class_join_codes_live_encrypted
    CHECK (revoked_at IS NOT NULL OR code_ciphertext IS NOT NULL) NOT VALID;

ALTER TABLE app.class_join_codes
  VALIDATE CONSTRAINT class_join_codes_live_encrypted;

-- +goose Down

-- The revocations stay: a legacy code past its expiry redeems nothing either way.
ALTER TABLE app.class_join_codes
  DROP CONSTRAINT IF EXISTS class_join_codes_live_encrypted;
