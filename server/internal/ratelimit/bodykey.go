package ratelimit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// JSONFieldKey builds a KeyFunc that buckets on one field of a JSON body,
// reading at most maxBytes and restoring the body for the handler.
//
// Callers pass a normaliser so that two spellings of the same value cannot buy
// two buckets.
func JSONFieldKey(field string, maxBytes int64) KeyFunc {
	return JSONFieldKeyFunc(field, maxBytes, nil)
}

// JSONFieldKeyFunc is JSONFieldKey with a caller-supplied canonicaliser.
//
// Lowercasing is enough for an email and NOT enough for a join code. A code is
// accepted "with or without the dash and in any case" (§6.1), so `K7M3-P9QR`
// and `k7m3p9qr` are one code -- and keyed on the raw value they are two
// buckets, which hands an attacker a fresh allowance for every spelling of the
// same secret. The key has to be whatever the LOOKUP will canonicalise to, not
// whatever was typed.
func JSONFieldKeyFunc(field string, maxBytes int64, canonical func(string) string) KeyFunc {
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
		if err := json.Unmarshal(buf, &fields); err != nil {
			return ""
		}
		raw, ok := fields[field]
		if !ok {
			return ""
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return ""
		}
		if canonical != nil {
			return canonical(value)
		}
		return strings.ToLower(strings.TrimSpace(value))
	}
}
