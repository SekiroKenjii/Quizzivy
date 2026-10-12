//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func altImage(t *testing.T, tx pgx.Tx, owner string) string {
	t.Helper()
	var id string
	err := tx.QueryRow(context.Background(), `INSERT INTO app.media_assets
		(kind,storage_key,mime_type,bytes,original_filename,checksum_sha256,uploaded_by,owner_id)
		VALUES ('image',$1,'image/png',100,'meo.png',$2,$3,$3) RETURNING id::text`,
		"image/"+groupIdentity(t), make([]byte, 32), owner).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func frozenAlt(t *testing.T, q db.Querier, versionID string) *string {
	t.Helper()
	var alt *string
	if err := q.QueryRow(context.Background(), `
		SELECT q.media_alt FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1 ORDER BY s.ordinal, q.ordinal LIMIT 1`, versionID).Scan(&alt); err != nil {
		t.Fatal(err)
	}
	return alt
}

func bankAlt(t *testing.T, q db.Querier, questionID string) *string {
	t.Helper()
	var alt *string
	if err := q.QueryRow(context.Background(), `SELECT media_alt FROM app.questions WHERE id = $1`, questionID).Scan(&alt); err != nil {
		t.Fatal(err)
	}
	return alt
}

func wantAlt(t *testing.T, step string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		shown := "<nil>"
		if got != nil {
			shown = *got
		}
		t.Fatalf("%s: alt text = %s, want %q", step, shown, want)
	}
}

func wantNoAlt(t *testing.T, step string, got *string) {
	t.Helper()
	if got != nil {
		t.Fatalf("%s: alt text = %q, want NULL", step, *got)
	}
}

func imageInput(asset string, alt *string) questions.Input {
	return questions.Input{Type: questions.ShortAnswer, Prompt: "Mô tả bức ảnh", Points: "1.00", Tags: []string{}, MediaAssetID: &asset, MediaAlt: alt}
}

func (b *builder) setAlt(questionID, asset string, alt *string) {
	b.t.Helper()
	if _, err := b.qsvc.Commands.Update.Handle(context.Background(), questionscommand.Update{Request: questions.WriteRequest{
		ID: questionID, ActorID: b.author, Input: imageInput(asset, alt),
	}}); err != nil {
		b.t.Fatalf("set alt text of %s: %v", questionID, err)
	}
}

func TestEditingTheAltTextAfterPublishLeavesTheVersionUnchanged(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	ctx := context.Background()
	asset := altImage(t, f.conn, f.author)
	original := "Một chú mèo ngồi trên ghế"
	question := b.question(imageInput(asset, &original))
	draft := b.draft("Đề có ảnh", question)

	version, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	wantAlt(t, "frozen at publish", frozenAlt(t, f.conn, version.ID), original)

	b.setAlt(question, asset, groupValue("Đã sửa sau khi xuất bản"))
	wantAlt(t, "the bank after the edit", bankAlt(t, f.conn, question), "Đã sửa sau khi xuất bản")
	wantAlt(t, "the version after the edit", frozenAlt(t, f.conn, version.ID), original)

	b.setAlt(question, asset, nil)
	wantNoAlt(t, "the bank after clearing", bankAlt(t, f.conn, question))
	wantAlt(t, "the version after clearing", frozenAlt(t, f.conn, version.ID), original)

	request := domain.VersionRequest{Request: reqFor(draft.ID, b.author), Version: 1}
	if err := f.conn.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id = $1`, draft.ID).Scan(&request.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	restored, err := b.tests.Commands.CreateDraftFromVersion.Handle(ctx, command.CreateDraftFromVersion{Request: request})
	if err != nil {
		t.Fatalf("restore as draft: %v", err)
	}
	copyID := restored.Sections[0].QuestionIDs[0]
	if copyID == question {
		t.Fatal("restore reused the bank question")
	}
	wantAlt(t, "the restored copy", bankAlt(t, f.conn, copyID), original)
	wantNoAlt(t, "the bank question the restore left alone", bankAlt(t, f.conn, question))
}

func TestAPublishWithoutAltTextFreezesNull(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	asset := altImage(t, f.conn, f.author)
	withImage := b.question(imageInput(asset, nil))
	plain := b.shortAnswer("Không có ảnh", "1.00")
	version, err := b.publish(b.draft("Đề không có văn bản thay thế", withImage, plain).ID)
	if err != nil {
		t.Fatal(err)
	}
	var withAlt int
	if err := f.conn.QueryRow(context.Background(), `
		SELECT count(*) FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1 AND q.media_alt IS NOT NULL`, version.ID).Scan(&withAlt); err != nil {
		t.Fatal(err)
	}
	if withAlt != 0 {
		t.Fatalf("%d frozen questions carry alt text, want none", withAlt)
	}
}

func TestAltTextOnAnAudioQuestionIsRefusedBeforeItIsWritten(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	audio := storedGroupAsset(t, f.conn, f.author, "audio")
	in := questions.Input{
		Type: questions.ShortAnswer, Prompt: "Nghe và trả lời", Points: "1.00", Tags: []string{},
		MediaAssetID: &audio, MediaAlt: groupValue("Một đoạn hội thoại"), Audio: &questions.AudioPolicy{AllowSeek: true},
	}
	_, err := b.qsvc.Commands.Create.Handle(context.Background(), questionscommand.Create{Request: questions.WriteRequest{Input: in, ActorID: b.author}})
	var invalid *questions.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "mediaAlt" {
		t.Fatalf("err = %v, want one field error on mediaAlt", err)
	}
	var written int
	if err := f.conn.QueryRow(context.Background(), `SELECT count(*) FROM app.questions WHERE created_by = $1`, f.author).Scan(&written); err != nil || written != 0 {
		t.Fatalf("a refused question left %d rows (%v)", written, err)
	}
}

func TestAGroupMemberKeepsItsAltTextThroughPublishDuplicateAndRestore(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	image := storedGroupAsset(t, tx, author, "image")
	original := "Một chú mèo ngồi trên ghế"
	bundle := storedGroupFixture(t, "")
	bundle.Questions[0].Input.MediaAssetID = &image
	bundle.Questions[0].Input.MediaAlt = &original
	bundle.Questions[0].MediaAssetKind = groupValue("image")
	scope := access.Scope{UserID: author}

	stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now(), Scope: scope, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	wantAlt(t, "the group answer", stored.Bundle.Questions[0].Input.MediaAlt, original)
	read, err := groups.Get(ctx, everyone, stored.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantAlt(t, "the group read back", read.Bundle.Questions[0].Input.MediaAlt, original)

	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: scope}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}
	frozenMember := func(step string) {
		t.Helper()
		var alt *string
		if err := tx.QueryRow(ctx, `
			SELECT q.media_alt FROM app.test_version_group_members m
			  JOIN app.test_version_questions q ON q.id = m.question_id
			  JOIN app.test_version_groups g ON g.id = m.group_id
			  JOIN app.test_version_sections s ON s.id = g.test_version_section_id
			 WHERE s.test_version_id = $1 AND m.ordinal = 0`, published.ID).Scan(&alt); err != nil {
			t.Fatal(err)
		}
		wantAlt(t, step, alt, original)
	}
	frozenMember("frozen at publish")

	edited := "Đã sửa sau khi xuất bản"
	read.Bundle.Questions[0].Input.MediaAlt = &edited
	mutation := groupMutation(read, author)
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	changed, err := groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: read.Bundle})
	if err != nil {
		t.Fatal(err)
	}
	wantAlt(t, "the draft group after the edit", changed.Bundle.Questions[0].Input.MediaAlt, edited)
	frozenMember("the version after the draft group's edit")

	memberAlt := func(owner string) *string {
		t.Helper()
		var id string
		if err := tx.QueryRow(ctx, `SELECT u.group_id::text FROM app.test_section_units u JOIN app.test_sections s ON s.id=u.test_section_id
			WHERE s.test_id=$1 AND u.group_id IS NOT NULL`, owner).Scan(&id); err != nil {
			t.Fatal(err)
		}
		group, err := groups.Get(ctx, everyone, id)
		if err != nil {
			t.Fatal(err)
		}
		return group.Bundle.Questions[0].Input.MediaAlt
	}

	copied, err := repo.Duplicate(ctx, domain.DuplicateInput{ID: testID, ActorID: author, Now: time.Now(), Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	wantAlt(t, "the duplicate copies the draft", memberAlt(copied.ID), edited)

	req := domain.VersionRequest{Request: domain.Request{ID: testID, ActorID: author, Scope: scope}, Version: 1}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&req.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateDraftFromVersion(ctx, req, time.Now()); err != nil {
		t.Fatal(err)
	}
	wantAlt(t, "the restore copies the version", memberAlt(testID), original)
	frozenMember("the version after the restore")
}
