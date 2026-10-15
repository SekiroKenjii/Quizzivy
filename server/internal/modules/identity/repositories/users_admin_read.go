package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/paging"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AdminUsersDefaultLimit and AdminUsersMaxLimit are the Users table's page
// sizes; the contract allows 10, 20 and 50.
const (
	AdminUsersDefaultLimit = 20
	AdminUsersMaxLimit     = 50
)

const (
	anyRow        = `TRUE`
	isAdminRole   = `COALESCE(r.builtin_key, '') = 'admin'`
	isStudentRole = `r.id IN (SELECT s.id FROM app.student_like_roles s)`
	isDisabled    = `u.disabled_at IS NOT NULL`
)

func adminTabCondition(tab domain.AdminTab) string {
	switch tab {
	case domain.TabTeachers:
		return `NOT (` + isAdminRole + `) AND NOT (` + isStudentRole + `)`
	case domain.TabStudents:
		return isStudentRole
	case domain.TabAdmins:
		return isAdminRole
	case domain.TabDisabled:
		return isDisabled
	default:
		return anyRow
	}
}

func adminFilters(in domain.AdminUserQuery, withTab bool) ([]any, []string) {
	args := []any{}
	where := []string{anyRow}
	if withTab {
		where = append(where, adminTabCondition(in.Tab))
	}
	if in.Status != "" {
		args = append(args, string(in.Status))
		where = append(where, fmt.Sprintf(`%s = $%d::text`, accountStatusSQL, len(args)))
	}
	if in.Query != "" {
		args = append(args, db.EscapeLike(in.Query))
		where = append(where, fmt.Sprintf(`app.immutable_unaccent(lower(u.full_name || ' ' || u.email))
			LIKE '%%' || app.immutable_unaccent(lower($%d)) || '%%' ESCAPE '\'`, len(args)))
	}
	if in.Email != "" {
		args = append(args, in.Email)
		where = append(where, fmt.Sprintf(`lower(u.email) = lower($%d)`, len(args)))
	}
	return args, where
}

// List returns one page of the accounts the query matches, newest first, with
// the classes each teaches or has joined. The page is chosen before the class
// summary is computed, so the summary runs for at most a page of accounts.
func (a *AdminUsers) List(ctx context.Context, in domain.AdminUserQuery) ([]domain.AdminUser, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, AdminUsersDefaultLimit, AdminUsersMaxLimit)
	args, where := adminFilters(in, true)
	from := `
		  FROM app.users u
		  JOIN app.roles r ON r.id = u.role_id
		 WHERE ` + strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := a.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("users: count: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := a.Query(ctx, `
		WITH page AS (
		SELECT u.id`+from+`
		 ORDER BY u.id DESC
		 LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args))+`)`+
		adminUserSelect+`
		 WHERE u.id IN (SELECT id FROM page)
		 ORDER BY u.id DESC`, args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("users: list: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AdminUser, 0, limit)
	for rows.Next() {
		user, err := scanAdminUser(rows)
		if err != nil {
			return nil, paging.Page{}, err
		}
		out = append(out, user)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("users: list: %w", err)
	}
	return out, page, nil
}

// Facets counts every tab under the query's search, email and status.
func (a *AdminUsers) Facets(ctx context.Context, in domain.AdminUserQuery) (domain.AdminUserFacets, error) {
	args, where := adminFilters(in, false)
	var f domain.AdminUserFacets
	if err := a.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE `+adminTabCondition(domain.TabTeachers)+`),
		       count(*) FILTER (WHERE `+isStudentRole+`),
		       count(*) FILTER (WHERE `+isAdminRole+`),
		       count(*) FILTER (WHERE `+isDisabled+`)
		  FROM app.users u
		  JOIN app.roles r ON r.id = u.role_id
		 WHERE `+strings.Join(where, "\n		   AND "), args...).
		Scan(&f.All, &f.Teachers, &f.Students, &f.Admins, &f.Disabled); err != nil {
		return domain.AdminUserFacets{}, fmt.Errorf("users: facets: %w", err)
	}
	return f, nil
}

// Detail reads one account, whatever its role, with its creator, classes, what
// it owns, the tests it is taking and its latest activity.
func (a *AdminUsers) Detail(ctx context.Context, id string) (domain.AdminUserDetail, error) {
	user, err := scanAdminUser(a.QueryRow(ctx, adminUserSelect+` WHERE u.id = $1::uuid`, id))
	if err != nil {
		return domain.AdminUserDetail{}, err
	}
	detail := domain.AdminUserDetail{User: user}
	if err := a.facts(ctx, id, &detail); err != nil {
		return domain.AdminUserDetail{}, err
	}
	if detail.Classes, err = a.classes(ctx, id); err != nil {
		return domain.AdminUserDetail{}, err
	}
	if detail.Events, err = a.events(ctx, id); err != nil {
		return domain.AdminUserDetail{}, err
	}
	return detail, nil
}

func (a *AdminUsers) facts(ctx context.Context, id string, detail *domain.AdminUserDetail) error {
	var creatorID, creatorName *string
	err := a.QueryRow(ctx, `
		SELECT cb.id::text, cb.full_name,
		       r.builtin_key = 'admin' AND u.disabled_at IS NULL
		         AND NOT EXISTS (SELECT 1
		                           FROM app.users o
		                           JOIN app.roles ro ON ro.id = o.role_id
		                          WHERE ro.builtin_key = 'admin' AND o.disabled_at IS NULL AND o.id <> u.id),
		       (SELECT count(*)::int FROM app.classes c WHERE c.teacher_id = u.id),
		       (SELECT count(*)::int FROM app.tests t WHERE t.owner_id = u.id AND t.deleted_at IS NULL),
		       (SELECT count(*)::int FROM app.questions q WHERE q.owner_id = u.id AND q.deleted_at IS NULL),
		       (SELECT count(*)::int FROM app.media_assets m WHERE m.owner_id = u.id AND m.deleted_at IS NULL),
		       (SELECT count(*)::int FROM app.assignments g WHERE g.created_by = u.id),
		       (SELECT count(*)::int FROM app.attempts t WHERE t.student_id = u.id AND t.status = 'in_progress')
		  FROM app.users u
		  JOIN app.roles r ON r.id = u.role_id
		  LEFT JOIN app.users cb ON cb.id = u.created_by
		 WHERE u.id = $1::uuid`, id).
		Scan(&creatorID, &creatorName, &detail.LastActiveAdmin,
			&detail.Owned.Classes, &detail.Owned.Tests, &detail.Owned.Questions, &detail.Owned.Media, &detail.Owned.Assignments,
			&detail.InProgressAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("users: facts: %w", err)
	}
	if creatorID != nil && creatorName != nil {
		detail.CreatedBy = &domain.AdminUserRef{ID: *creatorID, FullName: *creatorName}
	}
	return nil
}

func (a *AdminUsers) classes(ctx context.Context, id string) ([]domain.AdminClassRef, error) {
	rows, err := a.Query(ctx, `
		SELECT c.id::text, c.name
		  FROM app.users u
		 CROSS JOIN LATERAL (`+userClasses+`) c
		 WHERE u.id = $1::uuid
		 ORDER BY c.at, c.id
		 LIMIT $2::int`, id, domain.MaxUserClasses)
	if err != nil {
		return nil, fmt.Errorf("users: classes: %w", err)
	}
	defer rows.Close()
	out := []domain.AdminClassRef{}
	for rows.Next() {
		var c domain.AdminClassRef
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("users: scan class: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (a *AdminUsers) events(ctx context.Context, id string) ([]domain.UserEvent, error) {
	rows, err := a.Query(ctx, `
		SELECT kind, at, detail FROM (
		  SELECT 'sign_in' AS kind, u.last_sign_in_at AS at, u.last_sign_in_method AS detail
		    FROM app.users u WHERE u.id = $1::uuid AND u.last_sign_in_at IS NOT NULL
		  UNION ALL
		  (SELECT 'audit', l.occurred_at, l.action
		     FROM app.audit_log l WHERE l.actor_user_id = $1::uuid
		    ORDER BY l.occurred_at DESC LIMIT $2::int)
		  UNION ALL
		  (SELECT 'submission', t.submitted_at, x.title
		     FROM app.attempts t
		     JOIN app.test_versions v ON v.id = t.test_version_id
		     JOIN app.tests x ON x.id = v.test_id
		    WHERE t.student_id = $1::uuid AND t.submitted_at IS NOT NULL
		    ORDER BY t.submitted_at DESC LIMIT $2::int)
		  UNION ALL
		  (SELECT 'class_joined', m.joined_at, c.name
		     FROM app.class_members m JOIN app.classes c ON c.id = m.class_id
		    WHERE m.user_id = $1::uuid
		    ORDER BY m.joined_at DESC LIMIT $2::int)) e
		 ORDER BY at DESC, kind
		 LIMIT $2::int`, id, domain.MaxUserEvents)
	if err != nil {
		return nil, fmt.Errorf("users: events: %w", err)
	}
	defer rows.Close()
	out := []domain.UserEvent{}
	for rows.Next() {
		var kind string
		var at time.Time
		var detail *string
		if err := rows.Scan(&kind, &at, &detail); err != nil {
			return nil, fmt.Errorf("users: scan event: %w", err)
		}
		out = append(out, domain.UserEvent{Kind: domain.UserEventKind(kind), At: at, Detail: detail})
	}
	return out, rows.Err()
}
