// Package review is the teacher's side of an attempt: the paper with its
// grading key, the answers with their scores, and the manual grading that
// closes the loop (§8). It is the one reader of `sample_answer`, `is_correct`
// and the accepted answers that hands them to a screen, and that screen is
// behind /admin.
package review

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

var (
	ErrNotFound   = errors.New("review: attempt not found")
	ErrInProgress = errors.New("review: attempt is still in progress")
	ErrVoided     = errors.New("review: attempt is voided")
	ErrIncomplete = errors.New("review: a manual answer is still ungraded")
)

type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool, now: time.Now} }

type Option struct {
	ID        string
	Ordinal   int
	Text      string
	IsCorrect bool
}

type Blank struct {
	ID            string
	Ordinal       int
	CaseSensitive bool
	Accepted      []string
}

// Question is a frozen version question with everything the grader may see.
type Question struct {
	ID           string
	Type         string
	Prompt       string
	Points       float64
	Media        *attempts.Media
	Audio        *attempts.AudioPolicy
	Transcript   *string
	Explanation  *string
	SampleAnswer *string
	Options      []Option
	Blanks       []Blank
}

// Answer is one saved answer and what it has earned so far.
type Answer struct {
	Payload        []byte
	AutoScore      *float64
	ManualScore    *float64
	RequiresManual bool
	GraderComment  *string
}

// Review is G-03's data in one read.
type Review struct {
	Attempt     attempts.Attempt
	Score       attempts.Score
	TestTitle   string
	MaxAttempts int
	// PublishedAt is when the version froze; the questions carry no clock of their own.
	PublishedAt time.Time
	Questions   []Question
	Answers     map[string]Answer
	AudioPlays  map[string]int
}

// Get reads one attempt for review, in the version's own order rather than
// the student's shuffled one: question 23 is the essay for every paper.
func (s *Store) Get(ctx context.Context, attemptID string) (Review, error) {
	var (
		out   Review
		total *float64
	)
	a := &out.Attempt
	err := s.pool.QueryRow(ctx, `
		SELECT at.id::text, at.assignment_id::text, at.student_id::text, at.test_version_id::text,
		       at.attempt_no, at.status, at.started_at, at.deadline_at, at.submitted_at, at.graded_at,
		       at.focus_loss_count, at.flagged, at.score_total,
		       asg.max_attempts, t.title, v.published_at
		  FROM app.attempts at
		  JOIN app.assignments asg ON asg.id = at.assignment_id
		  JOIN app.tests t ON t.id = asg.test_id
		  JOIN app.test_versions v ON v.id = at.test_version_id
		 WHERE at.id = $1::uuid`, attemptID).Scan(
		&a.ID, &a.AssignmentID, &a.StudentID, &a.TestVersionID,
		&a.AttemptNo, &a.Status, &a.StartedAt, &a.DeadlineAt, &a.SubmittedAt, &a.GradedAt,
		&a.FocusLossCount, &a.Flagged, &total,
		&out.MaxAttempts, &out.TestTitle, &out.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	if err != nil {
		return Review{}, fmt.Errorf("review: read attempt: %w", err)
	}

	if out.Questions, err = s.questions(ctx, a.TestVersionID); err != nil {
		return Review{}, err
	}
	if out.Answers, err = s.answers(ctx, a.ID); err != nil {
		return Review{}, err
	}
	if out.AudioPlays, err = s.audioPlays(ctx, a.ID); err != nil {
		return Review{}, err
	}
	if total != nil {
		out.Score.Total = *total
	}
	for _, ans := range out.Answers {
		switch {
		case ans.ManualScore != nil:
			out.Score.Earned += *ans.ManualScore
		case ans.RequiresManual:
			out.Score.PendingManual++
		case ans.AutoScore != nil:
			out.Score.Earned += *ans.AutoScore
		}
	}
	return out, nil
}

func (s *Store) questions(ctx context.Context, versionID string) ([]Question, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT q.id::text, q.type::text, q.prompt, q.points,
		       q.media_asset_id::text, q.media_asset_kind::text, m.mime_type, m.original_filename,
		       m.bytes, m.duration_ms, m.created_at,
		       q.audio_max_plays, q.audio_allow_seek, q.audio_show_transcript_after,
		       q.transcript, q.explanation, q.sample_answer
		  FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		  LEFT JOIN app.media_assets m ON m.id = q.media_asset_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY s.ordinal, q.ordinal`, versionID)
	if err != nil {
		return nil, fmt.Errorf("review: read questions: %w", err)
	}
	defer rows.Close()

	var out []Question
	at := map[string]int{}
	for rows.Next() {
		var (
			q                                      Question
			mediaID, mediaKind, mimeType, filename *string
			mediaBytes, durationMs, maxPlays       *int
			createdAt                              *time.Time
			allowSeek, showTranscript              *bool
		)
		if err := rows.Scan(&q.ID, &q.Type, &q.Prompt, &q.Points,
			&mediaID, &mediaKind, &mimeType, &filename, &mediaBytes, &durationMs, &createdAt,
			&maxPlays, &allowSeek, &showTranscript,
			&q.Transcript, &q.Explanation, &q.SampleAnswer); err != nil {
			return nil, fmt.Errorf("review: scan question: %w", err)
		}
		if mediaID != nil {
			q.Media = &attempts.Media{ID: *mediaID, DurationMs: durationMs}
			if mediaKind != nil {
				q.Media.Kind = *mediaKind
			}
			if mimeType != nil {
				q.Media.MimeType = *mimeType
			}
			if filename != nil {
				q.Media.Filename = *filename
			}
			if mediaBytes != nil {
				q.Media.Bytes = *mediaBytes
			}
			if createdAt != nil {
				q.Media.CreatedAt = *createdAt
			}
		}
		if allowSeek != nil && showTranscript != nil {
			q.Audio = &attempts.AudioPolicy{
				MaxPlays: maxPlays, AllowSeek: *allowSeek, ShowTranscriptAfterSubmit: *showTranscript,
			}
		}
		at[q.ID] = len(out)
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("review: read questions: %w", err)
	}
	if err := s.attachOptions(ctx, versionID, out, at); err != nil {
		return nil, err
	}
	return out, s.attachBlanks(ctx, versionID, out, at)
}

func (s *Store) attachOptions(ctx context.Context, versionID string, qs []Question, at map[string]int) error {
	rows, err := s.pool.Query(ctx, `
		SELECT o.test_version_question_id::text, o.id::text, o.ordinal, o.text, o.is_correct
		  FROM app.test_version_options o
		  JOIN app.test_version_questions q ON q.id = o.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY o.ordinal`, versionID)
	if err != nil {
		return fmt.Errorf("review: read options: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var questionID string
		var o Option
		if err := rows.Scan(&questionID, &o.ID, &o.Ordinal, &o.Text, &o.IsCorrect); err != nil {
			return fmt.Errorf("review: scan option: %w", err)
		}
		if i, ok := at[questionID]; ok {
			qs[i].Options = append(qs[i].Options, o)
		}
	}
	return rows.Err()
}

func (s *Store) attachBlanks(ctx context.Context, versionID string, qs []Question, at map[string]int) error {
	rows, err := s.pool.Query(ctx, `
		SELECT b.test_version_question_id::text, b.id::text, b.ordinal, b.case_sensitive,
		       coalesce((SELECT array_agg(ba.answer ORDER BY ba.id)
		                   FROM app.test_version_blank_answers ba
		                  WHERE ba.test_version_blank_id = b.id), '{}')
		  FROM app.test_version_blanks b
		  JOIN app.test_version_questions q ON q.id = b.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY b.ordinal`, versionID)
	if err != nil {
		return fmt.Errorf("review: read blanks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var questionID string
		var b Blank
		if err := rows.Scan(&questionID, &b.ID, &b.Ordinal, &b.CaseSensitive, &b.Accepted); err != nil {
			return fmt.Errorf("review: scan blank: %w", err)
		}
		if i, ok := at[questionID]; ok {
			qs[i].Blanks = append(qs[i].Blanks, b)
		}
	}
	return rows.Err()
}

func (s *Store) answers(ctx context.Context, attemptID string) (map[string]Answer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT question_id::text, payload, auto_score, manual_score, requires_manual, grader_comment
		  FROM app.attempt_answers WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("review: read answers: %w", err)
	}
	defer rows.Close()
	out := map[string]Answer{}
	for rows.Next() {
		var id string
		var a Answer
		if err := rows.Scan(&id, &a.Payload, &a.AutoScore, &a.ManualScore, &a.RequiresManual, &a.GraderComment); err != nil {
			return nil, fmt.Errorf("review: scan answer: %w", err)
		}
		out[id] = a
	}
	return out, rows.Err()
}

func (s *Store) audioPlays(ctx context.Context, attemptID string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT question_id::text, plays FROM app.attempt_audio_plays WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("review: read audio plays: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var plays int
		if err := rows.Scan(&id, &plays); err != nil {
			return nil, fmt.Errorf("review: scan audio plays: %w", err)
		}
		out[id] = plays
	}
	return out, rows.Err()
}
