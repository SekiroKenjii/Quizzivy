package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/core/router"
	"quizzivy/internal/platform/ratelimit"
)

type spelledAccount struct {
	bare      string
	spellings []string
}

var spelledAccounts = []spelledAccount{
	{
		bare: "an@example.com",
		spellings: []string{
			"an@example.com",
			"AN@Example.COM",
			"  an@example.com  ",
			"<an@example.com>",
			"Nguyễn Văn An <an@example.com>",
			`"an"@example.com`,
			`"AN"@example.com`,
			"an@example.com (ghi chú)",
			`"Văn An" <an@example.com>`,
			"Cô An <AN@example.com>",
		},
	},
	{
		bare: "nguyễn.văn.ệ@example.com",
		spellings: []string{
			"nguyễn.văn.ệ@example.com",
			"NGUYỄN.VĂN.Ệ@EXAMPLE.COM",
			"Nguyễn.Văn.Ệ@Example.com",
			"\tNGUYỄN.văn.ệ@example.com\n",
			"<NGUYỄN.VĂN.Ệ@example.com>",
			`"NGUYỄN.VĂN.Ệ"@example.com`,
			"Nguyễn Văn Ệ <Nguyễn.Văn.Ệ@example.com>",
			"nguyễn.văn.ệ@example.com (ghi chú)",
			`"Văn Ệ" <NGUYỄN.VĂN.Ệ@example.com>`,
			"Cô Ệ <nguyễn.văn.ệ@EXAMPLE.com>",
		},
	},
}

func (a spelledAccount) another(i int) string {
	return fmt.Sprintf("Tên số %d <%s>", i, a.bare)
}

func (a spelledAccount) spelledAtLeast(n int) []string {
	out := append([]string(nil), a.spellings...)
	for i := len(out); i < n; i++ {
		out = append(out, a.another(i))
	}
	return out
}

func loginAs(t *testing.T, h http.Handler, address, email string) int {
	t.Helper()
	body, err := json.Marshal(map[string]string{"email": email, "password": "guess-the-password"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = address + ":40000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestOneAccountWrittenManyWaysSpendsOneBudgetPerAddress(t *testing.T) {
	for _, account := range spelledAccounts {
		t.Run(account.bare, func(t *testing.T) {
			h := newTestRouter(t, fakeDB{})
			spellings := account.spelledAtLeast(15)
			for i, email := range spellings[:10] {
				if got := loginAs(t, h, schoolAddress, email); got == http.StatusTooManyRequests {
					t.Fatalf("spelling %d (%q) was limited within the ten a minute", i+1, email)
				}
			}
			for _, email := range spellings[10:] {
				if got := loginAs(t, h, schoolAddress, email); got != http.StatusTooManyRequests {
					t.Errorf("spelling %q after ten for the same account from one address: %d, want 429", email, got)
				}
			}
			if got := loginAs(t, h, schoolAddress, "binh@example.com"); got == http.StatusTooManyRequests {
				t.Error("another account's first login from the same address was limited")
			}
		})
	}
}

func TestOneAccountWrittenManyWaysSpendsOneHourlyBudgetFromEveryAddress(t *testing.T) {
	const hourly = 20
	for _, account := range spelledAccounts {
		t.Run(account.bare, func(t *testing.T) {
			h := newTestRouter(t, fakeDB{})
			spellings := account.spelledAtLeast(hourly + 1)
			for i, email := range spellings[:hourly] {
				address := fmt.Sprintf("198.51.100.%d", i+1)
				if got := loginAs(t, h, address, email); got == http.StatusTooManyRequests {
					t.Fatalf("spelling %d (%q) from its own address was limited within the twenty an hour", i+1, email)
				}
			}
			if got := loginAs(t, h, "198.51.100.200", spellings[hourly]); got != http.StatusTooManyRequests {
				t.Errorf("the 21st spelling of one account from a 21st address: %d, want 429", got)
			}
			if got := loginAs(t, h, "198.51.100.201", "binh@example.com"); got == http.StatusTooManyRequests {
				t.Error("another account's first login was limited by the first account's hourly budget")
			}
		})
	}
}

func TestAnAddressNoParserAcceptsIsStillBucketedByItsOwnText(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	const refused = `"an b"@example.com`
	for i := range 10 {
		if got := loginAs(t, h, schoolAddress, refused); got == http.StatusTooManyRequests {
			t.Fatalf("login %d with an address the contract refuses was limited early", i+1)
		}
	}
	if got := loginAs(t, h, schoolAddress, refused); got != http.StatusTooManyRequests {
		t.Errorf("the 11th login with an address the contract refuses: %d, want 429", got)
	}
	if got := loginAs(t, h, schoolAddress, `"an c"@example.com`); got == http.StatusTooManyRequests {
		t.Error("a different refused text shared the first one's bucket")
	}
}

func loginBuckets(t *testing.T) map[string]ratelimit.Keyed {
	t.Helper()
	route, ok := router.RateLimits().Lookup("POST /auth/login")
	if !ok {
		t.Fatal("POST /auth/login has no registry entry")
	}
	out := map[string]ratelimit.Keyed{}
	for _, bucket := range route.Keyed {
		out[bucket.Name] = bucket
	}
	for _, name := range []string{"perAddressAndEmail", "perEmail"} {
		if _, ok := out[name]; !ok {
			t.Fatalf("POST /auth/login has no %s bucket", name)
		}
	}
	return out
}

func keyOf(t *testing.T, bucket ratelimit.Keyed, address, email string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"email": email})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(body)))
	return bucket.Key(ratelimit.WithAddress(req, address))
}

func TestBothAccountBucketsKeyOnTheBareLowerCasedAddress(t *testing.T) {
	buckets := loginBuckets(t)
	for name, bucket := range buckets {
		keys := map[string]string{}
		for _, account := range spelledAccounts {
			want := keyOf(t, bucket, schoolAddress, account.bare)
			if want == "" {
				t.Fatalf("%s gives %q no key", name, account.bare)
			}
			if other, seen := keys[want]; seen {
				t.Fatalf("%s gives %q and %q the same key", name, other, account.bare)
			}
			keys[want] = account.bare
			for _, email := range account.spelledAtLeast(12) {
				if got := keyOf(t, bucket, schoolAddress, email); got != want {
					t.Errorf("%s key of %q = %q, want %q (the key of %q)", name, email, got, want, account.bare)
				}
			}
		}
		sameAcrossAddresses := keyOf(t, bucket, schoolAddress, "an@example.com") == keyOf(t, bucket, "198.51.100.9", "an@example.com")
		if want := name == "perEmail"; sameAcrossAddresses != want {
			t.Errorf("%s: the key is the same from two addresses = %v, want %v", name, sameAcrossAddresses, want)
		}
	}
}
