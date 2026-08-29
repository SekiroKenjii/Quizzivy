package attempts

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"slices"
)

// shuffle puts items into presentation order.
//
// [D-02] The permutation is a pure function of the seed and each item's own id.
// A reload must not reshuffle a paper: answers are keyed by question id and
// survive either way, but a student reading "câu 4" and a teacher reviewing
// "câu 4" have to mean the same question, and an option list that reorders
// under someone mid-answer is its own small betrayal.
//
// Sorting by a keyed hash rather than running a seeded Fisher-Yates over the
// slice is the whole point. Fisher-Yates permutes the order it is given, so it
// is a function of the query's row order as much as of the seed -- add an index,
// change a JOIN, and the same seed yields a different paper. Hashing each id
// independently has no such input.
func shuffle[T any](seed int64, salt string, items []T, id func(T) string) []T {
	ranked := make([]rankedItem[T], len(items))
	for i, item := range items {
		ranked[i] = rankedItem[T]{rank: rank(seed, salt, id(item)), id: id(item), item: item}
	}
	slices.SortFunc(ranked, func(a, b rankedItem[T]) int {
		// The id tiebreak makes this a total order even on a hash collision,
		// which a comparison on rank alone would leave to sort's discretion.
		return cmp.Or(cmp.Compare(a.rank, b.rank), cmp.Compare(a.id, b.id))
	})

	out := make([]T, len(ranked))
	for i, r := range ranked {
		out[i] = r.item
	}
	return out
}

type rankedItem[T any] struct {
	rank uint64
	id   string
	item T
}

// rank folds the salt in so that one seed permutes each question's options
// differently -- without it every question of the same length would present its
// options in a visibly parallel order.
//
// SHA-256 rather than the cheap non-cryptographic hash this was first written
// with. FNV-1a's avalanche is too weak for the job: consuming the seed before
// the id leaves it as little more than an offset, and the tests below found
// seeds 16 apart dealing byte-identical papers, and thirty questions sharing
// two option orders between them. A couple of hundred hashes are drawn per
// attempt, once, so there is nothing here worth optimising for.
//
// The separator keeps ("ab", "c") from hashing as ("a", "bc"). Both arguments
// are ids today, but only by habit.
func rank(seed int64, salt, id string) uint64 {
	h := sha256.New()
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(seed))
	h.Write(b[:])
	h.Write([]byte(salt))
	h.Write([]byte{0})
	h.Write([]byte(id))
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}

// present applies §7's two shuffle switches. Blanks are never shuffled: a
// blank's ordinal is its position in the prompt text, so reordering them would
// renumber the sentence the student is reading.
func present(seed int64, shuffleQuestions, shuffleOptions bool, qs []Question) []Question {
	if shuffleQuestions {
		qs = shuffle(seed, "questions", qs, func(q Question) string { return q.ID })
	}
	if !shuffleOptions {
		return qs
	}
	for i, q := range qs {
		qs[i].Options = shuffle(seed, q.ID, q.Options, func(o Option) string { return o.ID })
	}
	return qs
}
