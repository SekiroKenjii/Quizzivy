package domain_test

import (
	"fmt"
	"math/rand/v2"
	"quizzivy/internal/modules/attempts/domain"
	"slices"
	"strings"
	"testing"
)

func questions(n int) []domain.Question {
	out := make([]domain.Question, n)
	for i := range out {
		out[i] = domain.Question{
			ID: fmt.Sprintf("q-%02d", i),
			Options: []domain.Option{
				{ID: fmt.Sprintf("q-%02d-a", i)}, {ID: fmt.Sprintf("q-%02d-b", i)},
				{ID: fmt.Sprintf("q-%02d-c", i)}, {ID: fmt.Sprintf("q-%02d-d", i)},
			},
		}
	}
	return out
}

func order(qs []domain.Question) string {
	ids := make([]string, len(qs))
	for i, q := range qs {
		ids[i] = q.ID
	}
	return strings.Join(ids, ",")
}

func TestTheSameSeedAlwaysDealsTheSamePaper(t *testing.T) {
	const seed = 0x5eed
	want := order(domain.Deal.Present(seed, true, true, nil, questions(12)))

	for i := range 1000 {
		if got := order(domain.Deal.Present(seed, true, true, nil, questions(12))); got != want {
			t.Fatalf("run %d dealt a different paper\n got %s\nwant %s", i, got, want)
		}
	}
}

func TestDifferentSeedsDealDifferentPapers(t *testing.T) {
	seen := map[string]int64{}
	var collisions int
	for seed := int64(1); seed <= 200; seed++ {
		got := order(domain.Deal.Present(seed, true, false, nil, questions(12)))
		if prior, ok := seen[got]; ok {
			collisions++
			t.Logf("seeds %d and %d agree: %s", prior, seed, got)
		}
		seen[got] = seed
	}
	// Not "all distinct": with 12!
	if collisions > 1 {
		t.Fatalf("%d seed pairs dealt an identical paper; the seed is barely reaching the order", collisions)
	}
}

// The property the design actually turns on. Fisher-Yates would pass every test
// above and fail this one: it permutes the order it is handed, so adding an
// index or changing a JOIN would silently re-deal a paper mid-attempt.
func TestThePaperDoesNotDependOnTheOrderRowsArriveIn(t *testing.T) {
	const seed = 918273645
	want := order(domain.Deal.Present(seed, true, true, nil, questions(20)))

	shuffled := questions(20)
	source := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		source.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		if got := order(domain.Deal.Present(seed, true, true, nil, slices.Clone(shuffled))); got != want {
			t.Fatalf("a reordered query re-dealt the paper\n got %s\nwant %s", got, want)
		}
	}
}

func TestOptionsOfDifferentQuestionsDoNotMoveInLockstep(t *testing.T) {
	dealt := domain.Deal.Present(7, false, true, nil, questions(30))

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
	// A blank's ordinal is its position in the prompt text.
	q := domain.Question{ID: "q1", Blanks: []domain.Blank{{ID: "b1", Ordinal: 1}, {ID: "b2", Ordinal: 2}, {ID: "b3", Ordinal: 3}}}
	for seed := int64(1); seed <= 100; seed++ {
		got := domain.Deal.Present(seed, true, true, nil, []domain.Question{q})[0].Blanks
		for i, b := range got {
			if b.Ordinal != i+1 {
				t.Fatalf("seed %d reordered blanks: %v", seed, got)
			}
		}
	}
}

func TestNothingIsLostOrDuplicatedInTheDeal(t *testing.T) {
	for seed := int64(1); seed <= 100; seed++ {
		dealt := domain.Deal.Present(seed, true, true, nil, questions(25))
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

func sectioned(counts ...int) ([]domain.Section, []domain.Question) {
	var sections []domain.Section
	var qs []domain.Question
	for s, n := range counts {
		id := fmt.Sprintf("s-%d", s)
		sections = append(sections, domain.Section{ID: id, Title: id})
		for i := range n {
			qs = append(qs, domain.Question{ID: fmt.Sprintf("s-%d-q-%02d", s, i), SectionID: id})
		}
	}
	return sections, qs
}

func TestSectionsNeverInterleave(t *testing.T) {
	sections, qs := sectioned(10, 5, 9)
	for seed := int64(1); seed <= 200; seed++ {
		dealt := domain.Deal.Present(seed, true, false, sections, slices.Clone(qs))
		if len(dealt) != len(qs) {
			t.Fatalf("seed %d dealt %d of %d questions", seed, len(dealt), len(qs))
		}
		for i, q := range dealt {
			want := "s-0"
			switch {
			case i >= 15:
				want = "s-2"
			case i >= 10:
				want = "s-1"
			}
			if q.SectionID != want {
				t.Fatalf("seed %d put %s at position %d, inside %s", seed, q.ID, i, want)
			}
		}
	}
}

func TestSectionOrderIsTheTestOrderNotTheRowOrder(t *testing.T) {
	sections, qs := sectioned(4, 4)
	reversed := slices.Clone(qs)
	slices.Reverse(reversed)

	want := order(domain.Deal.Present(3, true, false, sections, slices.Clone(qs)))
	if got := order(domain.Deal.Present(3, true, false, sections, reversed)); got != want {
		t.Fatalf("reversed rows re-dealt the paper\n got %s\nwant %s", got, want)
	}
	if !strings.HasPrefix(want, "s-0-") {
		t.Fatalf("the first section listed did not come first: %s", want)
	}
}

func TestASingleSectionDealsAsBeforeSectionsExisted(t *testing.T) {
	sections, qs := sectioned(12)
	flat := slices.Clone(qs)
	for i := range flat {
		flat[i].SectionID = ""
	}
	for seed := int64(1); seed <= 100; seed++ {
		listed := order(domain.Deal.Present(seed, true, false, sections, slices.Clone(qs)))
		unlisted := order(domain.Deal.Present(seed, true, false, nil, slices.Clone(flat)))
		if listed != unlisted {
			t.Fatalf("seed %d: one section deals differently from no section\n got %s\nwant %s", seed, listed, unlisted)
		}
	}
}

func TestGroupsShuffleAsUnitsWithStableMemberOrder(t *testing.T) {
	sections, qs := sectioned(6, 4)
	for i := 1; i < 4; i++ {
		qs[i].GroupID = "group-a"
		qs[i].GroupOrdinal = i - 1
	}
	for i := 7; i < 10; i++ {
		qs[i].GroupID = "group-b"
		qs[i].GroupOrdinal = i - 7
	}
	positions := map[string]bool{}
	random := rand.New(rand.NewPCG(73, 19))
	for seed := int64(1); seed <= 100; seed++ {
		got := domain.Deal.Present(seed, true, false, sections, qs)
		positions[order(got)] = true
		if len(got) != len(qs) {
			t.Fatal("group dealing lost a question")
		}
		seen := make(map[string]bool)
		for i, q := range got {
			if seen[q.ID] {
				t.Fatal("group dealing duplicated a question")
			}
			seen[q.ID] = true
			expectedSection := "s-0"
			if i >= 6 {
				expectedSection = "s-1"
			}
			if q.SectionID != expectedSection {
				t.Fatal("group crossed its section")
			}
			if q.GroupID != "" && q.GroupOrdinal > 0 {
				if i == 0 || got[i-1].GroupID != q.GroupID || got[i-1].GroupOrdinal != q.GroupOrdinal-1 {
					t.Fatalf("group separated or reordered at seed %d: %s", seed, order(got))
				}
			}
		}
		reordered := slices.Clone(qs)
		random.Shuffle(len(reordered), func(i, j int) { reordered[i], reordered[j] = reordered[j], reordered[i] })
		if other := domain.Deal.Present(seed, true, false, sections, reordered); order(other) != order(got) {
			t.Fatalf("unordered SQL rows changed group deal at seed %d", seed)
		}
	}
	if len(positions) < 5 {
		t.Fatal("groups did not move across seeds")
	}
	if got := domain.Deal.Present(19, false, false, sections, qs); order(got) != order(qs) {
		t.Fatal("disabled shuffle changed authored order")
	}
}

func TestFixedGroupOptionsKeepAuthoredOrderAndDoNotMutateInput(t *testing.T) {
	qs := questions(4)
	qs[0].GroupID = "group-a"
	qs[0].FixedOptionOrder = true
	original := slices.Clone(qs[0].Options)
	other := slices.Clone(qs[1].Options)
	varied := false
	for seed := int64(1); seed <= 30; seed++ {
		got := domain.Deal.Present(seed, false, true, nil, qs)
		if !slices.EqualFunc(got[0].Options, original, func(a, b domain.Option) bool { return a.ID == b.ID }) {
			t.Fatal("fixed labels changed")
		}
		if !slices.EqualFunc(got[1].Options, other, func(a, b domain.Option) bool { return a.ID == b.ID }) {
			varied = true
		}
		if !slices.EqualFunc(qs[1].Options, other, func(a, b domain.Option) bool { return a.ID == b.ID }) {
			t.Fatal("presentation mutated the loaded paper")
		}
	}
	if !varied {
		t.Fatal("fixed group member disabled shuffling for unrelated questions")
	}
}
