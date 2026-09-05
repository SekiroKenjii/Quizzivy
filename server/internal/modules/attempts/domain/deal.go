package domain

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"slices"
)

// DealManager deals a paper from a seed: the same seed always deals the same
// paper, whatever order the rows arrive in.
type DealManager struct{}

// Present applies §7's two Shuffle switches. Blanks are never shuffled: a
// blank's ordinal is its position in the prompt text, so reordering them would
// renumber the sentence the student is reading.
func (DealManager) Present(seed int64, shuffleQuestions, shuffleOptions bool, qs []Question) []Question {
	if shuffleQuestions {
		qs = Shuffle(seed, "questions", qs, func(q Question) string { return q.ID })
	}
	if !shuffleOptions {
		return qs
	}
	for i, q := range qs {
		qs[i].Options = Shuffle(seed, q.ID, q.Options, func(o Option) string { return o.ID })
	}
	return qs
}

// Shuffle puts items into presentation order.
func Shuffle[T any](seed int64, salt string, items []T, id func(T) string) []T {
	ranked := make([]rankedItem[T], len(items))
	for i, item := range items {
		ranked[i] = rankedItem[T]{rank: rank(seed, salt, id(item)), id: id(item), item: item}
	}
	slices.SortFunc(ranked, func(a, b rankedItem[T]) int {
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
