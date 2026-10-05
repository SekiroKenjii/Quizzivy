//go:build integration

package application_test

import (
	"context"
	"github.com/google/uuid"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsdomain "quizzivy/internal/modules/tests/domain"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"
	"time"
)

func TestReplacementPublicPublishWaitKeepsV1AndNextPublishUsesV2(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	old := w.asset(t, w.a, domain.KindAudio, 20)
	test, section := w.section(t, w.a)
	group := w.group(t, w.a, &section)
	member := w.question(t, w.a, old, domain.KindAudio, &group)
	w.exec(t, `UPDATE app.questions SET media_asset_id=NULL,media_asset_kind=NULL,audio_allow_seek=NULL,audio_show_transcript_after=NULL WHERE id=$1`, member)
	raw := []byte(`{"format":"semantic_v1","blocks":[{"type":"audio","assetId":"` + old + `","label":"Nghe"}]}`)
	stimulus := w.id(t, `INSERT INTO app.group_stimuli(group_id,ordinal,title,content) VALUES($1,0,'Nghe',$2) RETURNING id::text`, group, raw)
	standalone := w.question(t, w.a, old, domain.KindAudio, nil)
	recording := w.id(t, `INSERT INTO app.group_recordings(group_id,media_asset_id,max_plays,allow_seek,show_transcript_after_submit) VALUES($1,$2,2,false,false) RETURNING id::text`, group, old)
	w.exec(t, `INSERT INTO app.group_stimulus_assets(stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id) VALUES($1,$2,$3,'audio',$4)`, stimulus, group, old, recording)
	w.exec(t, `INSERT INTO app.test_section_units(test_section_id,ordinal,group_id) VALUES($1,0,$2)`, section, group)
	w.exec(t, `INSERT INTO app.test_section_units(test_section_id,ordinal,question_id) VALUES($1,1,$2)`, section, standalone)
	tx := w.blocker(t)
	pid := tx.Conn().PgConn().PID()
	replacementExec(t, tx, `SELECT id FROM app.question_groups WHERE id=$1 FOR SHARE`, group)
	in := w.input(old, w.a, domain.KindAudio, 30, 30)
	answer := w.startReplacement(in)
	w.blocked(t, tr, "ORDER BY g.id FOR UPDATE OF g", pid)
	qrepo := questionsrepo.NewPostgres(db.NewContext(w.pool))
	publisher := testsrepo.NewPostgres(db.NewContext(tx), qrepo, w.repo).WithGroupQuestions(adapters.GroupQuestions{})
	req := testsdomain.PublishRequest{TestID: test, ActorID: w.a, Scope: access.Scope{UserID: w.a}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	first, err := publisher.Publish(ctx, req, time.Now(), testsdomain.Publishing.Validate)
	if err != nil {
		t.Fatalf("public publish while replacement waits: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := w.text(t, `SELECT updated_at::text FROM app.tests WHERE id=$1`, test)
	out := replacementWait(t, answer)
	if out.err != nil {
		t.Fatal(out.err)
	}
	if after := w.text(t, `SELECT updated_at::text FROM app.tests WHERE id=$1`, test); after != before {
		t.Fatalf("replacement changed test revision %s -> %s", before, after)
	}
	frozen := `SELECT q.media_asset_id::text FROM app.test_version_questions q JOIN app.test_version_sections s ON s.id=q.test_version_section_id WHERE s.test_version_id=$1 AND q.media_asset_id IS NOT NULL`
	if got := w.text(t, frozen, first.ID); got != old {
		t.Fatalf("v1 changed=%s", got)
	}
	publisher = testsrepo.NewPostgres(db.NewContext(w.pool), qrepo, w.repo).WithGroupQuestions(adapters.GroupQuestions{})
	second, err := publisher.Publish(context.Background(), req, time.Now(), testsdomain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.text(t, frozen, second.ID); got != out.result.Asset.ID {
		t.Fatalf("v2=%s", got)
	}
	if got := w.text(t, frozen, first.ID); got != old {
		t.Fatalf("v1 changed after v2=%s", got)
	}
	assignment := w.id(t, `INSERT INTO app.assignments(test_id,test_version_id,opens_at,closes_at,duration_minutes,created_by) VALUES($1,$2,now(),now()+interval '1 hour',60,$3) RETURNING id::text`, test, first.ID, w.a)
	w.exec(t, `INSERT INTO app.attempts(assignment_id,test_version_id,student_id,attempt_no,session_id,shuffle_seed,beacon_token_hash,deadline_at) VALUES($1,$2,$3,1,$4,1,decode(repeat('00',32),'hex'),now()+interval '1 hour')`, assignment, first.ID, w.b, uuid.NewString())
	app := application.New(w.repo, newFakeStore(), audioProbe{})
	signed, err := app.Queries.MintForStudent.Handle(context.Background(), query.MintForStudent{StudentID: w.b, AssetID: old})
	if err != nil || signed.URL == "" {
		t.Fatalf("student old asset=%+v %v", signed, err)
	}
	kinds, err := (adapters.MediaKinds{Media: app}).Kind(context.Background(), access.Scope{UserID: w.a}, old)
	if err != nil || kinds != string(domain.KindAudio) {
		t.Fatalf("import binding port old asset=%v %v", kinds, err)
	}
}
