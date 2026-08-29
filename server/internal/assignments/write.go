package assignments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/audit"
)

var (
	ErrNotFound = errors.New("assignments: not found")
	// ErrTestNotPublished also covers a version id that does not exist: from
	// the caller's side both mean "that is not something you can assign".
	ErrTestNotPublished = errors.New("assignments: test version is not published")
	ErrVersionLocked    = errors.New("assignments: attempts exist")
)

type FieldError struct{ Field, Message string }

type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "assignments: " + strings.Join(parts, "; ")
}

// Request is the actor behind a write, for the audit row.
type Request struct {
	ID        string
	ActorID   string
	IP        string
	UserAgent string
}

type WriteInput struct {
	TestVersionID string
	ClassIDs      []string
	StudentIDs    []string
	OpensAt       time.Time
	ClosesAt      time.Time
	DurationMin   int
	MaxAttempts   int
	ShuffleQ      bool
	ShuffleO      bool
	Review        Review
	Integrity     Integrity
	// CloseNow is an action, not a state: it sets closed_at and there is no
	// value of it that reopens an assignment.
	CloseNow bool
	Now      time.Time
}

func validate(in WriteInput) error {
	var fields []FieldError

	if !in.ClosesAt.After(in.OpensAt) {
		fields = append(fields, FieldError{"window.closesAt", "Thời điểm đóng phải sau thời điểm mở."})
	}

	// An assignment with no targets reaches nobody, and nothing downstream says
	// so: the list would show 0/0 and the student home would simply be empty.
	if len(in.ClassIDs) == 0 && len(in.StudentIDs) == 0 {
		fields = append(fields, FieldError{"targets", "Chọn ít nhất một lớp hoặc một học viên."})
	}

	// Remove together with the auto_submit implementation (T-5.1). Storing it
	// now would put a promise on the student's intro page that §10.2 makes
	// verbatim and nothing yet keeps.
	if in.Integrity.OnLimitExceeded == "auto_submit" {
		fields = append(fields, FieldError{
			"integrity.onLimitExceeded",
			"Chế độ tự động nộp bài chưa khả dụng.",
		})
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func (s *Store) Create(ctx context.Context, req Request, in WriteInput) (Assignment, error) {
	if err := validate(in); err != nil {
		return Assignment{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Assignment{}, fmt.Errorf("assignments: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	testID, err := publishedTestFor(ctx, tx, in.TestVersionID)
	if err != nil {
		return Assignment{}, err
	}
	if err := checkTargets(ctx, tx, in); err != nil {
		return Assignment{}, err
	}

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.assignments
		       (test_id, test_version_id, opens_at, closes_at, closed_at,
		        duration_minutes, max_attempts, shuffle_questions, shuffle_options,
		        review_show_score, review_show_correct_answers, review_show_explanations,
		        integrity_require_fullscreen, integrity_block_copy_paste,
		        integrity_max_focus_loss, integrity_on_limit_exceeded, integrity_min_away_ms,
		        created_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
		        $13, $14, $15, $16::app.integrity_action, $17, $18::uuid)
		RETURNING id::text`,
		testID, in.TestVersionID, in.OpensAt, in.ClosesAt, closedAt(in),
		in.DurationMin, in.MaxAttempts, in.ShuffleQ, in.ShuffleO,
		in.Review.ShowScore, in.Review.ShowCorrectAnswers, in.Review.ShowExplanations,
		in.Integrity.RequireFullscreen, in.Integrity.BlockCopyPaste,
		in.Integrity.MaxFocusLoss, in.Integrity.OnLimitExceeded, in.Integrity.MinAwayMs,
		req.ActorID).Scan(&id); err != nil {
		return Assignment{}, fmt.Errorf("assignments: insert: %w", err)
	}

	if err := writeTargets(ctx, tx, id, in); err != nil {
		return Assignment{}, err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "assignment.created",
		Entity:      "assignment",
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	}); err != nil {
		return Assignment{}, err
	}

	created, err := s.get(ctx, tx, id)
	if err != nil {
		return Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, fmt.Errorf("assignments: commit create: %w", err)
	}
	return created, nil
}

func (s *Store) Update(ctx context.Context, req Request, in WriteInput) (Assignment, error) {
	if err := validate(in); err != nil {
		return Assignment{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Assignment{}, fmt.Errorf("assignments: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Locked so the attempt count below cannot be overtaken by a student
	// starting between the check and the write.
	var currentVersionID string
	var currentClosedAt *time.Time
	switch err := tx.QueryRow(ctx, `
		SELECT test_version_id::text, closed_at FROM app.assignments
		 WHERE id = $1::uuid FOR UPDATE`, req.ID).
		Scan(&currentVersionID, &currentClosedAt); {
	case err == nil:
	case errors.Is(err, pgx.ErrNoRows):
		return Assignment{}, ErrNotFound
	default:
		return Assignment{}, fmt.Errorf("assignments: load for update: %w", err)
	}

	testID, err := publishedTestFor(ctx, tx, in.TestVersionID)
	if err != nil {
		return Assignment{}, err
	}
	if in.TestVersionID != currentVersionID {
		var started bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM app.attempts WHERE assignment_id = $1::uuid)`,
			req.ID).Scan(&started); err != nil {
			return Assignment{}, fmt.Errorf("assignments: attempt check: %w", err)
		}
		if started {
			return Assignment{}, ErrVersionLocked
		}
	}
	if err := checkTargets(ctx, tx, in); err != nil {
		return Assignment{}, err
	}

	next := currentClosedAt
	if in.CloseNow && next == nil {
		next = &in.Now
	}

	if _, err := tx.Exec(ctx, `
		UPDATE app.assignments
		   SET test_id = $2::uuid, test_version_id = $3::uuid,
		       opens_at = $4, closes_at = $5, closed_at = $6,
		       duration_minutes = $7, max_attempts = $8,
		       shuffle_questions = $9, shuffle_options = $10,
		       review_show_score = $11, review_show_correct_answers = $12,
		       review_show_explanations = $13,
		       integrity_require_fullscreen = $14, integrity_block_copy_paste = $15,
		       integrity_max_focus_loss = $16,
		       integrity_on_limit_exceeded = $17::app.integrity_action,
		       integrity_min_away_ms = $18
		 WHERE id = $1::uuid`,
		req.ID, testID, in.TestVersionID, in.OpensAt, in.ClosesAt, next,
		in.DurationMin, in.MaxAttempts, in.ShuffleQ, in.ShuffleO,
		in.Review.ShowScore, in.Review.ShowCorrectAnswers, in.Review.ShowExplanations,
		in.Integrity.RequireFullscreen, in.Integrity.BlockCopyPaste,
		in.Integrity.MaxFocusLoss, in.Integrity.OnLimitExceeded,
		in.Integrity.MinAwayMs); err != nil {
		return Assignment{}, fmt.Errorf("assignments: update: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM app.assignment_classes WHERE assignment_id = $1::uuid`, req.ID); err != nil {
		return Assignment{}, fmt.Errorf("assignments: clear class targets: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.assignment_students WHERE assignment_id = $1::uuid`, req.ID); err != nil {
		return Assignment{}, fmt.Errorf("assignments: clear student targets: %w", err)
	}
	if err := writeTargets(ctx, tx, req.ID, in); err != nil {
		return Assignment{}, err
	}

	action := "assignment.updated"
	if in.CloseNow && currentClosedAt == nil {
		action = "assignment.closed"
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      action,
		Entity:      "assignment",
		EntityID:    &req.ID,
		OccurredAt:  in.Now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	}); err != nil {
		return Assignment{}, err
	}

	saved, err := s.get(ctx, tx, req.ID)
	if err != nil {
		return Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, fmt.Errorf("assignments: commit update: %w", err)
	}
	return saved, nil
}

func closedAt(in WriteInput) *time.Time {
	if in.CloseNow {
		return &in.Now
	}
	return nil
}

// publishedTestFor resolves the version's test and proves it is assignable.
//
// It returns the test id because app.assignments carries both, and the D-17
// composite FK rejects any pairing the caller invents.
func publishedTestFor(ctx context.Context, tx pgx.Tx, versionID string) (string, error) {
	var testID string
	err := tx.QueryRow(ctx, `
		SELECT v.test_id::text
		  FROM app.test_versions v
		  JOIN app.tests t ON t.id = v.test_id
		 WHERE v.id = $1::uuid AND t.deleted_at IS NULL AND t.status = 'published'`,
		versionID).Scan(&testID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrTestNotPublished
	}
	if err != nil {
		return "", fmt.Errorf("assignments: resolve version: %w", err)
	}
	return testID, nil
}

// checkTargets rejects ids that name nothing, rather than letting the FK fail.
//
// A foreign-key violation would surface as a 500 with no indication of which of
// forty ids was wrong.
func checkTargets(ctx context.Context, tx pgx.Tx, in WriteInput) error {
	var fields []FieldError

	if len(in.ClassIDs) > 0 {
		missing, err := missingIDs(ctx, tx,
			`SELECT id::text FROM app.classes WHERE id = ANY($1::uuid[])`, in.ClassIDs)
		if err != nil {
			return err
		}
		for _, id := range missing {
			fields = append(fields, FieldError{"targets.classIds", "Không tìm thấy lớp " + id + "."})
		}
	}

	if len(in.StudentIDs) > 0 {
		missing, err := missingIDs(ctx, tx,
			`SELECT id::text FROM app.users
			  WHERE id = ANY($1::uuid[]) AND role = 'student' AND disabled_at IS NULL`,
			in.StudentIDs)
		if err != nil {
			return err
		}
		for _, id := range missing {
			fields = append(fields, FieldError{"targets.studentIds", "Không tìm thấy học viên " + id + "."})
		}
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func missingIDs(ctx context.Context, tx pgx.Tx, query string, want []string) ([]string, error) {
	rows, err := tx.Query(ctx, query, want)
	if err != nil {
		return nil, fmt.Errorf("assignments: check targets: %w", err)
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("assignments: check targets: %w", err)
		}
		found[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("assignments: check targets: %w", err)
	}

	var missing []string
	for _, id := range want {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

func writeTargets(ctx context.Context, tx pgx.Tx, assignmentID string, in WriteInput) error {
	if len(in.ClassIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.assignment_classes (assignment_id, class_id)
			SELECT $1::uuid, unnest($2::uuid[])
			ON CONFLICT DO NOTHING`, assignmentID, in.ClassIDs); err != nil {
			return fmt.Errorf("assignments: insert class targets: %w", err)
		}
	}
	if len(in.StudentIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.assignment_students (assignment_id, user_id)
			SELECT $1::uuid, unnest($2::uuid[])
			ON CONFLICT DO NOTHING`, assignmentID, in.StudentIDs); err != nil {
			return fmt.Errorf("assignments: insert student targets: %w", err)
		}
	}
	return nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
