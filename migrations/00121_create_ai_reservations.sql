-- +goose Up

-- The credits held for one AI call in flight (T-D4.31). A call reserves its
-- ceiling before it reaches the provider; settling it moves what was metered
-- from reserved to used and deletes the row, releasing it returns the ceiling.
-- A row that outlives its call (the process died) is returned by the sweep
-- after ten minutes, so a crash never keeps credits from the pool. The foreign
-- key makes a reservation impossible without its teacher's row, and so the
-- month's, and holds that row while a call is in flight.
--
-- feature is a checked slug and not an enum, as notifications.kind is (D-28):
-- a release that adds a feature adds no migration.
CREATE TABLE app.ai_reservations (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  month      date NOT NULL,
  user_id    uuid NOT NULL,
  feature    text NOT NULL,
  credits    bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ai_reservations_owner_fkey
    FOREIGN KEY (month, user_id) REFERENCES app.ai_teacher_credits (month, user_id),
  CONSTRAINT ai_reservations_feature_check
    CHECK (feature ~ '^[a-z][a-z_]{0,39}$'),
  CONSTRAINT ai_reservations_credits_check
    CHECK (credits BETWEEN 1 AND 100000)
);

CREATE INDEX ai_reservations_owner_idx ON app.ai_reservations (month, user_id);
CREATE INDEX ai_reservations_created_idx ON app.ai_reservations (created_at);

-- +goose Down
DROP TABLE app.ai_reservations;
