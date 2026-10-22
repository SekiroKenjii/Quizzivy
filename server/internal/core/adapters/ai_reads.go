package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

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
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/cqrs"
)

const (
	groupKind        = "group"
	questionKind     = "question"
	fixedOptionOrder = "fixed"
)

// AITests reads a test's draft for the ai module from the tests and questions modules' handlers. It hands the assistant plain text: the prompts, options, keys and explanations of the draft's questions and the shared material of its groups, and nothing about a student.
type AITests struct {
	Test     cqrs.QueryHandler[testsquery.Get, testsdomain.Test]
	Group    cqrs.QueryHandler[testsquery.Group, testsdomain.StoredGroup]
	Question cqrs.QueryHandler[questionsquery.Get, questionsdomain.Question]
}

// Passage reads the shared material named stimulusID from a group of the draft of testID. It answers aidomain.ErrNotFound for a test the scope does not reach and for a passage the test does not hold.
func (r AITests) Passage(ctx context.Context, scope access.Scope, testID, stimulusID string) (aidomain.Passage, error) {
	test, err := r.Test.Handle(ctx, testsquery.Get{ID: testID, Scope: scope})
	if errors.Is(err, testsdomain.ErrNotFound) {
		return aidomain.Passage{}, aidomain.ErrNotFound
	}
	if err != nil {
		return aidomain.Passage{}, err
	}
	for _, id := range groupIDs(test) {
		group, err := r.Group.Handle(ctx, testsquery.Group{ID: id, Scope: scope})
		if errors.Is(err, testsdomain.ErrNotFound) {
			continue
		}
		if err != nil {
			return aidomain.Passage{}, err
		}
		for _, stimulus := range group.Bundle.Group.Stimuli {
			if strings.EqualFold(stimulus.ID, stimulusID) {
				return aidomain.Passage{Title: stimulus.Title, Text: plainText(stimulus.Content)}, nil
			}
		}
	}
	return aidomain.Passage{}, aidomain.ErrNotFound
}

// Draft reads the questions of the draft of testID in the order the builder shows them: each section's units in turn, a standalone question as one number and a group as one number per member, in the group's order. A question the scope cannot read is left out but keeps its number, so a number is the question's place in the builder; a group the scope cannot read is left out whole, because its members cannot be counted. It answers aidomain.ErrNotFound for a test the scope does not reach.
func (r AITests) Draft(ctx context.Context, scope access.Scope, testID string) (aidomain.Paper, error) {
	test, err := r.Test.Handle(ctx, testsquery.Get{ID: testID, Scope: scope})
	if errors.Is(err, testsdomain.ErrNotFound) {
		return aidomain.Paper{}, aidomain.ErrNotFound
	}
	if err != nil {
		return aidomain.Paper{}, err
	}
	var paper aidomain.Paper
	number := 0
	for _, section := range test.Sections {
		for _, unit := range outlineUnits(section) {
			read, counted, err := r.readUnit(ctx, scope, unit, number+1)
			if err != nil {
				return aidomain.Paper{}, err
			}
			paper.Questions = append(paper.Questions, read...)
			number += counted
		}
	}
	return paper, nil
}

// Section reads the section named sectionID of the draft of testID: its title, its instructions, and the type and prompt of each of its standalone questions and of each member of its groups, in the order the builder shows them. A question or group the scope cannot read is left out. It answers aidomain.ErrNotFound for a test the scope does not reach and for a section the test does not hold.
func (r AITests) Section(ctx context.Context, scope access.Scope, testID, sectionID string) (aidomain.Section, error) {
	test, err := r.Test.Handle(ctx, testsquery.Get{ID: testID, Scope: scope})
	if errors.Is(err, testsdomain.ErrNotFound) {
		return aidomain.Section{}, aidomain.ErrNotFound
	}
	if err != nil {
		return aidomain.Section{}, err
	}
	for _, section := range test.Sections {
		if !strings.EqualFold(section.ID, sectionID) {
			continue
		}
		read := aidomain.Section{Title: section.Title, Instructions: deref(section.Instructions)}
		for _, unit := range outlineUnits(section) {
			questions, _, err := r.readUnit(ctx, scope, unit, 1)
			if err != nil {
				return aidomain.Section{}, err
			}
			for _, q := range questions {
				read.Questions = append(read.Questions, aidomain.SectionQuestion{Type: q.Type, Prompt: q.Prompt})
			}
		}
		return read, nil
	}
	return aidomain.Section{}, aidomain.ErrNotFound
}

func outlineUnits(section testsdomain.Section) []testsdomain.SectionUnit {
	if len(section.Units) > 0 {
		return section.Units
	}
	units := make([]testsdomain.SectionUnit, len(section.QuestionIDs))
	for i, id := range section.QuestionIDs {
		units[i] = testsdomain.SectionUnit{Kind: questionKind, ID: id}
	}
	return units
}

func (r AITests) readUnit(ctx context.Context, scope access.Scope, unit testsdomain.SectionUnit, first int) ([]aidomain.PaperQuestion, int, error) {
	if unit.Kind == groupKind {
		return r.readGroup(ctx, scope, unit.ID, first)
	}
	question, err := r.Question.Handle(ctx, questionsquery.Get{ID: unit.ID, Scope: scope})
	if errors.Is(err, questionsdomain.ErrNotFound) {
		return nil, 1, nil
	}
	if err != nil {
		return nil, 0, err
	}
	return []aidomain.PaperQuestion{paperQuestion(question, first)}, 1, nil
}

func (r AITests) readGroup(ctx context.Context, scope access.Scope, groupID string, first int) ([]aidomain.PaperQuestion, int, error) {
	stored, err := r.Group.Handle(ctx, testsquery.Group{ID: groupID, Scope: scope})
	if errors.Is(err, testsdomain.ErrNotFound) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	passage := groupPassage(stored)
	byID := make(map[string]testsdomain.GroupQuestion, len(stored.Bundle.Questions))
	for _, question := range stored.Bundle.Questions {
		byID[strings.ToLower(question.ID)] = question
	}
	gapped := map[string]bool{}
	for _, stimulus := range stored.Bundle.Group.Stimuli {
		for _, gap := range stimulus.Gaps {
			gapped[strings.ToLower(gap.QuestionID)] = true
		}
	}
	members := stored.Bundle.Group.Members
	var read []aidomain.PaperQuestion
	for i, member := range members {
		question, ok := byID[strings.ToLower(member.QuestionID)]
		if !ok {
			continue
		}
		item := paperQuestion(questionOf(question), first+i)
		item.GroupID, item.Passage, item.OrderFixed = groupID, passage, member.OptionOrder == fixedOptionOrder
		item.Gapped = gapped[strings.ToLower(member.QuestionID)]
		read = append(read, item)
	}
	return read, len(members), nil
}

func groupPassage(stored testsdomain.StoredGroup) string {
	var texts []string
	for _, stimulus := range stored.Bundle.Group.Stimuli {
		if text := plainText(stimulus.Content); text != "" {
			texts = append(texts, strings.TrimSpace(stimulus.Title+"\n"+text))
		}
	}
	return strings.Join(texts, "\n\n")
}

func questionOf(member testsdomain.GroupQuestion) questionsdomain.Question {
	in := member.Input
	question := questionsdomain.Question{
		ID: member.ID, Type: in.Type, Prompt: in.Prompt, PromptContent: in.PromptContent,
		Explanation: in.Explanation, ExplanationContent: in.ExplanationContent, SampleAnswer: in.SampleAnswer, MaxWords: in.MaxWords,
	}
	for i, o := range in.Options {
		question.Options = append(question.Options, questionsdomain.Option{ID: deref(o.ID), Ordinal: i, Text: o.Text, IsCorrect: o.IsCorrect, Content: o.Content})
	}
	for _, b := range in.Blanks {
		question.Blanks = append(question.Blanks, questionsdomain.Blank{ID: deref(b.ID), GapID: b.GapID, Ordinal: b.Ordinal, AcceptedAnswers: b.AcceptedAnswers, CaseSensitive: b.CaseSensitive})
	}
	return question
}

func groupIDs(test testsdomain.Test) []string {
	var ids []string
	for _, section := range test.Sections {
		for _, unit := range section.Units {
			if unit.Kind == groupKind {
				ids = append(ids, unit.ID)
			}
		}
	}
	return ids
}

func plainText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	document, err := content.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(document.PlainText())
}

func plainDocument(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}
	var document struct {
		Blocks []struct {
			Type    string `json:"type"`
			Content []struct {
				Type  string            `json:"type"`
				Marks []json.RawMessage `json:"marks"`
			} `json:"content"`
		} `json:"blocks"`
	}
	if json.Unmarshal(raw, &document) != nil {
		return false
	}
	for _, block := range document.Blocks {
		if block.Type != textParagraph {
			return false
		}
		for _, node := range block.Content {
			if node.Type != "text" || len(node.Marks) > 0 {
				return false
			}
		}
	}
	return true
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func readOptions(options []questionsdomain.Option) ([]aidomain.Option, bool) {
	sorted := slices.Clone(options)
	slices.SortFunc(sorted, func(a, b questionsdomain.Option) int { return a.Ordinal - b.Ordinal })
	out := make([]aidomain.Option, len(sorted))
	plain := true
	for i, o := range sorted {
		out[i] = aidomain.Option{ID: o.ID, Text: o.Text, Correct: o.IsCorrect}
		plain = plain && plainDocument(o.Content)
	}
	return out, plain
}

func readBlanks(blanks []questionsdomain.Blank) []aidomain.Blank {
	sorted := slices.Clone(blanks)
	slices.SortFunc(sorted, func(a, b questionsdomain.Blank) int { return a.Ordinal - b.Ordinal })
	out := make([]aidomain.Blank, len(sorted))
	for i, b := range sorted {
		out[i] = aidomain.Blank{ID: b.ID, Accepted: slices.Clone(b.AcceptedAnswers)}
	}
	return out
}

func paperQuestion(q questionsdomain.Question, number int) aidomain.PaperQuestion {
	options, plainOptions := readOptions(q.Options)
	return aidomain.PaperQuestion{
		ID: q.ID, Number: number, Type: string(q.Type), Prompt: q.Prompt, Explanation: deref(q.Explanation), Sample: deref(q.SampleAnswer),
		Rich:    !plainOptions || !plainDocument(q.PromptContent) || !plainDocument(q.ExplanationContent),
		Options: options, Blanks: readBlanks(q.Blanks), MaxWords: derefInt(q.MaxWords),
	}
}

func derefInt(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// AIQuestions reads the bank for the ai module from the questions module's handlers.
type AIQuestions struct {
	Question cqrs.QueryHandler[questionsquery.Get, questionsdomain.Question]
	List     cqrs.QueryHandler[questionsquery.List, questionsquery.ListResult]
}

// Get reads one stored question the scope reaches, and answers aidomain.ErrNotFound for any other.
func (r AIQuestions) Get(ctx context.Context, scope access.Scope, id string) (aidomain.Question, error) {
	q, err := r.Question.Handle(ctx, questionsquery.Get{ID: id, Scope: scope})
	if errors.Is(err, questionsdomain.ErrNotFound) {
		return aidomain.Question{}, aidomain.ErrNotFound
	}
	if err != nil {
		return aidomain.Question{}, err
	}
	options, _ := readOptions(q.Options)
	return aidomain.Question{
		ID: q.ID, Type: string(q.Type), Prompt: q.Prompt, Explanation: deref(q.Explanation), Sample: deref(q.SampleAnswer),
		Options: options, Blanks: readBlanks(q.Blanks),
	}, nil
}

const ownPageSize = 100

// Own reads up to limit of the questions the scope's user owns, newest first, and says whether there were more. The scope is narrowed to the user's own rows, so a caller who holds scope.all reads what any other teacher would.
func (r AIQuestions) Own(ctx context.Context, scope access.Scope, limit int) (aidomain.Bank, error) {
	var bank aidomain.Bank
	for page := 1; len(bank.Questions) <= limit; page++ {
		result, err := r.List.Handle(ctx, questionsquery.List{Input: questionsdomain.ListInput{Page: page, Limit: ownPageSize, Scope: scope.Own()}})
		if err != nil {
			return aidomain.Bank{}, err
		}
		for _, q := range result.Items {
			level := ""
			if q.Level != nil {
				level = string(*q.Level)
			}
			bank.Questions = append(bank.Questions, aidomain.BankQuestion{
				ID: q.ID, Type: string(q.Type), Prompt: q.Prompt, Level: level, Tags: slices.Clone(q.Tags), UsedInTests: q.UsedInTests,
			})
		}
		if len(result.Items) < ownPageSize || page*ownPageSize >= result.Page.Total {
			break
		}
	}
	if len(bank.Questions) > limit {
		bank.Questions, bank.Truncated = bank.Questions[:limit], true
	}
	return bank, nil
}

// AIGrading reads one student answer for the ai module from the attempts module's review handler, and hands over the question and the answer text without the attempt, the student or the class.
type AIGrading struct {
	Review cqrs.QueryHandler[attemptsquery.Review, attemptsdomain.Review]
}

// Answer reads the answer to questionID on attemptID. It answers aidomain.ErrNotFound for a paper the scope does not reach or a question not on it, aidomain.ErrAttemptInProgress and aidomain.ErrAttemptVoided for a paper that cannot be marked, and an empty Answer for a question left blank or answered other than in words.
func (g AIGrading) Answer(ctx context.Context, scope access.Scope, attemptID, questionID string) (aidomain.GradedAnswer, error) {
	review, err := g.Review.Handle(ctx, attemptsquery.Review{AttemptID: attemptID, Scope: scope})
	if errors.Is(err, attemptsdomain.ErrPaperNotFound) || errors.Is(err, attemptsdomain.ErrNotFound) {
		return aidomain.GradedAnswer{}, aidomain.ErrNotFound
	}
	if err != nil {
		return aidomain.GradedAnswer{}, err
	}
	switch review.Attempt.Status {
	case attemptsdomain.InProgress:
		return aidomain.GradedAnswer{}, aidomain.ErrAttemptInProgress
	case attemptsdomain.Voided:
		return aidomain.GradedAnswer{}, aidomain.ErrAttemptVoided
	}
	for _, q := range review.Questions {
		if !strings.EqualFold(q.ID, questionID) {
			continue
		}
		saved := review.Answers[q.ID]
		out := aidomain.GradedAnswer{Type: q.Type, Prompt: q.Prompt, Points: q.Points, Answer: answerText(saved.Payload), Manual: saved.RequiresManual}
		if sample := deref(q.SampleAnswer); sample != "" {
			out.Accepted = []string{sample}
		}
		out.Criteria = rubricOf(q.Rubric)
		return out, nil
	}
	return aidomain.GradedAnswer{}, aidomain.ErrNotFound
}

func rubricOf(rubric []attemptsdomain.RubricCriterion) []aidomain.Criterion {
	sorted := slices.Clone(rubric)
	slices.SortFunc(sorted, func(a, b attemptsdomain.RubricCriterion) int { return a.Ordinal - b.Ordinal })
	var out []aidomain.Criterion
	for _, c := range sorted {
		out = append(out, aidomain.Criterion{ID: c.ID, Label: c.Label})
	}
	return out
}

func answerText(payload []byte) string {
	var answer struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if json.Unmarshal(payload, &answer) != nil || answer.Type != "text" {
		return ""
	}
	return answer.Value
}

// AIAnalysis reads how an assignment's papers did for the ai module from the attempts module's item analysis and wrong-answer handlers.
type AIAnalysis struct {
	Items cqrs.QueryHandler[attemptsquery.ItemAnalysis, attemptsdomain.ItemAnalysis]
	Wrong cqrs.QueryHandler[attemptsquery.WrongAnswers, attemptsdomain.WrongAnswers]
}

// Results reads the assignment's questions in paper order with their correct rate, key and most frequent wrong answers, over the papers the scope reaches. It answers aidomain.ErrNotFound for an assignment the scope does not reach.
func (r AIAnalysis) Results(ctx context.Context, scope access.Scope, assignmentID string) (aidomain.Results, error) {
	wrong, err := r.Wrong.Handle(ctx, attemptsquery.WrongAnswers{AssignmentID: assignmentID, Scope: scope})
	if errors.Is(err, attemptsdomain.ErrNotFound) {
		return aidomain.Results{}, aidomain.ErrNotFound
	}
	if err != nil {
		return aidomain.Results{}, err
	}
	analysis, err := r.Items.Handle(ctx, attemptsquery.ItemAnalysis{AssignmentID: assignmentID, Scope: scope})
	if errors.Is(err, attemptsdomain.ErrNotFound) {
		return aidomain.Results{}, aidomain.ErrNotFound
	}
	if err != nil {
		return aidomain.Results{}, err
	}
	figures := map[string]attemptsdomain.AnalysisItem{}
	for _, item := range analysis.Items {
		figures[item.QuestionID] = item
	}
	out := aidomain.Results{HandedIn: wrong.HandedIn}
	for _, q := range wrong.Questions {
		item := figures[q.QuestionID]
		read := aidomain.ResultQuestion{Number: q.Number, Type: q.Type, Prompt: item.PromptExcerpt, Key: q.Key, CorrectRate: item.CorrectRate}
		for _, w := range q.Wrong {
			read.Wrong = append(read.Wrong, aidomain.WrongAnswer{Text: w.Text, Count: w.Count})
		}
		out.Questions = append(out.Questions, read)
	}
	return out, nil
}

// AIPeople reads what the ai module needs to know of a user from the identity and access modules' handlers.
type AIPeople struct {
	User      cqrs.QueryHandler[identityquery.CurrentUser, identitydomain.User]
	Principal cqrs.QueryHandler[accessquery.ResolvePrincipal, access.Principal]
}

// Preferences reads the user's AI preferences. An omitted language is the language of their interface, and Vietnamese when that is none; reading student answers is on unless the user turned it off.
func (p AIPeople) Preferences(ctx context.Context, userID string) (aidomain.Preferences, error) {
	user, err := p.User.Handle(ctx, identityquery.CurrentUser{UserID: userID})
	if err != nil {
		return aidomain.Preferences{}, err
	}
	out := aidomain.Preferences{Language: aidomain.Vietnamese, ReadStudentAnswers: true}
	if locale := aidomain.Language(deref(user.Locale)); locale.Valid() {
		out.Language = locale
	}
	if ai := user.Preferences.AI; ai != nil {
		if chosen := aidomain.Language(deref(ai.Language)); chosen.Valid() {
			out.Language = chosen
		}
		if ai.ReadStudentAnswers != nil {
			out.ReadStudentAnswers = *ai.ReadStudentAnswers
		}
	}
	return out, nil
}

// Permissions reads the user's effective permissions, and answers aidomain.ErrNotFound for an id that names no user.
func (p AIPeople) Permissions(ctx context.Context, userID string) (access.Set, error) {
	principal, err := p.Principal.Handle(ctx, accessquery.ResolvePrincipal{UserID: userID})
	if errors.Is(err, accessdomain.ErrUnknownUser) {
		return access.Set{}, aidomain.ErrNotFound
	}
	if err != nil {
		return access.Set{}, err
	}
	return principal.Permissions, nil
}
