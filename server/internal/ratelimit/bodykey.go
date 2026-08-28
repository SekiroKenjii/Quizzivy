package ratelimit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// JSONFieldKey derives a bucket key from one field of a JSON request body.
//
// §6.5 asks for limits keyed on more than the client address: per-email on
// login, per-code on the join endpoints. Both live in the body, and the body is
// a single-use stream -- so this buffers it and puts it back, or the handler
// would receive nothing.
//
// The read is BOUNDED. Slurping an unbounded body to extract one field would
// hand an attacker a memory-exhaustion vector on the very endpoints the limit
// exists to protect, which would be an unusually self-defeating design. Bodies
// larger than the cap simply produce no key, so the per-IP limit still applies
// and the handler still sees the full body.
//
// The value is lowercased and trimmed so "A@x.com " and "a@x.com" share a
// bucket; otherwise case alone buys a fresh allowance.
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
		// Whatever happens next, the handler must still be able to read the
		// body it would have received.
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
