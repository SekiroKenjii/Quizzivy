package adapters_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/core/adapters"
	accessquery "quizzivy/internal/modules/access/application/query"
	accessdomain "quizzivy/internal/modules/access/domain"
	aidomain "quizzivy/internal/modules/ai/domain"
	attemptsquery "quizzivy/internal/modules/attempts/application/query"
	attemptsdomain "quizzivy/internal/modules/attempts/domain"
	identityquery "quizzivy/internal/modules/identity/application/query"
	identitydomain "quizzivy/internal/modules/identity/domain"
	questionsquery "quizzivy/internal/modules/questions/application/query"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	testsquery "quizzivy/internal/modules/tests/application/query"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/paging"
)

const (
	studentName  = "Nguyễn Kín Đáo"
	studentEmail = "kin-dao@example.com"
	studentID    = "0198f5f0-0000-7000-8000-0000000000a1"
	attemptID    = "0198f5f0-0000-7000-8000-0000000000b1"
	choiceID     = "0198f5f0-0000-7000-8000-0000000000c1"
	shortID      = "0198f5f0-0000-7000-8000-0000000000c2"
	blankID      = "0198f5f0-0000-7000-8000-0000000000c3"
)

func paragraph(text string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]}]}]}`, text))
}

func reviewOf(status attemptsdomain.Status, answers map[string]attemptsdomain.ReviewAnswer) attemptsdomain.Review {
	sample := "A teacher."
	return attemptsdomain.Review{
		Attempt:   attemptsdomain.Attempt{ID: attemptID, StudentID: studentID, AssignmentID: "as-1", Status: status},
		TestTitle: "Unit 5",
		Questions: []attemptsdomain.ReviewQuestion{
			{ID: choiceID, Type: "single_choice", Prompt: "Which word is a noun?", Points: 1},
			{ID: shortID, Type: "short_answer", Prompt: "Who teaches a class?", Points: 2, SampleAnswer: &sample},
		},
		Answers: answers,
	}
}

func grading(review attemptsdomain.Review, err error) adapters.AIGrading {
	return adapters.AIGrading{Review: cqrs.HandlerFunc[attemptsquery.Review, attemptsdomain.Review](func(context.Context, attemptsquery.Review) (attemptsdomain.Review, error) {
		return review, err
	})}
}

func TestAGradedAnswerCarriesTheQuestionAndTheTextAndNothingThatNamesTheStudent(t *testing.T) {
	typed := []byte(`{"type":"text","value":"The person is a teacher."}`)
	g := grading(reviewOf(attemptsdomain.Submitted, map[string]attemptsdomain.ReviewAnswer{shortID: {Payload: typed, RequiresManual: true}}), nil)

	got, err := g.Answer(context.Background(), access.Scope{UserID: "t"}, attemptID, strings.ToUpper(shortID))
	if err != nil {
		t.Fatal(err)
	}
	want := aidomain.GradedAnswer{Type: "short_answer", Prompt: "Who teaches a class?", Points: 2, Accepted: []string{"A teacher."}, Answer: "The person is a teacher.", Manual: true}
	if fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
		t.Errorf("answer %+v, want %+v", got, want)
	}
	for _, secret := range []string{studentName, studentEmail, studentID, attemptID, "as-1", "Unit 5"} {
		if strings.Contains(fmt.Sprintf("%+v", got), secret) {
			t.Errorf("the answer handed to the assistant carries %q", secret)
		}
	}
}

func TestAnAnswerThatIsNotWordsOrIsMissingIsReadAsEmpty(t *testing.T) {
	picked := []byte(`{"type":"choice","optionIds":["x"]}`)
	g := grading(reviewOf(attemptsdomain.Graded, map[string]attemptsdomain.ReviewAnswer{choiceID: {Payload: picked}}), nil)

	for name, id := range map[string]string{"a choice": choiceID, "an unanswered question": shortID} {
		got, err := g.Answer(context.Background(), access.Scope{UserID: "t"}, attemptID, id)
		if err != nil || got.Answer != "" {
			t.Errorf("%s: answer %q, %v, want it empty", name, got.Answer, err)
		}
	}
}

func TestAPaperThatCannotBeMarkedOrIsNotTheCallersAnswersAsSuch(t *testing.T) {
	ctx, scope := context.Background(), access.Scope{UserID: "t"}
	for name, c := range map[string]struct {
		g        adapters.AIGrading
		question string
		want     error
	}{
		"an unfinished paper":          {grading(reviewOf(attemptsdomain.InProgress, nil), nil), shortID, aidomain.ErrAttemptInProgress},
		"a voided paper":               {grading(reviewOf(attemptsdomain.Voided, nil), nil), shortID, aidomain.ErrAttemptVoided},
		"a paper that is not found":    {grading(attemptsdomain.Review{}, attemptsdomain.ErrPaperNotFound), shortID, aidomain.ErrNotFound},
		"an attempt that is not found": {grading(attemptsdomain.Review{}, attemptsdomain.ErrNotFound), shortID, aidomain.ErrNotFound},
		"a question off the paper":     {grading(reviewOf(attemptsdomain.Submitted, nil), nil), "0198f5f0-0000-7000-8000-0000000000ff", aidomain.ErrNotFound},
	} {
		if _, err := c.g.Answer(ctx, scope, attemptID, c.question); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	boom := errors.New("boom")
	if _, err := grading(attemptsdomain.Review{}, boom).Answer(ctx, scope, attemptID, shortID); !errors.Is(err, boom) {
		t.Errorf("another failure: %v", err)
	}
}

func TestTheReviewIsReadUnderTheCallersScope(t *testing.T) {
	var seen access.Scope
	g := adapters.AIGrading{Review: cqrs.HandlerFunc[attemptsquery.Review, attemptsdomain.Review](func(_ context.Context, q attemptsquery.Review) (attemptsdomain.Review, error) {
		seen = q.Scope
		return reviewOf(attemptsdomain.Submitted, nil), nil
	})}
	if _, err := g.Answer(context.Background(), access.Scope{UserID: "teacher-1", All: true}, attemptID, shortID); err != nil {
		t.Fatal(err)
	}
	if seen != (access.Scope{UserID: "teacher-1", All: true}) {
		t.Errorf("the review was read under %+v, want the caller's scope", seen)
	}
}

func bankQuestion(i int) questionsdomain.Question {
	level := questionsdomain.Level("b1")
	return questionsdomain.Question{ID: fmt.Sprintf("q-%03d", i), Type: "short_answer", Prompt: fmt.Sprintf("Prompt %d", i), Level: &level, Tags: []string{"nouns"}, UsedInTests: 2}
}

func bank(total int, scopes *[]access.Scope) adapters.AIQuestions {
	return adapters.AIQuestions{List: cqrs.HandlerFunc[questionsquery.List, questionsquery.ListResult](func(_ context.Context, q questionsquery.List) (questionsquery.ListResult, error) {
		*scopes = append(*scopes, q.Input.Scope)
		first := (q.Input.Page - 1) * q.Input.Limit
		var items []questionsdomain.Question
		for i := first; i < min(first+q.Input.Limit, total); i++ {
			items = append(items, bankQuestion(i))
		}
		return questionsquery.ListResult{Items: items, Page: paging.Page{Number: q.Input.Page, Size: q.Input.Limit, Total: total}}, nil
	})}
}

func TestTheBankIsReadNewestFirstInPagesUnderTheCallersOwnRowsAndCutAtTheLimit(t *testing.T) {
	var scopes []access.Scope
	got, err := bank(250, &scopes).Own(context.Background(), access.Scope{UserID: "teacher-1", All: true}, 120)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Questions) != 120 || !got.Truncated || got.Questions[0].ID != "q-000" || got.Questions[119].ID != "q-119" {
		t.Errorf("read %d questions truncated=%v, want 120 cut from 250", len(got.Questions), got.Truncated)
	}
	if first := got.Questions[0]; first.Level != "b1" || !slices.Equal(first.Tags, []string{"nouns"}) || first.UsedInTests != 2 || first.Type != "short_answer" {
		t.Errorf("a question reads %+v", first)
	}
	for _, scope := range scopes {
		if scope.All || scope.UserID != "teacher-1" {
			t.Errorf("a page was read under %+v: scope.all must not widen the cleanup", scope)
		}
	}
}

func TestABankThatFitsIsNotTruncatedAndAnEmptyOneIsEmpty(t *testing.T) {
	var scopes []access.Scope
	for _, total := range []int{0, 7, 100} {
		got, err := bank(total, &scopes).Own(context.Background(), access.Scope{UserID: "teacher-1"}, 2000)
		if err != nil || len(got.Questions) != total || got.Truncated {
			t.Errorf("a bank of %d read as %d questions truncated=%v, %v", total, len(got.Questions), got.Truncated, err)
		}
	}
}

func TestAStoredQuestionIsReadWithItsKeyAndAMissingOneIsNotFound(t *testing.T) {
	explanation, sample := "Because.", "A teacher."
	stored := questionsdomain.Question{
		ID: "q-1", Type: "single_choice", Prompt: "Which word is a noun?", Explanation: &explanation, SampleAnswer: &sample,
		Options: []questionsdomain.Option{
			{ID: "o2", Ordinal: 2, Text: "teacher", IsCorrect: true}, {ID: "o1", Ordinal: 1, Text: "quickly"},
		},
	}
	r := adapters.AIQuestions{Question: cqrs.HandlerFunc[questionsquery.Get, questionsdomain.Question](func(_ context.Context, q questionsquery.Get) (questionsdomain.Question, error) {
		if q.ID != "q-1" {
			return questionsdomain.Question{}, questionsdomain.ErrNotFound
		}
		return stored, nil
	})}
	got, err := r.Get(context.Background(), access.Scope{UserID: "t"}, "q-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Explanation != "Because." || got.Sample != "A teacher." || len(got.Options) != 2 || got.Options[0].ID != "o1" || !got.Options[1].Correct {
		t.Errorf("question %+v, want its options in order with the key marked", got)
	}
	if _, err := r.Get(context.Background(), access.Scope{UserID: "t"}, "other"); !errors.Is(err, aidomain.ErrNotFound) {
		t.Errorf("a question the scope does not reach: %v", err)
	}
}

func analysis(handedIn int, notFound bool) adapters.AIAnalysis {
	return adapters.AIAnalysis{
		Wrong: cqrs.HandlerFunc[attemptsquery.WrongAnswers, attemptsdomain.WrongAnswers](func(context.Context, attemptsquery.WrongAnswers) (attemptsdomain.WrongAnswers, error) {
			if notFound {
				return attemptsdomain.WrongAnswers{}, attemptsdomain.ErrNotFound
			}
			return attemptsdomain.WrongAnswers{HandedIn: handedIn, Questions: []attemptsdomain.QuestionWrongAnswers{
				{QuestionID: choiceID, Number: 1, Type: "single_choice", Key: "A", Wrong: []attemptsdomain.WrongAnswer{{Text: "B", Count: 3}}},
				{QuestionID: shortID, Number: 2, Type: "short_answer", Key: "A teacher."},
			}}, nil
		}),
		Items: cqrs.HandlerFunc[attemptsquery.ItemAnalysis, attemptsdomain.ItemAnalysis](func(context.Context, attemptsquery.ItemAnalysis) (attemptsdomain.ItemAnalysis, error) {
			return attemptsdomain.ItemAnalysis{Items: []attemptsdomain.AnalysisItem{{QuestionID: choiceID, PromptExcerpt: "Which word", CorrectRate: new(0.4)}}}, nil
		}),
	}
}

func TestTheResultsJoinTheWrongAnswersToTheCorrectRatesInPaperOrder(t *testing.T) {
	got, err := analysis(8, false).Results(context.Background(), access.Scope{UserID: "t"}, "as-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.HandedIn != 8 || len(got.Questions) != 2 {
		t.Fatalf("results %+v", got)
	}
	first := got.Questions[0]
	if first.Number != 1 || first.Key != "A" || first.Prompt != "Which word" || first.CorrectRate == nil || *first.CorrectRate != 0.4 || len(first.Wrong) != 1 || first.Wrong[0] != (aidomain.WrongAnswer{Text: "B", Count: 3}) {
		t.Errorf("first question %+v", first)
	}
	if second := got.Questions[1]; second.Number != 2 || second.Wrong != nil {
		t.Errorf("second question %+v", second)
	}
	if _, err := analysis(0, true).Results(context.Background(), access.Scope{UserID: "t"}, "as-1"); !errors.Is(err, aidomain.ErrNotFound) {
		t.Errorf("an assignment the scope does not reach: %v", err)
	}
}

func people(user identitydomain.User, principal access.Principal, principalErr error) adapters.AIPeople {
	return adapters.AIPeople{
		User: cqrs.HandlerFunc[identityquery.CurrentUser, identitydomain.User](func(context.Context, identityquery.CurrentUser) (identitydomain.User, error) {
			return user, nil
		}),
		Principal: cqrs.HandlerFunc[accessquery.ResolvePrincipal, access.Principal](func(context.Context, accessquery.ResolvePrincipal) (access.Principal, error) {
			return principal, principalErr
		}),
	}
}

func TestPreferencesDefaultToVietnameseAndReadingAnswersAndFollowWhatTheUserChose(t *testing.T) {
	english, vietnamese, off := "en", "vi", false
	for name, c := range map[string]struct {
		user identitydomain.User
		want aidomain.Preferences
	}{
		"a user who chose nothing":                {identitydomain.User{}, aidomain.Preferences{Language: aidomain.Vietnamese, ReadStudentAnswers: true}},
		"an interface in English":                 {identitydomain.User{Locale: &english}, aidomain.Preferences{Language: aidomain.English, ReadStudentAnswers: true}},
		"an interface locale that is not offered": {identitydomain.User{Locale: new("fr")}, aidomain.Preferences{Language: aidomain.Vietnamese, ReadStudentAnswers: true}},
		"an AI language over the interface's": {
			identitydomain.User{Locale: &english, Preferences: identitydomain.Preferences{AI: &identitydomain.AIPreferences{Language: &vietnamese}}},
			aidomain.Preferences{Language: aidomain.Vietnamese, ReadStudentAnswers: true},
		},
		"reading answers turned off": {
			identitydomain.User{Preferences: identitydomain.Preferences{AI: &identitydomain.AIPreferences{ReadStudentAnswers: &off}}},
			aidomain.Preferences{Language: aidomain.Vietnamese, ReadStudentAnswers: false},
		},
	} {
		got, err := people(c.user, access.Principal{}, nil).Preferences(context.Background(), "u")
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v, want %+v", name, got, err, c.want)
		}
	}
}

func TestPermissionsAreTheUsersEffectiveOnesAndAnUnknownUserIsNotFound(t *testing.T) {
	held := access.NewSet(access.AIUse, access.ContentTestsWrite)
	got, err := people(identitydomain.User{}, access.Principal{Permissions: held}, nil).Permissions(context.Background(), "u")
	if err != nil || !got.SubsetOf(held) || !held.SubsetOf(got) {
		t.Errorf("permissions %v, %v, want %v", got.Keys(), err, held.Keys())
	}
	if _, err := people(identitydomain.User{}, access.Principal{}, accessdomain.ErrUnknownUser).Permissions(context.Background(), "u"); !errors.Is(err, aidomain.ErrNotFound) {
		t.Errorf("an id that names no user: %v", err)
	}
}

const parksText = "Parks give a town room to breathe."

func memberOf(id string, input questionsdomain.Input) testsdomain.GroupQuestion {
	input.Prompt = "Prompt of " + id
	return testsdomain.GroupQuestion{ID: id, Input: input}
}

func storedGroup(id, stimulusID, text, order string, members ...testsdomain.GroupQuestion) testsdomain.StoredGroup {
	group := testsdomain.QuestionGroup{ID: id, Stimuli: []testsdomain.GroupStimulus{{ID: stimulusID, Title: "Parks", Content: paragraph(text)}}}
	for _, member := range members {
		group.Members = append(group.Members, testsdomain.GroupMember{QuestionID: member.ID, OptionOrder: order})
	}
	return testsdomain.StoredGroup{Bundle: testsdomain.GroupBundle{Group: group, Questions: members}}
}

func choiceInput() questionsdomain.Input {
	pointed := "o-b"
	return questionsdomain.Input{
		Type:    questionsdomain.SingleChoice,
		Options: []questionsdomain.OptionInput{{ID: nil, Text: "A park", IsCorrect: true}, {ID: &pointed, Text: "A road"}},
	}
}

func blankInput() questionsdomain.Input {
	blank := "b-1"
	return questionsdomain.Input{
		Type:   questionsdomain.FillBlank,
		Blanks: []questionsdomain.BlankInput{{ID: &blank, Ordinal: 1, AcceptedAnswers: []string{"green", "verdant"}}, {Ordinal: 0, AcceptedAnswers: []string{"open"}}},
	}
}

type groupReads struct{ fetched []string }

func draft(sections ...testsdomain.Section) adapters.AITests {
	return draftReading(&groupReads{}, sections...)
}

func draftReading(reads *groupReads, sections ...testsdomain.Section) adapters.AITests {
	stored := map[string]testsdomain.StoredGroup{
		"g-1": storedGroup("g-1", "s-1", parksText, "fixed", memberOf("q-2", choiceInput()), memberOf("q-3", blankInput())),
		"g-2": storedGroup("g-2", "s-2", "Rivers carry water to the sea.", "shuffle", memberOf("q-6", choiceInput())),
	}
	return adapters.AITests{
		Test: cqrs.HandlerFunc[testsquery.Get, testsdomain.Test](func(_ context.Context, q testsquery.Get) (testsdomain.Test, error) {
			if q.ID != "t-1" {
				return testsdomain.Test{}, testsdomain.ErrNotFound
			}
			return testsdomain.Test{ID: "t-1", Sections: sections}, nil
		}),
		Group: cqrs.HandlerFunc[testsquery.Group, testsdomain.StoredGroup](func(_ context.Context, q testsquery.Group) (testsdomain.StoredGroup, error) {
			reads.fetched = append(reads.fetched, q.ID)
			g, ok := stored[q.ID]
			if !ok {
				return testsdomain.StoredGroup{}, testsdomain.ErrNotFound
			}
			return g, nil
		}),
		Question: cqrs.HandlerFunc[questionsquery.Get, questionsdomain.Question](func(_ context.Context, q questionsquery.Get) (questionsdomain.Question, error) {
			if q.ID == "q-gone" {
				return questionsdomain.Question{}, questionsdomain.ErrNotFound
			}
			return questionsdomain.Question{ID: q.ID, Type: "short_answer", Prompt: "Prompt of " + q.ID}, nil
		}),
	}
}

func unitsOf(kind string, ids ...string) []testsdomain.SectionUnit {
	units := make([]testsdomain.SectionUnit, len(ids))
	for i, id := range ids {
		units[i] = testsdomain.SectionUnit{Kind: kind, ID: id}
	}
	return units
}

func numbersAndGroups(paper aidomain.Paper) string {
	var read []string
	for _, q := range paper.Questions {
		read = append(read, fmt.Sprintf("%d:%s:%s", q.Number, q.ID, q.GroupID))
	}
	return strings.Join(read, " ")
}

func TestADraftIsReadInTheBuildersOrderAStandaloneQuestionAndAGroupsMembersNumberedOneToThree(t *testing.T) {
	r := draft(testsdomain.Section{
		QuestionIDs: []string{"q-1"},
		Units:       append(unitsOf("question", "q-1"), unitsOf("group", "g-1")...),
	})
	paper, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := numbersAndGroups(paper), "1:q-1: 2:q-2:g-1 3:q-3:g-1"; got != want {
		t.Fatalf("paper %q, want %q", got, want)
	}
	standalone, choice, blank := paper.Questions[0], paper.Questions[1], paper.Questions[2]
	if standalone.Passage != "" || standalone.OrderFixed || standalone.GroupID != "" {
		t.Errorf("a standalone question reads %+v", standalone)
	}
	for _, member := range []aidomain.PaperQuestion{choice, blank} {
		if !strings.Contains(member.Passage, parksText) || !strings.Contains(member.Passage, "Parks") || !member.OrderFixed {
			t.Errorf("member %s reads %+v, want its group's passage and its fixed order", member.ID, member)
		}
	}
	if choice.Type != "single_choice" || choice.Prompt != "Prompt of q-2" {
		t.Errorf("a member's content is its own: %+v", choice)
	}
	if len(choice.Options) != 2 || choice.Options[0].ID != "" || choice.Options[0].Text != "A park" || !choice.Options[0].Correct || choice.Options[1].ID != "o-b" || choice.Options[1].Correct {
		t.Errorf("options %+v, want input order, an empty id for an option without one, and the key kept", choice.Options)
	}
	wantBlanks := []aidomain.Blank{{ID: "", Accepted: []string{"open"}}, {ID: "b-1", Accepted: []string{"green", "verdant"}}}
	if fmt.Sprintf("%+v", blank.Blanks) != fmt.Sprintf("%+v", wantBlanks) {
		t.Errorf("blanks %+v, want %+v: by ordinal, an empty id for a blank without one", blank.Blanks, wantBlanks)
	}
}

func TestAMemberWhoseGroupLetsOptionsShuffleMayBeReordered(t *testing.T) {
	r := draft(testsdomain.Section{Units: unitsOf("group", "g-2")})
	paper, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1")
	if err != nil || len(paper.Questions) != 1 {
		t.Fatalf("paper %+v, %v", paper, err)
	}
	if paper.Questions[0].OrderFixed {
		t.Error("a member of a group that allows shuffling reads as fixed")
	}
}

func TestADraftOfOnlyGroupMembersIsNotEmpty(t *testing.T) {
	r := draft(testsdomain.Section{Units: unitsOf("group", "g-1")})
	paper, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := numbersAndGroups(paper), "1:q-2:g-1 2:q-3:g-1"; got != want {
		t.Errorf("paper %q, want %q", got, want)
	}
}

func TestANumberIsAQuestionsPlaceInTheBuilderSoAnUnreadableQuestionKeepsItAndTheGroupAfterItFollowsOn(t *testing.T) {
	r := draft(testsdomain.Section{Units: append(unitsOf("question", "q-1", "q-gone"), unitsOf("group", "g-1")...)})
	paper, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := numbersAndGroups(paper), "1:q-1: 3:q-2:g-1 4:q-3:g-1"; got != want {
		t.Errorf("paper %q, want %q", got, want)
	}
	if _, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "other"); !errors.Is(err, aidomain.ErrNotFound) {
		t.Errorf("a test the scope does not reach: %v", err)
	}
}

func TestANumberKeepsCountingAcrossSections(t *testing.T) {
	r := draft(
		testsdomain.Section{Units: append(unitsOf("question", "q-1"), unitsOf("group", "g-1")...)},
		testsdomain.Section{Units: append(unitsOf("group", "g-2"), unitsOf("question", "q-5")...)},
	)
	paper, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := numbersAndGroups(paper), "1:q-1: 2:q-2:g-1 3:q-3:g-1 4:q-6:g-2 5:q-5:"; got != want {
		t.Errorf("paper %q, want %q", got, want)
	}
}

func TestASectionWithNoUnitsIsReadFromItsQuestionIDsAsStandaloneQuestions(t *testing.T) {
	r := draft(testsdomain.Section{QuestionIDs: []string{"q-1", "q-gone", "q-4"}})
	paper, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := numbersAndGroups(paper), "1:q-1: 3:q-4:"; got != want {
		t.Errorf("paper %q, want %q", got, want)
	}
}

func TestAGroupTheScopeCannotReadIsLeftOutWholeAndEachGroupIsFetchedOnce(t *testing.T) {
	reads := &groupReads{}
	r := draftReading(reads, testsdomain.Section{Units: append(unitsOf("group", "g-gone", "g-1"), unitsOf("question", "q-1")...)})
	paper, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := numbersAndGroups(paper), "1:q-2:g-1 2:q-3:g-1 3:q-1:"; got != want {
		t.Errorf("paper %q, want %q", got, want)
	}
	if got, want := strings.Join(reads.fetched, " "), "g-gone g-1"; got != want {
		t.Errorf("groups fetched %q, want %q: one fetch for each", got, want)
	}
}

func TestADraftThatCannotReadAGroupForAnyOtherReasonFails(t *testing.T) {
	broken := errors.New("database down")
	r := draft(testsdomain.Section{Units: unitsOf("group", "g-1")})
	r.Group = cqrs.HandlerFunc[testsquery.Group, testsdomain.StoredGroup](func(context.Context, testsquery.Group) (testsdomain.StoredGroup, error) {
		return testsdomain.StoredGroup{}, broken
	})
	if _, err := r.Draft(context.Background(), access.Scope{UserID: "t"}, "t-1"); !errors.Is(err, broken) {
		t.Errorf("err = %v, want the read failure", err)
	}
}

func TestAPassageIsFoundInTheDraftsGroupsOrNotFound(t *testing.T) {
	r := draft(testsdomain.Section{Units: unitsOf("group", "g-1")})

	got, err := r.Passage(context.Background(), access.Scope{UserID: "t"}, "t-1", "S-1")
	if err != nil || got.Title != "Parks" || got.Text != parksText {
		t.Errorf("passage %+v, %v", got, err)
	}
	for name, ids := range map[string][2]string{"a stimulus the test does not hold": {"t-1", "s-9"}, "a test the scope does not reach": {"other", "s-1"}} {
		if _, err := r.Passage(context.Background(), access.Scope{UserID: "t"}, ids[0], ids[1]); !errors.Is(err, aidomain.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestASectionIsReadWithItsTitleInstructionsAndEveryQuestionItHoldsGroupMembersIncluded(t *testing.T) {
	instructions := "Choose the best answer."
	r := draft(
		testsdomain.Section{ID: "sec-1", Title: "Part 1", Units: unitsOf("question", "q-9")},
		testsdomain.Section{ID: "sec-2", Title: "Part 2", Instructions: &instructions, Units: append(unitsOf("question", "q-1", "q-gone"), unitsOf("group", "g-1")...)},
	)
	got, err := r.Section(context.Background(), access.Scope{UserID: "t"}, "t-1", "SEC-2")
	if err != nil {
		t.Fatal(err)
	}
	var read []string
	for _, q := range got.Questions {
		read = append(read, q.Type+":"+q.Prompt)
	}
	want := []string{"short_answer:Prompt of q-1", "single_choice:Prompt of q-2", "fill_blank:Prompt of q-3"}
	if got.Title != "Part 2" || got.Instructions != instructions || !slices.Equal(read, want) {
		t.Errorf("section %+v, questions %q; want its title, instructions and %q", got, read, want)
	}
	for name, ids := range map[string][2]string{"a section the test does not hold": {"t-1", "sec-9"}, "a test the scope does not reach": {"other", "sec-1"}} {
		if _, err := r.Section(context.Background(), access.Scope{UserID: "t"}, ids[0], ids[1]); !errors.Is(err, aidomain.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestAnAnswerTheGraderMarkedItselfIsNotReportedAsWaitingForAMark(t *testing.T) {
	typed := []byte(`{"type":"text","value":"A teacher."}`)
	g := grading(reviewOf(attemptsdomain.Graded, map[string]attemptsdomain.ReviewAnswer{shortID: {Payload: typed}}), nil)

	got, err := g.Answer(context.Background(), access.Scope{UserID: "t"}, attemptID, shortID)
	if err != nil || got.Manual || got.Answer != "A teacher." {
		t.Errorf("answer %+v, %v, want the text read and Manual false", got, err)
	}
}
