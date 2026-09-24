//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	mediarepo "quizzivy/internal/modules/media/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testsquery "quizzivy/internal/modules/tests/application/query"
	testsdomain "quizzivy/internal/modules/tests/domain"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

func TestAttemptSharedContextIsFrozenSafeAndOwned(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	other := seedWorld(t, pool, openAssignment())
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	groupID, assetID := attemptGroupFixture(t, tx, w)
	_, otherAsset := attemptGroupFixture(t, tx, other)
	dbx := db.NewContext(tx)
	papers := testsapp.New(testsrepo.NewPostgres(dbx, nil, nil))
	svc := application.New(nil, nil, repositories.NewPostgres(dbx)).WithGroupContexts(papers.Queries.GroupContexts)
	reachable, err := mediarepo.ReachableByStudent(ctx, tx, w.student, assetID)
	if err != nil || reachable {
		t.Fatalf("targeting alone granted shared media: %v, %v", reachable, err)
	}
	session, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatal(err)
	}
	if len(session.Groups) != 1 || session.Groups[0].ID != groupID {
		t.Fatalf("missing context: %+v", session.Groups)
	}
	group := session.Groups[0]
	if !reflect.DeepEqual(group.QuestionIDs, []string{w.choice, w.blank}) || len(group.Stimuli) != 1 || len(group.Recordings) != 1 || *group.Recordings[0].Policy.MaxPlays != 2 {
		t.Fatalf("lost frozen membership/material/policy: %+v", group)
	}
	if group.Stimuli[0].Gaps[0].QuestionID != w.choice || group.AssetIDs[0] != assetID {
		t.Fatal("wrong shared bindings")
	}
	body, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{secretTranscript, secretBlankAnswer, secretExplanation, secretSampleAnswer, "IsCorrect", "AcceptedAnswers", "SampleAnswer", `"Transcript"`} {
		if strings.Contains(string(body), secret) {
			t.Fatalf("session leaked %s", secret)
		}
	}
	for _, student := range []string{w.student, w.outsider} {
		reachable, err := mediarepo.ReachableByStudent(ctx, tx, student, assetID)
		if err != nil || reachable != (student == w.student) {
			t.Fatalf("shared media reachability for %s: %v, %v", student, reachable, err)
		}
	}
	reachable, err = mediarepo.ReachableByStudent(ctx, tx, w.student, otherAsset)
	if err != nil || reachable {
		t.Fatalf("an unrelated frozen group granted media: %v, %v", reachable, err)
	}
	if _, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: session.Attempt.ID, StudentID: w.outsider}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("outsider read context: %v", err)
	}
	loaded, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: session.Attempt.ID, StudentID: w.student})
	if err != nil || !reflect.DeepEqual(loaded.Groups, session.Groups) {
		t.Fatalf("reload changed context: %v", err)
	}
	resumed, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil || resumed.SessionID == session.SessionID || !reflect.DeepEqual(resumed.Groups, session.Groups) {
		t.Fatalf("takeover changed context: %v", err)
	}
	unconfigured := application.New(nil, nil, repositories.NewPostgres(dbx))
	if _, err := unconfigured.Queries.Get.Handle(ctx, query.Get{AttemptID: session.Attempt.ID, StudentID: w.student}); !errors.Is(err, domain.ErrGroupContextUnavailable) {
		t.Fatalf("missing reader silently discarded context: %v", err)
	}
	absent, err := papers.Queries.GroupContexts.Handle(ctx, testsquery.GroupContexts{VersionID: "00000000-0000-7000-8000-000000000000"})
	if !errors.Is(err, testsdomain.ErrNotFound) || len(absent) != 0 {
		t.Fatalf("missing version: %+v, %v", absent, err)
	}
}

func attemptGroupFixture(t *testing.T, tx pgx.Tx, w world) (string, string) {
	t.Helper()
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	var section, group, asset, material, recording string
	if err := tx.QueryRow(ctx, `SELECT test_version_section_id::text FROM app.test_version_questions WHERE id=$1`, w.choice).Scan(&section); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_groups(test_version_section_id,title) VALUES($1,'Shared frozen passage') RETURNING id::text`, section).Scan(&group); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO app.test_version_group_members(group_id,test_version_section_id,question_id,ordinal,option_order)
        VALUES($1,$2,$3,0,'shuffle'),($1,$2,$4,1,'shuffle')`, group, section, w.choice, w.blank)
	exec(`INSERT INTO app.test_version_units(test_version_section_id,ordinal,group_id,question_id)
        VALUES($1,0,$2,NULL),($1,1,NULL,$3),($1,2,NULL,$4)`, section, group, w.essay, w.listening)
	if err := tx.QueryRow(ctx, `INSERT INTO app.media_assets(kind,storage_key,mime_type,bytes,duration_ms,original_filename,checksum_sha256,uploaded_by)
        VALUES('audio',$1,'audio/mpeg',100,1000,'shared.mp3',sha256(convert_to($1,'UTF8')),$2) RETURNING id::text`, "audio/group-"+group, w.admin).Scan(&asset); err != nil {
		t.Fatal(err)
	}
	content := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Shared passage "},{"type":"gap","id":"answer","label":"1"}]},{"type":"audio","assetId":"` + asset + `","label":"Shared recording"}]}`)
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_group_stimuli(group_id,ordinal,title,content) VALUES($1,0,'Material',$2) RETURNING id::text`, group, content).Scan(&material); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO app.test_version_group_gap_bindings(stimulus_id,group_id,gap_id,kind,question_id) VALUES($1,$2,'answer','question',$3)`, material, group, w.choice)
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_group_recordings(group_id,media_asset_id,max_plays,transcript) VALUES($1,$2,2,$3) RETURNING id::text`, group, asset, secretTranscript).Scan(&recording); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO app.test_version_group_assets(stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id) VALUES($1,$2,$3,'audio',$4)`, material, group, asset, recording)
	return group, asset
}
