package content

import (
	"strings"
	"unicode/utf8"
)

type validator struct {
	nodes      int
	characters int
	gaps       map[string]bool
	gapIDs     []string
	assets     map[string]string
	assetRefs  []AssetReference
}

func (v *validator) document(value any) (string, bool) {
	switch kind(value, "format") {
	case "legacy_markdown_v1":
		obj, ok := object(value, "format", "markdown")
		if !ok {
			return "", false
		}
		return v.string(obj["markdown"], 0, MaxText)
	case "semantic_v1":
		obj, ok := object(value, "format", "blocks")
		if !ok {
			return "", false
		}
		return v.blocks(obj["blocks"], false, "\n\n")
	default:
		return "", false
	}
}

func (v *validator) string(value any, min, max int) (string, bool) {
	result, ok := stringValue(value, min, max)
	v.characters += utf8.RuneCountInString(result)
	return result, ok && v.characters <= MaxText
}

func (v *validator) blocks(value any, inTable bool, separator string) (string, bool) {
	blocks, ok := array(value, 1, MaxNodes)
	if !ok {
		return "", false
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		part, ok := v.block(block, inTable)
		if !ok {
			return "", false
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, separator), true
}

func (v *validator) block(value any, inTable bool) (string, bool) {
	v.nodes++
	if v.nodes > MaxNodes {
		return "", false
	}
	switch kind(value, typeKey) {
	case paragraph, heading:
		return v.paragraph(value)
	case list:
		return v.list(value, inTable)
	case table:
		if inTable {
			return "", false
		}
		return v.table(value)
	case image:
		return v.asset(value, "alt", 1000)
	case audio:
		return v.asset(value, labelKey, 200)
	default:
		return "", false
	}
}

func (v *validator) paragraph(value any) (string, bool) {
	fields := []string{typeKey, contentKey}
	if kind(value, typeKey) == heading {
		fields = append(fields, "level")
	}
	obj, ok := object(value, fields...)
	if !ok {
		return "", false
	}
	if kind(value, typeKey) == heading {
		if _, ok := positiveInteger(obj["level"], 3); !ok {
			return "", false
		}
	}
	return v.inlines(obj[contentKey], false)
}

func (v *validator) list(value any, inTable bool) (string, bool) {
	obj, ok := object(value, typeKey, "ordered", "start", "items")
	ordered, boolean := obj["ordered"].(bool)
	start, number := positiveInteger(obj["start"], 9999)
	items, sequence := array(obj["items"], 1, MaxNodes)
	if !ok || !boolean || !number || !sequence || !ordered && start != 1 {
		return "", false
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		blocks, valid := array(item, 1, MaxNodes)
		if !valid || kind(blocks[0], typeKey) != paragraph {
			return "", false
		}
		part, valid := v.blocks(item, inTable, "\n")
		if !valid {
			return "", false
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n"), true
}

func (v *validator) asset(value any, field string, max int) (string, bool) {
	obj, ok := object(value, typeKey, assetKey, field)
	id, stringID := obj[assetKey].(string)
	if !ok || !stringID || !assetID.MatchString(id) {
		return "", false
	}
	id = strings.ToLower(id)
	assetKind := kind(value, typeKey)
	if previous, exists := v.assets[id]; exists && previous != assetKind {
		return "", false
	}
	if _, exists := v.assets[id]; !exists {
		v.assets[id] = assetKind
		v.assetRefs = append(v.assetRefs, AssetReference{ID: id, Kind: assetKind})
	}
	return v.string(obj[field], 1, max)
}

func (v *validator) inlines(value any, textOnly bool) (string, bool) {
	min := 0
	if textOnly {
		min = 1
	}
	nodes, ok := array(value, min, MaxNodes)
	if !ok {
		return "", false
	}
	var out strings.Builder
	for _, node := range nodes {
		if textOnly && kind(node, typeKey) != text {
			return "", false
		}
		part, valid := v.inline(node)
		if !valid {
			return "", false
		}
		out.WriteString(part)
	}
	return out.String(), true
}

func (v *validator) inline(value any) (string, bool) {
	v.nodes++
	if v.nodes > MaxNodes {
		return "", false
	}
	switch kind(value, typeKey) {
	case text:
		obj, ok := object(value, typeKey, text, "marks")
		if !ok || !validMarks(obj["marks"]) {
			return "", false
		}
		return v.string(obj[text], 1, MaxText)
	case lineBreak:
		_, ok := object(value, typeKey)
		return "\n", ok
	case gap:
		return v.gap(value)
	case link:
		obj, ok := object(value, typeKey, "href", contentKey)
		href, isString := obj["href"].(string)
		if !ok || !isString || !safeURL(href) {
			return "", false
		}
		return v.inlines(obj[contentKey], true)
	default:
		return "", false
	}
}

func (v *validator) gap(value any) (string, bool) {
	obj, ok := object(value, typeKey, "id", labelKey)
	id, stringID := obj["id"].(string)
	if !ok || !stringID || !gapID.MatchString(id) || v.gaps[id] {
		return "", false
	}
	v.gaps[id] = true
	v.gapIDs = append(v.gapIDs, id)
	label, valid := v.string(obj[labelKey], 1, 32)
	return "[" + label + "]", valid
}

func validMarks(value any) bool {
	marks, ok := array(value, 0, 6)
	if !ok {
		return false
	}
	seen := make(map[string]bool)
	for _, mark := range marks {
		name, isString := mark.(string)
		if !isString || seen[name] {
			return false
		}
		switch name {
		case "bold", "italic", "underline", "strike", "superscript", "subscript":
			seen[name] = true
		default:
			return false
		}
	}
	return !seen["superscript"] || !seen["subscript"]
}
