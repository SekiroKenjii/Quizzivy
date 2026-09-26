package content

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type decoder struct {
	json       *json.Decoder
	values     int
	characters int
}

func decode(raw []byte) (any, bool) {
	if len(raw) > MaxBytes || !utf8.Valid(raw) || !validEscapes(raw) {
		return nil, false
	}
	d := decoder{json: json.NewDecoder(bytes.NewReader(raw))}
	value, ok := d.value(0)
	if !ok {
		return nil, false
	}
	_, err := d.json.Token()
	return value, err == io.EOF
}

func validEscapes(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) || raw[i] != 'u' {
			continue
		}
		end, ok := escapeEnd(raw, i)
		if !ok {
			return false
		}
		i = end
	}
	return true
}

func escapeEnd(raw []byte, start int) (int, bool) {
	r, ok := escapedRune(raw, start)
	if !ok || r >= 0xdc00 && r <= 0xdfff {
		return 0, false
	}
	if r < 0xd800 || r > 0xdbff {
		return start + 4, true
	}
	if start+6 >= len(raw) || raw[start+5] != '\\' || raw[start+6] != 'u' {
		return 0, false
	}
	low, valid := escapedRune(raw, start+6)
	return start + 10, valid && low >= 0xdc00 && low <= 0xdfff
}

func escapedRune(raw []byte, start int) (uint64, bool) {
	if start+5 > len(raw) {
		return 0, false
	}
	r, err := strconv.ParseUint(string(raw[start+1:start+5]), 16, 16)
	return r, err == nil
}

func (d *decoder) value(depth int) (any, bool) {
	d.values++
	if depth > MaxDepth || d.values > MaxValues {
		return nil, false
	}
	token, err := d.json.Token()
	if err != nil {
		return nil, false
	}
	switch t := token.(type) {
	case json.Delim:
		switch t {
		case '{':
			return d.object(depth)
		case '[':
			return d.array(depth)
		default:
			return nil, false
		}
	case string:
		d.characters += utf8.RuneCountInString(t)
		return t, d.characters <= MaxStrings && !strings.ContainsRune(t, 0)
	default:
		return token, true
	}
}

func (d *decoder) object(depth int) (any, bool) {
	result := make(map[string]any)
	for d.json.More() {
		token, err := d.json.Token()
		key, ok := token.(string)
		if err != nil || !ok || len(result) >= MaxNodes {
			return nil, false
		}
		if _, duplicate := result[key]; duplicate {
			return nil, false
		}
		value, ok := d.value(depth + 1)
		if !ok {
			return nil, false
		}
		result[key] = value
	}
	end, err := d.json.Token()
	return result, err == nil && end == json.Delim('}')
}

func (d *decoder) array(depth int) (any, bool) {
	result := make([]any, 0)
	for d.json.More() {
		if len(result) >= MaxNodes {
			return nil, false
		}
		value, ok := d.value(depth + 1)
		if !ok {
			return nil, false
		}
		result = append(result, value)
	}
	end, err := d.json.Token()
	return result, err == nil && end == json.Delim(']')
}
