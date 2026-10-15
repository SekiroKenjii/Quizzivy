// Package retention holds the one rule for which attempts a retention cutoff
// reaches, as SQL, so the sweep, its dry run and the settings page's impact
// count agree on what an answer's age is.
package retention

import (
	"fmt"

	"quizzivy/internal/shared/schedule"
)

// RemovedAnswer is the JSON payload that replaces an answer removed under the
// retention policy.
const RemovedAnswer = `{"removed": true}`

// AttemptJoins is what follows FROM app.attempts at for the two rules below:
// the attempt's assignment, aliased a, and the override of the attempt's own
// student, aliased o, read NULL when there is none. The override is keyed on the
// attempt's columns, not on a.id, so that its primary key answers it however many
// overrides one student holds.
var AttemptJoins = `JOIN app.assignments a ON a.id = at.assignment_id
	LEFT JOIN app.assignment_student_overrides o ON o.assignment_id = at.assignment_id AND o.student_id = at.student_id`

// EndedBefore is the rule as a boolean SQL expression over AttemptJoins, with
// cutoff a SQL expression for the instant: the attempt has ended (submitted,
// timed out, graded or voided) before it, and the assignment closed for its
// student before it (schedule.CloseOf), so a student whose override keeps the
// assignment open past cutoff keeps their answers. An attempt ends at
// submitted_at, or at its deadline when it was voided before it was submitted.
// The statuses are listed so that a status added later has to be considered
// here, and an attempt in progress is never reached, however overdue: closing it
// scores it from the stored answers.
func EndedBefore(cutoff string) string {
	return fmt.Sprintf(`at.status IN ('submitted', 'timed_out', 'graded', 'voided')
	AND coalesce(at.submitted_at, at.deadline_at) < %[1]s AND %[2]s < %[1]s`, cutoff, schedule.CloseOf("o"))
}

// OverdueInProgressBefore is the attempts EndedBefore cannot reach for want of
// a close: in progress, with the deadline and the assignment's close for the
// student both before cutoff, over AttemptJoins.
func OverdueInProgressBefore(cutoff string) string {
	return fmt.Sprintf(`at.status = 'in_progress' AND at.deadline_at < %[1]s AND %[2]s < %[1]s`, cutoff, schedule.CloseOf("o"))
}
