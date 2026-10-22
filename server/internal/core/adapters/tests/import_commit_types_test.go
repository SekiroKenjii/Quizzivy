//go:build integration

package adapters_test

import (
	"context"
	importsdomain "quizzivy/internal/modules/imports/domain"
	"testing"

	"github.com/google/uuid"
)

func typedDraft() importsdomain.Draft {
	tfng := choice("q1", "The museum opens every day.", "NG")
	tfng.Type = "tfng"
	tfng.Options = []importsdomain.DraftOption{{ID: "q1T", Label: "T", Content: prose("True")}, {ID: "q1F", Label: "F", Content: prose("False")}, {ID: "q1NG", Label: "NG", Content: prose("Not given")}}
	two, n := choice("q2", "Which two activities does the club offer?", "A"), 2
	two.Type, two.SelectCount = "multiple_choice", &n
	two.Options = append(two.Options, importsdomain.DraftOption{ID: "q2C", Label: "C", Content: prose("chess")})
	two.Answer.OptionIDs = []string{"q2A", "q2C"}
	limited, words := choice("q3", "Where does the guide live?", "A"), 3
	limited.Type, limited.Options, limited.MaxWords = "short_answer", []importsdomain.DraftOption{}, &words
	limited.Answer = importsdomain.DraftAnswer{State: importsdomain.AnswerKnown, OptionIDs: []string{}, Text: "in Hue", Evidence: []importsdomain.SourceRef{}}
	grouped := tfng
	grouped.ID, grouped.Label, grouped.Prompt = "q4", "q4", prose("Guides speak French.")
	grouped.Options = []importsdomain.DraftOption{{ID: "q4T", Label: "T", Content: prose("True")}, {ID: "q4F", Label: "F", Content: prose("False")}, {ID: "q4NG", Label: "NG", Content: prose("Not given")}}
	grouped.Answer.OptionIDs = []string{"q4F"}
	group := importsdomain.DraftGroup{ID: "g1", Instructions: "Write TRUE, FALSE or NOT GIVEN.", Gaps: []importsdomain.GapLink{}, Questions: []importsdomain.DraftQuestion{grouped}, Source: []importsdomain.SourceRef{}}
	d := reviewedDraft()
	d.Sections[0].Items = []importsdomain.DraftItem{{Question: &tfng}, {Question: &two}, {Question: &limited}, {Group: &group}}
	return d
}

func TestCommitWritesTFNGSelectCountAndMaxWordsToTheBank(t *testing.T) {
	h := commitSetup(t)
	ctx := context.Background()
	importID := h.underReview(t)
	p, err := importsdomain.Plan(typedDraft(), "fallback", uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.committer.Materialize(ctx, p, h.by.ID, h.by, h.record(importID, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	rows, err := h.pool.Query(ctx, `SELECT q.prompt, q.type::text, q.select_count, q.max_words,
 ARRAY(SELECT o.text || CASE WHEN o.is_correct THEN '*' ELSE '' END FROM app.question_options o WHERE o.question_id=q.id ORDER BY o.ordinal)
 FROM app.questions q WHERE q.created_by=$1`, h.by.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var prompt, kind string
		var selectCount, maxWords *int
		var options []string
		if err := rows.Scan(&prompt, &kind, &selectCount, &maxWords, &options); err != nil {
			t.Fatal(err)
		}
		got[prompt] = kind
		switch prompt {
		case "The museum opens every day.":
			if kind != "tfng" || len(options) != 3 || options[0] != "True" || options[1] != "False" || options[2] != "Not given*" {
				t.Fatalf("standalone tfng: %s %v", kind, options)
			}
		case "Which two activities does the club offer?":
			if selectCount == nil || *selectCount != 2 {
				t.Fatalf("selectCount: %v", selectCount)
			}
		case "Where does the guide live?":
			if maxWords == nil || *maxWords != 3 {
				t.Fatalf("maxWords: %v", maxWords)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got["Where does the guide live?"] != "short_answer" {
		t.Fatalf("committed questions: %v", got)
	}
	var groupOptions []string
	var order string
	if err := h.pool.QueryRow(ctx, `SELECT q.context_option_order, ARRAY(SELECT o.text || CASE WHEN o.is_correct THEN '*' ELSE '' END FROM app.question_options o WHERE o.question_id=q.id ORDER BY o.ordinal)
 FROM app.questions q WHERE q.created_by=$1 AND q.context_group_id IS NOT NULL`, h.by.ID).Scan(&order, &groupOptions); err != nil {
		t.Fatal(err)
	}
	if order != "fixed" || len(groupOptions) != 3 || groupOptions[1] != "False*" {
		t.Fatalf("grouped tfng: %s %v", order, groupOptions)
	}
}
