//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"quizzivy/internal/core/adapters"
	importsdomain "quizzivy/internal/modules/imports/domain"
	importsrepo "quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/domain"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testsdomain "quizzivy/internal/modules/tests/domain"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"testing"
)

func TestReplacementKeepsReviewBytesAndImportMaterializerCanStillBindOldAsset(t *testing.T) {
	w := newReplacementWorld(t, nil)
	old := w.asset(t, w.a, domain.KindImage, 20)
	material := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"image","assetId":"` + old + `","alt":"Ảnh cũ"}]}`)
	draft := importsdomain.Draft{Version: importsdomain.DraftVersion, Title: "Bản xem xét", Notices: []importsdomain.Finding{}, Acknowledged: []string{}, Sections: []importsdomain.DraftSection{{ID: "section", Title: "Phần", Origin: importsdomain.SourceExplicit, Source: []importsdomain.SourceRef{}, Items: []importsdomain.DraftItem{{Group: &importsdomain.DraftGroup{ID: "group", Stimulus: material, Gaps: []importsdomain.GapLink{}, Questions: []importsdomain.DraftQuestion{}, Source: []importsdomain.SourceRef{}}}}}}}
	raw, err := json.Marshal(draft)
	if err != nil {
		t.Fatal(err)
	}
	imported := w.id(t, `INSERT INTO app.word_imports(created_by,request_id,title,status) VALUES($1,$2,'Review fixture','needs_review') RETURNING id::text`, w.a, uuid.NewString())
	w.exec(t, `INSERT INTO app.word_import_source_sets(import_id,revision,created_by) VALUES($1,1,$2)`, imported, w.a)
	w.exec(t, `UPDATE app.word_imports SET source_revision=1 WHERE id=$1`, imported)
	run := w.id(t, `INSERT INTO app.word_import_runs(import_id,source_revision,request_id,requested_by,expected_revision,pipeline_version,status,stage,result,completed_at) VALUES($1,1,$2,$3,1,'replacement-test','succeeded','ready','{}',now()) RETURNING id::text`, imported, uuid.NewString(), w.a)
	w.exec(t, `INSERT INTO app.word_import_drafts(import_id,run_id,body,edited_by) VALUES($1,$2,$3,$4)`, imported, run, raw, w.a)
	before := w.text(t, `SELECT to_jsonb(d)::text FROM app.word_import_drafts d WHERE import_id=$1`, imported)
	if _, err := w.repo.Replace(context.Background(), w.input(old, w.a, domain.KindImage, 30, 30)); err != nil {
		t.Fatal(err)
	}
	if after := w.text(t, `SELECT to_jsonb(d)::text FROM app.word_import_drafts d WHERE import_id=$1`, imported); after != before {
		t.Fatal("replacement changed import review bytes or revision")
	}
	media := application.New(w.repo, newFakeStore(), audioProbe{})
	kinds := adapters.MediaKinds{Media: media}
	committer := adapters.ImportCommitter{DB: db.NewContext(w.pool), Questions: func(dbx db.Context) *questionsapp.Application {
		return questionsapp.New(questionsrepo.NewPostgres(dbx), kinds)
	}, Tests: func(dbx db.Context) *testsapp.Application {
		q := questionsrepo.NewPostgres(dbx)
		return testsapp.New(testsrepo.NewPostgres(dbx, q, w.repo).WithGroupQuestions(adapters.GroupQuestions{})).WithGroups(testsrepo.NewGroupsPostgres(dbx, adapters.GroupQuestions{}, w.repo), kinds)
	}}
	member, groupID, stimulus := uuid.NewString(), uuid.NewString(), uuid.NewString()
	bundle := testsdomain.GroupBundle{Group: testsdomain.QuestionGroup{ID: groupID, Title: "Ngữ liệu giữ ảnh cũ", Members: []testsdomain.GroupMember{{QuestionID: member, OptionOrder: "shuffle"}}, Stimuli: []testsdomain.GroupStimulus{{ID: stimulus, Title: "Ảnh", Content: material, Gaps: []testsdomain.GroupGapBinding{}}}, Recordings: []testsdomain.GroupRecording{}}, Questions: []testsdomain.GroupQuestion{{ID: member, Input: questionsdomain.Input{Type: questionsdomain.ShortAnswer, Prompt: "Trả lời", Points: "1", Tags: []string{}}}}}
	plan := importsdomain.CommitPlan{Title: "Import after replacement", Sections: []importsdomain.PlanSection{{Title: "Phần", Units: []importsdomain.PlanUnit{{Group: &bundle}}}}}
	recorded := false
	testID, err := committer.Materialize(context.Background(), plan, w.a, actor.Actor{ID: w.actor, Scope: access.Scope{UserID: w.a}}, func(ctx context.Context, store importsdomain.CommitStore, id string) error {
		stored, err := (store.(*importsrepo.Postgres)).Draft(ctx, access.Scope{UserID: w.a}, imported)
		if err != nil {
			return err
		}
		recorded = stored.Draft.Title == draft.Title && id != ""
		return nil
	})
	if err != nil || !recorded || testID == "" {
		t.Fatalf("public materializer error=%v recorded=%v", err, recorded)
	}
	if bound := w.text(t, `SELECT media_asset_id::text FROM app.group_stimulus_assets WHERE group_id=$1`, groupID); bound != old {
		t.Fatalf("late import bound=%s", bound)
	}
	if after := w.text(t, `SELECT to_jsonb(d)::text FROM app.word_import_drafts d WHERE import_id=$1`, imported); after != before {
		t.Fatal("materialization changed review outside its record callback")
	}
}
