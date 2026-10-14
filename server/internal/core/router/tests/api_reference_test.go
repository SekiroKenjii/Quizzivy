package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"quizzivy/gen/openapi"
)

func TestTheApiReferenceSummaryIsAnAdminsAndNoTeachers(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())

	for _, user := range []string{teacherUser, studentUser} {
		if rec := sendAs(t, h, issuer, http.MethodGet, "/admin/api-reference", user, ""); rec.Code != http.StatusForbidden {
			t.Errorf("%s got %d on the summary, want 403", user, rec.Code)
		}
	}
	if rec := sendAs(t, h, issuer, http.MethodGet, "/admin/api-reference", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("an anonymous caller got %d, want 401", rec.Code)
	}

	rec := sendAs(t, h, issuer, http.MethodGet, "/admin/api-reference", adminUser, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("the Admin got %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Openapi       string `json:"openapi"`
		Version       string `json:"version"`
		EndpointCount int    `json:"endpointCount"`
		Groups        []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body, err)
	}
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	operations := 0
	for _, item := range spec.Paths.Map() {
		operations += len(item.Operations())
	}
	if body.Openapi != spec.OpenAPI || body.Version != spec.Info.Version || body.EndpointCount != operations {
		t.Errorf("summary %+v, want OpenAPI %s, version %s and %d operations", body, spec.OpenAPI, spec.Info.Version, operations)
	}
	if len(body.Groups) != len(spec.Tags) {
		t.Errorf("%d groups for %d declared tags", len(body.Groups), len(spec.Tags))
	}
}

func TestTheContractDownloadIsTheEmbeddedSpecAsAnAttachment(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())

	if rec := sendAs(t, h, issuer, http.MethodGet, "/admin/api-reference/openapi.json", teacherUser, ""); rec.Code != http.StatusForbidden {
		t.Errorf("a teacher got %d on the download, want 403", rec.Code)
	}

	rec := sendAs(t, h, issuer, http.MethodGet, "/admin/api-reference/openapi.json", adminUser, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("the Admin got %d: %s", rec.Code, rec.Body)
	}
	want, err := openapi.GetSpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rec.Body.Bytes(), want) {
		t.Errorf("the body is %d bytes and not the embedded spec's %d", rec.Body.Len(), len(want))
	}
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="quizzivy-openapi.json"` {
		t.Errorf("Content-Disposition %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", got)
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Error("the download is not a JSON document")
	}
}
