package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dashboarddomain "quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/paging"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Students struct{ pool *pgxpool.Pool }

func NewStudents(pool *pgxpool.Pool) *Students { return &Students{pool: pool} }

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// searchCondition and classCondition are shared by List and StudentFacets so the
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
func statusCondition(status domain.StudentStatus) string {
	switch status {
	case domain.StudentsDisabled:
		return `u.disabled_at IS NOT NULL`
	case domain.StudentsAny:
		return `TRUE`
	default:
		return `u.disabled_at IS NULL`
	}
}

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
		                  WHERE m.user_id = u.id), '[]'::jsonb)
		  FROM app.users u`

type rowScanner interface{ Scan(dest ...any) error }

func scanStudent(row rowScanner) (domain.Student, error) {
	var student domain.Student
	var classes []byte
	if err := row.Scan(&student.ID, &student.Email, &student.FullName,
		&student.HasPassword, &student.LinkedProviders,
		&student.MustChangePassword, &student.CreatedAt, &student.DisabledAt,
		&classes); err != nil {
		return domain.Student{}, fmt.Errorf("students: scan: %w", err)
	}
	if err := json.Unmarshal(classes, &student.Classes); err != nil {
		return domain.Student{}, fmt.Errorf("students: decode classes: %w", err)
	}
	return student, nil
}

// List returns one page of students, newest first.
//
// Search folds accents on both sides so "hân" and "han" find the same person --
// the same rule the question bank uses, and the one a Vietnamese-first product
// needs. There is no trigram index behind it: §1.3 caps this table at tens of
// rows, where an index would cost more to maintain than the scan it saves.
func (s *Students) List(ctx context.Context, in domain.StudentQuery) ([]domain.Student, paging.Page, error) {
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

	out := make([]domain.Student, 0, limit)
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

// Get returns one student, disabled or not.
func (s *Students) Get(ctx context.Context, id string) (domain.Student, error) {
	return s.get(ctx, id, true)
}

// get optionally sees disabled rows.
//
// Update needs that: it reads the row back after writing it, and a successful
// `disabled: true` would otherwise miss its own write and report 404 for an
// operation that landed -- telling an operator the revocation failed when it
// did not.
func (s *Students) get(ctx context.Context, id string, includeDisabled bool) (domain.Student, error) {
	where := ` WHERE u.id = $1::uuid AND u.role = 'student'`
	if !includeDisabled {
		where += ` AND u.disabled_at IS NULL`
	}

	student, err := scanStudent(s.pool.QueryRow(ctx, selectStudents+where, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Student{}, domain.ErrStudentNotFound
	}
	if err != nil {
		return domain.Student{}, err
	}
	return student, nil
}

// StudentFacets backs G-07's header. "StudentsActive" is the dashboard's window, not a second
// definition of the same word on a second screen.
//
// Counted over the same filters the page is showing, not over the whole table:
// a header reading "31 học viên" above a search that matched one is describing
// something the teacher cannot see.
func (s *Students) Facets(ctx context.Context, in domain.StudentQuery) (domain.StudentFacets, error) {
	args := []any{dashboarddomain.ActiveWindow}
	where := []string{`u.role = 'student'`, statusCondition(in.Status)}

	if in.Query != "" {
		args = append(args, likeEscaper.Replace(in.Query))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}
	if in.ClassID != "" {
		args = append(args, in.ClassID)
		where = append(where, fmt.Sprintf(classCondition, len(args)))
	}

	var f domain.StudentFacets
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
		return domain.StudentFacets{}, fmt.Errorf("students: facets: %w", err)
	}
	return f, nil
}
