package apidocs

import (
	"fmt"
	"net/http"
)

const scalarVersion = "1.67.0"

// Reference serves the API reference: our own page that loads Scalar's
// standalone bundle and points it at the spec the server itself serves.
func Reference(specPath string) http.Handler {
	page := fmt.Sprintf(`<!doctype html>
<html lang="vi">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Quizzivy API</title>
</head>
<body>
  <div id="app"></div>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@%s/dist/browser/standalone.js"></script>
  <script>
    Scalar.createApiReference('#app', { url: %q, theme: 'default', hideModels: false })
  </script>
</body>
</html>
`, scalarVersion, specPath)
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
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
