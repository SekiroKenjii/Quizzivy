package http

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	geoLabelKey apiCtxKey = 1

	cloudflareClientIPHeader = "CF-Connecting-IP"
	cloudflareCityHeader     = "CF-IPCity"
	cloudflareCountryHeader  = "CF-IPCountry"

	maxCityRunes = 70
)

// WithGeoLabel records where a request came from, as "City, CC", for the
// handlers that start a session. The location headers are Cloudflare's, so it
// reads them only when clientIPHeader names CF-Connecting-IP, the header the
// server already trusts for the address, and only on a request that carries
// that header. Under any other configuration, and for a request that did not
// pass through Cloudflare, no label is recorded.
func WithGeoLabel(clientIPHeader string) func(http.Handler) http.Handler {
	trusted := strings.EqualFold(strings.TrimSpace(clientIPHeader), cloudflareClientIPHeader)
	return func(next http.Handler) http.Handler {
		if !trusted {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.TrimSpace(r.Header.Get(cloudflareClientIPHeader)) == "" {
				next.ServeHTTP(w, r)
				return
			}
			label := geoLabel(r.Header.Get(cloudflareCityHeader), r.Header.Get(cloudflareCountryHeader))
			if label == "" {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), geoLabelKey, label)))
		})
	}
}

// GeoLabelFromContext is the label WithGeoLabel recorded, or "" for none.
func GeoLabelFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(geoLabelKey).(string); ok {
		return v
	}
	return ""
}

func geoLabel(city, country string) string {
	city, country = cleanCity(city), cleanCountry(country)
	switch {
	case city != "" && country != "":
		return city + ", " + country
	case city != "":
		return city
	default:
		return country
	}
}

func cleanCity(raw string) string {
	if decoded, err := url.PathUnescape(raw); err == nil {
		raw = decoded
	}
	if !utf8.ValidString(raw) {
		return ""
	}
	printable := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case unicode.IsPrint(r):
			return r
		default:
			return -1
		}
	}, raw)
	city := strings.Join(strings.Fields(printable), " ")
	if runes := []rune(city); len(runes) > maxCityRunes {
		city = strings.TrimSpace(string(runes[:maxCityRunes]))
	}
	return city
}

func cleanCountry(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if len(raw) != 2 || raw == "XX" {
		return ""
	}
	for i := range len(raw) {
		if raw[i] < 'A' || raw[i] > 'Z' {
			return ""
		}
	}
	return raw
}
