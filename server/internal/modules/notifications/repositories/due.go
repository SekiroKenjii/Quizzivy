package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/schedule"
	"quizzivy/internal/shared/visibility"
)

const (
	dueOpened  = "opened"
	dueSoon    = "due_soon"
	dueReady   = "ready"
	dueClosing = "closing"
)

const handedIn = `(at.status IN ('submitted','timed_out','graded')
	               OR (at.status = 'in_progress' AND at.deadline_at <= $2::timestamptz))`

var dueQuery = `
WITH me AS (
  SELECT u.id
    FROM app.users u
   WHERE u.id = $1::uuid AND u.disabled_at IS NULL
     AND u.role_id IN (SELECT r.id FROM app.student_like_roles r)
), mine AS (
  SELECT a.id AS assignment_id, t.title,
         greatest(a.opens_at, a.published_at) AS available_at,
         ` + schedule.CloseOf("o") + ` AS closes_at,
         a.max_attempts + coalesce(o.extra_attempts, 0) AS allowed,
         a.review_release, a.review_show_score,
         EXISTS (SELECT 1 FROM app.attempts at
                  WHERE at.assignment_id = a.id AND at.student_id = me.id
                    AND ` + handedIn + `) AS handed_in,
         (SELECT count(*) FROM app.attempts at
           WHERE at.assignment_id = a.id AND at.student_id = me.id
             AND at.status <> 'voided') AS spent
    FROM me
    JOIN (SELECT ast.assignment_id FROM app.assignment_students ast WHERE ast.user_id = $1::uuid
           UNION
          SELECT ac.assignment_id FROM app.class_members m
            JOIN app.assignment_classes ac ON ac.class_id = m.class_id
           WHERE m.user_id = $1::uuid) targeted ON true
    JOIN app.assignments a ON a.id = targeted.assignment_id AND a.published_at IS NOT NULL
    JOIN app.tests t ON t.id = a.test_id
    ` + schedule.OverrideJoin("me.id") + `
   WHERE ` + schedule.CloseOf("o") + ` > $3::timestamptz
)
SELECT '` + dueOpened + `', m.assignment_id::text, NULL::text, m.title, m.closes_at, NULL::text, 0
  FROM mine m
 WHERE m.available_at > $3::timestamptz AND m.available_at <= $2::timestamptz
   AND $2::timestamptz < m.closes_at
UNION ALL
SELECT '` + dueSoon + `', m.assignment_id::text, NULL::text, m.title, m.closes_at, lead.label, 0
  FROM mine m
 CROSS JOIN (VALUES ('` + string(domain.LeadDay) + `', interval '24 hours'),
                    ('` + string(domain.LeadHour) + `', interval '1 hour')) AS lead(label, before)
 WHERE NOT m.handed_in AND m.spent < m.allowed
   AND m.closes_at - lead.before > $3::timestamptz AND m.closes_at - lead.before <= $2::timestamptz
   AND m.closes_at - lead.before >= m.available_at
   AND $2::timestamptz < m.closes_at
UNION ALL
SELECT '` + dueReady + `', m.assignment_id::text, at.id::text, m.title, m.closes_at, NULL::text, 0
  FROM mine m
  JOIN app.attempts at ON at.assignment_id = m.assignment_id AND at.student_id = $1::uuid
 WHERE m.review_release = 'after_close' AND m.review_show_score
   AND m.closes_at > $3::timestamptz AND m.closes_at <= $2::timestamptz
   AND at.status IN ('submitted','timed_out','graded')
   AND NOT EXISTS (SELECT 1 FROM app.attempt_answers ans
                    WHERE ans.attempt_id = at.id AND ans.requires_manual AND ans.manual_score IS NULL)
UNION ALL
SELECT '` + dueClosing + `', a.id::text, NULL::text, t.title, ` + schedule.CloseOf("") + `, NULL::text, waiting.n::int
  FROM app.assignments a
  JOIN app.tests t ON t.id = a.test_id
  CROSS JOIN LATERAL (
       SELECT count(*) AS n
         FROM ` + visibility.Roster("a.id") + ` r
         JOIN app.users s ON s.id = r.user_id AND s.disabled_at IS NULL
                         AND s.role_id IN (SELECT k.id FROM app.student_like_roles k)
         LEFT JOIN app.assignment_student_overrides o ON o.assignment_id = a.id AND o.student_id = s.id
        WHERE ` + visibility.Papers(4, 1, "a.id", "s.id") + `
          AND ` + schedule.CloseOf("o") + ` = ` + schedule.CloseOf("") + `
          AND NOT EXISTS (SELECT 1 FROM app.attempts at
                           WHERE at.assignment_id = a.id AND at.student_id = s.id
                             AND ` + handedIn + `)) waiting
 WHERE a.published_at IS NOT NULL AND a.id IN ` + visibility.AssignmentIDs(1) + `
   AND ` + schedule.CloseOf("") + ` - interval '1 hour' > $3::timestamptz
   AND ` + schedule.CloseOf("") + ` - interval '1 hour' <= $2::timestamptz
   AND $2::timestamptz < ` + schedule.CloseOf("") + `
   AND waiting.n > 0`

type dueRow struct {
	what         string
	assignmentID string
	attemptID    *string
	title        string
	closesAt     time.Time
	lead         *string
	notSubmitted int
}

// Due reads, for userID as a student, the assignments that opened, the closes
// a day and an hour away, and the results an after-close release has made
// ready; and, for userID as a teacher, the assignments that close within the
// hour with a student they reach still to hand in. Every moment lies in
// (now - DueLookback, now]. A student's close is the one their override and an
// early close make it (schedule.CloseOf), and a teacher is counted only the
// students visibility.Papers lets them see, whose own close is the
// assignment's. A student who has handed an attempt in, or has none left, is
// not reminded.
func (p *Postgres) Due(ctx context.Context, userID string, now time.Time) ([]domain.Notice, error) {
	rows, err := db.QueryMany(ctx, p, dueQuery, []any{userID, now, now.Add(-domain.DueLookback), false}, scanDueRow)
	if err != nil {
		return nil, fmt.Errorf("notifications: read due items: %w", err)
	}
	notices := make([]domain.Notice, len(rows))
	for i, row := range rows {
		notices[i] = row.notice(userID)
	}
	return notices, nil
}

func scanDueRow(rows pgx.Rows) (dueRow, error) {
	var r dueRow
	err := rows.Scan(&r.what, &r.assignmentID, &r.attemptID, &r.title, &r.closesAt, &r.lead, &r.notSubmitted)
	return r, err
}

func (r dueRow) notice(userID string) domain.Notice {
	n := domain.Notice{UserID: userID, Merge: domain.Replace}
	assignment := &domain.Target{Route: domain.RouteAssignment, AssignmentID: r.assignmentID}
	student := &domain.Target{Route: domain.RouteStudentAssignment, AssignmentID: r.assignmentID}
	switch r.what {
	case dueOpened:
		n.Kind, n.Params, n.Target = domain.AssignmentOpened, domain.Opened{Title: r.title, ClosesAt: r.closesAt}, student
		n.DedupeKey = domain.OpenedKey(r.assignmentID)
	case dueSoon:
		n.Kind, n.Params, n.Target = domain.AssignmentDueSoon, domain.DueSoon{Title: r.title, ClosesAt: r.closesAt}, student
		n.DedupeKey = domain.DueSoonKey(r.assignmentID, r.closesAt, domain.Lead(*r.lead))
	case dueReady:
		n.Kind, n.Params = domain.ResultReady, domain.Ready{Title: r.title}
		n.Target = &domain.Target{Route: domain.RouteResult, AssignmentID: r.assignmentID, AttemptID: *r.attemptID}
		n.DedupeKey = domain.ReadyKey(*r.attemptID)
	case dueClosing:
		n.Kind, n.Params, n.Target = domain.AssignmentClosing, domain.Closing{Title: r.title, NotSubmitted: r.notSubmitted}, assignment
		n.DedupeKey = domain.ClosingKey(r.assignmentID, r.closesAt)
	}
	return n
}

const insertAbsent = `
	INSERT INTO app.notifications (user_id, kind, params, target, dedupe_key)
	SELECT $1::uuid, n.kind, n.params::jsonb, n.target::jsonb, n.dedupe_key
	  FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::text[])
	         AS n(kind, params, target, dedupe_key, event)
	 WHERE NOT EXISTS (
	       SELECT 1 FROM app.notification_preferences p
	        WHERE p.user_id = $1::uuid AND p.event = n.event AND NOT p.in_app)
	ON CONFLICT (user_id, dedupe_key) DO NOTHING`

// InsertAbsent writes the notices in one statement, each under the same
// switch Upsert applies, and leaves any row the user already holds under a
// notice's dedupe key as it is: unlike Upsert it never merges and never makes
// a read notification unread.
func (p *Postgres) InsertAbsent(ctx context.Context, userID string, notices []domain.Notice) (int, error) {
	if len(notices) == 0 {
		return 0, nil
	}
	kinds := make([]string, len(notices))
	params := make([]string, len(notices))
	targets := make([]*string, len(notices))
	keys := make([]string, len(notices))
	events := make([]*string, len(notices))
	for i, n := range notices {
		encoded, err := domain.Encode(n.Params)
		if err != nil {
			return 0, err
		}
		kinds[i], params[i], keys[i] = string(n.Kind), string(encoded), n.DedupeKey
		if n.Target != nil {
			target, err := json.Marshal(n.Target)
			if err != nil {
				return 0, fmt.Errorf("notifications: target: %w", err)
			}
			targets[i] = new(string(target))
		}
		if event, ok := n.Kind.Event(); ok {
			events[i] = new(string(event))
		}
	}
	tag, err := p.Exec(ctx, insertAbsent, userID, kinds, params, targets, keys, events)
	if err != nil {
		return 0, fmt.Errorf("notifications: insert due items: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
