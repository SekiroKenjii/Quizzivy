//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	questionsquery "quizzivy/internal/modules/questions/application/query"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func audioQuestion(asset string, pause *bool) questions.Input {
	return questions.Input{
		Type: questions.ShortAnswer, Prompt: "Nghe và trả lời", Points: "1.00", Tags: []string{},
		MediaAssetID: &asset, Audio: &questions.AudioPolicy{MaxPlays: groupValue(2), AllowPause: pause},
	}
}

func rawPause(t *testing.T, q db.Querier, sql string, args ...any) *bool {
	t.Helper()
	var pause *bool
	if err := q.QueryRow(context.Background(), sql, args...).Scan(&pause); err != nil {
		t.Fatal(err)
	}
	return pause
}

func bankPause(t *testing.T, q db.Querier, questionID string) *bool {
	t.Helper()
	return rawPause(t, q, `SELECT audio_allow_pause FROM app.questions WHERE id = $1`, questionID)
}

func frozenPause(t *testing.T, q db.Querier, versionID string) *bool {
	t.Helper()
	return rawPause(t, q, `
		SELECT q.audio_allow_pause FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1 AND q.media_asset_kind = 'audio'`, versionID)
}

func wantPause(t *testing.T, step string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: allow pause is NULL, want %v", step, want)
	}
	if *got != want {
		t.Fatalf("%s: allow pause = %v, want %v", step, *got, want)
	}
}

func wantNoPause(t *testing.T, step string, got *bool) {
	t.Helper()
	if got != nil {
		t.Fatalf("%s: allow pause = %v, want NULL", step, *got)
	}
}

func (b *builder) save(questionID string, in questions.Input) {
	b.t.Helper()
	if _, err := b.qsvc.Commands.Update.Handle(context.Background(), questionscommand.Update{Request: questions.WriteRequest{
		ID: questionID, ActorID: b.author, Input: in,
	}}); err != nil {
		b.t.Fatalf("save %s: %v", questionID, err)
	}
}

func (b *builder) pauseRuleOf(questionID string) bool {
	b.t.Helper()
	q, err := b.qsvc.Queries.Get.Handle(context.Background(), questionsquery.Get{ID: questionID, Scope: access.Scope{UserID: b.author}})
	if err != nil {
		b.t.Fatal(err)
	}
	if q.Audio == nil || q.Audio.AllowPause == nil {
		b.t.Fatalf("question %s reads back without a pause rule: %+v", questionID, q.Audio)
	}
	return *q.Audio.AllowPause
}

func TestAQuestionCreatedWithoutAllowPauseAllowsIt(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	asset := storedGroupAsset(t, f.conn, f.author, "audio")

	absent := b.question(audioQuestion(asset, nil))
	wantPause(t, "created with the field absent", bankPause(t, f.conn, absent), true)
	if !b.pauseRuleOf(absent) {
		t.Fatal("a question created with the field absent reads back as one that cannot be paused")
	}

	off := b.question(audioQuestion(asset, groupValue(false)))
	wantPause(t, "created with false", bankPause(t, f.conn, off), false)
	if b.pauseRuleOf(off) {
		t.Fatal("a question created with false reads back as one that can be paused")
	}

	image := b.question(imageInput(altImage(t, f.conn, f.author), nil))
	wantNoPause(t, "a question with an image", bankPause(t, f.conn, image))
}

func TestASaveThatLeavesAllowPauseOutKeepsTheStoredValue(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	asset := storedGroupAsset(t, f.conn, f.author, "audio")
	another := storedGroupAsset(t, f.conn, f.author, "audio")
	image := altImage(t, f.conn, f.author)

	question := b.question(audioQuestion(asset, groupValue(false)))
	b.save(question, audioQuestion(asset, nil))
	wantPause(t, "saved again with the field absent", bankPause(t, f.conn, question), false)

	b.save(question, audioQuestion(another, nil))
	wantPause(t, "saved with another recording and the field absent", bankPause(t, f.conn, question), false)

	b.save(question, audioQuestion(another, groupValue(true)))
	wantPause(t, "saved with true", bankPause(t, f.conn, question), true)
	b.save(question, audioQuestion(another, nil))
	wantPause(t, "saved with the field absent after true", bankPause(t, f.conn, question), true)

	b.save(question, audioQuestion(another, groupValue(false)))
	b.save(question, imageInput(image, nil))
	wantNoPause(t, "saved with an image in place of the audio", bankPause(t, f.conn, question))
	b.save(question, audioQuestion(asset, nil))
	wantPause(t, "an audio policy that is new in the request is a create", bankPause(t, f.conn, question), true)
}

func TestAPublishFreezesAllowPauseAndLaterEditsDoNotReachTheVersion(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	ctx := context.Background()
	asset := storedGroupAsset(t, f.conn, f.author, "audio")
	question := b.question(audioQuestion(asset, groupValue(false)))
	draft := b.draft("Đề nghe", question)

	version, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	wantPause(t, "frozen at publish", frozenPause(t, f.conn, version.ID), false)

	b.save(question, audioQuestion(asset, groupValue(true)))
	wantPause(t, "the bank after the edit", bankPause(t, f.conn, question), true)
	wantPause(t, "the version after the edit", frozenPause(t, f.conn, version.ID), false)

	preview, err := b.tests.Queries.Preview.Handle(ctx, query.Preview{TestID: draft.ID, Scope: access.Scope{UserID: b.author}})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Questions) != 1 || preview.Questions[0].AllowPause {
		t.Fatalf("the teacher's preview of the version = %+v, want one question that cannot be paused", preview.Questions)
	}

	request := domain.VersionRequest{Request: reqFor(draft.ID, b.author), Version: 1}
	if err := f.conn.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id = $1`, draft.ID).Scan(&request.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	restored, err := b.tests.Commands.CreateDraftFromVersion.Handle(ctx, command.CreateDraftFromVersion{Request: request})
	if err != nil {
		t.Fatalf("restore as draft: %v", err)
	}
	wantPause(t, "the restored copy", bankPause(t, f.conn, restored.Sections[0].QuestionIDs[0]), false)

	b.save(question, audioQuestion(asset, groupValue(false)))
	duplicate, err := b.qsvc.Commands.Duplicate.Handle(ctx, questionscommand.Duplicate{Request: questions.WriteRequest{ID: question, ActorID: b.author}})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	wantPause(t, "the duplicate", bankPause(t, f.conn, duplicate.ID), false)
}

func TestAVersionFrozenBeforeAllowPauseReadsTrue(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	ctx := context.Background()
	asset := storedGroupAsset(t, f.conn, f.author, "audio")
	draft := b.draft("Đề nghe cũ", b.question(audioQuestion(asset, groupValue(false))))
	version, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := f.conn.Exec(ctx, `
		UPDATE app.test_version_questions SET audio_allow_pause = NULL
		 WHERE test_version_section_id IN (SELECT id FROM app.test_version_sections WHERE test_version_id = $1)`, version.ID); err != nil {
		t.Fatal(err)
	}
	wantNoPause(t, "a version as the previous release froze it", frozenPause(t, f.conn, version.ID))

	preview, err := b.tests.Queries.Preview.Handle(ctx, query.Preview{TestID: draft.ID, Scope: access.Scope{UserID: b.author}})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Questions) != 1 || !preview.Questions[0].AllowPause {
		t.Fatalf("a version frozen before the control reads %+v, want one question that can be paused", preview.Questions)
	}
}

type pauseGroup struct {
	t       *testing.T
	tx      pgx.Tx
	author  string
	groups  *repositories.GroupsPostgres
	repo    *repositories.Postgres
	testID  string
	section string
	audio   string
}

func newPauseGroup(t *testing.T) *pauseGroup {
	t.Helper()
	tx, author, groups := groupTransaction(t)
	media := mediarepo.NewPostgres(db.NewContext(tx))
	testID, section, _ := snapshotDraft(t, tx, author)
	return &pauseGroup{
		t: t, tx: tx, author: author, groups: groups, testID: testID, section: section,
		audio: storedGroupAsset(t, tx, author, "audio"),
		repo:  repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{}),
	}
}

func (g *pauseGroup) bundle(recording, member *bool) domain.GroupBundle {
	g.t.Helper()
	bundle := storedGroupFixture(g.t, g.audio)
	bundle.Group.Recordings[0].Policy.AllowPause = recording
	narration := storedGroupAsset(g.t, g.tx, g.author, "audio")
	bundle.Questions[0].Input.MediaAssetID = &narration
	bundle.Questions[0].MediaAssetKind = groupValue("audio")
	bundle.Questions[0].Input.Audio = &questions.AudioPolicy{MaxPlays: groupValue(1), AllowPause: member}
	return bundle
}

func (g *pauseGroup) create(bundle domain.GroupBundle, mounted bool) domain.StoredGroup {
	g.t.Helper()
	in := domain.CreateGroupInput{Bundle: bundle, ActorID: g.author, Now: time.Now(), Scope: access.Scope{UserID: g.author}, Grants: bothKeys}
	if mounted {
		in.OwnerSectionID = &g.section
		if err := g.tx.QueryRow(context.Background(), `SELECT updated_at FROM app.tests WHERE id = $1`, g.testID).Scan(&in.ExpectedTestUpdatedAt); err != nil {
			g.t.Fatal(err)
		}
	}
	stored, err := g.groups.Create(context.Background(), in)
	if err != nil {
		g.t.Fatalf("create group: %v", err)
	}
	return stored
}

func (g *pauseGroup) update(stored domain.StoredGroup, bundle domain.GroupBundle) domain.StoredGroup {
	g.t.Helper()
	mutation := groupMutation(stored, g.author)
	if stored.OwnerSectionID != nil {
		if err := g.tx.QueryRow(context.Background(), `SELECT updated_at FROM app.tests WHERE id = $1`, g.testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
			g.t.Fatal(err)
		}
	}
	updated, err := g.groups.Update(context.Background(), domain.UpdateGroupInput{GroupMutation: mutation, Bundle: bundle})
	if err != nil {
		g.t.Fatalf("update group: %v", err)
	}
	return updated
}

func (g *pauseGroup) publish() domain.Version {
	g.t.Helper()
	version, err := g.repo.Publish(context.Background(), domain.PublishRequest{TestID: g.testID, ActorID: g.author, Scope: access.Scope{UserID: g.author}}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		g.t.Fatalf("publish: %v", err)
	}
	return version
}

func (g *pauseGroup) recordingPause(groupID string) *bool {
	g.t.Helper()
	return rawPause(g.t, g.tx, `SELECT allow_pause FROM app.group_recordings WHERE group_id = $1`, groupID)
}

func (g *pauseGroup) memberPause(groupID string) *bool {
	g.t.Helper()
	return rawPause(g.t, g.tx, `SELECT audio_allow_pause FROM app.questions WHERE context_group_id = $1 AND media_asset_kind = 'audio'`, groupID)
}

func (g *pauseGroup) frozenGroup(versionID string) string {
	g.t.Helper()
	var id string
	if err := g.tx.QueryRow(context.Background(), `
		SELECT g.id::text FROM app.test_version_groups g
		  JOIN app.test_version_sections s ON s.id = g.test_version_section_id
		 WHERE s.test_version_id = $1`, versionID).Scan(&id); err != nil {
		g.t.Fatal(err)
	}
	return id
}

func (g *pauseGroup) frozenRecordingPause(versionID string) *bool {
	g.t.Helper()
	return rawPause(g.t, g.tx, `
		SELECT r.allow_pause FROM app.test_version_group_recordings r
		  JOIN app.test_version_groups g ON g.id = r.group_id
		  JOIN app.test_version_sections s ON s.id = g.test_version_section_id
		 WHERE s.test_version_id = $1`, versionID)
}

func (g *pauseGroup) frozenMemberPause(versionID string) *bool {
	g.t.Helper()
	return rawPause(g.t, g.tx, `
		SELECT q.audio_allow_pause FROM app.test_version_group_members m
		  JOIN app.test_version_questions q ON q.id = m.question_id
		  JOIN app.test_version_sections s ON s.id = m.test_version_section_id
		 WHERE s.test_version_id = $1 AND q.media_asset_kind = 'audio'`, versionID)
}

func TestAGroupCreatedWithoutAllowPauseAllowsIt(t *testing.T) {
	g := newPauseGroup(t)
	stored := g.create(g.bundle(nil, nil), false)
	id := stored.Bundle.Group.ID
	wantPause(t, "the recording created with the field absent", g.recordingPause(id), true)
	wantPause(t, "the member created with the field absent", g.memberPause(id), true)
	if !stored.Bundle.Group.Recordings[0].Policy.PauseAllowed() || stored.Bundle.Group.Recordings[0].Policy.AllowPause == nil {
		t.Fatalf("the answer to the create carries %+v", stored.Bundle.Group.Recordings[0].Policy)
	}
	if audio := stored.Bundle.Questions[0].Input.Audio; audio == nil || audio.AllowPause == nil || !*audio.AllowPause {
		t.Fatalf("the member in the answer to the create carries %+v", audio)
	}
}

func TestAGroupSaveThatLeavesAllowPauseOutKeepsTheStoredRule(t *testing.T) {
	g := newPauseGroup(t)
	stored := g.create(g.bundle(groupValue(false), groupValue(false)), false)
	id := stored.Bundle.Group.ID
	wantPause(t, "the recording created with false", g.recordingPause(id), false)
	wantPause(t, "the member created with false", g.memberPause(id), false)

	omitted := stored.Bundle
	omitted.Group.Recordings[0].Policy.AllowPause = nil
	omitted.Questions[0].Input.Audio.AllowPause = nil
	stored = g.update(stored, omitted)
	wantPause(t, "the recording saved with the field absent", g.recordingPause(id), false)
	wantPause(t, "the member saved with the field absent", g.memberPause(id), false)
	if stored.Bundle.Group.Recordings[0].Policy.PauseAllowed() || stored.Bundle.Questions[0].Input.Audio.PauseAllowed() {
		t.Fatal("the answer to a save that left the field out says pausing is allowed")
	}

	on := stored.Bundle
	on.Group.Recordings[0].Policy.AllowPause = groupValue(true)
	on.Questions[0].Input.Audio.AllowPause = groupValue(true)
	stored = g.update(stored, on)
	wantPause(t, "the recording saved with true", g.recordingPause(id), true)
	wantPause(t, "the member saved with true", g.memberPause(id), true)

	omitted = stored.Bundle
	omitted.Group.Recordings[0].Policy.AllowPause = nil
	omitted.Questions[0].Input.Audio.AllowPause = nil
	g.update(stored, omitted)
	wantPause(t, "the recording saved with the field absent after true", g.recordingPause(id), true)
	wantPause(t, "the member saved with the field absent after true", g.memberPause(id), true)
}

func TestAReplacedRecordingAndANewMemberReadTrue(t *testing.T) {
	g := newPauseGroup(t)
	stored := g.create(g.bundle(groupValue(false), groupValue(false)), false)
	id := stored.Bundle.Group.ID
	wantPause(t, "the recording before the replacement", g.recordingPause(id), false)

	next := stored.Bundle
	next.Group.Recordings[0].ID = groupIdentity(t)
	next.Group.Recordings[0].Policy.AllowPause = nil
	narration := storedGroupAsset(t, g.tx, g.author, "audio")
	added := domain.GroupQuestion{ID: groupIdentity(t), MediaAssetKind: groupValue("audio"), Input: questions.Input{
		Type: questions.SingleChoice, Prompt: "Nghe và chọn", Points: "0.25", MediaAssetID: &narration,
		Audio:   &questions.AudioPolicy{MaxPlays: groupValue(1)},
		Options: []questions.OptionInput{{Text: "first", IsCorrect: true}, {Text: "second"}},
	}}
	next.Questions = append(next.Questions, added)
	next.Group.Members = append(next.Group.Members, domain.GroupMember{QuestionID: added.ID, OptionOrder: "fixed"})
	g.update(stored, next)

	wantPause(t, "the recording under a new id", g.recordingPause(id), true)
	var rules []bool
	rows, err := g.tx.Query(context.Background(), `SELECT audio_allow_pause FROM app.questions WHERE context_group_id = $1 AND media_asset_kind = 'audio' ORDER BY context_ordinal`, id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var rule *bool
		if err := rows.Scan(&rule); err != nil || rule == nil {
			t.Fatalf("member rule %v: %v", rule, err)
		}
		rules = append(rules, *rule)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || rules[0] || !rules[1] {
		t.Fatalf("member rules = %v, want the kept member false and the new member true", rules)
	}
}

func TestAGroupFreezesAllowPauseAndLaterEditsDoNotReachTheVersion(t *testing.T) {
	g := newPauseGroup(t)
	ctx := context.Background()
	stored := g.create(g.bundle(groupValue(false), groupValue(false)), true)
	version := g.publish()
	wantPause(t, "the recording frozen at publish", g.frozenRecordingPause(version.ID), false)
	wantPause(t, "the member frozen at publish", g.frozenMemberPause(version.ID), false)

	on := stored.Bundle
	on.Group.Recordings[0].Policy.AllowPause = groupValue(true)
	on.Questions[0].Input.Audio.AllowPause = groupValue(true)
	g.update(stored, on)
	id := stored.Bundle.Group.ID
	wantPause(t, "the draft recording after the edit", g.recordingPause(id), true)
	wantPause(t, "the draft member after the edit", g.memberPause(id), true)
	wantPause(t, "the frozen recording after the edit", g.frozenRecordingPause(version.ID), false)
	wantPause(t, "the frozen member after the edit", g.frozenMemberPause(version.ID), false)

	frozen, err := g.groups.Frozen(ctx, g.frozenGroup(version.ID))
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Group.Recordings[0].Policy.PauseAllowed() || frozen.Questions[0].Input.Audio.PauseAllowed() {
		t.Fatalf("the frozen reader says pausing is allowed: %+v", frozen.Group.Recordings[0].Policy)
	}

	preview, err := g.repo.Preview(ctx, access.Scope{UserID: g.author}, g.testID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Groups) != 1 || len(preview.Groups[0].Recordings) != 1 || preview.Groups[0].Recordings[0].Policy.PauseAllowed() {
		t.Fatalf("the teacher's preview of the version = %+v", preview.Groups)
	}
	contexts, err := g.repo.GroupContexts(ctx, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 1 || contexts[0].Recordings[0].Policy.PauseAllowed() {
		t.Fatalf("the student's group contexts = %+v", contexts)
	}
}

func TestAGroupVersionFrozenBeforeAllowPauseReadsTrue(t *testing.T) {
	g := newPauseGroup(t)
	ctx := context.Background()
	g.create(g.bundle(groupValue(false), groupValue(false)), true)
	version := g.publish()
	frozenID := g.frozenGroup(version.ID)
	if _, err := g.tx.Exec(ctx, `UPDATE app.test_version_group_recordings SET allow_pause = DEFAULT WHERE group_id = $1`, frozenID); err != nil {
		t.Fatal(err)
	}
	if _, err := g.tx.Exec(ctx, `UPDATE app.test_version_questions SET audio_allow_pause = NULL
		WHERE id IN (SELECT question_id FROM app.test_version_group_members WHERE group_id = $1)`, frozenID); err != nil {
		t.Fatal(err)
	}
	wantNoPause(t, "the member as the previous release froze it", g.frozenMemberPause(version.ID))

	frozen, err := g.groups.Frozen(ctx, frozenID)
	if err != nil {
		t.Fatal(err)
	}
	if !frozen.Group.Recordings[0].Policy.PauseAllowed() || !frozen.Questions[0].Input.Audio.PauseAllowed() {
		t.Fatalf("a group frozen before the control reads %+v from the frozen reader", frozen.Group.Recordings[0].Policy)
	}
	contexts, err := g.repo.GroupContexts(ctx, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 1 || !contexts[0].Recordings[0].Policy.PauseAllowed() {
		t.Fatalf("a group frozen before the control reads %+v for a student", contexts)
	}
}

func TestACopyADuplicateAndARestoreKeepTheGroupsPauseRule(t *testing.T) {
	g := newPauseGroup(t)
	ctx := context.Background()
	stored := g.create(g.bundle(groupValue(false), groupValue(false)), true)
	version := g.publish()

	var updated time.Time
	if err := g.tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id = $1`, g.testID).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	copied, err := g.groups.Copy(ctx, domain.CopyGroupInput{
		SourceID: stored.Bundle.Group.ID, ExpectedSourceRevision: stored.Revision, OwnerSectionID: &g.section, ExpectedTestUpdatedAt: updated,
		ActorID: g.author, Now: time.Now(), Scope: access.Scope{UserID: g.author}, Grants: bothKeys,
	})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	wantPause(t, "the copied recording", g.recordingPause(copied.Bundle.Group.ID), false)
	wantPause(t, "the copied member", g.memberPause(copied.Bundle.Group.ID), false)

	duplicate, err := g.repo.Duplicate(ctx, domain.DuplicateInput{ID: g.testID, ActorID: g.author, Now: time.Now(), Scope: access.Scope{UserID: g.author}})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	var duplicated []string
	rows, err := g.tx.Query(ctx, `SELECT u.group_id::text FROM app.test_section_units u JOIN app.test_sections s ON s.id = u.test_section_id
		WHERE s.test_id = $1 AND u.group_id IS NOT NULL`, duplicate.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		duplicated = append(duplicated, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(duplicated) != 2 {
		t.Fatalf("the duplicate holds %d groups, want the two the draft held", len(duplicated))
	}
	for _, id := range duplicated {
		wantPause(t, "a duplicated recording", g.recordingPause(id), false)
		wantPause(t, "a duplicated member", g.memberPause(id), false)
	}

	on := stored.Bundle
	on.Group.Recordings[0].Policy.AllowPause = groupValue(true)
	on.Questions[0].Input.Audio.AllowPause = groupValue(true)
	g.update(stored, on)

	request := domain.VersionRequest{Request: domain.Request{ID: g.testID, ActorID: g.author, Scope: access.Scope{UserID: g.author}}, Version: version.Version}
	if err := g.tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id = $1`, g.testID).Scan(&request.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := g.repo.CreateDraftFromVersion(ctx, request, time.Now()); err != nil {
		t.Fatalf("restore: %v", err)
	}
	var restored string
	if err := g.tx.QueryRow(ctx, `SELECT u.group_id::text FROM app.test_section_units u JOIN app.test_sections s ON s.id = u.test_section_id
		WHERE s.test_id = $1 AND u.group_id IS NOT NULL`, g.testID).Scan(&restored); err != nil {
		t.Fatal(err)
	}
	wantPause(t, "the restored recording", g.recordingPause(restored), false)
	wantPause(t, "the restored member", g.memberPause(restored), false)
}
