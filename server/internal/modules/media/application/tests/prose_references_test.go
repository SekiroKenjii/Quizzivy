//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"

	"github.com/jackc/pgx/v5/pgconn"
)

// DG-109 (D-064 ruling 4): an image or audio block in a question's prompt or
// explanation is a row of app.question_media_references, or of
// app.test_version_question_media_references once frozen (00154). Each reader
// below treats such a row exactly as it treats the question's attached media
// or a group's material, so "Used in", the delete check, the sweep and a
// student's reach never scan JSON.

func (w *mediaScope) shows(t *testing.T, question, field, asset string, kind domain.Kind) {
	t.Helper()
	w.exec(t, `INSERT INTO app.question_media_references (question_id, field, media_asset_id, media_asset_kind) VALUES ($1, $2, $3, $4::app.media_kind)`, question, field, asset, string(kind))
}

func (w *mediaScope) bare(t *testing.T, owner string, group *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, context_group_id, context_ordinal, context_option_order)
		VALUES ('short_answer', 'Nhìn hình', 1, $1, $1, $2::uuid, CASE WHEN $2::uuid IS NOT NULL THEN 0 END, CASE WHEN $2::uuid IS NOT NULL THEN 'shuffle' END) RETURNING id::text`, owner, group)
}

// frozenShows publishes a version of a new test of owner's holding one
// question whose field shows asset, and answers the version's section.
func (w *mediaScope) frozenShows(t *testing.T, owner, field, asset string, kind domain.Kind) string {
	t.Helper()
	section := w.versionSection(t, w.test(t, owner))
	question := w.id(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points) VALUES ($1, 0, 'short_answer', 'Nhìn hình', 1) RETURNING id::text`, section)
	w.exec(t, `INSERT INTO app.test_version_question_media_references (test_version_question_id, field, media_asset_id, media_asset_kind) VALUES ($1, $2, $3, $4::app.media_kind)`, question, field, asset, string(kind))
	return section
}

// sitting gives a new student an attempt on the version that holds section.
func (w *mediaScope) sitting(t *testing.T, section string) string {
	t.Helper()
	student := w.user(t, "student")
	assignment := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		SELECT v.test_id, v.id, now() - interval '1 hour', now() + interval '1 hour', 45, v.published_by, now()
		  FROM app.test_version_sections s JOIN app.test_versions v ON v.id = s.test_version_id WHERE s.id = $1 RETURNING id::text`, section)
	w.exec(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash, deadline_at)
		SELECT a.id, a.test_version_id, $2, 1, 'in_progress', gen_random_uuid(), 7, sha256('b'::bytea), now() + interval '1 hour'
		  FROM app.assignments a WHERE a.id = $1`, assignment, student)
	return student
}

func (w *mediaScope) reaches(t *testing.T, student, asset string) bool {
	t.Helper()
	ok, err := repositories.ReachableByStudent(context.Background(), w.tx, student, asset)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func (w *mediaScope) counts(t *testing.T, asset string) int {
	t.Helper()
	counts, err := repositories.QuestionCounts(context.Background(), w.tx, []string{asset})
	if err != nil {
		t.Fatal(err)
	}
	return counts[asset]
}

func (w *mediaScope) retire(t *testing.T, scope string, asset string) (domain.Retired, error) {
	t.Helper()
	return w.svc.Commands.Delete.Handle(context.Background(), command.Delete{Input: domain.DeleteInput{ID: asset, ActorID: scope}})
}

func TestAnAssetAQuestionShowsIsUsedAsIfItWereAttached(t *testing.T) {
	w := newMediaScope(t)
	for _, field := range []string{"prompt", "explanation"} {
		image := w.asset(t, w.b, domain.KindImage)
		question := w.bare(t, w.a, nil)
		if w.reads(t, w.scopeA, image) || w.counts(t, image) != 0 {
			t.Fatalf("%s: another teacher's unused image is readable or counted", field)
		}
		w.shows(t, question, field, image, domain.KindImage)
		if !w.reads(t, w.scopeA, image) {
			t.Errorf("%s: the teacher whose live question shows the image cannot read it", field)
		}
		if got := w.counts(t, image); got != 1 {
			t.Errorf("%s: the image is used by %d questions, want 1", field, got)
		}
		w.exec(t, `UPDATE app.questions SET deleted_at = now() WHERE id = $1`, question)
		if w.reads(t, w.scopeA, image) || w.counts(t, image) != 0 {
			t.Errorf("%s: a deleted question still uses the image", field)
		}
	}
}

func TestALiveQuestionShowingAnAssetKeepsItsFileAfterTheDelete(t *testing.T) {
	w := newMediaScope(t)
	shown, loose := w.asset(t, w.a, domain.KindImage), w.asset(t, w.a, domain.KindImage)
	w.shows(t, w.bare(t, w.a, nil), "explanation", shown, domain.KindImage)

	retired, err := w.retire(t, w.a, shown)
	if err != nil {
		t.Fatal(err)
	}
	if retired.DeleteAt != nil {
		t.Errorf("the file a live explanation shows is due for removal at %v; it must wait while the question names it", retired.DeleteAt)
	}
	retired, err = w.retire(t, w.a, loose)
	if err != nil || retired.DeleteAt == nil {
		t.Errorf("an unnamed file: %+v, %v; want a removal time", retired, err)
	}
}

func TestAPublishedPromptOrExplanationRefusesTheDeleteAndNamesItsTest(t *testing.T) {
	w := newMediaScope(t)
	for _, field := range []string{"prompt", "explanation"} {
		image := w.asset(t, w.a, domain.KindImage)
		w.frozenShows(t, w.a, field, image, domain.KindImage)
		_, err := w.retire(t, w.a, image)
		var referenced *domain.ReferencedError
		if !errors.As(err, &referenced) || len(referenced.Tests) != 1 || referenced.Tests[0].Version != 1 {
			t.Errorf("%s: delete answered %v, want the published version named", field, err)
		}
		if w.deleted(t, image) {
			t.Errorf("%s: the refused delete removed the image", field)
		}
		if !w.reads(t, w.scopeA, image) {
			t.Errorf("%s: the author cannot read the image of their published version", field)
		}
	}
}

func TestAGroupMemberShowingAnAssetRefusesTheDeleteAndNamesTheGroup(t *testing.T) {
	w := newMediaScope(t)
	audio := w.asset(t, w.a, domain.KindAudio)
	group := w.group(t, w.a, nil)
	w.shows(t, w.bare(t, w.a, &group), "explanation", audio, domain.KindAudio)

	_, err := w.retire(t, w.a, audio)
	var referenced *domain.ReferencedError
	if !errors.As(err, &referenced) || len(referenced.Groups) != 1 || referenced.Groups[0].ID != group {
		t.Fatalf("delete answered %v, want the group named", err)
	}
}

// TestAStudentReachesPromptMediaAndNeverExplanationMedia is D-064 ruling 6:
// an explanation image can show a worked answer, so no student signing path
// reaches it while the attempt runs; the result projection signs it (R6-13).
func TestAStudentReachesPromptMediaAndNeverExplanationMedia(t *testing.T) {
	w := newMediaScope(t)
	prompt, explanation := w.asset(t, w.a, domain.KindImage), w.asset(t, w.a, domain.KindImage)
	section := w.frozenShows(t, w.a, "prompt", prompt, domain.KindImage)
	question := w.id(t, `SELECT id::text FROM app.test_version_questions WHERE test_version_section_id = $1`, section)
	w.exec(t, `INSERT INTO app.test_version_question_media_references (test_version_question_id, field, media_asset_id, media_asset_kind) VALUES ($1, 'explanation', $2, 'image')`, question, explanation)
	sitter, outsider := w.sitting(t, section), w.user(t, "student")

	if !w.reaches(t, sitter, prompt) {
		t.Error("the student sitting the paper cannot reach its prompt image")
	}
	if w.reaches(t, sitter, explanation) {
		t.Error("the student sitting the paper reached an explanation image before the result")
	}
	if w.reaches(t, outsider, prompt) {
		t.Error("a student with no attempt reached the prompt image")
	}
}

func TestTheReferenceTablesHoldOnlyWhatTheirKeysAllow(t *testing.T) {
	w := newMediaScope(t)
	image, audio := w.asset(t, w.a, domain.KindImage), w.asset(t, w.a, domain.KindAudio)
	question := w.bare(t, w.a, nil)
	refused := func(name, code, sql string, args ...any) {
		t.Helper()
		sp, err := w.tx.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = sp.Rollback(context.Background()) }()
		_, err = sp.Exec(context.Background(), sql, args...)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != code {
			t.Errorf("%s: %v, want SQLSTATE %s", name, err, code)
		}
	}
	insert := `INSERT INTO app.question_media_references (question_id, field, media_asset_id, media_asset_kind) VALUES ($1, $2, $3, $4::app.media_kind)`
	refused("an image block naming a recording", "23503", insert, question, "prompt", audio, "image")
	refused("a field that holds no prose", "23514", insert, question, "transcript", image, "image")
	w.shows(t, question, "prompt", image, domain.KindImage)
	refused("the same asset twice in one field", "23505", insert, question, "prompt", image, "image")
	w.shows(t, question, "explanation", image, domain.KindImage)
	refused("removing the asset row while a question shows it", "23001", `DELETE FROM app.media_assets WHERE id = $1`, image)
	w.frozenShows(t, w.a, "prompt", audio, domain.KindAudio)
	w.exec(t, `DELETE FROM app.question_media_references WHERE media_asset_id = $1`, image)
	refused("removing the asset row while a published version shows it", "23001", `DELETE FROM app.media_assets WHERE id = $1`, audio)

	w.shows(t, question, "prompt", image, domain.KindImage)
	w.exec(t, `DELETE FROM app.questions WHERE id = $1`, question)
	if left := w.id(t, `SELECT count(*)::text FROM app.question_media_references WHERE question_id = $1`, question); left != "0" {
		t.Errorf("removing the question left %s of its references", left)
	}
}
