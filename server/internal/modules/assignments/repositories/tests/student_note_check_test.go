//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5/pgconn"
)

var javaScriptWhitespace = []rune{'\u0009', '\u000A', '\u000B', '\u000C', '\u000D', '\u0020', '\u00A0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200A', '\u2028', '\u2029', '\u202F', '\u205F', '\u3000', '\uFEFF'}

func violatesTheNoteCheck(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "assignments_student_note_check"
}

func TestTheNoteCheckTrimsTheSameWhitespaceTheCommandDoes(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()
	created := createFor(t, store, w, legalInput(w))
	setNote := func(note string) error {
		_, err := pool.Exec(ctx, `UPDATE app.assignments SET student_note = $1 WHERE id = $2::uuid`, note, created.ID)
		return err
	}

	blanks := []string{"\n\t ", "\r\n", " \u00A0 ", "\uFEFF\u3000", "\u2028\u2029", "\u000B\u000C"}
	for _, space := range javaScriptWhitespace {
		blanks = append(blanks, string(space), strings.Repeat(string(space), 3))
	}
	for _, blank := range blanks {
		if domain.StudentNoteOf(&blank) != nil {
			t.Errorf("the command stores %q, a note of nothing but whitespace", blank)
		}
		if err := setNote(blank); !violatesTheNoteCheck(err) {
			t.Errorf("the check let %q through: %v", blank, err)
		}
	}

	padded500 := "\n\t" + strings.Repeat("ệ", domain.MaxStudentNote) + "\u00A0" + "\uFEFF"
	kept := []string{"a", " a ", "Mang\u00A0theo", "\u0085", "a\u0085", "\u200B", strings.Repeat("ệ", domain.MaxStudentNote), padded500}
	for _, note := range kept {
		stored := domain.StudentNoteOf(&note)
		if stored == nil {
			t.Errorf("the command drops %q", note)
			continue
		}
		if err := setNote(*stored); err != nil {
			t.Errorf("the check refused %q, which the command stores: %v", *stored, err)
		}
		if err := setNote(note); err != nil {
			t.Errorf("the check refused %q before it was trimmed: %v", note, err)
		}
	}

	for _, long := range []string{strings.Repeat("a", domain.MaxStudentNote+1), "\n" + strings.Repeat("a", domain.MaxStudentNote+1) + "\u00A0"} {
		if err := setNote(long); !violatesTheNoteCheck(err) {
			t.Errorf("a note of %d bytes got through: %v", len(long), err)
		}
	}
}
