package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	identityhttp "quizzivy/internal/modules/identity/http"
)

func labelFor(t *testing.T, configured string, headers map[string]string) string {
	t.Helper()
	var got string
	handler := identityhttp.WithGeoLabel(configured)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = identityhttp.GeoLabelFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if n := utf8.RuneCountInString(got); n > 80 || !utf8.ValidString(got) {
		t.Fatalf("label %q is %d runes or invalid UTF-8, want valid and at most 80", got, n)
	}
	return got
}

func cloudflare(city, country string) map[string]string {
	return map[string]string{"CF-Connecting-IP": "203.0.113.9", "CF-IPCity": city, "CF-IPCountry": country}
}

func TestTheLabelIsBuiltOnlyWhenTheClientAddressComesFromCloudflare(t *testing.T) {
	for _, configured := range []string{"CF-Connecting-IP", "cf-connecting-ip", "  CF-Connecting-IP "} {
		if got := labelFor(t, configured, cloudflare("Ho Chi Minh City", "vn")); got != "Ho Chi Minh City, VN" {
			t.Errorf("configured %q: label = %q, want Ho Chi Minh City, VN", configured, got)
		}
	}
	for _, configured := range []string{"", "Fly-Client-IP", "X-Real-IP", "X-Forwarded-For"} {
		if got := labelFor(t, configured, cloudflare("Ho Chi Minh City", "VN")); got != "" {
			t.Errorf("configured %q: label = %q, want none: the location headers are trusted only behind Cloudflare", configured, got)
		}
	}
}

func TestARequestThatDidNotPassThroughCloudflareHasNoLabel(t *testing.T) {
	headers := map[string]string{"CF-IPCity": "Hanoi", "CF-IPCountry": "VN", "X-Forwarded-For": "198.51.100.1, 203.0.113.9"}
	if got := labelFor(t, "CF-Connecting-IP", headers); got != "" {
		t.Errorf("label = %q, want none without CF-Connecting-IP on the request", got)
	}
	headers["CF-Connecting-IP"] = "   "
	if got := labelFor(t, "CF-Connecting-IP", headers); got != "" {
		t.Errorf("label = %q, want none for a blank CF-Connecting-IP", got)
	}
}

func TestTheLabelNamesTheCityAndCountryItHas(t *testing.T) {
	cases := []struct {
		name          string
		city, country string
		want          string
	}{
		{"both", "Da Nang", "VN", "Da Nang, VN"},
		{"the country in lower case", "Hue", "vn", "Hue, VN"},
		{"the city alone", "Hanoi", "", "Hanoi"},
		{"the country alone", "", "VN", "VN"},
		{"neither", "", "", ""},
		{"an unknown country", "Hanoi", "XX", "Hanoi"},
		{"Tor", "", "T1", ""},
		{"a country of three letters", "", "VNM", ""},
		{"a country that is not letters", "Hanoi", "V1", "Hanoi"},
		{"a city in Vietnamese", "Thành phố Hồ Chí Minh", "VN", "Thành phố Hồ Chí Minh, VN"},
		{"a percent-encoded city", "S%C3%A3o%20Paulo", "BR", "São Paulo, BR"},
		{"an encoding that does not decode", "100%zz", "VN", "100%zz, VN"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := labelFor(t, "CF-Connecting-IP", cloudflare(c.city, c.country)); got != c.want {
				t.Errorf("label = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTheLabelHoldsOnlyPrintableTextOnOneLine(t *testing.T) {
	cases := []struct {
		name string
		city string
		want string
	}{
		{"a line break", "Ha\r\nNoi", "Ha Noi, VN"},
		{"tabs and runs of spaces", "  Da \t\t Nang  ", "Da Nang, VN"},
		{"a no-break space", "Da Nang", "Da Nang, VN"},
		{"a control character", "Ha\x00no\x07i", "Hanoi, VN"},
		{"a right-to-left override", "‮evil", "evil, VN"},
		{"a zero-width joiner", "Ha‍noi", "Hanoi, VN"},
		{"invalid UTF-8", "\xff\xfeHanoi", "VN"},
		{"only unprintable characters", "\x00\x01‮", "VN"},
		{"an encoded line break", "Ha%0D%0ASet-Cookie:%20x=y", "Ha Set-Cookie: x=y, VN"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := labelFor(t, "CF-Connecting-IP", cloudflare(c.city, "VN"))
			if got != c.want {
				t.Errorf("label = %q, want %q", got, c.want)
			}
			if strings.ContainsAny(got, "\r\n\t\x00") {
				t.Errorf("label %q holds a control character", got)
			}
		})
	}
}

func TestALongCityIsCutOnACharacterBoundaryAndTheLabelStaysInTheColumn(t *testing.T) {
	for _, city := range []string{strings.Repeat("a", 10_000), strings.Repeat("ế", 10_000), strings.Repeat("a ", 5_000), strings.Repeat("💥", 200)} {
		got := labelFor(t, "CF-Connecting-IP", cloudflare(city, "VN"))
		if !strings.HasSuffix(got, ", VN") {
			t.Errorf("label %.30q lost its country", got)
		}
		if n := utf8.RuneCountInString(got); n > 74 {
			t.Errorf("label is %d runes, want at most 74 so the 80-character column always holds it", n)
		}
	}
}
