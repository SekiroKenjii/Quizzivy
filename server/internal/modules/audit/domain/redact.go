package domain

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	hiddenValue    = "••••"
	hintPrefix     = hiddenValue + "-"
	hintLength     = 4
	shortestHinted = 8
)

var keySeparators = strings.NewReplacer("_", "", "-", "")

var hintedKeys = map[string]bool{"code": true, "joincode": true, "codehash": true}

var droppedKeys = map[string]bool{
	"password": true, "temporarypassword": true, "currentpassword": true, "newpassword": true,
	"oldpassword": true, "passwordhash": true, "token": true, "accesstoken": true,
	"refreshtoken": true, "secret": true, "clientsecret": true, "apikey": true,
}

// Change is one field an entry changed, with the value it had and the value it
// has. Either may be nil, which is JSON null.
type Change struct {
	Field string
	From  any
	To    any
}

// Presentation is what a stored diff shows: the fields it changed, and
// whatever else it recorded.
type Presentation struct {
	// Changes lists, in field order, each key whose value is {"old": …, "new": …}.
	Changes []Change
	// Details holds every other key, or nil when there is none.
	Details map[string]any
}

// Present reads diff, the JSON an entry was stored with, for a screen or a
// file. A key whose value is an object of exactly old and new is a change;
// every other key is a detail, and a diff that is not an object is the one
// detail "value". A key named code, joinCode or code_hash, at any depth, is
// shown as "••••-" and its last four characters, or as "••••" when its value
// is short or is not text; a key named password, temporaryPassword, token or
// secret, or one of their near variants, is left out, whatever was stored.
// Key names are compared without case, underscores and hyphens.
func Present(diff []byte) Presentation {
	out := Presentation{Changes: []Change{}}
	diff = bytes.TrimSpace(diff)
	if len(diff) == 0 || bytes.Equal(diff, []byte("null")) {
		return out
	}
	decoder := json.NewDecoder(bytes.NewReader(diff))
	decoder.UseNumber()
	var stored any
	if err := decoder.Decode(&stored); err != nil {
		return out
	}
	object, ok := stored.(map[string]any)
	if !ok {
		out.Details = map[string]any{"value": shown("", stored)}
		return out
	}
	for _, key := range sortedKeys(object) {
		if isDropped(key) {
			continue
		}
		if before, after, paired := pairOf(object[key]); paired {
			out.Changes = append(out.Changes, Change{Field: key, From: shown(key, before), To: shown(key, after)})
			continue
		}
		if out.Details == nil {
			out.Details = map[string]any{}
		}
		out.Details[key] = shown(key, object[key])
	}
	return out
}

func pairOf(v any) (before, after any, ok bool) {
	object, isObject := v.(map[string]any)
	if !isObject || len(object) != 2 {
		return nil, nil, false
	}
	before, hasOld := object["old"]
	after, hasNew := object["new"]
	return before, after, hasOld && hasNew
}

func shown(key string, v any) any {
	if hintedKeys[normalise(key)] {
		return hinted(v)
	}
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			if !isDropped(k) {
				out[k] = shown(k, child)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = shown("", child)
		}
		return out
	}
	return v
}

func hinted(v any) any {
	text, isText := v.(string)
	switch {
	case v == nil:
		return nil
	case !isText || utf8.RuneCountInString(text) < shortestHinted:
		return hiddenValue
	}
	runes := []rune(text)
	return hintPrefix + string(runes[len(runes)-hintLength:])
}

func isDropped(key string) bool { return droppedKeys[normalise(key)] }

func normalise(key string) string { return keySeparators.Replace(strings.ToLower(key)) }

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
