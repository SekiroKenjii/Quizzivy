package students

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/paging"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/dashboard"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ActiveWindow must stay the dashboard's window. Two meanings of "active" on
// two screens is worse than one screen missing the number.
const ActiveWindow = dashboard.ActiveWindow

// Membership is one class the student is in, and how they got there (D-10).
type Membership struct {
	ID        string
	Name      string
	JoinedVia string
	JoinedAt  time.Time
}

// Stats are §8's per-student teaching figures.
//
// Kept off Student's own identity fields for the same reason they are kept off
// the User schema: the login payload has no business carrying them.
type Stats struct {
	SubmittedCount int
	ScoreEarned    *float64
	ScoreTotal     *float64
	PendingManual  int
	FlaggedCount   int
	LiveAttempt    bool
	LastAttemptAt  *time.Time
}

// Student is §7's User narrowed to the role this listing returns, plus what
// G-07 draws beside it.
type Student struct {
	ID                 string
	Email              string
	FullName           string
	HasPassword        bool
	LinkedProviders    []string
	MustChangePassword bool
	CreatedAt          time.Time
	DisabledAt         *time.Time
	Classes            []Membership
	Stats              Stats
}

// Facets are G-07's header: "31 học viên · 23 hoạt động 7 ngày qua".
type Facets struct {
	Total           int
	ActiveLast7Days int
}

// Status selects which accounts a listing returns.
type Status string

const (
	Active   Status = "active"
	Disabled Status = "disabled"
	AnyState Status = "all"
)

type ListInput struct {
	// Status defaults to Active when empty.
	Status  Status
	Query   string
	ClassID string
	Page    int
	Limit   int
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// searchCondition and classCondition are shared by List and Facets so the
// header cannot end up counting a different set from the rows beneath it.
const searchCondition = `(
		app.immutable_unaccent(lower(u.full_name))
			LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'
		OR lower(u.email) LIKE '%%' || lower($%[1]d) || '%%' ESCAPE '\'
	)`

const classCondition = `EXISTS (SELECT 1 FROM app.class_members m
		  WHERE m.user_id = u.id AND m.class_id = $%d::uuid)`

// statusCondition is empty for "all": a listing that can never return a
// disabled account makes updateStudent's `disabled: false` unreachable.
func statusCondition(status Status) string {
	switch status {
	case Disabled:
		return `u.disabled_at IS NOT NULL`
	case AnyState:
		return `TRUE`
	default:
		return `u.disabled_at IS NULL`
	}
}

// StatsColumns and StatsJoins are the aggregates behind Stats, keyed on a users
// row aliased `u`, so another roster (G-06's class members) can carry the same
// figures without re-deriving them; StatsScanner reads the columns back.
const StatsColumns = `best.submitted_count, best.earned, best.total, best.pending_manual,
		       act.flagged_count, act.live, act.last_attempt_at`

const StatsJoins = `
		  LEFT JOIN LATERAL (
		    SELECT (SELECT count(DISTINCT a.assignment_id)
	              FROM app.attempts a
	             WHERE a.student_id = u.id
	               AND a.status IN ('submitted','timed_out','graded')) AS submitted_count,
		           sum(g.score_earned)            AS earned,
		           sum(g.score_total)             AS total,
		           coalesce(sum(g.pending_manual), 0) AS pending_manual
		      FROM (
		        -- One row per assignment: the best GRADED attempt, plus whether
		        -- that assignment has any submitted work at all.
		        SELECT DISTINCT ON (a.assignment_id)
		               a.assignment_id,
		               a.score_earned,
		               coalesce(a.score_total, v.total_points) AS score_total,
		               (SELECT count(*) FROM app.attempt_answers ans
		                 WHERE ans.attempt_id = a.id
		                   AND ans.requires_manual AND ans.manual_score IS NULL)
		                 AS pending_manual
		          FROM app.attempts a
		          JOIN app.test_versions v ON v.id = a.test_version_id
		         WHERE a.student_id = u.id
		           AND a.status = 'graded'
		           AND a.score_earned IS NOT NULL
		         ORDER BY a.assignment_id,
		                  a.score_earned / coalesce(a.score_total, v.total_points) DESC,
		                  a.attempt_no DESC
		      ) g
		  ) best ON TRUE

		  LEFT JOIN LATERAL (
		    SELECT count(*) FILTER (WHERE a.flagged AND a.status <> 'voided') AS flagged_count,
		           -- Nothing flips a stale in-progress attempt: there is no
		           -- scheduler anywhere in this system (D-18). Without the
		           -- deadline test an abandoned attempt reads "đang làm bài"
		           -- for ever.
		           bool_or(a.status = 'in_progress' AND a.deadline_at > now()) AS live,
		           -- The last time they TOUCHED it, not the last time they
		           -- submitted. An attempt that was started and abandoned keeps
		           -- submitted_at NULL for ever (nothing flips it -- D-18), so
		           -- max(submitted_at) reported that student as having never
		           -- started anything, while the header counted them active
		           -- because Facets keys off started_at. One screen, two answers.
		           -- GREATEST skips NULLs, so a submitted attempt still reports
		           -- its submission.
		           max(greatest(a.submitted_at, a.started_at)) AS last_attempt_at
		      FROM app.attempts a
		     WHERE a.student_id = u.id AND a.status <> 'voided'
		  ) act ON TRUE
`

// selectStudents is one statement for a whole page: the aggregates are lateral
// subqueries over the page's rows, never a query per student. N+1 on a list
// screen is §13.8's named default failure mode.
const selectStudents = `
		SELECT u.id::text, u.email, u.full_name,
		       u.password_hash IS NOT NULL,
		       coalesce((SELECT array_agg(i.provider::text)
		                   FROM app.user_identities i WHERE i.user_id = u.id), '{}'),
		       u.must_change_password, u.created_at, u.disabled_at,

		       coalesce((SELECT jsonb_agg(jsonb_build_object(
		                          'id', c.id, 'name', c.name,
		                          'joinedVia', m.joined_via, 'joinedAt', m.joined_at)
		                        ORDER BY m.joined_at DESC)
		                   FROM app.class_members m
		                   JOIN app.classes c ON c.id = m.class_id
		                  WHERE m.user_id = u.id), '[]'::jsonb),

		       		       ` + StatsColumns + `
		  FROM app.users u
		  ` + StatsJoins

type rowScanner interface{ Scan(dest ...any) error }

func scanStudent(row rowScanner) (Student, error) {
	var student Student
	var classes []byte
	var stats StatsScanner

	targets := append([]any{&student.ID, &student.Email, &student.FullName,
		&student.HasPassword, &student.LinkedProviders,
		&student.MustChangePassword, &student.CreatedAt, &student.DisabledAt,
		&classes}, stats.Targets()...)
	if err := row.Scan(targets...); err != nil {
		return Student{}, fmt.Errorf("students: scan: %w", err)
	}

	if err := json.Unmarshal(classes, &student.Classes); err != nil {
		return Student{}, fmt.Errorf("students: decode classes: %w", err)
	}
	student.Stats = stats.Stats()
	return student, nil
}

// StatsScanner receives StatsColumns for one student. Every aggregate is
// nullable on the way back -- a student with no attempts has no rows to sum.
type StatsScanner struct {
	submitted, pending, flagged *int
	earned, total               *float64
	live                        *bool
	last                        *time.Time
}

func (sc *StatsScanner) Targets() []any {
	return []any{&sc.submitted, &sc.earned, &sc.total, &sc.pending, &sc.flagged, &sc.live, &sc.last}
}

func (sc *StatsScanner) Stats() Stats {
	out := Stats{
		SubmittedCount: deref(sc.submitted),
		ScoreEarned:    sc.earned,
		ScoreTotal:     sc.total,
		PendingManual:  deref(sc.pending),
		FlaggedCount:   deref(sc.flagged),
		LiveAttempt:    sc.live != nil && *sc.live,
		LastAttemptAt:  sc.last,
	}
	// A total of zero would make the client divide by it.
	if out.ScoreTotal != nil && *out.ScoreTotal <= 0 {
		out.ScoreEarned, out.ScoreTotal = nil, nil
	}
	return out
}

func deref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// List returns one page of students, newest first.
//
// Search folds accents on both sides so "hân" and "han" find the same person --
// the same rule the question bank uses, and the one a Vietnamese-first product
// needs. There is no trigram index behind it: §1.3 caps this table at tens of
// rows, where an index would cost more to maintain than the scan it saves.
func (s *Store) List(ctx context.Context, in ListInput) ([]Student, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	var args []any
	where := []string{`u.role = 'student'`, statusCondition(in.Status)}

	if in.Query != "" {
		args = append(args, likeEscaper.Replace(in.Query))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}
	if in.ClassID != "" {
		args = append(args, in.ClassID)
		where = append(where, fmt.Sprintf(classCondition, len(args)))
	}
	from := `
		 WHERE ` + strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM app.users u`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("students: count: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, selectStudents+from+fmt.Sprintf(`
		 ORDER BY u.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("students: list: %w", err)
	}
	defer rows.Close()

	out := make([]Student, 0, limit)
	for rows.Next() {
		student, err := scanStudent(rows)
		if err != nil {
			return nil, paging.Page{}, err
		}
		out = append(out, student)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("students: list: %w", err)
	}
	return out, page, nil
}

var (
	ErrNotFound   = errors.New("students: not found")
	ErrEmailTaken = errors.New("students: email already in use")
)

// Get returns one student, disabled or not.
func (s *Store) Get(ctx context.Context, id string) (Student, error) {
	return s.get(ctx, id, true)
}

// get optionally sees disabled rows.
//
// Update needs that: it reads the row back after writing it, and a successful
// `disabled: true` would otherwise miss its own write and report 404 for an
// operation that landed -- telling an operator the revocation failed when it
// did not.
func (s *Store) get(ctx context.Context, id string, includeDisabled bool) (Student, error) {
	where := ` WHERE u.id = $1::uuid AND u.role = 'student'`
	if !includeDisabled {
		where += ` AND u.disabled_at IS NULL`
	}

	student, err := scanStudent(s.pool.QueryRow(ctx, selectStudents+where, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Student{}, ErrNotFound
	}
	if err != nil {
		return Student{}, err
	}
	return student, nil
}

// Facets backs G-07's header. "Active" is the dashboard's window, not a second
// definition of the same word on a second screen.
//
// Counted over the same filters the page is showing, not over the whole table:
// a header reading "31 học viên" above a search that matched one is describing
// something the teacher cannot see.
func (s *Store) Facets(ctx context.Context, in ListInput) (Facets, error) {
	args := []any{ActiveWindow}
	where := []string{`u.role = 'student'`, statusCondition(in.Status)}

	if in.Query != "" {
		args = append(args, likeEscaper.Replace(in.Query))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}
	if in.ClassID != "" {
		args = append(args, in.ClassID)
		where = append(where, fmt.Sprintf(classCondition, len(args)))
	}

	var f Facets
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM app.attempts a
		          WHERE a.student_id = u.id
		            AND a.status <> 'voided'
		            AND a.started_at > now() - $1::interval))
		  FROM app.users u
		 WHERE `+strings.Join(where, "\n		   AND "), args...).
		Scan(&f.Total, &f.ActiveLast7Days); err != nil {
		return Facets{}, fmt.Errorf("students: facets: %w", err)
	}
	return f, nil
}
