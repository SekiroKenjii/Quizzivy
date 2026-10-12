package httpx

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"
)

// MaintenanceSource tells the maintenance gate whether a window is under way.
type MaintenanceSource interface {
	// ActiveWindow returns the maintenance window under way now, if any.
	ActiveWindow(ctx context.Context) (startsAt, endsAt time.Time, active bool)
}

// MaintenanceRefusal runs on a request the maintenance gate refuses, before
// the 503 is written, so a caller can add headers to that answer.
type MaintenanceRefusal func(w http.ResponseWriter, r *http.Request)

var maintenanceExempt = map[string]bool{"/livez": true, "/healthz": true, "/public/status": true}

// Maintenance answers every request with 503 MAINTENANCE while a window is
// under way: the window in details, Retry-After in seconds until it ends, and a
// message in Vietnamese or, when Accept-Language prefers it, English. It lets
// GET /livez, GET /healthz and GET /public/status through, so health checks
// pass and a client can ask when the window ends. It belongs between CORS and
// the router: CORS answers preflights and labels the 503 so the SPA can read
// it, and no route, however authenticated, gets past it. A nil source is no
// gate. Each onRefuse runs on a refused request before the answer is written.
func Maintenance(source MaintenanceSource, onRefuse ...MaintenanceRefusal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if source == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if (r.Method == http.MethodGet || r.Method == http.MethodHead) && maintenanceExempt[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			startsAt, endsAt, active := source.ActiveWindow(r.Context())
			if !active {
				next.ServeHTTP(w, r)
				return
			}
			wait := int(math.Ceil(time.Until(endsAt).Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(max(wait, 1)))
			for _, refuse := range onRefuse {
				refuse(w, r)
			}
			WriteErrorWithDetails(w, r, http.StatusServiceUnavailable, CodeMaintenance, maintenanceMessage(r),
				map[string]any{
					"startsAt": startsAt.UTC().Format(time.RFC3339),
					"endsAt":   endsAt.UTC().Format(time.RFC3339),
				})
		})
	}
}

func maintenanceMessage(r *http.Request) string {
	return TextFor(r, "Quizzivy đang được cập nhật. Vui lòng quay lại khi cập nhật xong.", "Quizzivy is being updated. Please come back when the update ends.")
}
