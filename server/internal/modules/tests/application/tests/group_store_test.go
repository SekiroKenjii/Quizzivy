//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/core/adapters"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func groupIdentity(t *testing.T) string {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

func groupValue[T any](value T) *T { return &value }

func storedGroupFixture(t *testing.T, asset string) domain.GroupBundle {
	t.Helper()
	choice, blank := groupIdentity(t), groupIdentity(t)
	group := domain.QuestionGroup{
		ID: groupIdentity(t), Title: "Bài đọc dùng chung",
		Members: []domain.GroupMember{{QuestionID: choice, OptionOrder: "fixed"}, {QuestionID: blank, OptionOrder: "shuffle"}},
		Stimuli: []domain.GroupStimulus{{ID: groupIdentity(t), Title: "Bài đọc",
			Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"choice","label":"1"},{"type":"gap","id":"blank","label":"2"}]}]}`),
			Gaps:    []domain.GroupGapBinding{{Kind: "question", GapID: "choice", QuestionID: choice}, {Kind: "blank", GapID: "blank", QuestionID: blank, BlankGapID: groupValue("answer")}},
		}},
		Recordings: []domain.GroupRecording{},
	}
	if asset != "" {
		group.Stimuli = append(group.Stimuli, domain.GroupStimulus{ID: groupIdentity(t), Title: "Bài nghe", Gaps: []domain.GroupGapBinding{}, Content: json.RawMessage(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"audio","assetId":%q,"label":"Hội thoại"}]}`, asset))})
		group.Recordings = append(group.Recordings, domain.GroupRecording{ID: groupIdentity(t), AssetID: asset, Policy: questions.AudioPolicy{MaxPlays: groupValue(2)}, Transcript: groupValue("Lời thoại riêng")})
	}
	return domain.GroupBundle{Group: group, Questions: []domain.GroupQuestion{
		{ID: choice, Input: questions.Input{Type: questions.SingleChoice, Prompt: "Chọn từ", Points: "0.25", Options: []questions.OptionInput{{Text: "first", IsCorrect: true}, {Text: "second"}}}},
		{ID: blank, Input: questions.Input{Type: questions.FillBlank, Prompt: "[1]", Points: "0.75", PromptContent: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"answer","label":"1"}]}]}`), Blanks: []questions.BlankInput{{GapID: groupValue("answer"), Ordinal: 1, AcceptedAnswers: []string{"chính xác"}}}}},
	}}
}

func storedGroupAsset(t *testing.T, tx pgx.Tx, author, kind string) string {
	t.Helper()
	var id string
	err := tx.QueryRow(context.Background(), `INSERT INTO app.media_assets
		(kind,storage_key,mime_type,bytes,duration_ms,original_filename,checksum_sha256,uploaded_by)
		VALUES ($1::app.media_kind,$2,CASE WHEN $1='audio' THEN 'audio/mpeg' ELSE 'image/png' END,100,
		CASE WHEN $1='audio' THEN 1000 ELSE NULL END,'fixture', $3,$4) RETURNING id::text`, kind, groupIdentity(t), make([]byte, 32), author).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func groupTransaction(t *testing.T) (pgx.Tx, string, *repositories.GroupsPostgres) {
	t.Helper()
	pool := newPool(t)
	author := makeAuthor(t, pool)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	media := mediarepo.NewPostgres(db.NewContext(tx))
	return tx, author, repositories.NewGroupsPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media)
}

func TestGroupStoreRoundTripKeepsContextAndProtectsMedia(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	asset := storedGroupAsset(t, tx, author, "audio")
	source := storedGroupFixture(t, asset)
	written, err := repo.Create(ctx, domain.CreateGroupInput{Bundle: source, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Get(ctx, written.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Revision != 1 || loaded.OwnerSectionID != nil || loaded.ArchivedAt != nil || len(loaded.Bundle.Group.Stimuli) != 2 || len(loaded.Bundle.Group.Members) != 2 {
		t.Fatalf("incomplete graph: %+v", loaded)
	}
	if loaded.Bundle.Questions[1].Input.Blanks[0].AcceptedAnswers[0] != "chính xác" || *loaded.Bundle.Group.Recordings[0].Policy.MaxPlays != 2 || *loaded.Bundle.Group.Recordings[0].Transcript != "Lời thoại riêng" {
		t.Fatal("stored graph changed answers or playback policy")
	}
	media := mediarepo.NewPostgres(db.NewContext(tx))
	err = media.SoftDelete(ctx, mediadomain.DeleteInput{ID: asset, ActorID: author, Now: time.Now()})
	var blocked *mediadomain.ReferencedError
	if !errors.As(err, &blocked) || len(blocked.Groups) != 1 || blocked.Groups[0].ID != source.Group.ID || blocked.Groups[0].TestID != nil {
		t.Fatalf("media not protected by bank context: %+v, %v", blocked, err)
	}
	var events int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1 AND action='question_group.created'`, source.Group.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("aggregate audit count: %d, %v", events, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.group_stimulus_assets WHERE group_id=$1`, source.Group.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, source.Group.ID); err == nil {
		t.Fatal("read accepted missing relational media bindings")
	}
}

func TestGroupStoreRollsBackMembersAndAuditWhenAssetKindFails(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	image := storedGroupAsset(t, tx, author, "image")
	source := storedGroupFixture(t, image)
	if _, err := repo.Create(ctx, domain.CreateGroupInput{Bundle: source, ActorID: author, Now: time.Now()}); err == nil {
		t.Fatal("audio binding accepted an image asset")
	}
	for _, statement := range []string{
		`SELECT count(*) FROM app.question_groups WHERE id=$1`,
		`SELECT count(*) FROM app.questions WHERE context_group_id=$1`,
		`SELECT count(*) FROM app.audit_log WHERE entity_id=$1`,
	} {
		var count int
		if err := tx.QueryRow(ctx, statement, source.Group.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial group survived failure: %d, %v", count, err)
		}
	}
	var questionEvents int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=ANY($1::uuid[])`, []string{source.Questions[0].ID, source.Questions[1].ID}).Scan(&questionEvents); err != nil || questionEvents != 0 {
		t.Fatalf("member audits survived failed graph: %d, %v", questionEvents, err)
	}
}

func TestGroupStoreMountsEmptyGroupAndChecksDraftRevision(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	var testID, sectionID string
	var updated time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests (title,created_by) VALUES ('Draft',$1) RETURNING id::text,updated_at`, author).Scan(&testID, &updated); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_sections (test_id,ordinal,title) VALUES ($1,0,'Part') RETURNING id::text`, testID).Scan(&sectionID); err != nil {
		t.Fatal(err)
	}
	bundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupIdentity(t), Title: "Nhóm trống"}}
	in := domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &sectionID, ExpectedTestUpdatedAt: updated.Add(-time.Second), ActorID: author, Now: time.Now()}
	if _, err := repo.Create(ctx, in); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale outline accepted a group: %v", err)
	}
	in.ExpectedTestUpdatedAt = updated
	saved, err := repo.Create(ctx, in)
	if err != nil || saved.OwnerSectionID == nil || *saved.OwnerSectionID != sectionID {
		t.Fatalf("empty group create: %+v, %v", saved, err)
	}
	var groupID string
	if err := tx.QueryRow(ctx, `SELECT group_id::text FROM app.test_section_units WHERE test_section_id=$1 AND ordinal=0`, sectionID).Scan(&groupID); err != nil || groupID != bundle.Group.ID {
		t.Fatalf("empty group has no outline position: %s, %v", groupID, err)
	}
}

func TestGroupStoreCopySurvivesSourceEditsAndDeletion(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	asset := storedGroupAsset(t, tx, author, "audio")
	source, err := repo.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := source.Bundle.Copy(func() string { return groupIdentity(t) })
	if err != nil {
		t.Fatal(err)
	}
	copied, err := repo.Create(ctx, domain.CreateGroupInput{Bundle: bundle, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(copied)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE app.group_stimuli SET title='Changed source' WHERE group_id=$1`,
		`UPDATE app.group_recordings SET max_plays=9,transcript='Changed source' WHERE group_id=$1`,
		`DELETE FROM app.questions WHERE context_group_id=$1`,
		`DELETE FROM app.question_groups WHERE id=$1`,
	} {
		if _, err := tx.Exec(ctx, statement, source.Bundle.Group.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Get(ctx, copied.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(loaded)
	if err != nil || string(before) != string(after) {
		t.Fatalf("source edits/deletion changed stored copy: %v", err)
	}
	refs, err := mediarepo.GroupReferences(ctx, tx, asset)
	if err != nil || len(refs) != 1 || refs[0].ID != copied.Bundle.Group.ID {
		t.Fatalf("copy lost its own protected asset binding: %+v, %v", refs, err)
	}
}

func TestGroupStorePreservesStandaloneOrderWhenMounting(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	var testID, sectionID string
	var updated time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests (title,created_by) VALUES ('Draft',$1) RETURNING id::text,updated_at`, author).Scan(&testID, &updated); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_sections (test_id,ordinal,title) VALUES ($1,0,'Part') RETURNING id::text`, testID).Scan(&sectionID); err != nil {
		t.Fatal(err)
	}
	ids := []string{groupIdentity(t), groupIdentity(t)}
	for i, id := range ids {
		if _, err := tx.Exec(ctx, `INSERT INTO app.questions (id,type,prompt,points,created_by) VALUES ($1,'short_answer','Standalone',1,$2)`, id, author); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id,question_id,ordinal) VALUES ($1,$2,$3)`, sectionID, id, 1-i); err != nil {
			t.Fatal(err)
		}
	}
	bundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupIdentity(t), Title: "Nhóm trống"}}
	if _, err := repo.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &sectionID, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `SELECT coalesce(question_id,group_id)::text FROM app.test_section_units WHERE test_section_id=$1 ORDER BY ordinal`, sectionID)
	if err != nil {
		t.Fatal(err)
	}
	units, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(units) != 3 || units[0] != ids[1] || units[1] != ids[0] || units[2] != bundle.Group.ID {
		t.Fatalf("outline order changed: %+v, %v", units, err)
	}
}

func TestGroupStoreArchivedGroupStillProtectsMemberAudio(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	asset := storedGroupAsset(t, tx, author, "audio")
	bundle := storedGroupFixture(t, "")
	bundle.Questions[0].Input.MediaAssetID = &asset
	bundle.Questions[0].MediaAssetKind = groupValue("audio")
	bundle.Questions[0].Input.Audio = &questions.AudioPolicy{MaxPlays: groupValue(1)}
	if _, err := repo.Create(ctx, domain.CreateGroupInput{Bundle: bundle, ActorID: author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.question_groups SET archived_at=clock_timestamp() WHERE id=$1`, bundle.Group.ID); err != nil {
		t.Fatal(err)
	}
	media := mediarepo.NewPostgres(db.NewContext(tx))
	err := media.SoftDelete(ctx, mediadomain.DeleteInput{ID: asset, ActorID: author, Now: time.Now()})
	var blocked *mediadomain.ReferencedError
	if !errors.As(err, &blocked) || len(blocked.Groups) != 1 || blocked.Groups[0].ID != bundle.Group.ID {
		t.Fatalf("archival orphaned member audio: %+v, %v", blocked, err)
	}
}
