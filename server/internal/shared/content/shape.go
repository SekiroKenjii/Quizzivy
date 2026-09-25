package content

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	paragraph  = "paragraph"
	heading    = "heading"
	text       = "text"
	gap        = "gap"
	link       = "link"
	lineBreak  = "break"
	list       = "list"
	table      = "table"
	image      = "image"
	audio      = "audio"
	contentKey = "content"
	typeKey    = "type"
	labelKey   = "label"
	assetKey   = "assetId"
)

var (
	gapID     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	assetID   = regexp.MustCompile(`^([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}|00000000-0000-0000-0000-000000000000|ffffffff-ffff-ffff-ffff-ffffffffffff)$`)
	hostLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
)

func object(value any, fields ...string) (map[string]any, bool) {
	v, ok := value.(map[string]any)
	if !ok || len(v) != len(fields) {
		return nil, false
	}
	for _, key := range fields {
		if _, exists := v[key]; !exists {
			return nil, false
		}
	}
	return v, true
}

func kind(value any, key string) string {
	v, _ := value.(map[string]any)
	s, _ := v[key].(string)
	return s
}

func array(value any, min, max int) ([]any, bool) {
	v, ok := value.([]any)
	return v, ok && len(v) >= min && len(v) <= max
}

func stringValue(value any, min, max int) (string, bool) {
	v, ok := value.(string)
	n := utf8.RuneCountInString(v)
	return v, ok && n >= min && n <= max
}

func positiveInteger(value any, max int) (int, bool) {
	v, ok := value.(float64)
	return int(v), ok && v >= 1 && v <= float64(max) && math.Trunc(v) == v
}

func safeURL(value string) bool {
	if utf8.RuneCountInString(value) > MaxURL || len(value) < 8 || !strings.EqualFold(value[:8], "https://") {
		return false
	}
	for _, r := range value {
		if r == '\\' || r == 0xfeff || unicode.IsSpace(r) || r < 32 || r >= 127 && r <= 159 {
			return false
		}
	}
	authority := value[8:]
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	parts := strings.Split(authority, ":")
	if len(parts) > 2 || !validHost(parts[0]) {
		return false
	}
	if len(parts) == 2 && !validPort(parts[1]) {
		return false
	}
	return validPercentEncoding(value)
}

func validHost(host string) bool {
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !hostLabel.MatchString(label) {
			return false
		}
	}
	return true
}

func validPort(port string) bool {
	if len(port) < 1 || len(port) > 5 {
		return false
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

func validPercentEncoding(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			continue
		}
		if i+3 > len(value) {
			return false
		}
		if _, err := strconv.ParseUint(value[i+1:i+3], 16, 8); err != nil {
			return false
		}
		i += 2
	}
	return true
}
