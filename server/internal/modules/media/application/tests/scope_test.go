//go:build integration

package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type mediaScope struct {
	tx                     pgx.Tx
	svc                    *application.Application
	a, b, admin            string
	scopeA, scopeB, anyone access.Scope
}

func newMediaScope(t *testing.T) *mediaScope {
	t.Helper()
	pool := newPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	w := &mediaScope{tx: tx, svc: application.New(repositories.NewPostgres(db.NewContext(tx)), newFakeStore(), audioProbe{})}
	w.a, w.b, w.admin = w.user(t, "teacher"), w.user(t, "teacher"), w.user(t, "admin")
	w.scopeA, w.scopeB, w.anyone = access.Scope{UserID: w.a}, access.Scope{UserID: w.b}, access.Scope{UserID: w.admin, All: true}
	return w
}

func (w *mediaScope) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *mediaScope) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (w *mediaScope) user(t *testing.T, builtin string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Media scope', (SELECT id FROM app.roles WHERE builtin_key = $2)) RETURNING id::text`,
		uuid.NewString()+"@example.test", builtin)
}

func (w *mediaScope) asset(t *testing.T, owner string, kind domain.Kind) string {
	t.Helper()
	if kind == domain.KindAudio {
		return upload(t, w.svc, owner, "pham-vi.mp3").ID
	}
	return w.id(t, `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
		VALUES ('image', $1, 'image/png', 1, 'hinh.png', sha256(convert_to($1, 'UTF8')), $2, $2) RETURNING id::text`, "scope/"+uuid.NewString(), owner)
}

func audioPolicy(kind domain.Kind) *bool {
	if kind != domain.KindAudio {
		return nil
	}
	off := false
	return &off
}

func (w *mediaScope) question(t *testing.T, owner, asset string, kind domain.Kind, group *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after,
		                           context_group_id, context_ordinal, context_option_order)
		VALUES ('short_answer', 'Phạm vi', 1, $1, $1, $2, $3::app.media_kind, $4, $4,
		        $5::uuid, CASE WHEN $5::uuid IS NOT NULL THEN 0 END, CASE WHEN $5::uuid IS NOT NULL THEN 'shuffle' END) RETURNING id::text`, owner, asset, string(kind), audioPolicy(kind), group)
}

func (w *mediaScope) test(t *testing.T, owner string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề phạm vi', 'published', 1, $1, $1) RETURNING id::text`, owner)
}

func (w *mediaScope) group(t *testing.T, owner string, section *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.question_groups (title, created_by, owner_id, owner_section_id) VALUES ('Nhóm phạm vi', $1, $1, $2) RETURNING id::text`, owner, section)
}

func (w *mediaScope) stimulus(t *testing.T, group, asset string) string {
	t.Helper()
	stimulus := w.id(t, `INSERT INTO app.group_stimuli (group_id, ordinal, title, content) VALUES ($1, 0, 'Ngữ liệu', '{"format":"semantic_v1"}') RETURNING id::text`, group)
	w.exec(t, `INSERT INTO app.group_stimulus_assets (stimulus_id, group_id, media_asset_id, media_asset_kind) VALUES ($1, $2, $3, 'image')`, stimulus, group, asset)
	return stimulus
}

func (w *mediaScope) versionSection(t *testing.T, test string) string {
	t.Helper()
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) SELECT id, 1, 1, owner_id FROM app.tests WHERE id = $1 RETURNING id::text`, test)
	return w.id(t, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, version)
}

func (w *mediaScope) versionGroup(t *testing.T, test string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.test_version_groups (test_version_section_id, title) VALUES ($1, 'Nhóm đã xuất bản') RETURNING id::text`, w.versionSection(t, test))
}

func (w *mediaScope) removeTest(t *testing.T, test string) {
	t.Helper()
	w.exec(t, `UPDATE app.tests SET deleted_at = now() WHERE id = $1`, test)
}

func (w *mediaScope) readable(t *testing.T, scope access.Scope, ids ...string) map[string]domain.Kind {
	t.Helper()
	kinds, err := w.svc.Queries.Readable.Handle(context.Background(), query.Readable{Scope: scope, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	return kinds
}

func (w *mediaScope) reads(t *testing.T, scope access.Scope, id string) bool {
	t.Helper()
	_, ok := w.readable(t, scope, id)[id]
	return ok
}

func (w *mediaScope) list(t *testing.T, scope access.Scope, kind *domain.Kind) query.ListResult {
	t.Helper()
	result, err := w.svc.Queries.List.Handle(context.Background(), query.List{Input: domain.ListInput{Scope: scope, Kind: kind, Limit: repositories.MaxLimit}})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (w *mediaScope) totalBytes(t *testing.T, scope access.Scope, kind *domain.Kind) int64 {
	t.Helper()
	total, err := w.svc.Queries.TotalBytes.Handle(context.Background(), query.TotalBytes{Scope: scope, Kind: kind})
	if err != nil {
		t.Fatal(err)
	}
	return total
}

func (w *mediaScope) remove(scope access.Scope, id string) error {
	_, err := w.svc.Commands.Delete.Handle(context.Background(), command.Delete{Input: domain.DeleteInput{ID: id, ActorID: scope.UserID, All: scope.All}})
	return err
}

func (w *mediaScope) deleted(t *testing.T, id string) bool {
	t.Helper()
	var deleted bool
	if err := w.tx.QueryRow(context.Background(), `SELECT deleted_at IS NOT NULL FROM app.media_assets WHERE id = $1`, id).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	return deleted
}

func ids(assets []domain.Asset) []string {
	out := make([]string, len(assets))
	for i, a := range assets {
		out[i] = a.ID
	}
	return out
}

func TestAnUploadBelongsToItsUploader(t *testing.T) {
	w := newMediaScope(t)
	asset := upload(t, w.svc, w.a, "cua-a.mp3")
	var owner, uploader string
	if err := w.tx.QueryRow(context.Background(), `SELECT owner_id::text, uploaded_by::text FROM app.media_assets WHERE id = $1`, asset.ID).Scan(&owner, &uploader); err != nil {
		t.Fatal(err)
	}
	if owner != w.a || uploader != w.a {
		t.Errorf("the upload is owned by %s and recorded as uploaded by %s, want both %s", owner, uploader, w.a)
	}
}

func TestTheDuplicateWarningCountsOnlyTheOwnersUploads(t *testing.T) {
	w := newMediaScope(t)
	mine := upload(t, w.svc, w.a, "trung.mp3")
	upload(t, w.svc, w.b, "trung.mp3")
	store := repositories.NewPostgres(db.NewContext(w.tx))
	for owner, want := range map[string]int{w.a: 1, w.b: 1, w.admin: 0} {
		if got, err := store.CountByChecksum(context.Background(), owner, mine.ChecksumSHA256); err != nil || got != want {
			t.Errorf("%s's copies of the same bytes: %d (%v), want %d", owner, got, err, want)
		}
	}
}

func TestTheLibraryListsAndTotalsOnlyTheCallersOwnAssets(t *testing.T) {
	w := newMediaScope(t)
	mine := upload(t, w.svc, w.a, "cua-a.mp3")
	theirs := upload(t, w.svc, w.b, "cua-b.mp3")
	audio, image := domain.KindAudio, domain.KindImage

	for name, c := range map[string]struct {
		scope access.Scope
		kind  *domain.Kind
		want  []string
		bytes int64
	}{
		"A":              {w.scopeA, nil, []string{mine.ID}, mine.Bytes},
		"A's audio":      {w.scopeA, &audio, []string{mine.ID}, mine.Bytes},
		"A's images":     {w.scopeA, &image, []string{}, 0},
		"B":              {w.scopeB, nil, []string{theirs.ID}, theirs.Bytes},
		"the zero scope": {access.Scope{}, nil, []string{}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			result := w.list(t, c.scope, c.kind)
			if got := ids(result.Items); !slices.Equal(got, c.want) || result.Page.Total != len(c.want) {
				t.Errorf("the library lists %v with total %d, want %v", got, result.Page.Total, c.want)
			}
			if got := w.totalBytes(t, c.scope, c.kind); got != c.bytes {
				t.Errorf("the library totals %d bytes, want %d", got, c.bytes)
			}
		})
	}

	everything := ids(w.list(t, w.anyone, nil).Items)
	if !slices.Contains(everything, mine.ID) || !slices.Contains(everything, theirs.ID) {
		t.Errorf("scope.all lists %v, want both teachers' assets", everything)
	}
	if got := w.totalBytes(t, w.anyone, nil); got < mine.Bytes+theirs.Bytes {
		t.Errorf("scope.all totals %d bytes, want at least both teachers' %d", got, mine.Bytes+theirs.Bytes)
	}
}

func TestAnotherTeachersAssetCannotBeDeletedOrSeenInUse(t *testing.T) {
	w := newMediaScope(t)
	used := upload(t, w.svc, w.a, "dang-dung.mp3")
	w.exec(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after)
		VALUES ($1, 0, 'short_answer', 'Nghe', 1, $2, 'audio', false, false)`, w.versionSection(t, w.test(t, w.a)), used.ID)
	spare := upload(t, w.svc, w.a, "du-phong.mp3")

	for name, id := range map[string]string{"in use": used.ID, "unused": spare.ID, "missing": uuid.NewString()} {
		if err := w.remove(w.scopeB, id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B deleting A's %s asset: %v, want the not-found a missing asset gets", name, err)
		}
	}
	if w.deleted(t, used.ID) || w.deleted(t, spare.ID) {
		t.Error("B's refused delete still deleted A's asset")
	}
	var audited int
	if err := w.tx.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_log WHERE action = 'media.deleted' AND entity_id = ANY($1::uuid[])`, []string{used.ID, spare.ID}).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited != 0 {
		t.Errorf("B's refused deletes left %d audit rows", audited)
	}

	var blocked *domain.ReferencedError
	if err := w.remove(w.scopeA, used.ID); !errors.As(err, &blocked) || len(blocked.Tests) != 1 {
		t.Errorf("A deleting their asset in use: %v, want the refusal naming A's test", err)
	}
	var found bool
	for _, a := range w.list(t, w.scopeA, nil).Items {
		if a.ID == used.ID {
			found = true
			if a.UsageCount != 1 || len(a.UsedIn) != 1 || a.UsedIn[0].Version != 1 {
				t.Errorf("A's library shows usage %d in %+v, want 1 in version 1", a.UsageCount, a.UsedIn)
			}
		}
	}
	if !found {
		t.Error("A's asset in use is missing from A's library")
	}

	if err := w.remove(w.anyone, spare.ID); err != nil || !w.deleted(t, spare.ID) {
		t.Errorf("scope.all deleting A's unused asset: %v", err)
	}
}

func TestReadableFollowsOwnershipAndScopeAll(t *testing.T) {
	w := newMediaScope(t)
	image := w.asset(t, w.a, domain.KindImage)
	audio := w.asset(t, w.a, domain.KindAudio)
	theirs := w.asset(t, w.b, domain.KindImage)
	missing := uuid.NewString()

	for name, c := range map[string]struct {
		scope access.Scope
		want  map[string]domain.Kind
	}{
		"the owner":      {w.scopeA, map[string]domain.Kind{image: domain.KindImage, audio: domain.KindAudio}},
		"B":              {w.scopeB, map[string]domain.Kind{theirs: domain.KindImage}},
		"scope.all":      {w.anyone, map[string]domain.Kind{image: domain.KindImage, audio: domain.KindAudio, theirs: domain.KindImage}},
		"the zero scope": {access.Scope{}, map[string]domain.Kind{}},
	} {
		t.Run(name, func(t *testing.T) {
			got := w.readable(t, c.scope, image, audio, theirs, missing)
			if len(got) != len(c.want) {
				t.Errorf("reads %v, want %v", got, c.want)
			}
			for id, kind := range c.want {
				if got[id] != kind {
					t.Errorf("reads %s as %q, want %q", id, got[id], kind)
				}
			}
		})
	}

	w.exec(t, `UPDATE app.media_assets SET deleted_at = now() WHERE id = $1`, image)
	if w.reads(t, w.scopeA, image) || w.reads(t, w.anyone, image) {
		t.Error("a deleted asset is still readable")
	}
}

func TestReadableReachesAnAssetThroughWhatTheCallerOwns(t *testing.T) {
	w := newMediaScope(t)
	for name, c := range map[string]struct {
		kind  domain.Kind
		grant func(t *testing.T, asset string) func()
	}{
		"a live bank question": {domain.KindImage, func(t *testing.T, asset string) func() {
			question := w.question(t, w.b, asset, domain.KindImage, nil)
			return func() { w.exec(t, `UPDATE app.questions SET deleted_at = now() WHERE id = $1`, question) }
		}},
		"a member question of a bank group": {domain.KindImage, func(t *testing.T, asset string) func() {
			group := w.group(t, w.b, nil)
			question := w.question(t, w.b, asset, domain.KindImage, &group)
			return func() { w.exec(t, `DELETE FROM app.questions WHERE id = $1`, question) }
		}},
		"a bank group's stimulus": {domain.KindImage, func(t *testing.T, asset string) func() {
			stimulus := w.stimulus(t, w.group(t, w.b, nil), asset)
			return func() { w.exec(t, `DELETE FROM app.group_stimuli WHERE id = $1`, stimulus) }
		}},
		"a bank group's recording": {domain.KindAudio, func(t *testing.T, asset string) func() {
			recording := w.id(t, `INSERT INTO app.group_recordings (group_id, media_asset_id, allow_seek, show_transcript_after_submit) VALUES ($1, $2, false, false) RETURNING id::text`, w.group(t, w.b, nil), asset)
			return func() { w.exec(t, `DELETE FROM app.group_recordings WHERE id = $1`, recording) }
		}},
		"a section group of a live test": {domain.KindImage, func(t *testing.T, asset string) func() {
			test := w.test(t, w.b)
			section := w.id(t, `INSERT INTO app.test_sections (test_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, test)
			group := w.id(t, `INSERT INTO app.question_groups (title, created_by, owner_id, owner_section_id) VALUES ('Nhóm của đề', $1, $2, $3) RETURNING id::text`, w.b, w.admin, section)
			w.stimulus(t, group, asset)
			return func() { w.removeTest(t, test) }
		}},
		"a published version's question": {domain.KindImage, func(t *testing.T, asset string) func() {
			test := w.test(t, w.b)
			w.exec(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, media_asset_id, media_asset_kind) VALUES ($1, 0, 'short_answer', 'Xem', 1, $2, 'image')`,
				w.versionSection(t, test), asset)
			return func() { w.removeTest(t, test) }
		}},
		"a published version's group stimulus": {domain.KindImage, func(t *testing.T, asset string) func() {
			test := w.test(t, w.b)
			group := w.versionGroup(t, test)
			stimulus := w.id(t, `INSERT INTO app.test_version_group_stimuli (group_id, ordinal, title, content) VALUES ($1, 0, 'Ngữ liệu', '{"format":"semantic_v1"}') RETURNING id::text`, group)
			w.exec(t, `INSERT INTO app.test_version_group_assets (stimulus_id, group_id, media_asset_id, media_asset_kind) VALUES ($1, $2, $3, 'image')`, stimulus, group, asset)
			return func() { w.removeTest(t, test) }
		}},
		"a published version's group recording": {domain.KindAudio, func(t *testing.T, asset string) func() {
			test := w.test(t, w.b)
			w.exec(t, `INSERT INTO app.test_version_group_recordings (group_id, media_asset_id) VALUES ($1, $2)`, w.versionGroup(t, test), asset)
			return func() { w.removeTest(t, test) }
		}},
	} {
		t.Run(name, func(t *testing.T) {
			asset, bystander := w.asset(t, w.a, c.kind), w.asset(t, w.a, c.kind)
			if w.reads(t, w.scopeB, asset) {
				t.Fatal("B reads A's asset before anything of B's uses it")
			}
			revoke := c.grant(t, asset)
			if kinds := w.readable(t, w.scopeB, asset, bystander); len(kinds) != 1 || kinds[asset] != c.kind {
				t.Errorf("B reads %v through it, want only %s as %s", kinds, asset, c.kind)
			}
			if w.reads(t, access.Scope{}, asset) {
				t.Error("the zero scope reads it too")
			}
			revoke()
			if w.reads(t, w.scopeB, asset) {
				t.Error("B still reads A's asset once that use is gone")
			}
			if !w.reads(t, w.scopeA, asset) {
				t.Error("A lost their own asset")
			}
		})
	}
}

func TestReadableIgnoresUsesTheCallerDoesNotOwn(t *testing.T) {
	w := newMediaScope(t)
	asset := w.asset(t, w.a, domain.KindImage)
	groupOfA := w.group(t, w.a, nil)
	w.question(t, w.b, asset, domain.KindImage, &groupOfA)
	w.question(t, w.a, asset, domain.KindImage, nil)
	w.stimulus(t, w.group(t, w.a, nil), asset)
	testOfA := w.test(t, w.a)
	w.exec(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, media_asset_id, media_asset_kind) VALUES ($1, 0, 'short_answer', 'Xem', 1, $2, 'image')`,
		w.versionSection(t, testOfA), asset)
	section := w.id(t, `INSERT INTO app.test_sections (test_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, testOfA)
	w.stimulus(t, w.group(t, w.b, &section), asset)
	if w.reads(t, w.scopeB, asset) {
		t.Error("B reads A's asset through A's content, through a member question B owns inside A's group, or through a section group recorded as B's inside A's test")
	}
}
