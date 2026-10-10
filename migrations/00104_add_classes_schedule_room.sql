-- +goose Up

-- When and where a class meets, as the teacher words it (T-R4.18): a
-- label such as "Thứ 3, 5 · 18:00" and a room. Both are free text, nullable
-- with no default, so the previous release's insert keeps working; a class made
-- before these columns, or without either, reads NULL. The commands trim and
-- store NULL for a value that is blank, never an empty string, so each check
-- meets only 1 to 120 and 1 to 60 characters. The checks scan the table once
-- while the columns are added; classes holds a handful of rows per teacher.
ALTER TABLE app.classes
  ADD COLUMN schedule_label text
    CONSTRAINT classes_schedule_label_check
    CHECK (char_length(schedule_label) BETWEEN 1 AND 120),
  ADD COLUMN room text
    CONSTRAINT classes_room_check
    CHECK (char_length(room) BETWEEN 1 AND 60);

-- +goose Down

ALTER TABLE app.classes
  DROP CONSTRAINT classes_room_check,
  DROP CONSTRAINT classes_schedule_label_check,
  DROP COLUMN room,
  DROP COLUMN schedule_label;
