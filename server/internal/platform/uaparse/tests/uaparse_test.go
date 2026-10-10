package uaparse_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"quizzivy/internal/platform/uaparse"
)

func TestParseNamesTheSystemAndBrowserOfCommonAgents(t *testing.T) {
	cases := []struct {
		name  string
		agent string
		label string
		kind  uaparse.Kind
	}{
		{"Chrome on a Mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", "Mac · Chrome", uaparse.KindComputer},
		{"Safari on a Mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15", "Mac · Safari", uaparse.KindComputer},
		{"Edge on Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0", "Windows · Edge", uaparse.KindComputer},
		{"Firefox on Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0", "Windows · Firefox", uaparse.KindComputer},
		{"Opera on Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 OPR/111.0.0.0", "Windows · Opera", uaparse.KindComputer},
		{"Safari on an iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", "iPhone · Safari", uaparse.KindPhone},
		{"Chrome on an iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.6478.153 Mobile/15E148 Safari/604.1", "iPhone · Chrome", uaparse.KindPhone},
		{"Firefox on an iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/127.0 Mobile/15E148 Safari/605.1.15", "iPhone · Firefox", uaparse.KindPhone},
		{"Edge on an iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/126.0.2592.87 Version/17.0 Mobile/15E148 Safari/605.1.15", "iPhone · Edge", uaparse.KindPhone},
		{"Safari on an iPad", "Mozilla/5.0 (iPad; CPU OS 12_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/12.1.2 Mobile/15E148 Safari/604.1", "iPad · Safari", uaparse.KindTablet},
		{"an iPad asking for desktop sites", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15", "Mac · Safari", uaparse.KindComputer},
		{"Chrome on an Android phone", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36", "Android · Chrome", uaparse.KindPhone},
		{"Chrome on an Android tablet", "Mozilla/5.0 (Linux; Android 13; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", "Android · Chrome", uaparse.KindTablet},
		{"Samsung Internet", "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36", "Android · Samsung Internet", uaparse.KindPhone},
		{"Edge on Android", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36 EdgA/126.0.0.0", "Android · Edge", uaparse.KindPhone},
		{"Opera Touch on an iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1 OPT/5.1", "iPhone · Opera", uaparse.KindPhone},
		{"Firefox on Linux", "Mozilla/5.0 (X11; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0", "Linux · Firefox", uaparse.KindComputer},
		{"Chrome on ChromeOS", "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", "ChromeOS · Chrome", uaparse.KindComputer},
		{"a system and no browser", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "Windows", uaparse.KindComputer},
		{"a browser and no system", "Chrome/126.0.0.0", "Chrome", uaparse.KindUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := uaparse.Parse(c.agent)
			if got.Label() != c.label {
				t.Errorf("label = %q, want %q", got.Label(), c.label)
			}
			if got.Kind != c.kind {
				t.Errorf("kind = %q, want %q", got.Kind, c.kind)
			}
		})
	}
}

func TestParseKnowsNothingOfAnUnrecognisedAgent(t *testing.T) {
	for _, agent := range []string{"", "curl/8.4.0", "go-test", "Mozilla/5.0", "Dalvik/2.1.0", "\x00\x01\x02", "💥"} {
		got := uaparse.Parse(agent)
		if got.Label() != "" || got.Kind != uaparse.KindUnknown || got.System != uaparse.SystemUnknown || got.Browser != uaparse.BrowserUnknown {
			t.Errorf("Parse(%q) = %+v with label %q, want nothing known", agent, got, got.Label())
		}
	}
}

func TestParseReadsOnlyTheStartOfALongAgent(t *testing.T) {
	filler := strings.Repeat("a", 600)
	if got := uaparse.Parse(filler + "Macintosh Chrome/1"); got.Label() != "" {
		t.Errorf("a token past the first 512 bytes was read: %q", got.Label())
	}
	if got := uaparse.Parse("Macintosh Chrome/1 " + filler); got.Label() != "Mac · Chrome" {
		t.Errorf("a token at the start was not read: %q", got.Label())
	}
}

func TestParseNeverEchoesTheAgent(t *testing.T) {
	agents := []string{
		"<script>alert(1)</script> Macintosh Chrome/1 <img src=x onerror=alert(1)>",
		"Windows · Firefox · Spoofed Laptop Macintosh",
		"Macintosh ‮Chrome/1‬",
		"Macintosh; Chrome/1\r\nSet-Cookie: x=y",
		strings.Repeat("Linux Firefox/1 ", 2000),
	}
	for _, agent := range agents {
		assertClosed(t, agent, uaparse.Parse(agent))
	}
}

func TestParseOfRandomAgentsNeverLeavesTheClosedLists(t *testing.T) {
	tokens := []string{
		"Mozilla/5.0", "Macintosh", "Mac OS X", "Windows NT 10.0", "iPhone", "iPad", "Android 14", "Linux", "X11", "CrOS",
		"Chrome/126", "CriOS/126", "Safari/605", "Version/17", "Firefox/127", "FxiOS/1", "Edg/1", "EdgA/1", "EdgiOS/1", "OPR/1",
		"OPT/1", "SamsungBrowser/1", "Mobile", ";", "(", ")", " ", "\x00", "\xff\xfe", "é", "·", "<b>", "Opera",
	}
	rng := rand.New(rand.NewPCG(49, 2026))
	for range 5000 {
		var b strings.Builder
		for range rng.IntN(40) {
			b.WriteString(tokens[rng.IntN(len(tokens))])
		}
		agent := b.String()
		assertClosed(t, agent, uaparse.Parse(agent))
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"", "Macintosh Chrome/1", "iPhone Safari/1", "\xff", strings.Repeat("x", 10_000), "Linux\x00Firefox/1"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, agent string) {
		assertClosed(t, agent, uaparse.Parse(agent))
	})
}

func assertClosed(t *testing.T, agent string, got uaparse.Device) {
	t.Helper()
	systems, browsers := uaparse.Systems(), uaparse.Browsers()
	if got.System != uaparse.SystemUnknown && !slices.Contains(systems, got.System) {
		t.Fatalf("Parse(%.60q).System = %q, outside the closed list", agent, got.System)
	}
	if got.Browser != uaparse.BrowserUnknown && !slices.Contains(browsers, got.Browser) {
		t.Fatalf("Parse(%.60q).Browser = %q, outside the closed list", agent, got.Browser)
	}
	if !slices.Contains([]uaparse.Kind{uaparse.KindUnknown, uaparse.KindComputer, uaparse.KindPhone, uaparse.KindTablet}, got.Kind) {
		t.Fatalf("Parse(%.60q).Kind = %q, outside the closed list", agent, got.Kind)
	}
	label := got.Label()
	if !utf8.ValidString(label) || len(label) > 40 {
		t.Fatalf("Parse(%.60q).Label() = %q, want valid UTF-8 of at most 40 bytes", agent, label)
	}
	want := []string{}
	if got.System != uaparse.SystemUnknown {
		want = append(want, string(got.System))
	}
	if got.Browser != uaparse.BrowserUnknown {
		want = append(want, string(got.Browser))
	}
	if label != strings.Join(want, " · ") {
		t.Fatalf("Parse(%.60q).Label() = %q, want the closed parts %q joined", agent, label, want)
	}
}
