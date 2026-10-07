package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dashboarddomain "quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/paging"
	"quizzivy/internal/shared/visibility"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Students struct{ db.Repository }

func NewStudents(dbx db.Context) *Students { return &Students{Repository: db.NewRepository(dbx)} }

const searchCondition = `(
		app.immutable_unaccent(lower(u.full_name))
			LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'
		OR lower(u.email) LIKE '%%' || lower($%[1]d) || '%%' ESCAPE '\'
	)`

const classCondition = `EXISTS (SELECT 1 FROM app.class_members m JOIN app.classes c ON c.id = m.class_id
		  WHERE m.user_id = u.id AND m.class_id = $%d::uuid%s)`

const reachedStudent = `($2::boolean OR u.id IN ` + "%s" + `)`

const studentLike = `u.role_id IN (SELECT r.id FROM app.student_like_roles r)`

func scopedStudents(scope access.Scope, args []any) ([]any, []string, string) {
	where := []string{studentLike}
	if scope.All {
		return args, where, ``
	}
	args = append(args, opt.String(scope.UserID))
	owner := len(args)
	return args, append(where, `u.id IN `+visibility.StudentIDs(owner)), fmt.Sprintf(` AND c.teacher_id = $%d::uuid`, owner)
}

func filterStudents(in domain.StudentQuery, args []any) ([]any, []string, string) {
	args, where, classScope := scopedStudents(in.Scope, args)
	where = append(where, statusCondition(in.Status))
	if in.Query != "" {
		args = append(args, db.EscapeLike(in.Query))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}
	if in.ClassID != "" {
		args = append(args, in.ClassID)
		where = append(where, fmt.Sprintf(classCondition, len(args), classScope))
	}
	return args, where, classScope
}

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

func selectStudents(classScope string) string {
	return `
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
		                  WHERE m.user_id = u.id` + classScope + `), '[]'::jsonb)
		  FROM app.users u`
}

func scanStudent(row pgx.Row) (domain.Student, error) {
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

// List returns one page of the students the query's scope reaches, newest
// first, each with only the memberships of classes the scope reaches.
func (s *Students) List(ctx context.Context, in domain.StudentQuery) ([]domain.Student, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	args, where, classScope := filterStudents(in, []any{})
	from := `
		 WHERE ` + strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*) FROM app.users u`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("students: count: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.Query(ctx, selectStudents(classScope)+from+fmt.Sprintf(`
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

// Get returns one student the scope reaches, disabled or not, with only the
// memberships of classes the scope reaches. Another teacher's student answers
// ErrStudentNotFound, exactly as a missing one does.
func (s *Students) Get(ctx context.Context, scope access.Scope, id string) (domain.Student, error) {
	return s.get(ctx, scope, id, true)
}

func (s *Students) get(ctx context.Context, scope access.Scope, id string, includeDisabled bool) (domain.Student, error) {
	where := ` WHERE u.id = $1::uuid AND ` + studentLike + ` AND ` + fmt.Sprintf(reachedStudent, visibility.StudentIDs(3))
	if !includeDisabled {
		where += ` AND u.disabled_at IS NULL`
	}

	student, err := scanStudent(s.QueryRow(ctx, selectStudents(` AND ($2::boolean OR c.teacher_id = $3::uuid)`)+where,
		id, scope.All, opt.String(scope.UserID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Student{}, domain.ErrStudentNotFound
	}
	if err != nil {
		return domain.Student{}, err
	}
	return student, nil
}

// Account reads one account by id, whatever its role or reach, for a caller
// that already holds a parent row naming the user, such as an attempt under
// review. A missing id answers ErrStudentNotFound.
func (s *Students) Account(ctx context.Context, id string) (domain.Account, error) {
	var a domain.Account
	err := s.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.full_name, u.display_name,
		       CASE WHEN EXISTS (SELECT 1 FROM app.student_like_roles r WHERE r.id = u.role_id)
		            THEN 'student' ELSE 'admin' END,
		       u.password_hash IS NOT NULL,
		       coalesce((SELECT array_agg(i.provider::text)
		                   FROM app.user_identities i WHERE i.user_id = u.id), '{}'),
		       u.must_change_password, u.created_at
		  FROM app.users u
		 WHERE u.id = $1::uuid`, id).Scan(&a.ID, &a.Email, &a.FullName, &a.DisplayName, &a.Role,
		&a.HasPassword, &a.LinkedProviders, &a.MustChangePassword, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, domain.ErrStudentNotFound
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("students: account: %w", err)
	}
	return a, nil
}

// Facets backs G-07's header over the students the query's scope reaches;
// "active" counts only work on assignments the scope reaches. "StudentsActive"
// is the dashboard's window, not a second definition of the same word on a
// second screen.
func (s *Students) Facets(ctx context.Context, in domain.StudentQuery) (domain.StudentFacets, error) {
	args, where, _ := filterStudents(in, []any{dashboarddomain.ActiveWindow})
	worked := ``
	if !in.Scope.All {
		worked = ` AND a.assignment_id IN ` + visibility.AssignmentIDs(2)
	}

	var f domain.StudentFacets
	if err := s.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM app.attempts a
		          WHERE a.student_id = u.id
		            AND a.status <> 'voided'
		            AND a.started_at > now() - $1::interval`+worked+`))
		  FROM app.users u
		 WHERE `+strings.Join(where, "\n		   AND "), args...).
		Scan(&f.Total, &f.ActiveLast7Days); err != nil {
		return domain.StudentFacets{}, fmt.Errorf("students: facets: %w", err)
	}
	return f, nil
}
