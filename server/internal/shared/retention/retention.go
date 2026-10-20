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

// ClosedBefore is the boolean SQL expression over AttemptJoins that the
// assignment closed for the attempt's student before cutoff, a SQL expression for
// the instant: its early close or the end of its window, or the student's
// override when that closes later (schedule.CloseOf). A student whose override
// keeps the assignment open past cutoff can still sit it.
func ClosedBefore(cutoff string) string {
	return fmt.Sprintf(`%s < %s`, schedule.CloseOf("o"), cutoff)
}

// EndedBefore is the rule as a boolean SQL expression over AttemptJoins, with
// cutoff a SQL expression for the instant: the attempt has ended (submitted,
// timed out, graded or voided) before it, and the assignment closed for its
// student before it (ClosedBefore), so a student whose override keeps the
// assignment open past cutoff keeps their answers. An attempt ends at
// submitted_at, or at its deadline when it was voided before it was submitted.
// The statuses are listed so that a status added later has to be considered
// here, and an attempt in progress is never reached, however overdue: closing it
// scores it from the stored answers.
func EndedBefore(cutoff string) string {
	return fmt.Sprintf(`at.status IN ('submitted', 'timed_out', 'graded', 'voided')
	AND coalesce(at.submitted_at, at.deadline_at) < %[1]s AND %[2]s`, cutoff, ClosedBefore(cutoff))
}

// OverdueInProgressBefore is the attempts EndedBefore cannot reach for want of
// a close: in progress, with the deadline and the assignment's close for the
// student both before cutoff, over AttemptJoins.
func OverdueInProgressBefore(cutoff string) string {
	return fmt.Sprintf(`at.status = 'in_progress' AND at.deadline_at < %[1]s AND %[2]s`, cutoff, ClosedBefore(cutoff))
}

// Cutoff is the SQL expression for the instant months months before now on
// the UTC calendar, with months a SQL expression for an integer: the cutoff
// every rule above is given.
func Cutoff(months string) string {
	return fmt.Sprintf(`(CURRENT_TIMESTAMP AT TIME ZONE 'UTC' - make_interval(months => %s)) AT TIME ZONE 'UTC'`, months)
}

// AnonymousEmail is the email an anonymised account over app.users u holds.
const AnonymousEmail = `'anonymous-' || u.id::text || '@anonymous.invalid'`

// NotAnonymous is the boolean SQL expression over app.users u that the
// account has not been anonymised.
const NotAnonymous = `u.email <> ` + AnonymousEmail

// OwnsContent is the boolean SQL expression over app.users u that the account
// teaches a class or owns a test, question, group, media asset or assignment,
// which keeps it from being anonymised.
const OwnsContent = `(EXISTS (SELECT 1 FROM app.classes c WHERE c.teacher_id = u.id)
  OR EXISTS (SELECT 1 FROM app.tests t WHERE t.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.questions q WHERE q.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.question_groups g WHERE g.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.media_assets m WHERE m.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.assignments s WHERE s.created_by = u.id))`

// HasAttemptInProgress is the boolean SQL expression over app.users u that
// the account has an attempt in progress, which defers its anonymising.
const HasAttemptInProgress = `EXISTS (SELECT 1 FROM app.attempts p WHERE p.student_id = u.id AND p.status = 'in_progress')`

// CountAnswers counts, for the cutoff $1, the attempts that hold an answer the
// answers rule would remove and those answers, in that order.
var CountAnswers = `SELECT count(DISTINCT x.attempt_id), count(*)
	 FROM app.attempt_answers x
	 JOIN app.attempts at ON at.id = x.attempt_id ` + AttemptJoins + `
	WHERE ` + EndedBefore("$1") + ` AND x.payload <> '` + RemovedAnswer + `'::jsonb`

// EventsDue is what follows SELECT for the integrity events, aliased e, the
// events rule reaches for the cutoff $1: received and closed for the student
// before it.
var EventsDue = `FROM app.attempt_events e
 JOIN app.attempts at ON at.id = e.attempt_id ` + AttemptJoins + `
 WHERE e.received_at < $1 AND ` + ClosedBefore("$1")

// CountEvents counts the integrity events the events rule reaches for the
// cutoff $1.
var CountEvents = `SELECT count(e.id) ` + EventsDue

// CountAccounts counts the accounts disabled before the cutoff $1 and not yet
// anonymised, split three ways so each is counted once: those the accounts rule
// would anonymise, those it leaves because they own content, and those it
// leaves for now because an attempt is in progress.
const CountAccounts = `WITH due AS MATERIALIZED (
	SELECT ` + OwnsContent + ` AS owner, ` + HasAttemptInProgress + ` AS busy
	  FROM app.users u
	 WHERE u.disabled_at < $1 AND ` + NotAnonymous + `)
	SELECT count(*) FILTER (WHERE NOT owner AND NOT busy), count(*) FILTER (WHERE owner),
	       count(*) FILTER (WHERE NOT owner AND busy)
	  FROM due`
