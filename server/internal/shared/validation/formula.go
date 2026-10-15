package validation

var formulaTriggers = map[rune]bool{
	'=': true, '+': true, '-': true, '@': true, '\t': true, '\r': true,
	'＝': true, '＋': true, '－': true, '＠': true,
}

var leadingSpaces = map[rune]bool{' ': true, 0x00A0: true, 0x3000: true}

// FormulaLike reports whether a spreadsheet would run value as a formula: its
// first character, after any leading spaces, is "=", "+", "-", "@", a tab or a
// carriage return, or the full-width form of the first four. A file of
// downloadable text escapes such a value, so a value that must reach the reader
// unchanged, such as the address a sheet hands out, is refused where it enters.
func FormulaLike(value string) bool {
	for _, r := range value {
		if leadingSpaces[r] {
			continue
		}
		return formulaTriggers[r]
	}
	return false
}
