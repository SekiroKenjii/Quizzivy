package content

import (
	"fmt"
	"unicode/utf8"

	"quizzivy/internal/shared/validation"
)

// Composer composes the plain text of one write to NFC and keeps the fields that composing left longer than their limit.
// NFC shortens Vietnamese and most Latin text, but a few scripts grow when composed, so a value typed at its limit can
// pass the contract and still be refused by a stored limit; Err names those fields the way any other refused field is named.
type Composer struct {
	invalid validation.Error
}

// Text returns s composed. A limit of zero means the field has none; otherwise a composed s longer than limit characters is
// recorded against field and s is returned as it was typed.
func (c *Composer) Text(field, s string, limit int) string {
	composed := NFC(s)
	if limit > 0 && utf8.RuneCountInString(composed) > limit {
		c.invalid.Add(field, fmt.Sprintf("Nội dung không được dài quá %d ký tự.", limit))
		return s
	}
	return composed
}

// Optional is Text for a field that may be absent.
func (c *Composer) Optional(field string, s *string, limit int) *string {
	if s == nil {
		return nil
	}
	composed := c.Text(field, *s, limit)
	return &composed
}

// Each is Text for every element of a list; an element's field is field[i].
func (c *Composer) Each(field string, values []string, limit int) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = c.Text(fmt.Sprintf("%s[%d]", field, i), value, limit)
	}
	return out
}

// Refuse records field as refused with message, for a refusal the caller found itself.
func (c *Composer) Refuse(field, message string) { c.invalid.Add(field, message) }

// Err is the *validation.Error holding every refused field, or nil when there is none.
func (c *Composer) Err() error {
	if invalid := c.invalid.OrNil(); invalid != nil {
		return invalid
	}
	return nil
}
