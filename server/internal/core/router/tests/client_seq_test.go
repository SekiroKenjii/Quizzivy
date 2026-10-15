package router_test

import (
	"net/http"
	"strconv"
	"testing"
)

func TestAnEventSequenceBeyondTheColumnIsRefusedOnEveryJSONPath(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	events := func(seq int64) string {
		return `{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":` + strconv.FormatInt(seq, 10) + `}`
	}
	save := func(seq int64) string { return saveAnswersWithEvent(events(seq)) }
	flush := func(seq int64) string {
		return `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","events":[` + events(seq) + `]}`
	}

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   func(int64) string
	}{
		{"an autosave", http.MethodPatch, saveAnswersPath, save},
		{"a bearer flush", http.MethodPost, textBeaconPath, flush},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, over := range []int64{2147483648, 4294967296} {
				wantValidationFailure(t, sendAs(t, handler, issuer, tc.method, tc.path, studentUser, tc.body(over)), `Trường "events.0.clientSeq" không hợp lệ.`)
			}
			rec := sendAs(t, handler, issuer, tc.method, tc.path, studentUser, tc.body(2147483647))
			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("the largest number the column holds: status = %d, want 501 from a router with no attempts module: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
