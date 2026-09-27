package access

import (
	"cmp"
	"slices"
)

// Set is an immutable set of catalogue keys: a role's grants, or a principal's
// effective permissions. The zero value is the empty set.
type Set struct {
	keys map[Key]struct{}
}

// NewSet returns the set holding keys; a repeated key counts once.
func NewSet(keys ...Key) Set {
	m := make(map[Key]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return Set{keys: m}
}

// Has reports whether k is in s.
func (s Set) Has(k Key) bool {
	_, ok := s.keys[k]
	return ok
}

// Len returns the number of keys in s.
func (s Set) Len() int { return len(s.keys) }

// SubsetOf reports whether every key in s is also in other.
func (s Set) SubsetOf(other Set) bool {
	for k := range s.keys {
		if !other.Has(k) {
			return false
		}
	}
	return true
}

// Without returns s minus keys, leaving s unchanged.
func (s Set) Without(keys ...Key) Set {
	m := make(map[Key]struct{}, len(s.keys))
	for k := range s.keys {
		m[k] = struct{}{}
	}
	for _, k := range keys {
		delete(m, k)
	}
	return Set{keys: m}
}

// Keys returns the keys in s in catalogue order. A key outside the catalogue
// sorts after every catalogue key, in lexical order.
func (s Set) Keys() []Key {
	out := make([]Key, 0, len(s.keys))
	for k := range s.keys {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b Key) int {
		pa, okA := position[a]
		pb, okB := position[b]
		switch {
		case okA && okB:
			return cmp.Compare(pa, pb)
		case okA:
			return -1
		case okB:
			return 1
		}
		return cmp.Compare(a, b)
	})
	return out
}
