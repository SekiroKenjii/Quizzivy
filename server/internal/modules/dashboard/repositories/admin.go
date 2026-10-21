package repositories

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/limits"
)

var _ domain.AdminRepository = (*Postgres)(nil)

const accountsSQL = `
	SELECT count(*),
	       count(*) FILTER (WHERE u.disabled_at IS NULL),
	       count(*) FILTER (WHERE s.id IS NOT NULL),
	       count(*) FILTER (WHERE s.id IS NOT NULL AND u.created_at >= $1),
	       count(*) FILTER (WHERE s.id IS NULL AND u.disabled_at IS NULL),
	       count(*) FILTER (WHERE s.id IS NULL AND u.disabled_at IS NOT NULL),
	       count(*) FILTER (WHERE u.disabled_at IS NULL AND u.must_change_password),
	       count(*) FILTER (WHERE u.disabled_at IS NULL AND NOT u.must_change_password AND u.last_sign_in_at IS NULL),
	       min(u.created_at) FILTER (WHERE u.disabled_at IS NULL AND NOT u.must_change_password AND u.last_sign_in_at IS NULL)
	  FROM app.users u
	  LEFT JOIN app.student_like_roles s ON s.id = u.role_id`

// Accounts counts every user in one pass, classing each by app.student_like_roles alone (an account that is not a student target is a teacher, an Admin included) and the enabled ones by DG-54's order.
func (p *Postgres) Accounts(ctx context.Context, since time.Time) (domain.Accounts, error) {
	var out domain.Accounts
	err := p.QueryRow(ctx, accountsSQL, since).Scan(&out.Total, &out.Enabled, &out.Students, &out.StudentsNew, &out.Teachers,
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

// ActiveBetween counts the distinct accounts with an activity row on any day from..to, whether the account has been disabled since or not.
func (p *Postgres) ActiveBetween(ctx context.Context, from, to time.Time) (int, error) {
	var n int
	if err := p.QueryRow(ctx, `SELECT count(DISTINCT user_id) FROM app.user_activity_days WHERE day BETWEEN $1::date AND $2::date`, from, to).Scan(&n); err != nil {
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

// Held counts what the centre holds and uses: the seats and teachers and the
// stored files as shared/limits counts them, the fullest teacher's media
// library, the documents of the open imports, the attempts, the mail sent in
// window's day and month and the mail a cap moved to later (never tried yet),
// and the database's size.
func (p *Postgres) Held(ctx context.Context, window domain.EmailWindow) (domain.Held, error) {
	var out domain.Held
	accounts, err := limits.CountAccounts(ctx, p)
	if err != nil {
		return domain.Held{}, err
	}
	out.Seats, out.Teachers = accounts.Seats, accounts.Teachers
	files, err := limits.CountHeld(ctx, p)
	if err != nil {
		return domain.Held{}, err
	}
	out.MediaBytes, out.ImportBytes, out.ExportBytes = files.Media, files.Imports, files.Exports
	err = p.QueryRow(ctx, `
		SELECT (SELECT count(DISTINCT i.id)::int
		          FROM app.word_imports i JOIN app.word_import_sources s ON s.import_id = i.id
		         WHERE i.files_removed_at IS NULL),
		       (SELECT coalesce(max(b), 0)::bigint
		          FROM (SELECT sum(bytes) AS b FROM app.media_assets WHERE purged_at IS NULL GROUP BY owner_id) libraries),
		       (SELECT coalesce(sum(s.bytes), 0)::bigint
		          FROM app.word_import_sources s JOIN app.word_imports i ON i.id = s.import_id
		         WHERE i.status NOT IN ('committed', 'cancelled')),
		       (SELECT count(*) FROM app.attempts),
		       (SELECT count(*)::int FROM app.email_outbox WHERE sent_at >= $1 AND sent_at < $2),
		       (SELECT count(*)::int FROM app.email_outbox WHERE sent_at >= $3 AND sent_at < $4),
		       (SELECT count(*)::int FROM app.email_outbox
		         WHERE sent_at IS NULL AND failed_at IS NULL AND attempts = 0 AND deliver_after > $5),
		       (SELECT min(deliver_after) FROM app.email_outbox
		         WHERE sent_at IS NULL AND failed_at IS NULL AND attempts = 0 AND deliver_after > $5)`,
		window.DayStart, window.NextDay, window.MonthStart, window.NextMonth, window.Now,
	).Scan(&out.ImportCount, &out.LargestLibrary, &out.OpenImportBytes, &out.Attempts,
		&out.EmailsToday, &out.EmailsMonth, &out.EmailsHeld, &out.HeldUntil)
	if err != nil {
		return domain.Held{}, fmt.Errorf("dashboard: count what is held: %w", err)
	}
	if out.DatabaseBytes, err = limits.DatabaseSize(ctx, p); err != nil {
		return domain.Held{}, err
	}
	return out, nil
}

// Limits reads the usage limits an Admin stored.
func (p *Postgres) Limits(ctx context.Context) (limits.Stored, error) {
	return limits.ReadStored(ctx, p)
}
