//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func snapshotDraft(t *testing.T, tx pgx.Tx, author string) (string, string, time.Time) {
	t.Helper()
	ctx := context.Background()
	var testID, sectionID, questionID string
	var updated time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests (title,created_by) VALUES ('Grouped exam',$1) RETURNING id::text,updated_at`, author).Scan(&testID, &updated); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_sections (test_id,ordinal,title) VALUES ($1,0,'Part') RETURNING id::text`, testID).Scan(&sectionID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.questions (type,prompt,points,created_by) VALUES ('short_answer','Standalone',1,$1) RETURNING id::text`, author).Scan(&questionID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id,ordinal,question_id) VALUES ($1,0,$2)`, sectionID, questionID); err != nil {
		t.Fatal(err)
	}
	return testID, sectionID, updated
}

func frozenGroupDigest(t *testing.T, tx pgx.Tx, versionID string) string {
	t.Helper()
	var result string
	err := tx.QueryRow(context.Background(), `WITH groups AS (
		SELECT g.id,g.title,g.instructions FROM app.test_version_groups g
		JOIN app.test_version_sections s ON s.id=g.test_version_section_id WHERE s.test_version_id=$1
	), graph AS (
		SELECT 'group' AS kind,g.id::text AS id,jsonb_build_object('title',g.title,'instructions',g.instructions) AS body FROM groups g
		UNION ALL SELECT 'member',m.question_id::text,to_jsonb(m) FROM app.test_version_group_members m JOIN groups g ON g.id=m.group_id
		UNION ALL SELECT 'material',m.id::text,to_jsonb(m) FROM app.test_version_group_stimuli m JOIN groups g ON g.id=m.group_id
		UNION ALL SELECT 'gap',b.stimulus_id::text||b.gap_id,to_jsonb(b) FROM app.test_version_group_gap_bindings b JOIN groups g ON g.id=b.group_id
		UNION ALL SELECT 'recording',r.id::text,to_jsonb(r) FROM app.test_version_group_recordings r JOIN groups g ON g.id=r.group_id
		UNION ALL SELECT 'asset',a.stimulus_id::text||a.media_asset_id::text,to_jsonb(a) FROM app.test_version_group_assets a JOIN groups g ON g.id=a.group_id
		UNION ALL SELECT 'question',q.id::text,to_jsonb(q)-'source_question_id' FROM app.test_version_questions q
		 JOIN app.test_version_group_members m ON m.question_id=q.id JOIN groups g ON g.id=m.group_id
		UNION ALL SELECT 'option',o.id::text,to_jsonb(o) FROM app.test_version_options o
		 JOIN app.test_version_group_members m ON m.question_id=o.test_version_question_id JOIN groups g ON g.id=m.group_id
		UNION ALL SELECT 'blank',b.id::text,to_jsonb(b) FROM app.test_version_blanks b
		 JOIN app.test_version_group_members m ON m.question_id=b.test_version_question_id JOIN groups g ON g.id=m.group_id
		UNION ALL SELECT 'answer',a.test_version_blank_id::text||a.answer,to_jsonb(a) FROM app.test_version_blank_answers a
		 JOIN app.test_version_blanks b ON b.id=a.test_version_blank_id
		 JOIN app.test_version_group_members m ON m.question_id=b.test_version_question_id JOIN groups g ON g.id=m.group_id
	) SELECT jsonb_agg(body ORDER BY kind,id)::text FROM graph`, versionID).Scan(&result)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPublishedGroupGraphSurvivesDraftEditsRemovalAndMediaDeletion(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	asset := storedGroupAsset(t, tx, author, "audio")
	first, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	second, err := groups.Copy(ctx, domain.CopyGroupInput{SourceID: first.Bundle.Group.ID, ExpectedSourceRevision: first.Revision, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}
	if published.QuestionCount != 5 || published.TotalPoints != "3.00" {
		t.Fatalf("group interactions missing from totals: %+v", published)
	}
	versions, err := repo.ListVersions(ctx, testID)
	if err != nil || len(versions) != 1 || versions[0].AudioCount != 4 || versions[0].ManualCount != 1 {
		t.Fatalf("group listening counts: %+v, %v", versions, err)
	}
	before := frozenGroupDigest(t, tx, published.ID)
	var groupCount, scopes int
	if err := tx.QueryRow(ctx, `SELECT count(*),count(DISTINCT r.id) FROM app.test_version_group_recordings r
		JOIN app.test_version_groups g ON g.id=r.group_id JOIN app.test_version_sections s ON s.id=g.test_version_section_id
		WHERE s.test_version_id=$1`, published.ID).Scan(&groupCount, &scopes); err != nil || groupCount != 2 || scopes != 2 {
		t.Fatalf("shared file collapsed recording scopes: %d/%d, %v", groupCount, scopes, err)
	}
	first.Bundle.Group.Title = "Changed source"
	first.Bundle.Group.Recordings[0].Policy.MaxPlays = groupValue(99)
	mutation := groupMutation(first, author)
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	first, err = groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: first.Bundle})
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []domain.StoredGroup{first, second} {
		mutation := groupMutation(group, author)
		if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
			t.Fatal(err)
		}
		if err := groups.RemoveFromSection(ctx, mutation); err != nil {
			t.Fatal(err)
		}
	}
	if after := frozenGroupDigest(t, tx, published.ID); after != before {
		t.Fatal("source lifecycle changed frozen content, bindings, keys or recording policies")
	}
	err = media.SoftDelete(ctx, mediadomain.DeleteInput{ID: asset, ActorID: author, Now: time.Now()})
	var blocked *mediadomain.ReferencedError
	if !errors.As(err, &blocked) || len(blocked.Tests) != 1 || len(blocked.Groups) != 0 || blocked.Tests[0].ID != testID {
		t.Fatalf("frozen group lost media protection: %+v, %v", blocked, err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRejectsEmptyGroupAlongsideValidStandaloneQuestion(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	group := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupIdentity(t), Title: "Empty context"}}
	if _, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: group, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, nil).WithGroupQuestions(adapters.GroupQuestions{})
	_, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate)
	var invalid *domain.PublishValidationError
	if !errors.As(err, &invalid) || len(invalid.Violations) != 1 || invalid.Violations[0].Rule != domain.GroupValid || invalid.Violations[0].GroupID != group.Group.ID {
		t.Fatalf("empty group disappeared from publication: %+v, %v", invalid, err)
	}
	var versions int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.test_versions WHERE test_id=$1`, testID).Scan(&versions); err != nil || versions != 0 {
		t.Fatalf("failed publish left a version: %d, %v", versions, err)
	}
}
