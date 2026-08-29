package attempts

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func questions(n int) []Question {
	out := make([]Question, n)
	for i := range out {
		out[i] = Question{
			ID: fmt.Sprintf("q-%02d", i),
			Options: []Option{
				{ID: fmt.Sprintf("q-%02d-a", i)}, {ID: fmt.Sprintf("q-%02d-b", i)},
				{ID: fmt.Sprintf("q-%02d-c", i)}, {ID: fmt.Sprintf("q-%02d-d", i)},
			},
		}
	}
	return out
}

func order(qs []Question) string {
	ids := make([]string, len(qs))
	for i, q := range qs {
		ids[i] = q.ID
	}
	return strings.Join(ids, ",")
}

func TestTheSameSeedAlwaysDealsTheSamePaper(t *testing.T) {
	const seed = 0x5eed
	want := order(present(seed, true, true, questions(12)))

	for i := range 1000 {
		if got := order(present(seed, true, true, questions(12))); got != want {
			t.Fatalf("run %d dealt a different paper\n got %s\nwant %s", i, got, want)
		}
	}
}

func TestDifferentSeedsDealDifferentPapers(t *testing.T) {
	seen := map[string]int64{}
	var collisions int
	for seed := int64(1); seed <= 200; seed++ {
		got := order(present(seed, true, false, questions(12)))
		if prior, ok := seen[got]; ok {
			collisions++
			t.Logf("seeds %d and %d agree: %s", prior, seed, got)
		}
		seen[got] = seed
	}
	// Not "all distinct": with 12! orders, a repeat among 200 draws is possible
	// and would make this test flaky rather than correct. What must not happen
	// is the seed being ignored, which shows up as wholesale agreement.
	if collisions > 1 {
		t.Fatalf("%d seed pairs dealt an identical paper; the seed is barely reaching the order", collisions)
	}
}

// The property the design actually turns on. Fisher-Yates would pass every test
// above and fail this one: it permutes the order it is handed, so adding an
// index or changing a JOIN would silently re-deal a paper mid-attempt.
func TestThePaperDoesNotDependOnTheOrderRowsArriveIn(t *testing.T) {
	const seed = 918273645
	want := order(present(seed, true, true, questions(20)))

	shuffled := questions(20)
	source := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		source.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		if got := order(present(seed, true, true, slices.Clone(shuffled))); got != want {
			t.Fatalf("a reordered query re-dealt the paper\n got %s\nwant %s", got, want)
		}
	}
}

func TestOptionsOfDifferentQuestionsDoNotMoveInLockstep(t *testing.T) {
	dealt := present(7, false, true, questions(30))

	// Each question's options are salted with its own id, so two questions
	// holding the same number of options must not land on the same permutation
	// every time. Without the salt every question's options march together and
	// a student who spots one pattern has spotted them all.
	shapes := map[string]bool{}
	for _, q := range dealt {
		var suffixes []string
		for _, o := range q.Options {
			suffixes = append(suffixes, o.ID[len(o.ID)-1:])
		}
		shapes[strings.Join(suffixes, "")] = true
	}
	if len(shapes) < 4 {
		t.Fatalf("30 questions produced only %d option orders: %v", len(shapes), shapes)
	}
}

func TestBlanksAreNeverShuffled(t *testing.T) {
	// A blank's ordinal is its position in the prompt text. Reordering blanks
	// would renumber the sentence the student is reading.
	q := Question{ID: "q1", Blanks: []Blank{{ID: "b1", Ordinal: 1}, {ID: "b2", Ordinal: 2}, {ID: "b3", Ordinal: 3}}}
	for seed := int64(1); seed <= 100; seed++ {
		got := present(seed, true, true, []Question{q})[0].Blanks
		for i, b := range got {
			if b.Ordinal != i+1 {
				t.Fatalf("seed %d reordered blanks: %v", seed, got)
			}
		}
	}
}

func TestNothingIsLostOrDuplicatedInTheDeal(t *testing.T) {
	for seed := int64(1); seed <= 100; seed++ {
		dealt := present(seed, true, true, questions(25))
		if len(dealt) != 25 {
			t.Fatalf("seed %d dealt %d of 25 questions", seed, len(dealt))
		}
		seen := map[string]bool{}
		for _, q := range dealt {
			if seen[q.ID] {
				t.Fatalf("seed %d dealt %s twice", seed, q.ID)
			}
			seen[q.ID] = true
			if len(q.Options) != 4 {
				t.Fatalf("seed %d left %s with %d of 4 options", seed, q.ID, len(q.Options))
			}
		}
	}
}
