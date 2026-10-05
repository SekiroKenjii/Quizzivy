package content

import (
	"bytes"
	"encoding/json"
	"strings"
)

// WithAssetIDs returns an independently validated copy with matching asset identities replaced.
func (d Document) WithAssetIDs(ids map[string]string) (Document, error) {
	normalized := make(map[string]string, len(ids))
	for old, newID := range ids {
		if !assetID.MatchString(old) || !assetID.MatchString(newID) {
			return Document{}, ErrInvalidDocument
		}
		key := strings.ToLower(old)
		if existing, ok := normalized[key]; ok && existing != newID {
			return Document{}, ErrInvalidDocument
		}
		normalized[key] = newID
	}
	var value any
	if err := json.Unmarshal(d.data, &value); err != nil {
		return Document{}, ErrInvalidDocument
	}
	rebindAssets(value, normalized)
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return Document{}, ErrInvalidDocument
	}
	return Parse(bytes.TrimSpace(out.Bytes()))
}
func rebindAssets(value any, ids map[string]string) {
	switch node := value.(type) {
	case map[string]any:
		if node[typeKey] == image || node[typeKey] == audio {
			old, _ := node[assetKey].(string)
			if id, ok := ids[strings.ToLower(old)]; ok {
				node[assetKey] = id
			}
		}
		for _, child := range node {
			rebindAssets(child, ids)
		}
	case []any:
		for _, child := range node {
			rebindAssets(child, ids)
		}
	}
}
