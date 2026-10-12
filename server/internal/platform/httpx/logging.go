package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type recorderKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	writeErr    error
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	if s.wroteHeader {
		return
	}
	s.wroteHeader = code >= http.StatusOK
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (s *statusRecorder) Write(p []byte) (int, error) {
	s.wroteHeader = true
	n, err := s.ResponseWriter.Write(p)
	if err != nil && s.writeErr == nil {
		s.writeErr = err
	}
	return n, err
}

// Logging emits one structured line per request, keyed by the same request id
// the client sees. The status it logs is the last one asked for, so a request
// answered by StatusClientClosedRequest after its response began still reads
// 499, though the connection has carried only the first. It also remembers the
// first failed write of the response, which ClientGone reads.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), recorderKey{}, rec)))
			logger.Info(
				"http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFromContext(r.Context()),
			)
		})
	}
}
