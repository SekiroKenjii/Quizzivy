package adapters

import (
	"context"
	importsdomain "quizzivy/internal/modules/imports/domain"
	importsrepo "quizzivy/internal/modules/imports/repositories"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionscmd "quizzivy/internal/modules/questions/application/command"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	testsapp "quizzivy/internal/modules/tests/application"
	testscmd "quizzivy/internal/modules/tests/application/command"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
	"time"

	"github.com/jackc/pgx/v5"
)

// ImportCommitter materializes a reviewed import through the tests and questions use cases, all bound to one
// transaction: a failure at any step, including recording the commit, leaves no test, question or group behind.
type ImportCommitter struct {
	DB        db.Context
	Tests     func(db.Context) *testsapp.Application
	Questions func(db.Context) *questionsapp.Application
}

type materialization struct {
	tests     *testsapp.Application
	questions *questionsapp.Application
	by        actor.Actor
}

func (c ImportCommitter) Materialize(ctx context.Context, plan importsdomain.CommitPlan, by actor.Actor, record func(context.Context, importsdomain.CommitStore, string) error) (string, error) {
	var testID string
	err := c.DB.InTx(ctx, "commit word import", func(tx pgx.Tx) error {
		scoped := db.NewContext(tx)
		m := materialization{tests: c.Tests(scoped), questions: c.Questions(scoped), by: by}
		id, err := m.run(ctx, plan)
		if err != nil {
			return err
		}
		testID = id
		return record(ctx, importsrepo.NewPostgres(scoped), id)
	})
	return testID, err
}

func (m materialization) run(ctx context.Context, plan importsdomain.CommitPlan) (string, error) {
	test, err := m.tests.Commands.Create.Handle(ctx, testscmd.Create{Request: m.request(""), Title: plan.Title})
	if err != nil {
		return "", err
	}
	sections, err := m.standaloneQuestions(ctx, plan)
	if err != nil {
		return "", err
	}
	test, err = m.outline(ctx, test, sections)
	if err != nil {
		return "", err
	}
	updatedAt, grouped, err := m.groups(ctx, plan, test)
	if err != nil || !grouped {
		return test.ID, err
	}
	for i := range sections {
		sections[i].ID = test.Sections[i].ID
		sections[i].Units = interleave(plan.Sections[i], sections[i].QuestionIDs)
	}
	test.UpdatedAt = updatedAt
	_, err = m.outline(ctx, test, sections)
	return test.ID, err
}

func interleave(section importsdomain.PlanSection, questionIDs []string) []testsdomain.SectionUnit {
	units := make([]testsdomain.SectionUnit, 0, len(section.Units))
	next := 0
	for _, unit := range section.Units {
		if unit.Group != nil {
			units = append(units, testsdomain.SectionUnit{Kind: "group", ID: unit.Group.Group.ID})
			continue
		}
		units = append(units, testsdomain.SectionUnit{Kind: "question", ID: questionIDs[next]})
		next++
	}
	return units
}

func (m materialization) standaloneQuestions(ctx context.Context, plan importsdomain.CommitPlan) ([]testsdomain.SectionInput, error) {
	sections := make([]testsdomain.SectionInput, len(plan.Sections))
	for i, s := range plan.Sections {
		sections[i] = testsdomain.SectionInput{Title: s.Title, Instructions: s.Instructions, QuestionIDs: []string{}, Units: []testsdomain.SectionUnit{}, SetUnits: true}
		for _, unit := range s.Units {
			if unit.Question == nil {
				continue
			}
			created, err := m.questions.Commands.Create.Handle(ctx, questionscmd.Create{Request: questionsdomain.WriteRequest{Input: *unit.Question, ActorID: m.by.ID, IP: m.by.IP, UserAgent: m.by.UserAgent}})
			if err != nil {
				return nil, err
			}
			sections[i].QuestionIDs = append(sections[i].QuestionIDs, created.ID)
			sections[i].Units = append(sections[i].Units, testsdomain.SectionUnit{Kind: "question", ID: created.ID})
		}
	}
	return sections, nil
}

func (m materialization) groups(ctx context.Context, plan importsdomain.CommitPlan, test testsdomain.Test) (time.Time, bool, error) {
	updatedAt, grouped := test.UpdatedAt, false
	for i, s := range plan.Sections {
		sectionID := test.Sections[i].ID
		for _, unit := range s.Units {
			if unit.Group == nil {
				continue
			}
			stored, err := m.tests.Commands.CreateGroup.Handle(ctx, testscmd.CreateGroup{Bundle: *unit.Group, OwnerSectionID: &sectionID, ExpectedTestUpdatedAt: updatedAt, Actor: m.by})
			if err != nil {
				return updatedAt, grouped, err
			}
			if stored.TestUpdatedAt != nil {
				updatedAt = *stored.TestUpdatedAt
			}
			grouped = true
		}
	}
	return updatedAt, grouped, nil
}

func (m materialization) outline(ctx context.Context, test testsdomain.Test, sections []testsdomain.SectionInput) (testsdomain.Test, error) {
	return m.tests.Commands.Update.Handle(ctx, testscmd.Update{Request: m.request(test.ID), Input: testsdomain.UpdateInput{ExpectedUpdatedAt: test.UpdatedAt, Sections: sections, SetSections: true, GroupOutline: true}})
}

func (m materialization) request(id string) testsdomain.Request {
	return testsdomain.Request{ID: id, ActorID: m.by.ID, IP: m.by.IP, UserAgent: m.by.UserAgent}
}
