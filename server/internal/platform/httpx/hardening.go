package httpx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SecurityHeaders applies the API's browser protections, including error responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'none'; object-src 'none'")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if privateTree(r.URL.Path) {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// StreamingReadTimeout replaces the server-wide read deadline for streamed uploads, so a large file on a slow
// connection is not cut off; it stays below the server's write timeout.
const StreamingReadTimeout = 110 * time.Second

// LimitRequestBody bounds non-streaming requests before the contract validator buffers them; a body it cannot finish
// reading answers 408 REQUEST_INCOMPLETE, whatever stopped the read.
func LimitRequestBody(streaming map[string]struct{}, defaultLimit int64, routeLimits map[string]int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if streamingRequestBody(w, r, streaming, routeLimits) {
				next.ServeHTTP(w, r)
				return
			}
			limit := requestBodyLimit(r.Pattern, defaultLimit, routeLimits)
			if r.ContentLength > limit {
				WriteError(w, r, http.StatusRequestEntityTooLarge, CodeValidationFailed, bodyTooLarge(r, limit))
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
			_ = r.Body.Close()
			if int64(len(body)) > limit {
				WriteError(w, r, http.StatusRequestEntityTooLarge, CodeValidationFailed, bodyTooLarge(r, limit))
				return
			}
			if err != nil {
				WriteError(w, r, http.StatusRequestTimeout, CodeRequestIncomplete,
					TextFor(r, "Máy chủ chưa nhận đủ dữ liệu gửi lên. Vui lòng thử lại.", "The server did not receive the whole request. Try again."))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}

func bodyTooLarge(r *http.Request, limit int64) string {
	return fmt.Sprintf(TextFor(r, "Dữ liệu gửi lên vượt quá giới hạn %g MiB.", "The submitted data exceeds the %g MiB limit."), float64(limit)/(1<<20))
}

func streamingRequestBody(w http.ResponseWriter, r *http.Request, streaming map[string]struct{}, limits map[string]int64) bool {
	if r.Body == nil {
		return true
	}
	if _, ok := streaming[r.Pattern]; !ok {
		return false
	}
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(StreamingReadTimeout))
	if limit := limits[r.Pattern]; limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
	return true
}

func requestBodyLimit(pattern string, fallback int64, limits map[string]int64) int64 {
	if configured := limits[pattern]; configured > 0 {
		return configured
	}
	return fallback
}

var privateTrees = []string{"/auth/", "/app/", "/teacher/", "/admin/", "/me/"}

func privateTree(path string) bool {
	for _, prefix := range privateTrees {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
