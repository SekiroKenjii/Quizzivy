package repositories

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/modules/dashboard/domain"
)

var _ domain.AdminRepository = (*Postgres)(nil)

const accountsSQL = `
	SELECT count(*),
	       count(*) FILTER (WHERE s.id IS NOT NULL),
	       count(*) FILTER (WHERE s.id IS NOT NULL AND u.created_at >= $1),
	       count(*) FILTER (WHERE s.id IS NULL AND r.builtin_key IS DISTINCT FROM 'admin'),
	       count(*) FILTER (WHERE s.id IS NULL AND r.builtin_key IS DISTINCT FROM 'admin' AND u.disabled_at IS NOT NULL),
	       count(*) FILTER (WHERE u.disabled_at IS NULL AND u.must_change_password),
	       count(*) FILTER (WHERE u.disabled_at IS NULL AND NOT u.must_change_password AND u.last_sign_in_at IS NULL),
	       min(u.created_at) FILTER (WHERE u.disabled_at IS NULL AND NOT u.must_change_password AND u.last_sign_in_at IS NULL)
	  FROM app.users u
	  JOIN app.roles r ON r.id = u.role_id
	  LEFT JOIN app.student_like_roles s ON s.id = u.role_id`

// Accounts counts every user in one pass, classing each by app.student_like_roles and the built-in Admin role, and the enabled ones by DG-54's order.
func (p *Postgres) Accounts(ctx context.Context, since time.Time) (domain.Accounts, error) {
	var out domain.Accounts
	err := p.QueryRow(ctx, accountsSQL, since).Scan(&out.Total, &out.Students, &out.StudentsNew, &out.Teachers,
		&out.TeachersDisabled, &out.MustChangePassword, &out.NeverSignedIn, &out.OldestNeverSignedIn)
	if err != nil {
		return domain.Accounts{}, fmt.Errorf("dashboard: count accounts: %w", err)
	}
	return out, nil
}

// MustChangePassword counts the enabled accounts on a temporary password, the number Accounts calls MustChangePassword.
func (p *Postgres) MustChangePassword(ctx context.Context) (int, error) {
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE disabled_at IS NULL AND must_change_password`).Scan(&n); err != nil {
		return 0, fmt.Errorf("dashboard: count temporary passwords: %w", err)
	}
	return n, nil
}

// ActiveSince counts the distinct accounts with an activity row on or after day.
func (p *Postgres) ActiveSince(ctx context.Context, day time.Time) (int, error) {
	var n int
	if err := p.QueryRow(ctx, `SELECT count(DISTINCT user_id) FROM app.user_activity_days WHERE day >= $1::date`, day).Scan(&n); err != nil {
		return 0, fmt.Errorf("dashboard: count active accounts: %w", err)
	}
	return n, nil
}

// ActiveByDay counts the student targets and the other accounts with an activity row on each day from..to that has one.
func (p *Postgres) ActiveByDay(ctx context.Context, from, to time.Time) ([]domain.ActiveDay, error) {
	rows, err := p.Query(ctx, `
		SELECT d.day,
		       count(*) FILTER (WHERE s.id IS NOT NULL),
		       count(*) FILTER (WHERE s.id IS NULL)
		  FROM app.user_activity_days d
		  JOIN app.users u ON u.id = d.user_id
		  LEFT JOIN app.student_like_roles s ON s.id = u.role_id
		 WHERE d.day BETWEEN $1::date AND $2::date
		 GROUP BY d.day
		 ORDER BY d.day`, from, to)
	if err != nil {
		return nil, fmt.Errorf("dashboard: count active days: %w", err)
	}
	defer rows.Close()
	var out []domain.ActiveDay
	for rows.Next() {
		var day domain.ActiveDay
		if err := rows.Scan(&day.Date, &day.Students, &day.Staff); err != nil {
			return nil, fmt.Errorf("dashboard: scan active day: %w", err)
		}
		out = append(out, day)
	}
	return out, rows.Err()
}

// ClassCounts counts the open and the archived classes of every teacher.
func (p *Postgres) ClassCounts(ctx context.Context) (domain.ClassCounts, error) {
	var out domain.ClassCounts
	err := p.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE archived_at IS NULL), count(*) FILTER (WHERE archived_at IS NOT NULL)
		  FROM app.classes`).Scan(&out.Active, &out.Archived)
	if err != nil {
		return domain.ClassCounts{}, fmt.Errorf("dashboard: count classes: %w", err)
	}
	return out, nil
}

// DisabledTeacherClasses counts the classes, archived ones too, whose teacher is disabled, and names the newest.
func (p *Postgres) DisabledTeacherClasses(ctx context.Context) (domain.DisabledTeacherClasses, error) {
	var out domain.DisabledTeacherClasses
	err := p.QueryRow(ctx, `
		WITH hit AS (
		  SELECT c.id, c.name, c.created_at, t.full_name
		    FROM app.classes c
		    JOIN app.users t ON t.id = c.teacher_id
		   WHERE t.disabled_at IS NOT NULL)
		SELECT (SELECT count(*) FROM hit), coalesce(n.name, ''), coalesce(n.full_name, '')
		  FROM (VALUES (1)) anchor(one)
		  LEFT JOIN LATERAL (SELECT name, full_name FROM hit ORDER BY created_at DESC, id DESC LIMIT 1) n ON TRUE`,
	).Scan(&out.Count, &out.ClassName, &out.TeacherName)
	if err != nil {
		return domain.DisabledTeacherClasses{}, fmt.Errorf("dashboard: count classes of disabled teachers: %w", err)
	}
	return out, nil
}

// Held sums the media files not yet purged, the source documents of the imports whose files are still kept and the number of those imports, and counts the attempts.
func (p *Postgres) Held(ctx context.Context) (domain.Held, error) {
	var out domain.Held
	err := p.QueryRow(ctx, `
		SELECT (SELECT coalesce(sum(bytes), 0)::bigint FROM app.media_assets WHERE purged_at IS NULL),
		       imp.bytes, imp.n,
		       (SELECT count(*) FROM app.attempts)
		  FROM (SELECT coalesce(sum(s.bytes), 0)::bigint AS bytes, count(DISTINCT s.import_id)::int AS n
		          FROM app.word_import_sources s
		          JOIN app.word_imports i ON i.id = s.import_id
		         WHERE i.files_removed_at IS NULL) imp`,
	).Scan(&out.MediaBytes, &out.ImportBytes, &out.ImportCount, &out.Attempts)
	if err != nil {
		return domain.Held{}, fmt.Errorf("dashboard: count what is held: %w", err)
	}
	return out, nil
}
