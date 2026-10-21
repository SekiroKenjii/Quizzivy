package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/paging"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	AdminDefaultLimit = 20
	AdminMaxLimit     = 50
)

const teacherName = `coalesce(t.display_name, t.full_name)`

const teacherSource = `
	  JOIN app.users t ON t.id = c.teacher_id`

func folded(n int) string {
	return fmt.Sprintf(`EXISTS (SELECT FROM unnest(ARRAY[c.name, t.full_name, t.display_name]) AS f(v)
	   WHERE app.immutable_unaccent(lower(f.v)) LIKE '%%' || app.immutable_unaccent(lower($%d)) || '%%' ESCAPE '\')`, n)
}

func adminClause(in domain.AdminListInput) ([]any, []string) {
	args, where := []any{}, []string{"TRUE"}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, db.EscapeLike(q))
		where = append(where, folded(len(args)))
	}
	if in.DisabledTeacher {
		where = append(where, "t.disabled_at IS NOT NULL")
	}
	return args, where
}

// ListAll implements domain.Repository.
func (s *Postgres) ListAll(ctx context.Context, in domain.AdminListInput) ([]domain.AdminClass, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, AdminDefaultLimit, AdminMaxLimit)
	args, where := adminClause(in)
	if in.Tab == "archived" {
		where = append(where, "c.archived_at IS NOT NULL")
	} else {
		where = append(where, "c.archived_at IS NULL")
	}
	condition := " WHERE " + strings.Join(where, " AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*) FROM app.classes c`+teacherSource+condition,
		args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("count every class: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.Query(ctx, classColumns+`,
	       t.id::text, `+teacherName+`, t.avatar_key, t.disabled_at IS NOT NULL`+
		classSource+teacherSource+condition+fmt.Sprintf(
		` ORDER BY c.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("list every class: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AdminClass, 0, limit)
	for rows.Next() {
		var t domain.ClassTeacher
		c, err := scanClass(rows, &t.ID, &t.Name, &t.AvatarKey, &t.Disabled)
		if err != nil {
			return nil, paging.Page{}, fmt.Errorf("list every class: %w", err)
		}
		out = append(out, domain.AdminClass{Class: c, Teacher: t})
	}
	return out, page, rows.Err()
}

// AdminFacets implements domain.Repository.
func (s *Postgres) AdminFacets(ctx context.Context, in domain.AdminListInput) (domain.AdminFacets, error) {
	args, where := adminClause(in)
	var f domain.AdminFacets
	err := s.QueryRow(ctx, `
	SELECT count(*) FILTER (WHERE c.archived_at IS NULL),
	       count(*) FILTER (WHERE c.archived_at IS NOT NULL),
	       (SELECT count(*) FROM app.classes x
	          JOIN app.users y ON y.id = x.teacher_id AND y.disabled_at IS NOT NULL
	         WHERE x.archived_at IS NULL)
	  FROM app.classes c`+teacherSource+`
	 WHERE `+strings.Join(where, " AND "), args...).Scan(&f.Active, &f.Archived, &f.DisabledTeacherClasses)
	if err != nil {
		return domain.AdminFacets{}, fmt.Errorf("count every class's facets: %w", err)
	}
	return f, nil
}

func lockTeacher(ctx context.Context, tx pgx.Tx, teacherID string) (string, error) {
	var roleID, name string
	err := tx.QueryRow(ctx, `
		SELECT t.role_id::text, `+teacherName+`
		  FROM app.users t
		 WHERE t.id = $1::uuid AND t.disabled_at IS NULL
		   FOR SHARE`, teacherID).Scan(&roleID, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrInvalidTeacher
	}
	if err != nil {
		return "", fmt.Errorf("lock teacher: %w", err)
	}
	var admin bool
	var keys []string
	err = tx.QueryRow(ctx, `
		SELECT coalesce(r.builtin_key, '') = 'admin',
		       coalesce(array_agg(rp.permission_key) FILTER (WHERE rp.permission_key IS NOT NULL), '{}')
		  FROM (SELECT r.id, r.builtin_key FROM app.roles r WHERE r.id = $1::uuid FOR SHARE) r
		  LEFT JOIN app.role_permissions rp ON rp.role_id = r.id
		 GROUP BY r.builtin_key`, roleID).Scan(&admin, &keys)
	if err != nil {
		return "", fmt.Errorf("lock teacher role: %w", err)
	}
	granted := access.All()
	if !admin {
		granted = make([]access.Key, len(keys))
		for i, key := range keys {
			granted[i] = access.Key(key)
		}
	}
	if !slices.Contains(access.Workspaces(access.NewSet(granted...)), access.TeacherWorkspace) {
		return "", domain.ErrInvalidTeacher
	}
	return name, nil
}

// CreateForTeacher implements domain.Repository.
func (s *Postgres) CreateForTeacher(ctx context.Context, in domain.CreateForTeacherInput) (domain.Class, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Class{}, fmt.Errorf("begin create class for teacher: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := lockTeacher(ctx, tx, in.TeacherID); err != nil {
		return domain.Class{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.classes (id, name, self_join_enabled, created_at, teacher_id, schedule_label, room)
		VALUES ($1::uuid, $2, true, $3, $4::uuid, $5, $6)`,
		in.ClassID, in.Name, in.Now, in.TeacherID, in.ScheduleLabel, in.Room); err != nil {
		return domain.Class{}, fmt.Errorf("create class for teacher: %w", err)
	}
	diff, err := json.Marshal(map[string]string{"onBehalfOf": in.TeacherID})
	if err != nil {
		return domain.Class{}, fmt.Errorf("encode class audit: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      "class.created",
		Entity:      entityClass,
		EntityID:    &in.ClassID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
		Diff:        diff,
	}); err != nil {
		return domain.Class{}, err
	}
	if code := in.Code; code != nil {
		var codeID string
		var uses int
		if err := tx.QueryRow(ctx, issueCode,
			code.CodeID, in.ClassID, code.CodeHash, code.Ciphertext, code.KeyID,
			code.Hint, code.ExpiresAt, code.MaxUses, in.ActorUserID, in.Now,
		).Scan(&codeID, &uses); err != nil {
			return domain.Class{}, fmt.Errorf("issue code: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &in.ActorUserID,
			Action:      "class.join_code_issued",
			Entity:      entityJoinCode,
			EntityID:    &codeID,
			OccurredAt:  in.Now,
			IP:          in.IP,
			UserAgent:   in.UserAgent,
		}); err != nil {
			return domain.Class{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Class{}, fmt.Errorf("commit create class for teacher: %w", err)
	}
	return s.Get(ctx, anyClass, in.ClassID)
}

type teacherDiff struct {
	Teacher ownerChange `json:"teacher"`
}

type ownerChange struct {
	Old *string `json:"old"`
	New string  `json:"new"`
}

// ChangeTeacher implements domain.Repository.
func (s *Postgres) ChangeTeacher(ctx context.Context, in domain.ChangeTeacherInput) (domain.TeacherChange, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.TeacherChange{}, fmt.Errorf("begin change teacher: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	name, err := lockTeacher(ctx, tx, in.TeacherID)
	if errors.Is(err, domain.ErrInvalidTeacher) {
		return domain.TeacherChange{}, missingClassFirst(ctx, tx, in.ClassID)
	}
	if err != nil {
		return domain.TeacherChange{}, err
	}

	out := domain.TeacherChange{TeacherName: name}
	err = tx.QueryRow(ctx, `
		SELECT c.teacher_id::text,
		       coalesce((SELECT t.disabled_at IS NOT NULL FROM app.users t WHERE t.id = c.teacher_id), false)
		  FROM app.classes c
		 WHERE c.id = $1::uuid
		   FOR NO KEY UPDATE OF c`, in.ClassID).Scan(&out.Previous, &out.PreviousDisabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TeacherChange{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.TeacherChange{}, fmt.Errorf("lock class %s: %w", in.ClassID, err)
	}

	if out.Previous == nil || *out.Previous != in.TeacherID {
		out.Changed = true
		if err := writeTeacher(ctx, tx, in, out.Previous); err != nil {
			return domain.TeacherChange{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TeacherChange{}, fmt.Errorf("commit change teacher: %w", err)
	}
	out.Class, err = s.Get(ctx, anyClass, in.ClassID)
	return out, err
}

func missingClassFirst(ctx context.Context, tx pgx.Tx, classID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.classes WHERE id = $1::uuid)`, classID).Scan(&exists); err != nil {
		return fmt.Errorf("change teacher: %w", err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	return domain.ErrInvalidTeacher
}

func writeTeacher(ctx context.Context, tx pgx.Tx, in domain.ChangeTeacherInput, previous *string) error {
	if _, err := tx.Exec(ctx, `UPDATE app.classes SET teacher_id = $2::uuid WHERE id = $1::uuid`,
		in.ClassID, in.TeacherID); err != nil {
		return fmt.Errorf("change teacher of %s: %w", in.ClassID, err)
	}
	diff, err := json.Marshal(teacherDiff{Teacher: ownerChange{Old: previous, New: in.TeacherID}})
	if err != nil {
		return fmt.Errorf("encode class audit: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      "class.teacher_changed",
		Entity:      entityClass,
		EntityID:    &in.ClassID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
		Diff:        diff,
	})
}
