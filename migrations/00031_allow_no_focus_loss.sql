-- +goose Up

ALTER TABLE app.assignments DROP CONSTRAINT assignments_integrity_max_focus_loss_check;
ALTER TABLE app.assignments ADD CONSTRAINT assignments_integrity_max_focus_loss_check
  CHECK (integrity_max_focus_loss >= -1);

-- +goose Down

UPDATE app.assignments SET integrity_max_focus_loss = 1 WHERE integrity_max_focus_loss = -1;
ALTER TABLE app.assignments DROP CONSTRAINT assignments_integrity_max_focus_loss_check;
ALTER TABLE app.assignments ADD CONSTRAINT assignments_integrity_max_focus_loss_check
  CHECK (integrity_max_focus_loss >= 0);
