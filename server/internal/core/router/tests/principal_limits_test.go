package router_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/ratelimit"
)

const (
	resetPasswordPath = "/teacher/students/01935000-0000-7000-8000-0000000000e1/reset-password"
	centreAddress     = "203.0.113.50"
	homeAddress       = "198.51.100.60"
)

var thePerActorLimits = map[string][]ratelimit.Rule{
	"POST /teacher/students/{id}/reset-password": {{Burst: 5, Window: time.Minute}, {Burst: 30, Window: time.Hour}},
	"POST /teacher/students":                     {{Burst: 30, Window: time.Minute}, {Burst: 300, Window: time.Hour}},
	"POST /teacher/classes/{id}/join-code":       {{Burst: 10, Window: time.Minute}, {Burst: 60, Window: time.Hour}},
	"GET /teacher/classes/{id}/join-code":        {{Burst: 60, Window: time.Minute}, {Burst: 600, Window: time.Hour}},
	"POST /admin/docs-session":                   {{Burst: 5, Window: time.Minute}, {Burst: 30, Window: time.Hour}},
	"GET /teacher/assignments/results.csv":       {{Burst: 10, Window: time.Minute}, {Burst: 60, Window: time.Hour}},
	"PUT /me/avatar":                             {{Burst: 10, Window: time.Hour}},
}

func sendFrom(t *testing.T, h http.Handler, issuer *identitytoken.Issuer, method, path, userID, address, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = address + ":40000"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		raw, err := issuer.Issue(userID, "admin", 0)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+raw)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func resetFrom(t *testing.T, h http.Handler, issuer *identitytoken.Issuer, userID, address string) *httptest.ResponseRecorder {
	t.Helper()
	return sendFrom(t, h, issuer, http.MethodPost, resetPasswordPath, userID, address, "")
}

func spendResetBudget(t *testing.T, h http.Handler, issuer *identitytoken.Issuer, userID string, addresses ...string) {
	t.Helper()
	for i := range 5 {
		address := addresses[i%len(addresses)]
		if rec := resetFrom(t, h, issuer, userID, address); rec.Code != http.StatusNotImplemented {
			t.Fatalf("reset %d from %s: %d, want it to reach the handler within the budget of five", i+1, address, rec.Code)
		}
	}
}

func TestTwoActorsBehindOneAddressKeepSeparateBudgets(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	spendResetBudget(t, h, issuer, teacherUser, centreAddress)

	sixth := resetFrom(t, h, issuer, teacherUser, centreAddress)
	if sixth.Code != http.StatusTooManyRequests {
		t.Fatalf("the first actor's sixth reset in a minute: %d, want 429", sixth.Code)
	}
	if seconds, err := strconv.Atoi(sixth.Header().Get("Retry-After")); err != nil || seconds < 1 {
		t.Errorf("Retry-After = %q, want whole seconds of at least 1", sixth.Header().Get("Retry-After"))
	}
	if rec := resetFrom(t, h, issuer, adminUser, centreAddress); rec.Code != http.StatusNotImplemented {
		t.Errorf("a second actor's first reset behind the same address: %d, want it to reach the handler", rec.Code)
	}
	if rec := resetFrom(t, h, issuer, teacherUser, centreAddress); rec.Code != http.StatusTooManyRequests {
		t.Errorf("the first actor's next reset after the second actor's: %d, want 429: the registry must hold both budgets", rec.Code)
	}
}

func TestOneActorFromTwoAddressesSharesOneBudget(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	spendResetBudget(t, h, issuer, teacherUser, centreAddress, homeAddress)

	for _, address := range []string{centreAddress, homeAddress, "192.0.2.77"} {
		if rec := resetFrom(t, h, issuer, teacherUser, address); rec.Code != http.StatusTooManyRequests {
			t.Errorf("the sixth reset, sent from %s: %d, want 429 from the actor's one budget", address, rec.Code)
		}
	}
}

func TestAPrincipalLimitAnswersAsTheAddressLimitDoes(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	spendResetBudget(t, h, issuer, teacherUser, centreAddress)
	byPrincipal := resetFrom(t, h, issuer, teacherUser, centreAddress)

	var byAddress *httptest.ResponseRecorder
	for range 130 {
		byAddress = sendFrom(t, h, issuer, http.MethodPost, "/app/classes/join", studentUser, centreAddress, "")
		if byAddress.Code == http.StatusTooManyRequests {
			break
		}
	}

	for name, rec := range map[string]*httptest.ResponseRecorder{"principal": byPrincipal, "address": byAddress} {
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("the %s limiter past its budget: %d, want 429", name, rec.Code)
		}
		if seconds, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || seconds < 1 {
			t.Errorf("the %s limiter's Retry-After = %q, want whole seconds of at least 1", name, rec.Header().Get("Retry-After"))
		}
	}
	if a, b := byPrincipal.Header().Get("Content-Type"), byAddress.Header().Get("Content-Type"); a != b {
		t.Errorf("Content-Type %q from the principal limiter, %q from the address limiter", a, b)
	}
	principal, address := errorEnvelope(t, byPrincipal), errorEnvelope(t, byAddress)
	if principal.Code != "RATE_LIMITED" || principal.Code != address.Code {
		t.Errorf("envelope code %q from the principal limiter, %q from the address limiter, want RATE_LIMITED from both", principal.Code, address.Code)
	}
	if principal.Message == "" || principal.Message != address.Message {
		t.Errorf("message %q from the principal limiter, %q from the address limiter", principal.Message, address.Message)
	}
	if principal.RequestID == "" {
		t.Error("the principal limiter's 429 carries no requestId")
	}
}

type envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

func errorEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var body struct {
		Error envelope `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response is not the error envelope: %v", err)
	}
	return body.Error
}

func TestThePrincipalLimiterRunsAfterTheGateAndBeforeTheBody(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	for name, c := range map[string]struct {
		user string
		want int
	}{
		"anonymous": {"", http.StatusUnauthorized},
		"student":   {studentUser, http.StatusForbidden},
	} {
		for i := 1; i <= 7; i++ {
			if rec := resetFrom(t, h, issuer, c.user, centreAddress); rec.Code != c.want {
				t.Fatalf("%s request %d: %d, want %d every time: the gate answers before the principal limiter", name, i, rec.Code, c.want)
			}
		}
	}

	oversized := strings.Repeat(" ", 2<<20)
	if rec := sendFrom(t, h, issuer, http.MethodPost, resetPasswordPath, adminUser, centreAddress, oversized); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("an oversized body within the budget: %d, want 413 from the body limit", rec.Code)
	}

	spendResetBudget(t, h, issuer, teacherUser, centreAddress)
	if rec := sendFrom(t, h, issuer, http.MethodPost, resetPasswordPath, teacherUser, centreAddress, oversized); rec.Code != http.StatusTooManyRequests {
		t.Errorf("an oversized body past the budget: %d, want 429 before the body limit's 413", rec.Code)
	}
}

func TestThePrincipalAssertionActuallyFails(t *testing.T) {
	spec := freshSpec(t)
	keyed := func(patterns ...string) *ratelimit.Registry {
		reg := ratelimit.NewRegistry()
		for _, pattern := range patterns {
			reg.AddKeyed(pattern).WithKey("perActor", ratelimit.PrincipalKey, 100, ratelimit.PerMinute(10))
		}
		return reg
	}

	err := httpx.AssertPrincipalRoutesGated(spec, "bearerAuth", keyed("POST /auth/login", "POST /teacher/students"))
	if err == nil {
		t.Fatal("an entry on an open operation must be refused: no principal exists there")
	}
	if !strings.Contains(err.Error(), "POST /auth/login: is an open operation") {
		t.Errorf("error does not name the open operation:\n%s", err)
	}
	if strings.Contains(err.Error(), "POST /teacher/students") {
		t.Errorf("error names a gated operation:\n%s", err)
	}

	err = httpx.AssertPrincipalRoutesGated(spec, "bearerAuth", keyed("POST /teacher/nothing"))
	if err == nil || !strings.Contains(err.Error(), "POST /teacher/nothing: is not an operation") {
		t.Errorf("err = %v, want it to name the entry that is no operation of the contract", err)
	}

	err = httpx.AssertPrincipalRoutesGated(spec, "bearerAuth", keyed(theOpenSeven...))
	for _, open := range theOpenSeven {
		if err == nil || !strings.Contains(err.Error(), open+": is an open operation") {
			t.Errorf("err = %v, want it to name %q", err, open)
		}
	}

	if err := httpx.AssertPrincipalRoutesGated(spec, "bearerAuth", ratelimit.NewRegistry()); err != nil {
		t.Errorf("an empty registry was refused: %v", err)
	}
	if err := httpx.AssertPrincipalRoutesGated(spec, "bearerAuth", router.PrincipalRateLimits()); err != nil {
		t.Errorf("the router's own registry was refused: %v", err)
	}
}

func TestTheRouterAssertsThePrincipalRegistryItEnforces(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "router.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse router.go: %v", err)
	}
	registry := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		fn, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := fn.X.(*ast.Ident); !ok || pkg.Name != "httpx" {
			return true
		}
		if last, ok := call.Args[len(call.Args)-1].(*ast.Ident); ok {
			registry[fn.Sel.Name] = last.Name
		}
		return true
	})
	asserted, enforced := registry["AssertPrincipalRoutesGated"], registry["PrincipalRateLimit"]
	if asserted == "" {
		t.Fatal("router.go never calls httpx.AssertPrincipalRoutesGated on a named registry: a bad entry would not stop the start-up")
	}
	if asserted != enforced {
		t.Errorf("router.go asserts %q and enforces %q, want one registry", asserted, enforced)
	}
}

func TestTheRouterStopsOnThePrincipalAssertionsError(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "router.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse router.go: %v", err)
	}
	returned := false
	ast.Inspect(file, func(n ast.Node) bool {
		check, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		assign, ok := check.Init.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || fn.Sel.Name != "AssertPrincipalRoutesGated" {
			return true
		}
		for _, stmt := range check.Body.List {
			ret, ok := stmt.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 2 {
				continue
			}
			if failure, ok := ret.Results[1].(*ast.Ident); ok && failure.Name == "err" {
				returned = true
			}
		}
		return true
	})
	if !returned {
		t.Error("router.go does not return the error of httpx.AssertPrincipalRoutesGated: a bad entry would not stop the start-up")
	}
}

func TestEveryAuthenticatedCredentialMinterIsLimitedPerActorOnly(t *testing.T) {
	spec := freshSpec(t)
	open := openPatterns(spec)
	byAddress, byPrincipal := router.RateLimits(), router.PrincipalRateLimits()
	authenticated := map[string]bool{}
	eachOperation(spec, func(pattern string, op *openapi3.Operation) {
		if !theCredentialMinters[op.OperationID] {
			return
		}
		_, perAddress := byAddress.Lookup(pattern)
		_, perActor := byPrincipal.Lookup(pattern)
		if _, isOpen := open[pattern]; isOpen {
			if !perAddress || perActor {
				t.Errorf("%s is open: per address %v, per actor %v, want it per address only", pattern, perAddress, perActor)
			}
			return
		}
		authenticated[op.OperationID] = true
		if !perActor {
			t.Errorf("%s mints a credential behind a token and is not in PrincipalRateLimits()", pattern)
		}
		if perAddress {
			t.Errorf("%s is still in RateLimits(): staff behind one address would share its budget", pattern)
		}
	})
	for _, id := range []string{"ResetStudentPassword", "CreateStudent", "RotateJoinCode", "GetJoinCode", "OpenDocsSession"} {
		if !authenticated[id] {
			t.Errorf("%s was not found among the minters behind a token: the walk is looking at the wrong thing", id)
		}
	}
}

func TestEveryPerActorLimitHasOneBucketKeyedByThePrincipal(t *testing.T) {
	reg := router.PrincipalRateLimits()
	got := reg.Patterns()
	slices.Sort(got)
	want := make([]string, 0, len(thePerActorLimits))
	for pattern := range thePerActorLimits {
		want = append(want, pattern)
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("PrincipalRateLimits() holds %v, want %v", got, want)
	}

	r := httptest.NewRequest(http.MethodPost, "/teacher/students", nil)
	for pattern, rules := range thePerActorLimits {
		route, _ := reg.Lookup(pattern)
		if route.PerIP != nil {
			t.Errorf("%s keeps a per-address bucket", pattern)
		}
		if len(route.Keyed) != 1 || route.Keyed[0].Name != "perActor" {
			t.Errorf("%s has keyed buckets %+v, want the one named perActor", pattern, route.Keyed)
			continue
		}
		bucket := route.Keyed[0]
		if !slices.Equal(bucket.Limiter.Rules(), rules) {
			t.Errorf("%s is limited to %v, want %v", pattern, bucket.Limiter.Rules(), rules)
		}
		if got := bucket.Key(ratelimit.WithPrincipal(ratelimit.WithAddress(r, centreAddress), teacherUser)); got != teacherUser {
			t.Errorf("%s keys its bucket by %q, want the principal's user id", pattern, got)
		}
		if got := bucket.Key(ratelimit.WithAddress(r, centreAddress)); got != "" {
			t.Errorf("%s keys its bucket by %q with no principal recorded, want it skipped", pattern, got)
		}
	}
}
