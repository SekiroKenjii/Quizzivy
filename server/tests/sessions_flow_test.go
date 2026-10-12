//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/platform/config"
)

// The signed-in devices, over HTTP, through the whole stack. /auth sits
// outside the isolation suite, so the cross-user 404 is proved here.

const (
	macChromeAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	sessionsPath   = "/auth/sessions"
)

func (c *client) callWith(headers map[string]string, method, path string, payload any) (int, http.Header, map[string]any) {
	c.w.t.Helper()
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			c.w.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.w.server.URL+path, reader)
	if err != nil {
		c.w.t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.w.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	body := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			c.w.t.Fatalf("%s %s answered %d with a body that is not a JSON object: %s", method, path, resp.StatusCode, raw)
		}
	}
	return resp.StatusCode, resp.Header, body
}

func (c *client) sessions() []map[string]any {
	c.w.t.Helper()
	body := c.must(http.StatusOK, http.MethodGet, sessionsPath, nil)
	raw, ok := body["items"].([]any)
	if !ok {
		c.w.t.Fatalf("GET %s: items is not a list: %v", sessionsPath, body)
	}
	items := make([]map[string]any, len(raw))
	for i, item := range raw {
		items[i] = item.(map[string]any)
	}
	return items
}

func (c *client) currentFamily() string {
	c.w.t.Helper()
	for _, item := range c.sessions() {
		if item["current"] == true {
			return item["familyId"].(string)
		}
	}
	c.w.t.Fatal("no session is current")
	return ""
}

func (c *client) refresh() {
	c.w.t.Helper()
	body := c.must(http.StatusOK, http.MethodPost, "/auth/refresh", nil)
	c.token = body["accessToken"].(string)
}

func (c *client) errorCode(body map[string]any) string {
	c.w.t.Helper()
	envelope, ok := body["error"].(map[string]any)
	if !ok {
		c.w.t.Fatalf("not an error envelope: %v", body)
	}
	return envelope["code"].(string)
}

func bearerOnly(w *world, token string) *client {
	return &client{w: w, http: &http.Client{}, token: token}
}

func TestAnotherUsersSessionAnswersAsAnUnknownOne(t *testing.T) {
	w := boot(t)
	aliceEmail, alicePassword := w.createStaff("admin")
	bobEmail, bobPassword := w.createStaff("admin")
	alice, bob := w.browser(), w.browser()
	alice.login(aliceEmail, alicePassword)
	bob.login(bobEmail, bobPassword)
	bobFamily := bob.currentFamily()

	for _, item := range alice.sessions() {
		if item["familyId"] == bobFamily {
			t.Fatal("Bob's session is in Alice's list")
		}
	}
	otherStatus, otherBody := alice.call(http.MethodDelete, sessionsPath+"/"+bobFamily, nil)
	unknownStatus, unknownBody := alice.call(http.MethodDelete, sessionsPath+"/"+uuid.NewString(), nil)

	if otherStatus != http.StatusNotFound || unknownStatus != http.StatusNotFound {
		t.Fatalf("Bob's session answered %d and an unknown one %d, want 404 for both", otherStatus, unknownStatus)
	}
	other, unknown := otherBody["error"].(map[string]any), unknownBody["error"].(map[string]any)
	if other["code"] != "NOT_FOUND" || other["code"] != unknown["code"] || other["message"] != unknown["message"] {
		t.Errorf("Bob's session answered %v, an unknown one %v: they must read alike", other, unknown)
	}
	bob.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	bob.must(http.StatusOK, http.MethodPost, "/auth/refresh", nil)
	alice.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	if got := bob.currentFamily(); got != bobFamily {
		t.Errorf("Bob's session changed from %s to %s", bobFamily, got)
	}
}

func TestRevokingADeviceEndsItsTokenAtOnceAndTheCallingDeviceRecovers(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("admin")
	laptop, phone := w.browser(), w.browser()
	laptop.login(email, password)
	phone.login(email, password)
	laptopFamily := laptop.currentFamily()

	listed := phone.sessions()
	if len(listed) != 2 || listed[0]["current"] != true || listed[1]["familyId"] != laptopFamily || listed[1]["current"] != false {
		t.Fatalf("the phone's list = %v, want the phone first and current, then the laptop", listed)
	}
	phone.must(http.StatusNoContent, http.MethodDelete, sessionsPath+"/"+laptopFamily, nil)

	if status, _ := laptop.call(http.MethodGet, "/auth/me", nil); status != http.StatusUnauthorized {
		t.Errorf("the revoked laptop's access token answered %d on this machine, want 401 at once", status)
	}
	status, body := laptop.call(http.MethodPost, "/auth/refresh", nil)
	if status != http.StatusUnauthorized || laptop.errorCode(body) != "REFRESH_TOKEN_INVALID" {
		t.Errorf("the revoked laptop's refresh = %d %v, want 401 REFRESH_TOKEN_INVALID", status, body)
	}
	if status, _ := phone.call(http.MethodGet, "/auth/me", nil); status != http.StatusUnauthorized {
		t.Errorf("the calling phone's old access token answered %d, want 401: the epoch moved", status)
	}
	phone.refresh()
	phone.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	if items := phone.sessions(); len(items) != 1 || items[0]["current"] != true {
		t.Errorf("the phone's list after the revoke = %v, want only itself", items)
	}

	status, body = phone.call(http.MethodDelete, sessionsPath+"/"+phone.currentFamily(), nil)
	if status != http.StatusConflict || phone.errorCode(body) != "SESSION_IS_CURRENT" {
		t.Errorf("revoking the calling session = %d %v, want 409 SESSION_IS_CURRENT", status, body)
	}
	phone.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
}

func TestARevokedDevicesTokenStopsWorkingOnAnotherMachineWithinTenSeconds(t *testing.T) {
	here, elsewhere := boot(t), boot(t)
	email, password := here.createStaff("admin")
	laptop, phone := here.browser(), here.browser()
	laptop.login(email, password)
	phone.login(email, password)
	laptopFamily := laptop.currentFamily()
	laptopThere := bearerOnly(elsewhere, laptop.token)
	laptopThere.must(http.StatusOK, http.MethodGet, "/auth/me", nil)

	phone.must(http.StatusNoContent, http.MethodDelete, sessionsPath+"/"+laptopFamily, nil)

	deadline := time.Now().Add(12 * time.Second)
	for {
		status, _ := laptopThere.call(http.MethodGet, "/auth/me", nil)
		if status == http.StatusUnauthorized {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the revoked laptop's token still answered %d on another machine after 12 seconds: R2's cache bound is 10", status)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func TestRevokingTheOthersKeepsTheCallingDeviceAndAnEmptyCallMovesNothing(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("admin")
	first, second, third := w.browser(), w.browser(), w.browser()
	first.login(email, password)
	second.login(email, password)
	third.login(email, password)

	body := third.must(http.StatusOK, http.MethodPost, sessionsPath+"/revoke-others", nil)

	if body["revoked"] != float64(2) {
		t.Errorf("revoked = %v, want 2", body["revoked"])
	}
	for name, c := range map[string]*client{"first": first, "second": second} {
		if status, _ := c.call(http.MethodGet, "/auth/me", nil); status != http.StatusUnauthorized {
			t.Errorf("the %s device's token answered %d, want 401", name, status)
		}
		if status, _ := c.call(http.MethodPost, "/auth/refresh", nil); status != http.StatusUnauthorized {
			t.Errorf("the %s device refreshed: %d", name, status)
		}
	}
	if status, _ := third.call(http.MethodGet, "/auth/me", nil); status != http.StatusUnauthorized {
		t.Errorf("the calling device's old token answered %d, want 401: the epoch moved", status)
	}
	third.refresh()
	third.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	if items := third.sessions(); len(items) != 1 || items[0]["current"] != true {
		t.Errorf("the list after the call = %v, want only the calling device", items)
	}

	again := third.must(http.StatusOK, http.MethodPost, sessionsPath+"/revoke-others", nil)
	if again["revoked"] != float64(0) {
		t.Errorf("a second call revoked %v, want 0", again["revoked"])
	}
	third.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
}

func TestWithoutACookieThatNamesALiveSessionTheListShowsNoCurrentAndNothingIsRevoked(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("admin")
	otherEmail, otherPassword := w.createStaff("admin")
	laptop, phone, stranger := w.browser(), w.browser(), w.browser()
	laptop.login(email, password)
	phone.login(email, password)
	stranger.login(otherEmail, otherPassword)
	laptopFamily := laptop.currentFamily()

	crossed := &client{w: w, http: stranger.http, token: phone.token}
	for name, c := range map[string]*client{"no cookie": bearerOnly(w, phone.token), "another user's cookie": crossed} {
		items := c.sessions()
		if len(items) != 2 {
			t.Errorf("%s: %d sessions listed, want the user's two", name, len(items))
		}
		for _, item := range items {
			if item["current"] != false {
				t.Errorf("%s: a session is current", name)
			}
		}
		if status, body := c.call(http.MethodDelete, sessionsPath+"/"+laptopFamily, nil); status != http.StatusUnauthorized || c.errorCode(body) != "UNAUTHORIZED" {
			t.Errorf("%s: revoke one = %d %v, want 401 UNAUTHORIZED", name, status, body)
		}
		if status, body := c.call(http.MethodPost, sessionsPath+"/revoke-others", nil); status != http.StatusUnauthorized || c.errorCode(body) != "UNAUTHORIZED" {
			t.Errorf("%s: revoke others = %d %v, want 401 UNAUTHORIZED", name, status, body)
		}
	}

	laptop.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	phone.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	stranger.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
}

func TestTheDeviceAndLocationAreListedBehindCloudflareAndNowhereElse(t *testing.T) {
	behind := boot(t, func(c *config.Config) { c.ClientIPHeader = "CF-Connecting-IP" })
	email, password := behind.createStaff("admin")
	edge := func(city, country string) map[string]string {
		return map[string]string{"User-Agent": macChromeAgent, "CF-Connecting-IP": "203.0.113.9", "CF-IPCity": city, "CF-IPCountry": country}
	}
	browser := behind.browser()
	status, _, body := browser.callWith(edge("Ho Chi Minh City", "VN"), http.MethodPost, "/auth/login", map[string]any{"email": email, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login: %d %v", status, body)
	}
	browser.token = body["accessToken"].(string)

	status, header, listed := browser.callWith(nil, http.MethodGet, sessionsPath, nil)
	if status != http.StatusOK {
		t.Fatalf("list: %d %v", status, listed)
	}
	if got := header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	item := listed["items"].([]any)[0].(map[string]any)
	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if want := []string{"current", "device", "deviceKind", "familyId", "lastUsedAt", "location"}; !slices.Equal(keys, want) {
		t.Errorf("a session carries %v, want exactly %v: no address, user agent or token", keys, want)
	}
	if item["device"] != "Mac · Chrome" || item["deviceKind"] != "computer" || item["location"] != "Ho Chi Minh City, VN" || item["current"] != true {
		t.Errorf("session = %v, want Mac · Chrome, computer, Ho Chi Minh City, VN, current", item)
	}
	if _, err := time.Parse(time.RFC3339, item["lastUsedAt"].(string)); err != nil {
		t.Errorf("lastUsedAt %v is not an RFC 3339 time", item["lastUsedAt"])
	}

	for _, c := range []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"a refresh from another city", edge("Da Nang", "VN"), "Da Nang, VN"},
		{"a refresh that carries no location", map[string]string{"User-Agent": macChromeAgent, "CF-Connecting-IP": "203.0.113.9"}, "Da Nang, VN"},
	} {
		status, _, refreshed := browser.callWith(c.headers, http.MethodPost, "/auth/refresh", nil)
		if status != http.StatusOK {
			t.Fatalf("%s: %d %v", c.name, status, refreshed)
		}
		browser.token = refreshed["accessToken"].(string)
		if got := browser.sessions()[0]["location"]; got != c.want {
			t.Errorf("%s: location = %v, want %q", c.name, got, c.want)
		}
	}

	away := boot(t)
	other := away.browser()
	for name, headers := range map[string]map[string]string{
		"a server not behind Cloudflare": edge("Hanoi", "VN"),
		"a forwarded address":            {"User-Agent": macChromeAgent, "X-Forwarded-For": "203.0.113.9", "CF-IPCity": "Hanoi", "CF-IPCountry": "VN"},
	} {
		status, _, signedIn := other.callWith(headers, http.MethodPost, "/auth/login", map[string]any{"email": email, "password": password})
		if status != http.StatusOK {
			t.Fatalf("%s: login %d %v", name, status, signedIn)
		}
		other.token = signedIn["accessToken"].(string)
		for _, session := range other.sessions() {
			if session["current"] == true && session["location"] != nil {
				t.Errorf("%s: location = %v, want null", name, session["location"])
			}
		}
	}
}

func TestTheRevokeOperationsAreLimitedPerUser(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("admin")
	browser := w.browser()
	browser.login(email, password)

	for i := 1; i <= 11; i++ {
		status, _ := browser.call(http.MethodDelete, sessionsPath+"/"+uuid.NewString(), nil)
		want := http.StatusNotFound
		if i == 11 {
			want = http.StatusTooManyRequests
		}
		if status != want {
			t.Fatalf("revoke %d answered %d, want %d", i, status, want)
		}
	}
	for i := 1; i <= 6; i++ {
		status, _ := browser.call(http.MethodPost, sessionsPath+"/revoke-others", nil)
		want := http.StatusOK
		if i == 6 {
			want = http.StatusTooManyRequests
		}
		if status != want {
			t.Fatalf("revoke others %d answered %d, want %d", i, status, want)
		}
	}
	browser.must(http.StatusOK, http.MethodGet, sessionsPath, nil)
}
