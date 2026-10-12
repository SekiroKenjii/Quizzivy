package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

var nulEscape = []byte(`\u0000`)

func hasNUL(s string) bool {
	return strings.IndexByte(s, 0) >= 0
}

func nulSentence(r *http.Request) string {
	return TextFor(r, "Không được chứa ký tự NUL (U+0000).", "Must not contain the NUL character (U+0000).")
}

func writeNULInField(w http.ResponseWriter, r *http.Request, field string) {
	if field == "" {
		WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, genericSentence(r))
		return
	}
	WriteErrorWithDetails(w, r, http.StatusBadRequest, CodeValidationFailed, fieldSentence(r, field),
		map[string]any{field: nulSentence(r)})
}

func writeNULInParameter(w http.ResponseWriter, r *http.Request, name string) {
	if name == "" {
		WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, genericSentence(r))
		return
	}
	WriteErrorWithDetails(w, r, http.StatusBadRequest, CodeValidationFailed, parameterSentence(r, name),
		map[string]any{name: nulSentence(r)})
}

func withoutNULInURL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, found := nulInQuery(r); found {
			writeNULInParameter(w, r, name)
			return
		}
		if hasNUL(r.URL.Path) {
			WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, genericSentence(r))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func nulInQuery(r *http.Request) (name string, found bool) {
	if !strings.Contains(r.URL.RawQuery, "%00") {
		return "", false
	}
	query := r.URL.Query()
	for key := range query {
		if hasNUL(key) {
			return "", true
		}
	}
	var names []string
	for key, values := range query {
		if slices.ContainsFunc(values, hasNUL) {
			names = append(names, key)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	return slices.Min(names), true
}

func nulInBody(body []byte) (field string, found bool) {
	if !bytes.Contains(body, nulEscape) {
		return "", false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var walk nulWalk
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", false
		}
		located, hit, done := walk.read(token)
		if hit {
			return located, true
		}
		if done {
			return "", false
		}
	}
}

type nulWalk struct {
	frames []nulFrame
}

type nulFrame struct {
	object  bool
	key     string
	reading bool
	index   int
}

func (w *nulWalk) read(token json.Token) (field string, found, done bool) {
	switch value := token.(type) {
	case json.Delim:
		if value == '{' || value == '[' {
			w.frames = append(w.frames, nulFrame{object: value == '{'})
			return "", false, false
		}
		w.frames = w.frames[:len(w.frames)-1]
	case string:
		if w.expectsKey() {
			if hasNUL(value) {
				return w.path(len(w.frames) - 1), true, false
			}
			top := &w.frames[len(w.frames)-1]
			top.key, top.reading = value, true
			return "", false, false
		}
		if hasNUL(value) {
			return w.path(len(w.frames)), true, false
		}
	}
	return "", false, w.completes()
}

func (w *nulWalk) expectsKey() bool {
	if len(w.frames) == 0 {
		return false
	}
	top := w.frames[len(w.frames)-1]
	return top.object && !top.reading
}

func (w *nulWalk) completes() (done bool) {
	if len(w.frames) == 0 {
		return true
	}
	top := &w.frames[len(w.frames)-1]
	if top.object {
		top.reading = false
	} else {
		top.index++
	}
	return false
}

func (w *nulWalk) path(depth int) string {
	parts := make([]string, 0, depth)
	for _, frame := range w.frames[:depth] {
		if frame.object {
			parts = append(parts, frame.key)
		} else {
			parts = append(parts, strconv.Itoa(frame.index))
		}
	}
	return strings.Join(parts, ".")
}
