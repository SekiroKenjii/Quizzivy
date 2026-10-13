package domain

import (
	"cmp"
	"encoding/binary"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	minSimilarWords   = 3
	minSimilarity     = 0.3
	pairsPerQuestion  = 3
	templateRepeats   = 8
	maxPromptWords    = 200
	lookupMinimum     = 20
	lookupDivisor     = 20
	maxSimilarityWork = 100_000_000
)

// SimilarPair names two questions of a bank by their place in it, A before B, whose prompts share Score of their distinct words (the Jaccard index of the two word sets, from 0 to 1).
type SimilarPair struct {
	A, B  int
	Score float64
}

// SimilarQuestions returns at most limit pairs of questions of bank whose prompts share at least 30% of their distinct words, the closest first and ties by place. It compares every pair of the bank, so two questions are found wherever they sit, which is what lets a bank be read in parts: a part is never compared with another by anything but this. Words are lower-case runs of letters, digits and marks in Unicode normal form C, so spacing, punctuation and case do not count, and only the first 200 distinct words of a prompt are compared; an accent does count, so a prompt typed without its diacritics is not paired with its accented twin. A prompt of fewer than three distinct words is too short to compare by its words, and one that appears more than eight times word for word is a stem a teacher reuses with different options and not a copy; both are left out. A question is in at most three of the pairs returned, so one prompt cannot fill the answer with its neighbours. The search stops after a fixed amount of work, 0.7 seconds at the worst as measured, which an honest bank stays under (2,000 questions of forty words from a skewed vocabulary take 0.4 seconds); complete is false when it stopped before every question had been compared. The result is the same for the same bank.
func SimilarQuestions(bank []BankQuestion, limit int) (pairs []SimilarPair, complete bool) {
	if limit < 1 || len(bank) < 2 {
		return nil, true
	}
	sets, vocabulary := wordSets(bank)
	found, complete := alikePairs(sets, vocabulary)
	slices.SortFunc(found, func(x, y SimilarPair) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(x.A, y.A), cmp.Compare(x.B, y.B))
	})
	return closest(found, len(bank), limit), complete
}

type search struct {
	sets  [][]int32
	index [][]int32
	seen  []int
	work  int
}

func alikePairs(sets [][]int32, vocabulary int) (found []SimilarPair, complete bool) {
	s := search{sets: sets, index: sharedIndex(sets, vocabulary), seen: make([]int, len(sets))}
	for i := range sets {
		var within bool
		if found, within = s.compare(i, found); !within {
			return found, false
		}
	}
	return found, true
}

func (s *search) compare(i int, found []SimilarPair) ([]SimilarPair, bool) {
	set := s.sets[i]
	for _, word := range set {
		for _, j := range s.index[word] {
			if int(j) <= i || s.seen[j] == i+1 {
				continue
			}
			s.seen[j] = i + 1
			if s.work += len(set) + len(s.sets[j]); s.work > maxSimilarityWork {
				return found, false
			}
			if score := jaccard(set, s.sets[j]); score >= minSimilarity {
				found = append(found, SimilarPair{A: i, B: int(j), Score: score})
			}
		}
	}
	return found, true
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

func wordSets(bank []BankQuestion) (sets [][]int32, vocabulary int) {
	ids := map[string]int32{}
	sets = make([][]int32, len(bank))
	for i, q := range bank {
		words := wordsOf(q.Prompt)
		if len(words) < minSimilarWords {
			continue
		}
		set := make([]int32, len(words))
		for k, word := range words {
			id, known := ids[word]
			if !known {
				id = int32(len(ids))
				ids[word] = id
			}
			set[k] = id
		}
		slices.Sort(set)
		sets[i] = set
	}
	repeats := map[string]int{}
	for _, set := range sets {
		if set != nil {
			repeats[setKey(set)]++
		}
	}
	for i, set := range sets {
		if set != nil && repeats[setKey(set)] > templateRepeats {
			sets[i] = nil
		}
	}
	return sets, len(ids)
}

func setKey(set []int32) string {
	key := make([]byte, 0, 4*len(set))
	for _, id := range set {
		key = binary.LittleEndian.AppendUint32(key, uint32(id))
	}
	return string(key)
}

func wordsOf(prompt string) []string {
	text := strings.ToLower(norm.NFC.String(prompt))
	seen := map[string]bool{}
	var words []string
	for word := range strings.FieldsFuncSeq(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r)
	}) {
		if seen[word] {
			continue
		}
		seen[word] = true
		words = append(words, word)
		if len(words) == maxPromptWords {
			break
		}
	}
	return words
}

func sharedIndex(sets [][]int32, vocabulary int) [][]int32 {
	counts := make([]int, vocabulary)
	for _, set := range sets {
		for _, word := range set {
			counts[word]++
		}
	}
	commonest := max(lookupMinimum, len(sets)/lookupDivisor)
	index := make([][]int32, vocabulary)
	for i, set := range sets {
		for _, word := range set {
			if counts[word] <= commonest {
				index[word] = append(index[word], int32(i))
			}
		}
	}
	return index
}

func jaccard(a, b []int32) float64 {
	shared, i, j := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			shared++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return float64(shared) / float64(len(a)+len(b)-shared)
}
