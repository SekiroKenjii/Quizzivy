package domain_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"quizzivy/internal/modules/ai/domain"
)

func distinctBank(n int) []domain.BankQuestion {
	bank := make([]domain.BankQuestion, n)
	for i := range bank {
		bank[i] = domain.BankQuestion{ID: fmt.Sprintf("b%d", i), Type: "short_answer", Prompt: fmt.Sprintf("apple%d banana%d cherry%d", i, i, i)}
	}
	return bank
}

func TestTwoQuestionsAreComparedWhereverTheySitInTheBank(t *testing.T) {
	bank := distinctBank(2000)
	bank[3].Prompt = "Which tense does the sentence use: she has lived in Hanoi since 2019?"
	bank[1950].Prompt = "Which tense does the sentence use: she has lived in Hanoi since 2020?"
	pairs := domain.SimilarQuestions(bank, 100)
	if len(pairs) != 1 || pairs[0].A != 3 || pairs[0].B != 1950 || pairs[0].Score < 0.8 {
		t.Fatalf("pairs %+v, want the one pair at 3 and 1950", pairs)
	}
}

func TestTheDecksDuplicatePairIsACandidateAndAnUnrelatedPairIsNot(t *testing.T) {
	bank := []domain.BankQuestion{
		{Prompt: "Choose the correct form: She ___ in Hanoi since 2019."},
		{Prompt: "Choose the correct form: He ___ in Da Nang since 2020."},
		{Prompt: "Which word is closest in meaning to “reluctant”?"},
		{Prompt: "Name the capital city of France and one river that flows through it."},
	}
	pairs := domain.SimilarQuestions(bank, 10)
	if len(pairs) != 1 || pairs[0].A != 0 || pairs[0].B != 1 {
		t.Errorf("pairs %+v, want only the deck's first pair", pairs)
	}
}

func TestTheClosestPairsComeFirstAndTiesGoByPlace(t *testing.T) {
	bank := []domain.BankQuestion{
		{Prompt: "one two three four five six"},
		{Prompt: "one two three four five seven"},
		{Prompt: "one two three four five six"},
		{Prompt: "nine ten eleven twelve thirteen"},
		{Prompt: "nine ten eleven twelve thirteen"},
	}
	pairs := domain.SimilarQuestions(bank, 10)
	got := make([][2]int, len(pairs))
	for i, p := range pairs {
		got[i] = [2]int{p.A, p.B}
	}
	want := [][2]int{{0, 2}, {3, 4}, {0, 1}, {1, 2}}
	if !slices.Equal(got, want) {
		t.Errorf("pairs %v, want %v: exact copies first, by place, then the one-word differences", got, want)
	}
	if limited := domain.SimilarQuestions(bank, 2); len(limited) != 2 || limited[0] != pairs[0] || limited[1] != pairs[1] {
		t.Errorf("a limit of two gave %+v", limited)
	}
}

func TestAQuestionIsInAtMostThreePairs(t *testing.T) {
	bank := make([]domain.BankQuestion, 10)
	for i := range bank {
		bank[i] = domain.BankQuestion{Prompt: fmt.Sprintf("the sunny garden gate opens at seven tag%d", i)}
	}
	degree := map[int]int{}
	for _, p := range domain.SimilarQuestions(bank, 100) {
		degree[p.A]++
		degree[p.B]++
	}
	for question, n := range degree {
		if n > 3 {
			t.Errorf("question %d is in %d pairs", question, n)
		}
	}
	if len(degree) == 0 {
		t.Error("no pair found among ten alike prompts")
	}
}

func TestAStemUsedManyTimesAndAPromptTooShortToCompareAreLeftOut(t *testing.T) {
	var bank []domain.BankQuestion
	for range 9 {
		bank = append(bank, domain.BankQuestion{Prompt: "Choose the correct answer for the blank."})
	}
	bank = append(bank, domain.BankQuestion{Prompt: "Define abate."}, domain.BankQuestion{Prompt: "Define abate."})
	if pairs := domain.SimilarQuestions(bank, 100); len(pairs) != 0 {
		t.Errorf("pairs %+v, want none: a stem nine questions reuse is a template and two words are too few", pairs)
	}
	eight := bank[:8]
	if pairs := domain.SimilarQuestions(eight, 100); len(pairs) == 0 {
		t.Error("a stem used eight times is a copy and found")
	}
}

func TestCaseSpacingPunctuationAndNormalisationDoNotCount(t *testing.T) {
	composed := "Tại sao bầu trời có màu xanh?"
	decomposed := norm.NFD.String(composed)
	bank := []domain.BankQuestion{
		{Prompt: composed},
		{Prompt: "  TẠI   SAO, bầu trời có màu xanh ... "},
		{Prompt: decomposed},
	}
	if composed == decomposed {
		t.Fatal("the fixture is not in two forms")
	}
	pairs := domain.SimilarQuestions(bank, 10)
	if len(pairs) == 0 || pairs[0].Score != 1 {
		t.Errorf("pairs %+v, want the three spellings of one prompt to match completely", pairs)
	}
}

func TestSimilarQuestionsIsDeterministicAndBoundedOnABigBank(t *testing.T) {
	var bank []domain.BankQuestion
	for i := range 2000 {
		var words []string
		for j := range 9 {
			words = append(words, fmt.Sprintf("w%d", (i*7+j*13)%400))
		}
		bank = append(bank, domain.BankQuestion{Prompt: strings.Join(words, " ")})
	}
	first := domain.SimilarQuestions(bank, 100)
	second := domain.SimilarQuestions(bank, 100)
	if !slices.Equal(first, second) {
		t.Error("the same bank gave two answers")
	}
	if len(first) > 100 {
		t.Errorf("%d pairs for a limit of 100", len(first))
	}
	if domain.SimilarQuestions(bank, 0) != nil || domain.SimilarQuestions(bank[:1], 10) != nil {
		t.Error("a limit of none and a bank of one find pairs")
	}
}
