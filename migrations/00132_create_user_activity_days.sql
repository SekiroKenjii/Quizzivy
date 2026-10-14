-- +goose Up

-- T-R5.4 (DG-54): one row for each local day (APP_TIMEZONE) on which a user
-- made an authenticated request, for the Admin console's daily chart and its
-- active counts. The API inserts a day at most once per user and instance
-- every five minutes and ignores a row that already exists, so the table
-- holds facts, and a deleted account's days go with it through the cascade.
CREATE TABLE app.user_activity_days (
  user_id uuid NOT NULL REFERENCES app.users (id) ON DELETE CASCADE,
  day     date NOT NULL,
  PRIMARY KEY (user_id, day)
);

-- The chart and the counts read a range of days across every user.
CREATE INDEX user_activity_days_day_idx ON app.user_activity_days (day);

-- +goose Down

DROP TABLE app.user_activity_days;
