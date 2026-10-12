//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
)

type resourceSlot struct {
	In   string `json:"in"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type scopedOp struct {
	id       string
	method   string
	path     string
	slots    []resourceSlot
	listing  string
	optional map[string]bool
}

func scopedOperations(t *testing.T) []scopedOp {
	t.Helper()
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	var out []scopedOp
	for path, item := range spec.Paths.Map() {
		if !strings.HasPrefix(path, "/teacher/") && !strings.HasPrefix(path, "/app/") && !strings.HasPrefix(path, "/me/") {
			continue
		}
		for method, op := range item.Operations() {
			s := scopedOp{id: strings.ToLower(op.OperationID[:1]) + op.OperationID[1:], method: method, path: path, optional: map[string]bool{}}
			for _, p := range append(item.Parameters, op.Parameters...) {
				if p.Value.In == "query" && !p.Value.Required {
					s.optional[p.Value.Name] = true
				}
			}
			if raw, ok := op.Extensions["x-resource"]; ok {
				encoded, _ := json.Marshal(raw)
				if err := json.Unmarshal(encoded, &s.slots); err != nil {
					t.Fatalf("%s: x-resource: %v", s.id, err)
				}
			}
			s.listing, _ = op.Extensions["x-resource-list"].(string)
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

type isoCase struct {
	student bool
	params  map[string]string
	query   map[string]string
	body    func(x *iso, own *party, slot string) any
	format  string
	fresh   func(x *iso, own *party) *party
	link    map[string][]string
	as      map[string]string
	excuse  map[string]string
	blind   map[string]string
}

type iso struct {
	t      *testing.T
	w      *world
	a, b   *party
	admin  *client
	cases  map[string]isoCase
	shared string
	made   []string
}

func setPointer(t *testing.T, root any, pointer string, value string, must bool) (any, bool) {
	t.Helper()
	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	var walk func(node any, i int) (any, bool)
	walk = func(node any, i int) (any, bool) {
		part := strings.NewReplacer("~1", "/", "~0", "~").Replace(parts[i])
		last := i == len(parts)-1
		switch part {
		case "+":
			m, ok := node.(map[string]any)
			if !ok || len(m) == 0 || !last {
				return node, false
			}
			for _, v := range m {
				return map[string]any{value: v}, true
			}
		case "-":
			arr, ok := node.([]any)
			if !ok {
				return node, false
			}
			if last {
				return []any{value}, true
			}
			if len(arr) == 0 {
				return node, false
			}
			next, ok := walk(arr[0], i+1)
			arr[0] = next
			return arr, ok
		case "*":
			m, ok := node.(map[string]any)
			if !ok || len(m) == 0 {
				return node, false
			}
			for k, v := range m {
				if last {
					m[k] = value
					return m, true
				}
				next, ok := walk(v, i+1)
				m[k] = next
				return m, ok
			}
		}
		m, ok := node.(map[string]any)
		if !ok {
			return node, false
		}
		if last {
			if _, present := m[part]; !present {
				return node, false
			}
			m[part] = value
			return m, true
		}
		child, present := m[part]
		if !present {
			return node, false
		}
		next, ok := walk(child, i+1)
		m[part] = next
		return m, ok
	}
	out, ok := walk(root, 0)
	if !ok && must {
		t.Fatalf("the base body has no place for %s", pointer)
	}
	return out, ok
}

func normalized(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func (c isoCase) key(slot, kind string) string {
	if k, ok := c.as[slot]; ok {
		return k
	}
	return kind
}

func (c isoCase) excused(slot string) string {
	if r, ok := c.excuse[slot]; ok {
		return r
	}
	return c.excuse["*"]
}

func (x *iso) request(op scopedOp, c isoCase, own *party, slot string, value string) (string, string, *payload) {
	return x.build(op, c, own, slot, value, false)
}

func (x *iso) build(op scopedOp, c isoCase, own *party, slot string, value string, bare bool) (string, string, *payload) {
	x.t.Helper()
	values := map[string]string{}
	for _, s := range op.slots {
		key := s.In + " " + s.Name
		switch {
		case key == slot:
			values[key] = value
		case s.Kind == "none":
			values[key] = uuid.NewString()
		default:
			values[key] = own.id(c.key(key, s.Kind))
		}
	}
	path := op.path
	query := url.Values{}
	for k, v := range c.query {
		query.Set(k, v)
	}
	for _, s := range op.slots {
		switch s.In {
		case "path":
			path = strings.ReplaceAll(path, "{"+s.Name+"}", values[s.In+" "+s.Name])
		case "query":
			if !bare || !op.optional[s.Name] {
				query.Set(s.Name, values[s.In+" "+s.Name])
			}
		}
	}
	for k, v := range c.params {
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}
	if strings.Contains(path, "{") {
		x.t.Fatalf("%s: path %s still has a parameter", op.id, path)
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	if c.format == "png" {
		return op.method, path, new(filePayload(x.t, "thay.png", tinyPNG(x.t)))
	}
	if c.format == "avatar" {
		return op.method, path, new(filePayload(x.t, "chan-dung.png", avatarPNG(x.t)))
	}
	if c.format == "docx" {
		return op.method, path, new(filePayload(x.t, "de.docx", tinyDocx(x.t)))
	}
	if c.body == nil {
		return op.method, path, nil
	}
	name := ""
	if strings.HasPrefix(slot, "body ") {
		name = strings.TrimPrefix(slot, "body ")
	}
	body := normalized(c.body(x, own, name))
	for _, s := range op.slots {
		key := s.In + " " + s.Name
		present := s.In != "body"
		if s.In == "body" && s.Kind != "none" {
			body, present = setPointer(x.t, body, s.Name, values[key], s.Name == name)
		}
		if !present {
			continue
		}
		for _, linked := range c.link[key] {
			body, _ = setPointer(x.t, body, linked, values[key], key == slot)
		}
	}
	return op.method, path, new(jsonPayload(x.t, body))
}

var (
	uuidPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	timePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
)

func answer(s sent) string {
	if s.json == nil {
		return fmt.Sprintf("%d %d bytes", s.status, len(s.body))
	}
	body := any(s.json)
	if e, ok := s.json["error"].(map[string]any); ok {
		delete(e, "requestId")
		body = e
	}
	raw, _ := json.Marshal(body)
	masked := timePattern.ReplaceAllString(uuidPattern.ReplaceAllString(string(raw), "<id>"), "<time>")
	return fmt.Sprintf("%d %s", s.status, masked)
}

func success(s sent) bool { return s.status >= 200 && s.status < 300 }

func (x *iso) substitute(op scopedOp, c isoCase) {
	x.t.Helper()
	caller := func(p *party) *party {
		if c.student {
			return p.student
		}
		return p
	}
	for _, s := range op.slots {
		if s.Kind == "none" {
			continue
		}
		slot := s.In + " " + s.Name
		where := op.id + " " + slot
		key := c.key(slot, s.Kind)
		own := caller(x.b)
		if c.fresh != nil {
			own = c.fresh(x, own)
			x.made = append(x.made, own.all()...)
		}
		foreign := caller(x.a)
		theirID, missingID := foreign.id(key), uuid.NewString()
		method, path, body := x.request(op, c, own, slot, theirID)
		theirs := own.c.send(method, path, body)
		if c.fresh != nil {
			own = c.fresh(x, caller(x.b))
			x.made = append(x.made, own.all()...)
		}
		method, path, body = x.request(op, c, own, slot, missingID)
		missing := own.c.send(method, path, body)
		excused := c.excused(slot)
		if success(missing) && excused == "" {
			x.t.Errorf("%s: a missing %s answered %s: the case never reaches the check", where, s.Kind, answer(missing))
			continue
		}
		if answer(theirs) != answer(missing) {
			x.t.Errorf("%s: another owner's %s answered\n  %s\nwhere a missing one answers\n  %s", where, s.Kind, answer(theirs), answer(missing))
		}
		if leaked := mentions([]byte(strings.ReplaceAll(string(theirs.body), theirID, "")), foreign); len(leaked) > 0 {
			x.t.Errorf("%s: the answer to another owner's %s names their %v", where, s.Kind, leaked)
		}
		if c.blind[slot] != "" {
			continue
		}
		if c.fresh != nil {
			own = c.fresh(x, caller(x.b))
			x.made = append(x.made, own.all()...)
		}
		method, path, body = x.request(op, c, own, slot, own.id(key))
		mine := own.c.send(method, path, body)
		if answer(mine) == answer(missing) {
			x.t.Errorf("%s: the caller's own %s answers as a missing one does (%s): the case proves nothing", where, s.Kind, answer(mine))
		}
	}
}
