package db_test

import (
	"database/sql"
	"testing"
)

// [D-13] The deferrable ordinal uniques exist so drag-and-drop reordering (§8)
// can write the new ordinals directly, instead of the two-phase negative-offset
// dance -- move everything to -1,-2,-3, then back -- that exists purely to dodge
// a uniqueness check and doubles the writes.

// TestSetBasedPermutationNeedsNoDeferral records the behaviour that makes the
// plan's stated justification wrong, so nobody re-derives it from the doc.
func TestSetBasedPermutationNeedsNoDeferral(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "single_choice", "Chọn đáp án đúng")
		seedOptions(t, tx, q)

		mustExec(t, tx,
			`UPDATE app.question_options SET ordinal = (ordinal + 1) % 3 WHERE question_id = $1`, q)

		if got, want := optionOrder(t, tx, q), []string{"C", "A", "B"}; !equalOrder(got, want) {
			t.Errorf("order after a set-based permutation = %v, want %v", got, want)
		}
	})
}

// TestPerRowReorderNeedsDeferral is the case D-13 actually exists for: writing
// the new ordinals one statement at a time, as a reorder request does.
func TestPerRowReorderNeedsDeferral(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "single_choice", "Chọn đáp án đúng")
		seedOptions(t, tx, q)

		// Moving A from 0 to 1 collides with B, which has not moved yet.
		rejectsWith(t, tx, "question_options_ordinal_key",
			`UPDATE app.question_options SET ordinal = 1 WHERE question_id = $1 AND text = 'A'`, q)
	})
}

// TestPerRowReorderSucceedsWhenDeferred completes the pair: the same sequence,
// deferred, is the reorder the builder issues.
func TestPerRowReorderSucceedsWhenDeferred(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "single_choice", "Chọn đáp án đúng")
		seedOptions(t, tx, q)

		mustExec(t, tx, `SET CONSTRAINTS app.question_options_ordinal_key DEFERRED`)
		// A -> 1, B -> 2, C -> 0, one row at a time, colliding at every step.
		mustExec(t, tx, `UPDATE app.question_options SET ordinal = 1 WHERE question_id = $1 AND text = 'A'`, q)
		mustExec(t, tx, `UPDATE app.question_options SET ordinal = 2 WHERE question_id = $1 AND text = 'B'`, q)
		mustExec(t, tx, `UPDATE app.question_options SET ordinal = 0 WHERE question_id = $1 AND text = 'C'`, q)
		mustExec(t, tx, `SET CONSTRAINTS app.question_options_ordinal_key IMMEDIATE`)

		if got, want := optionOrder(t, tx, q), []string{"C", "A", "B"}; !equalOrder(got, want) {
			t.Errorf("order after a deferred per-row reorder = %v, want %v", got, want)
		}
	})
}

// TestDeferralDoesNotForgiveARealDuplicate: deferring moves WHEN the check runs,
// not whether it runs. A genuine duplicate must still fail, or D-13 would have
// traded a constraint for a convenience.
func TestDeferralDoesNotForgiveARealDuplicate(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "single_choice", "Chọn đáp án đúng")
		seedOptions(t, tx, q)

		mustExec(t, tx, `SET CONSTRAINTS app.question_options_ordinal_key DEFERRED`)
		// Two options both claiming ordinal 0 -- not a permutation.
		mustExec(t, tx, `UPDATE app.question_options SET ordinal = 0 WHERE question_id = $1`, q)
		rejectsWith(t, tx, "question_options_ordinal_key",
			`SET CONSTRAINTS app.question_options_ordinal_key IMMEDIATE`)
	})
}

// TestBlankOrdinalsAreDeferrableToo covers the other draft-editable ordinal
// unique, since the editor reorders blanks by the same gesture.
func TestBlankOrdinalsAreDeferrableToo(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "fill_blank", "Điền {{1}} và {{2}}")
		mustExec(t, tx, `INSERT INTO app.question_blanks (question_id, ordinal) VALUES ($1,1),($1,2)`, q)

		// Per-row, the case that needs deferral.
		mustExec(t, tx, `SET CONSTRAINTS app.question_blanks_ordinal_key DEFERRED`)
		mustExec(t, tx, `UPDATE app.question_blanks SET ordinal = 2 WHERE question_id = $1 AND ordinal = 1`, q)
		mustExec(t, tx, `UPDATE app.question_blanks SET ordinal = 1 WHERE question_id = $1 AND ordinal = 2 AND id <> (
		                   SELECT id FROM app.question_blanks WHERE question_id = $1 AND ordinal = 2 ORDER BY id LIMIT 1)`, q)
		mustExec(t, tx, `SET CONSTRAINTS app.question_blanks_ordinal_key IMMEDIATE`)
	})
}

func seedOptions(t *testing.T, tx *sql.Tx, questionID string) {
	t.Helper()
	mustExec(t, tx,
		`INSERT INTO app.question_options (question_id, ordinal, text, is_correct)
		 VALUES ($1,0,'A',true), ($1,1,'B',false), ($1,2,'C',false)`, questionID)
}

// optionOrder returns the option texts indexed by their ordinal.
func optionOrder(t *testing.T, tx *sql.Tx, questionID string) []string {
	t.Helper()
	rows, err := tx.Query(
		`SELECT text FROM app.question_options WHERE question_id = $1 ORDER BY ordinal`, questionID)
	if err != nil {
		t.Fatalf("reading options: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var order []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatalf("scan: %v", err)
		}
		order = append(order, text)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading options: %v", err)
	}
	return order
}

func equalOrder(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
