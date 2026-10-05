//go:build integration

package application_test

import (
	"context"
	"crypto/sha256"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"
	"time"
)

type replacementWorld struct {
	pending     []<-chan replacementAnswer
	pool        *pgxpool.Pool
	repo        *repositories.Postgres
	users       []string
	a, b, actor string
}

func newReplacementWorld(t *testing.T, tracer pgx.QueryTracer) *replacementWorld {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("parse private database config")
	}
	if cfg.ConnConfig.Database == "" {
		t.Fatal("TEST_DATABASE_URL must name a database")
	}
	cfg.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal("create private database pool")
	}
	t.Cleanup(pool.Close)
	var name, role string
	var version int
	if err := pool.QueryRow(context.Background(), `SELECT current_database(),current_user,current_setting('server_version_num')::integer`).Scan(&name, &role, &version); err != nil {
		t.Fatal(err)
	}
	if name != cfg.ConnConfig.Database || version < 180000 || role != "quizzivy_migrate" {
		t.Fatalf("unexpected database boundary: %s %s %d", name, role, version)
	}
	t.Logf("private pool %s role %s PG %d", name, role, version)
	w := &replacementWorld{pool: pool, repo: repositories.NewPostgres(db.NewContext(pool))}
	t.Cleanup(func() { w.cleanup(t) })
	w.a = w.user(t)
	w.b = w.user(t)
	w.actor = w.user(t)
	return w
}
func (w *replacementWorld) user(t *testing.T) string {
	t.Helper()
	id := uuid.NewString()
	w.users = append(w.users, id)
	w.exec(t, `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Replacement fixture',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, id, id+"@example.test")
	return id
}
func (w *replacementWorld) cleanup(t *testing.T) {
	t.Helper()
	w.finishPending(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, sql := range []string{
		`DELETE FROM app.audit_log WHERE actor_user_id=ANY($1::uuid[])`,
		`DELETE FROM app.word_import_drafts WHERE import_id IN(SELECT id FROM app.word_imports WHERE created_by=ANY($1::uuid[]))`,
		`DELETE FROM app.word_import_runs WHERE import_id IN(SELECT id FROM app.word_imports WHERE created_by=ANY($1::uuid[]))`,
		`UPDATE app.word_imports SET source_revision=NULL WHERE created_by=ANY($1::uuid[])`,
		`DELETE FROM app.word_import_source_sets WHERE import_id IN(SELECT id FROM app.word_imports WHERE created_by=ANY($1::uuid[]))`,
		`DELETE FROM app.word_imports WHERE created_by=ANY($1::uuid[])`,
		`DELETE FROM app.attempts WHERE student_id=ANY($1::uuid[])`,
		`DELETE FROM app.assignments WHERE created_by=ANY($1::uuid[])`,
		`DELETE FROM app.test_section_units WHERE test_section_id IN(SELECT s.id FROM app.test_sections s JOIN app.tests t ON t.id=s.test_id WHERE t.owner_id=ANY($1::uuid[]))`,
		`DELETE FROM app.test_section_questions WHERE test_section_id IN(SELECT s.id FROM app.test_sections s JOIN app.tests t ON t.id=s.test_id WHERE t.owner_id=ANY($1::uuid[]))`,
		`DELETE FROM app.questions WHERE owner_id=ANY($1::uuid[])`,
		`DELETE FROM app.test_versions WHERE test_id IN(SELECT id FROM app.tests WHERE owner_id=ANY($1::uuid[]))`,
		`DELETE FROM app.group_stimulus_assets WHERE group_id IN(SELECT id FROM app.question_groups WHERE owner_id=ANY($1::uuid[]))`,
		`DELETE FROM app.group_stimuli WHERE group_id IN(SELECT id FROM app.question_groups WHERE owner_id=ANY($1::uuid[]))`,
		`DELETE FROM app.group_recordings WHERE group_id IN(SELECT id FROM app.question_groups WHERE owner_id=ANY($1::uuid[]))`,
		`DELETE FROM app.question_groups WHERE owner_id=ANY($1::uuid[])`,
		`DELETE FROM app.tests WHERE owner_id=ANY($1::uuid[])`,
		`DELETE FROM app.media_assets WHERE owner_id=ANY($1::uuid[]) OR uploaded_by=ANY($1::uuid[])`,
		`DELETE FROM app.users WHERE id=ANY($1::uuid[])`,
	} {
		if _, err := w.pool.Exec(ctx, sql, w.users); err != nil {
			t.Errorf("owned replacement cleanup: %v", err)
		}
	}
	var count int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE id=ANY($1::uuid[])`, w.users).Scan(&count); err != nil || count != 0 {
		t.Errorf("owned users remain=%d error=%v", count, err)
	}
}
func (w *replacementWorld) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}
func (w *replacementWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func (w *replacementWorld) asset(t *testing.T, owner string, kind domain.Kind, size int64) string {
	t.Helper()
	duration := (*int)(nil)
	mime := "image/png"
	if kind == domain.KindAudio {
		duration = new(1000)
		mime = "audio/mpeg"
	}
	return w.id(t, `INSERT INTO app.media_assets(kind,storage_key,mime_type,bytes,duration_ms,original_filename,checksum_sha256,uploaded_by,owner_id,display_name,default_max_plays) VALUES($1::app.media_kind,$2,$3,$4,$5,'original.png',sha256(convert_to($2,'UTF8')),$6,$6,'Tên gốc',CASE WHEN $1='audio' THEN 3 END) RETURNING id::text`, string(kind), "replace-fixture/"+uuid.NewString(), mime, size, duration, owner)
}
func (w *replacementWorld) input(old, owner string, kind domain.Kind, size, quota int64) domain.ReplaceInput {
	id := uuid.NewString()
	sum := sha256.Sum256([]byte(id))
	duration := (*int)(nil)
	mime := "image/png"
	if kind == domain.KindAudio {
		duration = new(2000)
		mime = "audio/mpeg"
	}
	return domain.ReplaceInput{ID: old, Scope: access.Scope{UserID: w.actor, All: true}, Asset: domain.InsertInput{ID: id, Kind: kind, StorageKey: "replace-fixture/" + id, MimeType: mime, Bytes: size, DurationMs: duration, OriginalFilename: "replacement.png", ChecksumSHA256: sum[:], OwnerID: owner, UploaderID: w.actor, QuotaBytes: quota, Now: time.Now()}}
}
func (w *replacementWorld) group(t *testing.T, owner string, section *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.question_groups(title,created_by,owner_id,owner_section_id) VALUES('Nhóm',$1,$1,$2) RETURNING id::text`, owner, section)
}
func (w *replacementWorld) question(t *testing.T, owner, asset string, kind domain.Kind, group *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.questions(type,prompt,points,created_by,owner_id,media_asset_id,media_asset_kind,audio_allow_seek,audio_show_transcript_after,context_group_id,context_ordinal,context_option_order) VALUES('short_answer','Hỏi',1,$1,$1,$2,$3::app.media_kind,$4,$4,$5::uuid,CASE WHEN $5::uuid IS NOT NULL THEN (SELECT count(*) FROM app.questions WHERE context_group_id=$5)::integer END,CASE WHEN $5::uuid IS NOT NULL THEN 'shuffle' END) RETURNING id::text`, owner, asset, string(kind), audioPolicy(kind), group)
}
func (w *replacementWorld) stimulus(t *testing.T, group, asset string) string {
	t.Helper()
	raw := `{"format":"semantic_v1","blocks":[{"type":"image","assetId":"` + asset + `","alt":"Giữ nguyên"},{"type":"paragraph","content":[{"type":"text","text":"Nội dung","marks":["bold"]}]}]}`
	id := w.id(t, `INSERT INTO app.group_stimuli(group_id,ordinal,title,content) VALUES($1,0,'Ngữ liệu',$2) RETURNING id::text`, group, []byte(raw))
	w.exec(t, `INSERT INTO app.group_stimulus_assets(stimulus_id,group_id,media_asset_id,media_asset_kind) VALUES($1,$2,$3,'image')`, id, group, asset)
	return id
}
func (w *replacementWorld) section(t *testing.T, owner string) (string, string) {
	t.Helper()
	test := w.id(t, `INSERT INTO app.tests(title,status,created_by,owner_id) VALUES('Đề','draft',$1,$1) RETURNING id::text`, owner)
	section := w.id(t, `INSERT INTO app.test_sections(test_id,ordinal,title) VALUES($1,0,'Phần') RETURNING id::text`, test)
	return test, section
}
func (w *replacementWorld) scalar(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func (w *replacementWorld) text(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var s string
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (w *replacementWorld) finishPending(t *testing.T) {
	t.Helper()
	for _, answer := range w.pending {
		select {
		case <-answer:
		case <-time.After(12 * time.Second):
			t.Error("owned replacement did not stop before fixture cleanup")
		}
	}
}
