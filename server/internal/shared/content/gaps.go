package content

import "encoding/json"

// WithGapIDs returns an independently validated copy with exactly the supplied gap identities.
func (d Document) WithGapIDs(ids map[string]string) (Document, error) {
	if len(ids) != len(d.gaps) {
		return Document{}, ErrInvalidDocument
	}
	for _, id := range d.gaps {
		if !gapID.MatchString(ids[id]) {
			return Document{}, ErrInvalidDocument
		}
	}
	var value any
	if err := json.Unmarshal(d.data, &value); err != nil {
		return Document{}, ErrInvalidDocument
	}
	rebindGaps(value, ids)
	raw, err := json.Marshal(value)
	if err != nil {
		return Document{}, ErrInvalidDocument
	}
	return Parse(raw)
}

func rebindGaps(value any, ids map[string]string) {
	switch node := value.(type) {
	case map[string]any:
		if node["type"] == "gap" {
			id, _ := node["id"].(string)
			node["id"] = ids[id]
			return
		}
		for _, child := range node {
			rebindGaps(child, ids)
		}
	case []any:
		for _, child := range node {
			rebindGaps(child, ids)
		}
	}
}
