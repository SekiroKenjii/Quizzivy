package ratelimit

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// KeyFunc derives a bucket key from a request. Returning "" skips that bucket.
type KeyFunc func(*http.Request) string

// Route is the limiting policy for one operation: the per-address bucket every
// public route has (§6.5), then its keyed buckets in the order they were
// added. A request is refused by the first bucket that is exhausted. PerIP is
// nil on a route AddKeyed registered, which is limited by its keyed buckets
// alone.
type Route struct {
	PerIP *Limiter
	Keyed []Keyed
}

// Keyed is one keyed bucket. Name is the bucket's name in the contract's
// x-rate-limit block; a Key that yields "" skips the bucket for that request.
type Keyed struct {
	Name    string
	Key     KeyFunc
	Limiter *Limiter
}

// Registry maps a Go 1.22 mux pattern ("POST /join/preview") to its policy.
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

// AddKeyed registers a policy with no per-address bucket, for the keyed
// buckets WithKey then adds. `pattern` is "METHOD /path".
func (reg *Registry) AddKeyed(pattern string) *Route {
	route := &Route{}
	reg.routes[normalize(pattern)] = route
	return route
}

// WithKey adds a keyed bucket after those already on the route.
func (r *Route) WithKey(name string, key KeyFunc, capacity int, rules ...Rule) *Route {
	r.Keyed = append(r.Keyed, Keyed{Name: name, Key: key, Limiter: New(capacity, rules...)})
	return r
}

// Compose joins several keys into one bucket key, and yields "" when any part
// is "", so a composite bucket is skipped whenever a part is missing.
func Compose(keys ...KeyFunc) KeyFunc {
	return func(r *http.Request) string {
		parts := make([]string, len(keys))
		for i, key := range keys {
			if parts[i] = key(r); parts[i] == "" {
				return ""
			}
		}
		return strings.Join(parts, "\x00")
	}
}

type addressKey struct{}

// WithAddress records on the request the client address the rate limiter
// resolved, for Address to read back inside a composite key.
func WithAddress(r *http.Request, address string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), addressKey{}, address))
}

// Address is the KeyFunc for the client address WithAddress recorded, or ""
// when none was.
func Address(r *http.Request) string {
	address, _ := r.Context().Value(addressKey{}).(string)
	return address
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
