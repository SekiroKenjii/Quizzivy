// Package apidocs serves the API reference page and the contract it renders.
package apidocs

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
)

// ScalarVersion and ScalarIntegrity pin the reference bundle to exact bytes;
// a version bump without the matching sha384 makes the browser refuse the script.
const (
	ScalarVersion   = "1.67.0"
	ScalarIntegrity = "sha384-6c7Vmx+i0yi8gBbltn0x1cavD+zsMGw2xmXXVyacPJLIGBxwaVimW5TW0WiW17Ir"
)

// ScalarURL is the pinned bundle's address, the only external script the page may run.
const ScalarURL = "https://cdn.jsdelivr.net/npm/@scalar/api-reference@" + ScalarVersion + "/dist/browser/standalone.js"

// Reference serves the API reference: our own page that loads Scalar's
// standalone bundle, integrity-checked, and points it at the spec the server
// itself serves. Its CSP admits that bundle and the page's one inline script
// only, and lets the page talk to its own origin only.
func Reference(specPath string) http.Handler {
	boot := fmt.Sprintf(`
    Scalar.createApiReference('#app', { url: %q, servers: [{ url: window.location.origin }], theme: 'default', hideModels: false, withDefaultFonts: false, telemetry: false, showDeveloperTools: 'never', agent: { disabled: true }, mcp: { disabled: true } })
  `, specPath)
	digest := sha256.Sum256([]byte(boot))
	policy := fmt.Sprintf("default-src 'none'; script-src 'sha256-%s' %s; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'",
		base64.StdEncoding.EncodeToString(digest[:]), ScalarURL)
	page := fmt.Sprintf(`<!doctype html>
<html lang="vi">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Quizzivy API</title>
</head>
<body>
  <div id="app"></div>
  <script src="%s" integrity="%s" crossorigin="anonymous"></script>
  <script>%s</script>
</body>
</html>
`, ScalarURL, ScalarIntegrity, boot)
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", policy)
		_, _ = w.Write([]byte(page))
	})
}

// Spec serves the contract as JSON for the reference page and for any client
// that wants the document the server was built from.
func Spec(document func() ([]byte, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, err := document()
		if err != nil {
			http.Error(w, "the contract could not be read", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	})
}
