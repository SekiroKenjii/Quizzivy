package router_test

import (
	"net/http"
	"quizzivy/gen/openapi"
	"strings"
	"testing"
)

func TestImportOperationsRequireTeacherBeforeReadingSourceBytes(t *testing.T) {
	issuer := testIssuer(t)
	handler := newAuthTestRouter(t, issuer)
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for path, item := range spec.Paths.Map() {
		if !strings.HasPrefix(path, "/admin/imports") {
			continue
		}
		path = strings.NewReplacer("{id}", "01935000-0000-7000-8000-000000000001", "{sourceId}", "01935000-0000-7000-8000-000000000002").Replace(path)
		for method := range item.Operations() {
			target := path
			if method == http.MethodPost && strings.HasSuffix(path, "/sources") {
				target += "?role=exam&uploadId=01935000-0000-7000-8000-000000000003&expectedRevision=1"
			}
			if strings.HasSuffix(path, "/source") {
				target += "?role=exam"
			}
			for _, role := range []string{"", "student"} {
				response := requestAs(t, handler, issuer, method, target, role)
				want := http.StatusForbidden
				if role == "" {
					want = http.StatusUnauthorized
				}
				if response.Code != want {
					t.Fatalf("%s %s (%s): %d", method, path, role, response.Code)
				}
			}
			count++
		}
	}
	if count != 13 {
		t.Fatalf("protected import operations: %d", count)
	}
}
func TestImportUploadParametersRejectInvalidUUIDAndRoles(t *testing.T) {
	issuer := testIssuer(t)
	handler := newAuthTestRouter(t, issuer)
	for _, query := range []string{"role=exam&uploadId=bad&expectedRevision=1", "role=student&uploadId=01935000-0000-7000-8000-000000000001&expectedRevision=1", "role=exam&uploadId=01935000-0000-7000-8000-000000000001&expectedRevision=0"} {
		response := requestAs(t, handler, issuer, http.MethodPost, "/admin/imports/01935000-0000-7000-8000-000000000001/sources?"+query, "admin")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid upload accepted: %d %s", response.Code, response.Body.String())
		}
	}
}
