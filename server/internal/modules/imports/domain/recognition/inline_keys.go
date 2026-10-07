package recognition

import "regexp"

var (
	multiKeyPrefix = regexp.MustCompile(`^\s*(?i:answer\s*key|answers|key|đáp\s*án)\s*:\s*`)
	multiKeyPair   = regexp.MustCompile(`(?:^|\s)(\d{1,3}(?:\.\d{1,2})?)\s*[-:.)]\s*`)
)

func prefixedKeyEntries(l *line, from int, origin keyOrigin) []keyEntry {
	text := matchable(l.text[from:])
	prefix := multiKeyPrefix.FindStringIndex(text)
	if prefix == nil {
		return nil
	}
	value := text[prefix[1]:]
	matches := multiKeyPair.FindAllStringSubmatchIndex(value, -1)
	if len(matches) < 2 || matches[0][0] != 0 {
		return nil
	}
	var entries []keyEntry
	for i, m := range matches {
		end := len(value)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		entry, ok := prefixedKeyEntry(l, from, text, prefix[1], value, m, end, origin)
		if !ok {
			return nil
		}
		entries = append(entries, entry)
	}
	l.consumeAll()
	return entries
}

func prefixedKeyEntry(l *line, from int, text string, prefix int, value string, m []int, end int, origin keyOrigin) (keyEntry, bool) {
	labelText := value[m[2]:m[3]]
	lb, ok := questionLabel([]rune(labelText + "."))
	if !ok {
		return keyEntry{}, false
	}
	start := from + runeCount(text, prefix+m[1])
	stop := from + runeCount(text, prefix+end)
	start, stop = trimmed(l.text, start, stop)
	if start >= stop {
		return keyEntry{}, false
	}
	at := from + runeCount(text, prefix+m[2])
	return keyEntry{keyValue: keyValue{value: matchable(l.text[start:stop]), source: l.refs(at, stop), origin: origin}, label: lb}, true
}

func (e *exam) prefixedKeyLine(l *line) bool {
	entries := prefixedKeyEntries(l, 0, keySameFile)
	if len(entries) == 0 {
		return false
	}
	e.closeQuestion()
	e.keyEntries = append(e.keyEntries, entries...)
	return true
}
