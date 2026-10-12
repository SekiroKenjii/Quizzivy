package httpx

import (
	"net/http"
	"strings"
)

// WriteNotFound answers 404 NOT_FOUND for a path the API does not serve, in
// Vietnamese or, when Accept-Language prefers it, English.
func WriteNotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusNotFound, CodeNotFound, TextFor(r, "Không tìm thấy đường dẫn.", "The path was not found."))
}

// WriteMethodNotAllowed answers 405 METHOD_NOT_ALLOWED for a path the API
// serves under other methods only, with allow, the methods it does serve,
// joined in the Allow header, in Vietnamese or, when Accept-Language prefers
// it, English.
func WriteMethodNotAllowed(w http.ResponseWriter, r *http.Request, allow []string) {
	w.Header().Set("Allow", strings.Join(allow, ", "))
	WriteError(w, r, http.StatusMethodNotAllowed, CodeMethodNotAllowed,
		TextFor(r, "Phương thức này không dùng được cho đường dẫn này.", "This method is not allowed for this path."))
}
