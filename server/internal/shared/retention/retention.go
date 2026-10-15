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

// EndedBefore is the rule as a boolean SQL expression over a row of
// app.attempts aliased at and its row of app.assignments aliased a, with
// cutoff a SQL expression for the instant: the attempt has ended (submitted,
// timed out, graded or voided) before it, and its assignment closed before it
// (schedule.CloseOf). An attempt ends at submitted_at, or at its deadline when
// it was voided before it was submitted. The statuses are listed so that a
// status added later has to be considered here, and an attempt in progress is
// never reached, however overdue: closing it scores it from the stored answers.
func EndedBefore(cutoff string) string {
	return fmt.Sprintf(`at.status IN ('submitted', 'timed_out', 'graded', 'voided')
	AND coalesce(at.submitted_at, at.deadline_at) < %[1]s AND %[2]s < %[1]s`, cutoff, schedule.CloseOf(""))
}

// OverdueInProgressBefore is the attempts EndedBefore cannot reach for want of
// a close: in progress, with the deadline and the assignment's close both
// before cutoff, over the same aliases.
func OverdueInProgressBefore(cutoff string) string {
	return fmt.Sprintf(`at.status = 'in_progress' AND at.deadline_at < %[1]s AND %[2]s < %[1]s`, cutoff, schedule.CloseOf(""))
}
