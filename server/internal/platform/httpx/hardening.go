package httpx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		if strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/app/") || strings.HasPrefix(r.URL.Path, "/admin/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// LimitRequestBody bounds non-streaming requests before the contract validator buffers them.
func LimitRequestBody(streaming map[string]struct{}, defaultLimit int64, routeLimits map[string]int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, exempt := streaming[r.Pattern]; exempt || r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}
			limit := requestBodyLimit(r.Pattern, defaultLimit, routeLimits)
			message := fmt.Sprintf("Dữ liệu gửi lên vượt quá giới hạn %g MiB.", float64(limit)/(1<<20))
			if r.ContentLength > limit {
				WriteError(w, r, http.StatusRequestEntityTooLarge, CodeValidationFailed, message)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
			_ = r.Body.Close()
			if int64(len(body)) > limit {
				WriteError(w, r, http.StatusRequestEntityTooLarge, CodeValidationFailed, message)
				return
			}
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, "Không đọc được dữ liệu gửi lên.")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}

func requestBodyLimit(pattern string, fallback int64, limits map[string]int64) int64 {
	if configured := limits[pattern]; configured > 0 {
		return configured
	}
	return fallback
}
