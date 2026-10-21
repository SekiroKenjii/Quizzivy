// Package matchkey derives the keys a matching question's choices and items
// are named by (D-063). A key comes from the stored ordinal, never from the
// order a payload delivers, so the editor, the paper, grading and the result
// name the same choice the same way.
package matchkey

import "strings"

var numerals = []struct {
	value  int
	symbol string
}{
	{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"},
	{50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"},
}

// Option is a matching choice's key: its ordinal plus one as a lower-case
// roman numeral, so ordinal 0 is "i" and 11 is "xii". A negative ordinal has
// no key and gives "".
func Option(ordinal int) string {
	if ordinal < 0 {
		return ""
	}
	n := ordinal + 1
	var b strings.Builder
	for _, numeral := range numerals {
		for n >= numeral.value {
			b.WriteString(numeral.symbol)
			n -= numeral.value
		}
	}
	return b.String()
}

// Item is a matching item's key: its ordinal as capital letters, so ordinal
// 0 is "A", 19 is "T" and 26 is "AA". A negative ordinal has no key and
// gives "".
func Item(ordinal int) string {
	if ordinal < 0 {
		return ""
	}
	var key []byte
	for n := ordinal + 1; n > 0; n = (n - 1) / 26 {
		key = append([]byte{byte('A' + (n-1)%26)}, key...)
	}
	return string(key)
}
