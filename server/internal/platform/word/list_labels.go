package word

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

func (r *resolver) resolveNumbering(p Paragraph, style styleResult, structures map[Locator]Structure) (*NumberingLabel, error) {
	props := mergeNumProps(prop(r.styles.paragraphDefaults, propertyNumbering).Children, style.numbering)
	direct := prop(p.Properties, propertyNumbering)
	props = mergeNumProps(props, direct.Children)
	if len(props) == 0 && style.complete {
		return nil, nil
	}
	id, valid := numberID(val(props, propertyNumberID))
	invalidProperties := duplicateProperties(p.Properties) || duplicateProperties(direct.Children)
	if valid && id == "0" && !invalidProperties {
		return nil, nil
	}
	label := &NumberingLabel{ID: id, Level: -1}
	if r.ambiguousNumbering(p, structures) {
		r.finding("NUMBERING_CONTEXT_REQUIRES_REVIEW", p.Locator)
		return label, nil
	}
	if !valid || !style.complete || invalidProperties {
		r.numbering.tainted = true
		r.finding("NUMBERING_REFERENCE_REQUIRES_REVIEW", p.Locator)
		return label, nil
	}
	list, err := r.list(id)
	if err != nil {
		return nil, err
	}
	level, valid := paragraphLevel(direct, props, style, list)
	label.Level = level
	if !valid || !list.valid {
		r.numbering.tainted = true
	}
	if !valid || !list.valid || r.numbering.tainted {
		r.finding("NUMBERING_CONTEXT_REQUIRES_REVIEW", p.Locator)
		return label, nil
	}
	return r.advanceLabel(p, list, label)
}

func (r *resolver) advanceLabel(p Paragraph, list *listDefinition, label *NumberingLabel) (*NumberingLabel, error) {
	level := label.Level
	label.Sources = append(slices.Clone(list.sources), list.levels[level].sources...)
	if err := r.retainLocations(label.Sources); err != nil {
		return nil, err
	}
	label.Suffix = list.levels[level].suffix
	label.Text, label.Resolved = advanceList(list, level)
	if err := r.spend(len(label.Text) + len(label.Sources) + 1); err != nil {
		return nil, err
	}
	if !label.Resolved {
		list.valid = false
		label.Text = ""
		r.finding("NUMBERING_LABEL_REQUIRES_REVIEW", p.Locator)
	}
	return label, nil
}

func paragraphLevel(direct Property, props []Property, style styleResult, list *listDefinition) (int, bool) {
	if value := val(direct.Children, propertyLevel); value != "" {
		return listIndex(value)
	}
	if len(style.numbering) == 0 {
		if value := val(props, propertyLevel); value != "" {
			return listIndex(value)
		}
		return 0, true
	}
	level := -1
	for i, definition := range list.levels {
		if definition.style != "" && slices.Contains(style.ids, definition.style) {
			if level >= 0 {
				return -1, false
			}
			level = i
		}
	}
	return level, level >= 0
}

func (r *resolver) ambiguousNumbering(p Paragraph, structures map[Locator]Structure) bool {
	if p.Part != r.source.MainPart || r.numbering.ambiguousBody {
		return true
	}
	for _, loc := range p.Containers {
		if structures[loc].Kind == elementTextBox {
			return true
		}
	}
	return false
}

func advanceList(list *listDefinition, index int) (string, bool) {
	level := list.levels[index]
	if !level.present || !level.valid {
		return "", false
	}
	if list.seen[index] {
		list.counts[index]++
	} else {
		list.counts[index] = level.start
	}
	list.seen[index] = true
	for i := index + 1; i < len(list.levels); i++ {
		if list.levels[i].restart >= index {
			list.seen[i] = false
		}
	}
	return renderLabel(list, index)
}

func renderLabel(list *listDefinition, index int) (string, bool) {
	level := list.levels[index]
	if level.format == "bullet" {
		if strings.ContainsFunc(level.pattern, func(r rune) bool { return unicode.Is(unicode.Co, r) }) {
			return "", false
		}
		return level.pattern, true
	}
	if level.format == "none" {
		return "", true
	}
	var result strings.Builder
	for i := 0; i < len(level.pattern); i++ {
		if level.pattern[i] != '%' || i+1 == len(level.pattern) || level.pattern[i+1] < '1' || level.pattern[i+1] > '9' {
			result.WriteByte(level.pattern[i])
			continue
		}
		i++
		reference := int(level.pattern[i] - '1')
		if reference > index {
			continue
		}
		text, valid := referenceNumber(list, reference, level.legal)
		if !valid {
			return "", false
		}
		result.WriteString(text)
		if result.Len() > 1024 {
			return "", false
		}
	}
	return result.String(), result.Len() <= 1024
}

func referenceNumber(list *listDefinition, index int, legal bool) (string, bool) {
	level := list.levels[index]
	if !level.present || !level.valid {
		return "", false
	}
	value := level.start
	if list.seen[index] {
		value = list.counts[index]
	}
	format := level.format
	if legal {
		format = formatDecimal
	}
	return formatNumber(value, format)
}

func formatNumber(value int64, format string) (string, bool) {
	switch format {
	case formatDecimal:
		return strconv.FormatInt(value, 10), true
	case "decimalZero":
		text := strconv.FormatInt(value, 10)
		if value < 10 {
			text = "0" + text
		}
		return text, true
	case "upperLetter", "lowerLetter":
		if value < 1 || value > 26 {
			return "", false
		}
		first := int64('A')
		if format == "lowerLetter" {
			first = 'a'
		}
		return string(rune(first + value - 1)), true
	case "upperRoman", "lowerRoman":
		text, valid := romanNumber(value)
		if format == "lowerRoman" {
			text = strings.ToLower(text)
		}
		return text, valid
	default:
		return "", false
	}
}

func romanNumber(value int64) (string, bool) {
	if value < 1 || value > 3999 {
		return "", false
	}
	var text strings.Builder
	for _, pair := range []struct {
		number int64
		text   string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for value >= pair.number {
			text.WriteString(pair.text)
			value -= pair.number
		}
	}
	return text.String(), true
}
