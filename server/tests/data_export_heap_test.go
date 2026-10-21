//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"testing"
	"time"
)

const (
	bulkStudents        = 1000
	bulkAttemptsPerHead = 50
	exportHeapBudget    = 24 << 20
)

func (w *world) bulkAttempts(assignmentID, versionID, tag string) int {
	w.t.Helper()
	ctx := context.Background()
	var question string
	if err := w.pool.QueryRow(ctx, `SELECT q.id::text FROM app.test_version_questions q JOIN app.test_version_sections s ON s.id = q.test_version_section_id WHERE s.test_version_id = $1::uuid LIMIT 1`, versionID).Scan(&question); err != nil {
		w.t.Fatalf("the version's question: %v", err)
	}
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(ctx, `DELETE FROM app.attempts WHERE assignment_id = $1::uuid`, assignmentID); err != nil {
			w.t.Errorf("removing the bulk attempts: %v", err)
		}
		if _, err := w.pool.Exec(ctx, `DELETE FROM app.users WHERE email LIKE $1`, "bulk-"+tag+"-%"); err != nil {
			w.t.Errorf("removing the bulk students: %v", err)
		}
	})
	if _, err := w.pool.Exec(ctx, `
		INSERT INTO app.users (email, full_name, role_id)
		SELECT 'bulk-' || $1::text || '-' || g || '@example.com', 'Học viên số ' || g,
		       (SELECT id FROM app.roles WHERE builtin_key = 'student')
		  FROM generate_series(1, $2::int) g`, tag, bulkStudents); err != nil {
		w.t.Fatalf("bulk students: %v", err)
	}
	if _, err := w.pool.Exec(ctx, `
		INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed,
		                          beacon_token_hash, started_at, deadline_at, submitted_at, graded_at, score_earned, score_total)
		SELECT $1::uuid, $2::uuid, u.id, n, 'graded', gen_random_uuid(), 7, decode(repeat('ab', 32), 'hex'),
		       now() - interval '2 hours', now() - interval '1 hour', now() - interval '90 minutes', now() - interval '80 minutes', 1, 1
		  FROM app.users u CROSS JOIN generate_series(1, $4::int) n
		 WHERE u.email LIKE $3`, assignmentID, versionID, "bulk-"+tag+"-%", bulkAttemptsPerHead); err != nil {
		w.t.Fatalf("bulk attempts: %v", err)
	}
	if _, err := w.pool.Exec(ctx, `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload, auto_score)
		SELECT a.id, $2::uuid, jsonb_build_object('type', 'text', 'value', repeat('Một câu trả lời khá dài của học viên. ', 20)), 1
		  FROM app.attempts a WHERE a.assignment_id = $1::uuid`, assignmentID, question); err != nil {
		w.t.Fatalf("bulk answers: %v", err)
	}
	var n int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.attempts WHERE assignment_id = $1::uuid`, assignmentID).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func TestTheExportOfFiftyThousandAttemptsStreamsInBoundedMemory(t *testing.T) {
	w := bootAssistedWithStorage(t, nil)
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	teacherEmail, teacherPassword := w.createStaff("teacher")
	teacher := w.signedIn(teacherEmail, teacherPassword)
	tag := nonce(t)
	classID := teacher.class("Lớp khối lượng " + tag)
	_, versionID := teacher.publishedVersion("Đề khối lượng " + tag)
	assignment := teacher.assign(versionID, classID)
	if got := w.bulkAttempts(id(assignment), versionID, tag); got != bulkStudents*bulkAttemptsPerHead {
		t.Fatalf("the fixture holds %d attempts, want %d", got, bulkStudents*bulkAttemptsPerHead)
	}

	queued := admin.must(http.StatusAccepted, http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": true})
	runner := w.exportRunner()

	previous := debug.SetGCPercent(10)
	defer debug.SetGCPercent(previous)
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak atomic.Uint64
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > peak.Load() {
				peak.Store(m.HeapAlloc)
			}
			select {
			case <-done:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()
	started := time.Now()
	worked, err := runner.RunOne(context.Background())
	took := time.Since(started)
	close(done)
	<-stopped
	if err != nil || !worked {
		t.Fatalf("the worker ran %v with error %v", worked, err)
	}

	got := admin.must(http.StatusOK, http.MethodGet, "/admin/data-exports/"+id(queued), nil)
	if got["status"] != "ready" {
		t.Fatalf("the export reads %v", got)
	}
	growth := int64(peak.Load()) - int64(base.HeapAlloc)
	t.Logf("50,000 attempts and 50,000 answers: %.1f MiB zipped in %s, heap grew by at most %.1f MiB (budget %d MiB)",
		got["bytes"].(float64)/(1<<20), took.Round(time.Millisecond), float64(growth)/(1<<20), exportHeapBudget>>20)
	if growth > exportHeapBudget {
		t.Errorf("the heap grew by %.1f MiB while the export was built, over the %d MiB budget", float64(growth)/(1<<20), exportHeapBudget>>20)
	}
}
