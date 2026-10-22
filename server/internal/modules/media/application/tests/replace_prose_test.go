//go:build integration

package application_test

import (
	"context"
	"strings"
	"testing"

	"quizzivy/internal/modules/media/domain"
)

// shows gives question a prompt, or an explanation, that shows asset with alt,
// and the reference row the bank write keeps beside it (DG-109).
func (w *replacementWorld) shows(t *testing.T, question, field, asset string) {
	t.Helper()
	raw := `{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Hỏi","marks":["bold"]}]},{"type":"image","assetId":"` + asset + `","alt":"Giữ nguyên"}]}`
	column, text := "prompt", "prompt"
	if field == "explanation" {
		column, text = "explanation", "explanation"
	}
	w.exec(t, `UPDATE app.questions SET `+column+`_content=$2, `+text+`=$3 WHERE id=$1`, question, []byte(raw), "Hỏi\n\nGiữ nguyên")
	w.exec(t, `INSERT INTO app.question_media_references(question_id,field,media_asset_id,media_asset_kind) VALUES($1,$2,$3,'image')`, question, field, asset)
}

// TestReplacementRepointsProseAndItsReferencesAndLeavesFrozenRows is T-R6.5b's
// replace: a live question's prompt and explanation, and their reference rows,
// follow the replacement as a group's material does; a question counts once
// whichever of its parts named the file; another owner's question and a
// published version keep the old file.
func TestReplacementRepointsProseAndItsReferencesAndLeavesFrozenRows(t *testing.T) {
	w := newReplacementWorld(t, nil)
	old := w.asset(t, w.a, domain.KindImage, 20)
	other := w.asset(t, w.a, domain.KindImage, 20)
	plain := w.asset(t, w.a, domain.KindImage, 20)
	shown := w.question(t, w.a, plain, domain.KindImage, nil)
	w.shows(t, shown, "prompt", old)
	w.shows(t, shown, "explanation", old)
	both := w.question(t, w.a, old, domain.KindImage, nil)
	w.shows(t, both, "prompt", old)
	bank := w.group(t, w.a, nil)
	member := w.question(t, w.a, other, domain.KindImage, &bank)
	w.shows(t, member, "explanation", old)
	foreign := w.question(t, w.b, plain, domain.KindImage, nil)
	w.shows(t, foreign, "prompt", old)
	test, _ := w.section(t, w.a)
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 1, $2) RETURNING id::text`, test, w.a)
	section := w.id(t, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, version)
	frozen := w.id(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points) VALUES ($1, 0, 'short_answer', 'Hỏi', 1) RETURNING id::text`, section)
	w.exec(t, `INSERT INTO app.test_version_question_media_references (test_version_question_id, field, media_asset_id, media_asset_kind) VALUES ($1, 'prompt', $2, 'image')`, frozen, old)

	out, err := w.repo.Replace(context.Background(), w.input(old, w.a, domain.KindImage, 30, 500))
	if err != nil {
		t.Fatal(err)
	}
	if out.Repointed != (domain.ReplacementCounts{Questions: 3, Groups: 1}) || out.Left != (domain.ReplacementCounts{Questions: 1}) {
		t.Fatalf("counts repointed=%+v left=%+v", out.Repointed, out.Left)
	}
	for _, id := range []string{shown, both, member} {
		raw := w.text(t, `SELECT coalesce(prompt_content::text,'') || coalesce(explanation_content::text,'') FROM app.questions WHERE id=$1`, id)
		if strings.Contains(raw, old) || !strings.Contains(raw, out.Asset.ID) || !strings.Contains(raw, "Giữ nguyên") || !strings.Contains(raw, "bold") {
			t.Fatalf("question %s holds %s", id, raw)
		}
		if n := w.scalar(t, `SELECT count(*) FROM app.question_media_references WHERE question_id=$1 AND media_asset_id=$2`, id, old); n != 0 {
			t.Fatalf("question %s keeps %d rows naming the old file", id, n)
		}
	}
	if n := w.scalar(t, `SELECT count(*) FROM app.question_media_references WHERE question_id=$1 AND media_asset_id=$2`, shown, out.Asset.ID); n != 2 {
		t.Fatalf("the prompt and explanation hold %d rows naming the new file, want 2", n)
	}
	if got := w.text(t, `SELECT media_asset_id::text FROM app.questions WHERE id=$1`, shown); got != plain {
		t.Fatalf("an attachment of another file moved to %s", got)
	}
	if raw := w.text(t, `SELECT prompt_content::text FROM app.questions WHERE id=$1`, foreign); !strings.Contains(raw, old) {
		t.Fatalf("another owner's prompt moved: %s", raw)
	}
	if n := w.scalar(t, `SELECT count(*) FROM app.question_media_references WHERE question_id=$1 AND media_asset_id=$2`, foreign, old); n != 1 {
		t.Fatalf("another owner's reference moved")
	}
	if n := w.scalar(t, `SELECT count(*) FROM app.test_version_question_media_references WHERE test_version_question_id=$1 AND media_asset_id=$2`, frozen, old); n != 1 {
		t.Fatal("the published version's reference moved")
	}
	if got := w.scalar(t, `SELECT revision FROM app.question_groups WHERE id=$1`, bank); got != 2 {
		t.Fatalf("the member's group revision=%d", got)
	}
}
