package ratelimit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// JSONFieldKey builds a KeyFunc that buckets on one string field of a JSON
// body, reading at most maxBytes and restoring the body for the handler. It
// decodes the first JSON value as the handler does, so trailing bytes cannot
// hide the field. A body over maxBytes, or a value longer than maxRunes, yields
// no key: the route's body limit and its schema refuse such a request before
// any handler runs.
func JSONFieldKey(field string, maxBytes int64, maxRunes int) KeyFunc {
	return JSONFieldKeyFunc(field, maxBytes, maxRunes, nil)
}

// JSONFieldKeyFunc is JSONFieldKey with a caller-supplied canonicaliser.
func JSONFieldKeyFunc(field string, maxBytes int64, maxRunes int, canonical func(string) string) KeyFunc {
	return func(r *http.Request) string {
		if r.Body == nil {
			return ""
		}

		limited := io.LimitReader(r.Body, maxBytes+1)
		buf, err := io.ReadAll(limited)
		r.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(buf), r.Body), r.Body}

		if err != nil || int64(len(buf)) > maxBytes {
			return ""
		}

		var fields map[string]json.RawMessage
		if err := json.NewDecoder(bytes.NewReader(buf)).Decode(&fields); err != nil {
			return ""
		}
		raw, ok := fields[field]
		if !ok {
			return ""
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil || utf8.RuneCountInString(value) > maxRunes {
			return ""
		}
		if canonical != nil {
			return canonical(value)
		}
		return strings.ToLower(strings.TrimSpace(value))
	}
}
