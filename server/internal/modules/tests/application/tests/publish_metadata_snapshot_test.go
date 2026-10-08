//go:build integration

package application_test

import (
	"context"
	questions "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"
)

func TestPublishedMetadataAndLegacyNineOptionsAreImmutable(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()
	level, skill := questions.Level("pre_a1"), questions.Skill("grammar")
	input := questions.Input{Type: questions.SingleChoice, Prompt: "Legacy metadata", Points: "1", Tags: []string{}, Level: &level, Skill: &skill}
	for i := range 9 {
		input.Options = append(input.Options, questions.OptionInput{Text: "Choice", IsCorrect: i == 0})
	}
	repo := questionsrepo.NewPostgres(db.NewContext(f.conn))
	q, err := repo.Create(ctx, questions.WriteInput{Input: input, ActorID: f.author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	draft := f.builder.draft("Metadata snapshot", q.ID)
	version, err := f.builder.publish(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	changed := questions.Level("c2")
	input.Level = &changed
	input.Skill = nil
	input.Options = input.Options[:8]
	if _, err := repo.Update(ctx, questions.WriteInput{ID: q.ID, Input: input, ActorID: f.author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	var frozenLevel, frozenSkill *string
	var options int
	if err := f.conn.QueryRow(ctx, `SELECT q.level,q.skill,(SELECT count(*) FROM app.test_version_options o WHERE o.test_version_question_id=q.id) FROM app.test_version_questions q JOIN app.test_version_sections s ON s.id=q.test_version_section_id WHERE s.test_version_id=$1`, version.ID).Scan(&frozenLevel, &frozenSkill, &options); err != nil {
		t.Fatal(err)
	}
	if frozenLevel == nil || *frozenLevel != "pre_a1" || frozenSkill == nil || *frozenSkill != "grammar" || options != 9 {
		t.Fatalf("snapshot changed level=%v skill=%v options=%d", frozenLevel, frozenSkill, options)
	}
}
