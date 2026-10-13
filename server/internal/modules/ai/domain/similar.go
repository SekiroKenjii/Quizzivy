package domain

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	minSimilarWords  = 3
	minSimilarity    = 0.3
	pairsPerQuestion = 3
	templateRepeats  = 8
	lookupMinimum    = 20
	lookupDivisor    = 20
)

// SimilarPair names two questions of a bank by their place in it, A before B, whose prompts share Score of their distinct words (the Jaccard index of the two word sets, from 0 to 1).
type SimilarPair struct {
	A, B  int
	Score float64
}

// SimilarQuestions returns at most limit pairs of questions of bank whose prompts share at least 30% of their distinct words, the closest first and ties by place. It compares every pair of the bank, so two questions are found wherever they sit, which is what lets a bank be read in parts: a part is never compared with another by anything but this. Words are lower-case runs of letters, digits and marks in Unicode normal form C, so spacing, punctuation and case do not count. A prompt of fewer than three distinct words is too short to compare by its words, and one that appears more than eight times word for word is a stem a teacher reuses with different options and not a copy; both are left out. A question is in at most three of the pairs returned, so one prompt cannot fill the answer with its neighbours. The result is the same for the same bank.
func SimilarQuestions(bank []BankQuestion, limit int) []SimilarPair {
	if limit < 1 || len(bank) < 2 {
		return nil
	}
	sets := wordSets(bank)
	index := sharedIndex(sets)
	var found []SimilarPair
	seen := make([]int, len(sets))
	for i, set := range sets {
		stamp := i + 1
		for _, word := range set {
			for _, j := range index[word] {
				if j <= i || seen[j] == stamp {
					continue
				}
				seen[j] = stamp
				if score := jaccard(set, sets[j]); score >= minSimilarity {
					found = append(found, SimilarPair{A: i, B: j, Score: score})
				}
			}
		}
	}
	slices.SortFunc(found, func(x, y SimilarPair) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(x.A, y.A), cmp.Compare(x.B, y.B))
	})
	return closest(found, len(bank), limit)
}

func closest(found []SimilarPair, questions, limit int) []SimilarPair {
	degree := make([]int, questions)
	var out []SimilarPair
	for _, pair := range found {
		if degree[pair.A] == pairsPerQuestion || degree[pair.B] == pairsPerQuestion {
			continue
		}
		degree[pair.A]++
		degree[pair.B]++
		out = append(out, pair)
		if len(out) == limit {
			break
		}
	}
	return out
}

func wordSets(bank []BankQuestion) [][]string {
	sets := make([][]string, len(bank))
	repeats := map[string]int{}
	for i, q := range bank {
		words := wordsOf(q.Prompt)
		if len(words) < minSimilarWords {
			continue
		}
		sets[i] = words
		repeats[strings.Join(words, " ")]++
	}
	for i, words := range sets {
		if words != nil && repeats[strings.Join(words, " ")] > templateRepeats {
			sets[i] = nil
		}
	}
	return sets
}

func wordsOf(prompt string) []string {
	text := strings.ToLower(norm.NFC.String(prompt))
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r)
	})
	slices.Sort(words)
	return slices.Compact(words)
}

func sharedIndex(sets [][]string) map[string][]int {
	counts := map[string]int{}
	for _, set := range sets {
		for _, word := range set {
			counts[word]++
		}
	}
	commonest := max(lookupMinimum, len(sets)/lookupDivisor)
	index := map[string][]int{}
	for i, set := range sets {
		for _, word := range set {
			if counts[word] <= commonest {
				index[word] = append(index[word], i)
			}
		}
	}
	return index
}

func jaccard(a, b []string) float64 {
	shared, i, j := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch c := strings.Compare(a[i], b[j]); {
		case c == 0:
			shared++
			i++
			j++
		case c < 0:
			i++
		default:
			j++
		}
	}
	return float64(shared) / float64(len(a)+len(b)-shared)
}
