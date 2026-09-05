package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// KeyFunc derives a bucket key from a request. Returning "" skips that bucket.
type KeyFunc func(*http.Request) string

// Route is the limiting policy for one operation.
type Route struct {
	// Per-IP is always present on a public route (§6.5).
	PerIP  *Limiter
	Key    KeyFunc
	PerKey *Limiter
}

// Registry maps a Go 1.22 mux pattern ("POST /join/preview") to its policy.
//
// Keyed by pattern rather than by concrete URL so `/admin/tests/{id}` is one
// entry, and so it lines up with the OpenAPI paths the startup assertion reads.
type Registry struct {
	routes map[string]*Route
}

func NewRegistry() *Registry {
	return &Registry{routes: make(map[string]*Route)}
}

// Add registers a per-IP policy. `pattern` is "METHOD /path".
func (reg *Registry) Add(pattern string, capacity int, rules ...Rule) *Route {
	route := &Route{PerIP: New(capacity, rules...)}
	reg.routes[normalize(pattern)] = route
	return route
}

// WithKey attaches the secondary bucket.
func (r *Route) WithKey(key KeyFunc, capacity int, rules ...Rule) *Route {
	r.Key = key
	r.PerKey = New(capacity, rules...)
	return r
}

func (reg *Registry) Lookup(pattern string) (*Route, bool) {
	route, ok := reg.routes[normalize(pattern)]
	return route, ok
}

// Patterns lists everything registered. Used by the startup assertion.
func (reg *Registry) Patterns() []string {
	out := make([]string, 0, len(reg.routes))
	for k := range reg.routes {
		out = append(out, k)
	}
	return out
}

func normalize(pattern string) string {
	return strings.Join(strings.Fields(pattern), " ")
}

// ClientIP derives the per-IP bucket key from one named header, falling back to
// RemoteAddr.
//
// The header is named explicitly and X-Forwarded-For is rejected in config: a
// proxy appends to it, so the client controls the first entry and therefore its
// own bucket. CF-Connecting-IP is overwritten on every request.
func ClientIP(header string) KeyFunc {
	header = strings.TrimSpace(header)
	return func(r *http.Request) string {
		if header != "" {
			if v := strings.TrimSpace(r.Header.Get(header)); v != "" {
				return v
			}
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}

// Common windows from §6.5.
var (
	PerMinute = func(n int) Rule { return Rule{Burst: n, Window: time.Minute} }
	PerHour   = func(n int) Rule { return Rule{Burst: n, Window: time.Hour} }
)
