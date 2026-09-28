//go:build e2e

package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/platform/config"
)

func storageEnv(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func bootWithStorage(t *testing.T) *world {
	t.Helper()
	work := t.TempDir()
	if err := os.Chmod(work, 0o700); err != nil {
		t.Fatal(err)
	}
	return boot(t, func(c *config.Config) {
		c.S3Endpoint = storageEnv("S3_ENDPOINT", "http://localhost:9000")
		c.S3Region = storageEnv("S3_REGION", "us-east-1")
		c.S3Bucket = storageEnv("S3_BUCKET", "quizzivy-media")
		c.S3AccessKeyID = storageEnv("S3_ACCESS_KEY_ID", "quizzivy")
		c.S3SecretAccessKey = storageEnv("S3_SECRET_ACCESS_KEY", "quizzivy-dev-secret")
		c.S3ForcePathStyle = true
		c.SignedURLTTL = 10 * time.Minute
		c.ImportBucket = storageEnv("IMPORT_S3_BUCKET", "quizzivy-imports")
		c.ImportWorkDir = work
		c.ImportActorCount, c.ImportGlobalCount, c.ImportSourcesPerItem = 100, 1000, 32
		c.ImportActorMiB, c.ImportGlobalMiB = 256, 1024
		c.ImportProcessing = true
	})
}

type sent struct {
	status int
	body   []byte
	json   map[string]any
}

type payload struct {
	contentType string
	data        []byte
}

func jsonPayload(t *testing.T, v any) payload {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return payload{contentType: "application/json", data: raw}
}

func filePayload(t *testing.T, filename string, data []byte) payload {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return payload{contentType: w.FormDataContentType(), data: b.Bytes()}
}

func (c *client) send(method, path string, body *payload) sent {
	c.w.t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body.data)
	}
	req, err := http.NewRequest(method, c.w.server.URL+path, reader)
	if err != nil {
		c.w.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", body.contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.w.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	out := sent{status: resp.StatusCode, body: raw}
	var decoded map[string]any
	if json.Unmarshal(raw, &decoded) == nil {
		out.json = decoded
	}
	return out
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func tinyDocx(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Câu 1. Đề cách ly</w:t></w:r></w:p></w:body></w:document>`,
	} {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type party struct {
	c       *client
	userID  string
	ids     map[string]string
	session string
	beacon  string
	code    string
	student *party
}

func (p *party) id(kind string) string {
	p.c.w.t.Helper()
	v, ok := p.ids[kind]
	if !ok {
		p.c.w.t.Fatalf("the world holds no %s", kind)
	}
	return v
}

func (w *world) signedIn(email, password string) *client {
	c := w.browser()
	c.login(email, password)
	return c
}

func (w *world) teacherWorld(name string) *party {
	w.t.Helper()
	email, password := w.createStaff("teacher")
	c := w.signedIn(email, password)
	me := c.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	p := &party{c: c, userID: me["id"].(string), ids: map[string]string{}}

	upload := c.send(http.MethodPost, "/teacher/media", new(filePayload(w.t, name+".png", tinyPNG(w.t))))
	if upload.status != http.StatusCreated {
		w.t.Fatalf("%s's image upload: %d %s", name, upload.status, upload.body)
	}
	p.ids["media"] = upload.json["id"].(string)
	listening, err := os.ReadFile("../../web/tests/e2e/fixtures/unit5-listening.mp3")
	if err != nil {
		w.t.Fatal(err)
	}
	audio := c.send(http.MethodPost, "/teacher/media", new(filePayload(w.t, name+".mp3", listening)))
	if audio.status != http.StatusCreated {
		w.t.Fatalf("%s's audio upload: %d %s", name, audio.status, audio.body)
	}
	p.ids["audio"] = audio.json["id"].(string)

	question := c.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "single_choice", "prompt": "Từ nào là danh từ?", "points": 1, "mediaAssetId": p.ids["media"],
		"options": []map[string]any{{"text": "nhanh", "isCorrect": false}, {"text": "giáo viên", "isCorrect": true}},
	})
	p.ids["question"] = id(question)
	c.must(http.StatusOK, http.MethodPost, "/teacher/questions/tags", map[string]any{"questionIds": []string{p.ids["question"]}, "tags": []string{"cách ly"}})
	test := c.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề cách ly " + name + " " + nonce(w.t)})
	p.ids["test"] = id(test)
	test = c.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []map[string]any{{"title": "Phần 1", "questionIds": []string{p.ids["question"]}}},
	})
	p.ids["section"] = test["sections"].([]any)[0].(map[string]any)["id"].(string)
	versionID := id(c.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+p.ids["test"]+"/publish", nil))
	p.ids["test-version"] = versionID
	test = c.must(http.StatusOK, http.MethodGet, "/teacher/tests/"+p.ids["test"], nil)
	c.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+p.ids["test"], map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "title": "Đề cách ly có bản nháp " + name,
	})

	group := c.must(http.StatusCreated, http.MethodPost, "/teacher/question-groups", map[string]any{
		"bundle": map[string]any{
			"group":     map[string]any{"id": uuid.NewString(), "title": "Nhóm " + name, "members": []any{}, "stimuli": []any{}, "recordings": []any{}},
			"questions": []any{},
		},
	})
	p.ids["question-group"] = group["bundle"].(map[string]any)["group"].(map[string]any)["id"].(string)

	imported := c.must(http.StatusCreated, http.MethodPost, "/teacher/imports", map[string]any{"requestId": uuid.NewString(), "title": "Nhập " + name})
	p.ids["import"] = id(imported)
	source := c.send(http.MethodPost, "/teacher/imports/"+id(imported)+"/sources?role=exam&uploadId="+uuid.NewString()+"&expectedRevision="+jsonNumber(imported["revision"]),
		new(filePayload(w.t, "de-"+name+".docx", tinyDocx(w.t))))
	if source.status != http.StatusOK && source.status != http.StatusCreated {
		w.t.Fatalf("%s's import source: %d %s", name, source.status, source.body)
	}
	uploaded, _ := source.json["source"].(map[string]any)
	if uploaded == nil {
		w.t.Fatalf("%s's import source upload named no source: %s", name, source.body)
	}
	p.ids["import-source"] = uploaded["id"].(string)
	p.ids["reviewed-import"] = w.reviewedImport(c, p.userID, name)

	p.ids["class"] = c.class("Lớp cách ly " + name)
	p.code = c.must(http.StatusCreated, http.MethodPost, "/teacher/classes/"+p.ids["class"]+"/join-code", map[string]any{})["code"].(string)
	created := c.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "hocvien-" + name + "-" + nonce(w.t) + "@example.com", "fullName": "Học viên " + name, "classIds": []string{p.ids["class"]},
	})
	user := created["user"].(map[string]any)
	p.ids["student"] = user["id"].(string)

	assignment := c.assign(versionID, p.ids["class"])
	p.ids["assignment"] = id(assignment)

	learner := w.signedIn(user["email"].(string), created["temporaryPassword"].(string))
	session := learner.must(http.StatusOK, http.MethodPost, "/app/assignments/"+p.ids["assignment"]+"/attempts", nil)
	attempt := session["attempt"].(map[string]any)
	p.ids["attempt"] = id(attempt)
	p.ids["version-question"] = id(session["questions"].([]any)[0].(map[string]any))
	learner.must(http.StatusOK, http.MethodPost, "/app/attempts/"+p.ids["attempt"]+"/submit",
		map[string]any{"sessionId": session["sessionId"], "reason": "manual"})

	sitting := c.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "dang-lam-" + name + "-" + nonce(w.t) + "@example.com", "fullName": "Học viên đang làm " + name, "classIds": []string{p.ids["class"]},
	})
	sitter := w.signedIn(sitting["user"].(map[string]any)["email"].(string), sitting["temporaryPassword"].(string))
	listened := c.assign(w.listeningVersion(p), p.ids["class"])
	open := sitter.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(listened)+"/attempts", nil)
	stimulus := open["groups"].([]any)[0].(map[string]any)
	p.student = &party{c: sitter, userID: sitting["user"].(map[string]any)["id"].(string), session: open["sessionId"].(string), beacon: open["beaconToken"].(string), ids: map[string]string{
		"assignment": id(listened), "attempt": id(open["attempt"].(map[string]any)),
		"version-question":  id(open["questions"].([]any)[0].(map[string]any)),
		"version-recording": stimulus["recordings"].([]any)[0].(map[string]any)["id"].(string),
		"media":             p.ids["media"],
	}}
	return p
}

func (w *world) sharedStudent(p *party) (*client, string) {
	w.t.Helper()
	created := p.c.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "dung-chung-" + nonce(w.t) + "@example.com", "fullName": "Học viên học hai lớp", "classIds": []string{p.ids["class"]},
	})
	user := created["user"].(map[string]any)
	return w.signedIn(user["email"].(string), created["temporaryPassword"].(string)), user["id"].(string)
}

func (w *world) listeningVersion(p *party) string {
	w.t.Helper()
	test := p.c.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề nghe " + nonce(w.t)})
	test = p.c.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "sections": []map[string]any{{"title": "Phần nghe", "questionIds": []string{}}},
	})
	p.ids["listening-test"] = id(test)
	p.ids["listening-section"] = test["sections"].([]any)[0].(map[string]any)["id"].(string)
	group := p.c.must(http.StatusCreated, http.MethodPost, "/teacher/question-groups", map[string]any{
		"bundle":                groupBundle(p, uuid.NewString(), "/bundle/group/recordings/-/assetId"),
		"ownerSectionId":        p.ids["listening-section"],
		"expectedTestUpdatedAt": test["updatedAt"],
	})
	p.ids["listening-group"] = group["bundle"].(map[string]any)["group"].(map[string]any)["id"].(string)
	return id(p.c.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil))
}

func (w *world) reviewedImport(c *client, owner, name string) string {
	w.t.Helper()
	imported := c.must(http.StatusCreated, http.MethodPost, "/teacher/imports", map[string]any{"requestId": uuid.NewString(), "title": "Nhập đã xử lý " + name})
	upload := c.send(http.MethodPost, "/teacher/imports/"+id(imported)+"/sources?role=exam&uploadId="+uuid.NewString()+"&expectedRevision="+jsonNumber(imported["revision"]),
		new(filePayload(w.t, "da-xu-ly-"+name+".docx", tinyDocx(w.t))))
	if upload.status != http.StatusOK && upload.status != http.StatusCreated {
		w.t.Fatalf("%s's reviewed import source: %d %s", name, upload.status, upload.body)
	}
	if _, err := w.pool.Exec(context.Background(), `
		WITH run AS (
		  INSERT INTO app.word_import_runs (import_id, source_revision, request_id, requested_by, expected_revision, pipeline_version,
		                                    status, stage, result, completed_at)
		  SELECT i.id, i.source_revision, gen_random_uuid(), $2::uuid, i.revision, 'isolation-seed', 'succeeded', 'ready', '{}'::jsonb, now()
		    FROM app.word_imports i WHERE i.id = $1::uuid
		  RETURNING id, import_id
		), draft AS (
		  INSERT INTO app.word_import_drafts (import_id, run_id, body)
		  SELECT import_id, id, jsonb_build_object('title', 'Nhập đã xử lý', 'sections', '[]'::jsonb) FROM run
		  RETURNING import_id
		)
		UPDATE app.word_imports SET status = 'needs_review' WHERE id = (SELECT import_id FROM draft)`, id(imported), owner); err != nil {
		w.t.Fatalf("%s's reviewed import: %v", name, err)
	}
	return id(imported)
}

func jsonNumber(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func (w *world) snapshotOf(p *party) string {
	w.t.Helper()
	var digest string
	if err := w.pool.QueryRow(context.Background(), `
		WITH rows(r) AS (
		  SELECT row_to_json(x)::text FROM app.tests x WHERE x.owner_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.test_versions x JOIN app.tests t ON t.id = x.test_id WHERE t.owner_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.test_sections x JOIN app.tests t ON t.id = x.test_id WHERE t.owner_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.questions x WHERE x.owner_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.question_groups x WHERE x.owner_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.media_assets x WHERE x.owner_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.word_imports x WHERE x.created_by = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.word_import_sources x JOIN app.word_imports i ON i.id = x.import_id WHERE i.created_by = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.classes x WHERE x.teacher_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.class_join_codes x JOIN app.classes c ON c.id = x.class_id WHERE c.teacher_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.class_members x JOIN app.classes c ON c.id = x.class_id WHERE c.teacher_id = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.assignments x WHERE x.created_by = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.attempts x JOIN app.assignments a ON a.id = x.assignment_id WHERE a.created_by = $1::uuid
		  UNION ALL SELECT row_to_json(x)::text FROM app.attempt_answers x JOIN app.attempts at ON at.id = x.attempt_id JOIN app.assignments a ON a.id = at.assignment_id WHERE a.created_by = $1::uuid
		  UNION ALL SELECT row_to_json(u)::text FROM app.users u WHERE u.id = $2::uuid
		)
		SELECT md5(string_agg(r, '|' ORDER BY r)) || ':' || count(*) FROM rows`, p.userID, p.ids["student"]).Scan(&digest); err != nil {
		w.t.Fatalf("snapshot: %v", err)
	}
	return digest
}

func (p *party) all() []string {
	out := []string{p.userID}
	for _, v := range p.ids {
		out = append(out, v)
	}
	if p.student != nil {
		out = append(out, p.student.all()...)
	}
	return out
}

func (w *world) rowsNaming(ids []string, skip ...string) []string {
	w.t.Helper()
	ctx := context.Background()
	rows, err := w.pool.Query(ctx, `SELECT table_name::text FROM information_schema.tables WHERE table_schema = 'app' AND table_type = 'BASE TABLE'`)
	if err != nil {
		w.t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		w.t.Fatal(err)
	}
	var patterns []string
	for _, v := range ids {
		patterns = append(patterns, "%"+v+"%")
	}
	var out []string
	for _, table := range tables {
		if slices.Contains(skip, table) {
			continue
		}
		rows, err := w.pool.Query(ctx, `SELECT row_to_json(x)::text FROM `+pgx.Identifier{"app", table}.Sanitize()+` x WHERE row_to_json(x)::text LIKE ANY($1::text[])`, patterns)
		if err != nil {
			w.t.Fatal(err)
		}
		found, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			w.t.Fatal(err)
		}
		for _, row := range found {
			out = append(out, table+" "+row)
		}
	}
	sort.Strings(out)
	return out
}

func (w *world) crossReferences(a, b []string, skip ...string) []string {
	w.t.Helper()
	var out []string
	for _, row := range w.rowsNaming(a, skip...) {
		if slices.ContainsFunc(b, func(v string) bool { return strings.Contains(row, v) }) {
			out = append(out, row)
		}
	}
	return out
}

func mentions(body []byte, p *party) []string {
	var out []string
	for kind, v := range p.ids {
		if strings.Contains(string(body), v) {
			out = append(out, kind)
		}
	}
	return out
}
