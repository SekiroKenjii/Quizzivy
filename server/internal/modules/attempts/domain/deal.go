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

// Present applies §7's two Shuffle switches. Questions move only inside their
// section and sections keep the order given, so the navigator can group the
// numbers by part (S-06, S-08) and still count them in presentation order; a
// question whose section is not listed sorts after the listed ones. Blanks are
// never shuffled: a blank's ordinal is its position in the prompt text, so
// reordering them would renumber the sentence the student is reading.
func (DealManager) Present(seed int64, shuffleQuestions, shuffleOptions bool, sections []Section, qs []Question) []Question {
	if shuffleQuestions {
		qs = shuffleWithinSections(seed, sections, qs)
	}
	if !shuffleOptions {
		return qs
	}
	for i, q := range qs {
		qs[i].Options = Shuffle(seed, q.ID, q.Options, func(o Option) string { return o.ID })
	}
	return qs
}

func shuffleWithinSections(seed int64, sections []Section, qs []Question) []Question {
	order := make([]string, 0, len(sections))
	groups := map[string][]Question{}
	for _, sec := range sections {
		order = append(order, sec.ID)
		groups[sec.ID] = nil
	}
	for _, q := range qs {
		if _, listed := groups[q.SectionID]; !listed {
			order = append(order, q.SectionID)
		}
		groups[q.SectionID] = append(groups[q.SectionID], q)
	}

	out := make([]Question, 0, len(qs))
	for _, id := range order {
		out = append(out, Shuffle(seed, "questions", groups[id], func(q Question) string { return q.ID })...)
	}
	return out
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
