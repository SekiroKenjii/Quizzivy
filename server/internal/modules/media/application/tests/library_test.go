//go:build integration

package application_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/png"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

func (w *mediaScope) image(t *testing.T, owner, filename string, size int64) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
		VALUES ('image', $1, 'image/png', $2, $3, sha256(convert_to($1, 'UTF8')), $4, $4) RETURNING id::text`, "library/"+uuid.NewString(), size, filename, owner)
}

func (w *mediaScope) member(t *testing.T, owner, group string, ordinal int) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, context_group_id, context_ordinal, context_option_order)
		VALUES ('short_answer', 'Thành viên', 1, $1, $1, $2, $3, 'shuffle') RETURNING id::text`, owner, group, ordinal)
}

func (w *mediaScope) recording(t *testing.T, group, asset string) {
	t.Helper()
	w.exec(t, `INSERT INTO app.group_recordings (group_id, media_asset_id, allow_seek, show_transcript_after_submit) VALUES ($1, $2, false, false)`, group, asset)
}

func (w *mediaScope) frozen(t *testing.T, owner, asset string) {
	t.Helper()
	w.exec(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after)
		VALUES ($1, 0, 'short_answer', 'Nghe', 1, $2, 'audio', false, false)`, w.versionSection(t, w.test(t, owner)), asset)
}

func (w *mediaScope) page(t *testing.T, in domain.ListInput) query.ListResult {
	t.Helper()
	in.Limit = repositories.MaxLimit
	result, err := w.svc.Queries.List.Handle(context.Background(), query.List{Input: in})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (w *mediaScope) found(t *testing.T, in domain.ListInput) []string {
	t.Helper()
	result := w.page(t, in)
	found := ids(result.Items)
	if result.Page.Total != len(found) {
		t.Errorf("the page holds %d files and reports a total of %d", len(found), result.Page.Total)
	}
	slices.Sort(found)
	return found
}

func (w *mediaScope) row(t *testing.T, scope access.Scope, id string) domain.Asset {
	t.Helper()
	for _, a := range w.page(t, domain.ListInput{Scope: scope}).Items {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("%s is not in the library of %+v", id, scope)
	return domain.Asset{}
}

func (w *mediaScope) facets(t *testing.T, in domain.ListInput) domain.Facets {
	t.Helper()
	facets, err := w.svc.Queries.Facets.Handle(context.Background(), query.Facets{Input: in})
	if err != nil {
		t.Fatal(err)
	}
	return facets
}

func (w *mediaScope) usage(t *testing.T, scope access.Scope) domain.Usage {
	t.Helper()
	usage, err := w.svc.Queries.Usage.Handle(context.Background(), query.Usage{Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	return usage
}

func (w *mediaScope) update(scope access.Scope, id string, in domain.UpdateInput) (domain.Asset, error) {
	in.ID, in.ActorID, in.All = id, scope.UserID, scope.All
	return w.svc.Commands.Update.Handle(context.Background(), command.Update{Input: in})
}

func (w *mediaScope) audited(t *testing.T, action, entity string) []string {
	t.Helper()
	rows, err := w.tx.Query(context.Background(), `SELECT actor_user_id::text FROM app.audit_log WHERE action = $1 AND entity_id = $2 ORDER BY occurred_at, id`, action, entity)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actors []string
	for rows.Next() {
		var actor string
		if err := rows.Scan(&actor); err != nil {
			t.Fatal(err)
		}
		actors = append(actors, actor)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return actors
}

func sorted(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestQuestionCountCountsEachLiveQuestionOnceAndTheUnusedFilterFollowsIt(t *testing.T) {
	w := newMediaScope(t)
	audio, picture := domain.KindAudio, domain.KindImage

	standalone := w.asset(t, w.a, audio)
	w.question(t, w.a, standalone, audio, nil)
	w.question(t, w.b, standalone, audio, nil)
	gone := w.question(t, w.a, standalone, audio, nil)
	w.exec(t, `UPDATE app.questions SET deleted_at = now() WHERE id = $1`, gone)

	attached := w.asset(t, w.a, picture)
	carrying := w.group(t, w.a, nil)
	w.question(t, w.a, attached, picture, &carrying)
	w.stimulus(t, carrying, attached)

	recorded := w.asset(t, w.a, audio)
	listening := w.group(t, w.a, nil)
	w.recording(t, listening, recorded)
	w.member(t, w.a, listening, 0)
	w.member(t, w.a, listening, 1)

	shown := w.asset(t, w.a, picture)
	reading := w.group(t, w.a, nil)
	w.stimulus(t, reading, shown)
	w.member(t, w.a, reading, 0)
	w.member(t, w.b, reading, 1)
	w.member(t, w.a, reading, 2)

	shelved := w.asset(t, w.a, picture)
	w.stimulus(t, w.group(t, w.a, nil), shelved)

	published := w.asset(t, w.a, audio)
	w.frozen(t, w.a, published)

	spare := w.asset(t, w.a, picture)

	want := map[string]int{standalone: 2, attached: 1, recorded: 2, shown: 3, shelved: 0, published: 0, spare: 0}
	got := map[string]int{}
	for _, a := range w.page(t, domain.ListInput{Scope: w.scopeA}).Items {
		got[a.ID] = a.QuestionCount
	}
	if !maps.Equal(got, want) {
		t.Errorf("the library counts %v questions, want %v", got, want)
	}
	if usage := w.row(t, w.scopeA, published).UsageCount; usage != 1 {
		t.Errorf("the file a version holds lists %d versions, want 1 beside its 0 questions", usage)
	}

	if found := w.found(t, domain.ListInput{Scope: w.scopeA, Unused: true}); !slices.Equal(found, sorted(shelved, published, spare)) {
		t.Errorf("unused lists %v, want the files no question uses %v", found, sorted(shelved, published, spare))
	}
	if found := w.found(t, domain.ListInput{Scope: w.scopeA, Unused: true, Kind: &audio}); !slices.Equal(found, []string{published}) {
		t.Errorf("unused audio lists %v, want %v", found, []string{published})
	}
	if found := w.found(t, domain.ListInput{Scope: w.scopeA}); len(found) != 7 {
		t.Errorf("unused=false lists %d files, want all 7", len(found))
	}
	for name, in := range map[string]domain.ListInput{
		"no filter":       {Scope: w.scopeA},
		"audio":           {Scope: w.scopeA, Kind: &audio},
		"images":          {Scope: w.scopeA, Kind: &picture},
		"unused":          {Scope: w.scopeA, Unused: true},
		"unused pictures": {Scope: w.scopeA, Unused: true, Kind: &picture},
	} {
		if facets := w.facets(t, in); facets != (domain.Facets{All: 7, Audio: 3, Image: 4, Unused: 3}) {
			t.Errorf("under %s the tabs count %+v, want 7 files, 3 audio, 4 images and 3 unused", name, facets)
		}
	}

	for name, id := range map[string]string{"an empty group still holds": shelved, "a published version still holds": published} {
		if err := w.remove(w.scopeA, id); !errors.Is(err, domain.ErrReferenced) {
			t.Errorf("deleting the unused file %s: %v, want it still refused", name, err)
		}
	}
	if err := w.remove(w.scopeA, spare); err != nil {
		t.Errorf("deleting the file nothing holds: %v", err)
	}
}

func TestAReplacedFileLeavesTheLibraryAndKeepsServing(t *testing.T) {
	w := newMediaScope(t)
	ctx := context.Background()
	old := upload(t, w.svc, w.a, "ban-cu.mp3")
	newer := upload(t, w.svc, w.a, "ban-moi.mp3")
	picture := w.image(t, w.a, "hinh.png", 700)
	w.question(t, w.a, old.ID, domain.KindAudio, nil)

	if usage := w.usage(t, w.scopeA); usage.AudioBytes != old.Bytes+newer.Bytes || usage.ImageBytes != 700 {
		t.Fatalf("before the replace the library holds %+v, want %d of audio and 700 of images", usage, old.Bytes+newer.Bytes)
	}
	w.exec(t, `UPDATE app.media_assets SET replaced_by = $2 WHERE id = $1`, old.ID, newer.ID)

	for name, scope := range map[string]access.Scope{"the owner": w.scopeA, "scope.all": w.anyone} {
		if listed := ids(w.page(t, domain.ListInput{Scope: scope}).Items); slices.Contains(listed, old.ID) || !slices.Contains(listed, newer.ID) {
			t.Errorf("%s lists %v, want the newer file and not the replaced one", name, listed)
		}
	}
	if found := w.found(t, domain.ListInput{Scope: w.scopeA}); !slices.Equal(found, sorted(newer.ID, picture)) {
		t.Errorf("the library lists %v, want %v", found, sorted(newer.ID, picture))
	}
	if found := w.found(t, domain.ListInput{Scope: w.scopeA, Query: "ban-cu"}); len(found) != 0 {
		t.Errorf("a search finds the replaced file: %v", found)
	}
	if facets := w.facets(t, domain.ListInput{Scope: w.scopeA}); facets != (domain.Facets{All: 2, Audio: 1, Image: 1, Unused: 2}) {
		t.Errorf("the tabs count %+v, want the newer file and the picture, both unused", facets)
	}
	if usage := w.usage(t, w.scopeA); usage.AudioBytes != newer.Bytes || usage.ImageBytes != 700 {
		t.Errorf("the library holds %+v, want %d of audio and 700 of images", usage, newer.Bytes)
	}
	if total := w.totalBytes(t, w.scopeA, nil); total != newer.Bytes+700 {
		t.Errorf("the library totals %d bytes, want %d", total, newer.Bytes+700)
	}
	held := newer.Bytes + 700
	for name, c := range map[string]struct {
		add     int64
		refused bool
	}{"room for the rest": {100, false}, "one byte past": {101, true}} {
		err := repositories.RequireQuota(ctx, w.tx, repositories.QuotaCheck{OwnerID: w.a, Add: c.add, Quota: held + 100})
		if c.refused != errors.Is(err, domain.ErrQuotaExceeded) || (!c.refused && err != nil) {
			t.Errorf("%s: %v, want refused %v: the quota counts the replaced file", name, err, c.refused)
		}
	}

	if got, err := w.svc.Queries.Get.Handle(ctx, query.Get{ID: old.ID}); err != nil || got.ID != old.ID {
		t.Errorf("the replaced file no longer answers Get: %v", err)
	}
	if !w.reads(t, w.scopeA, old.ID) {
		t.Error("the replaced file is no longer readable by its owner")
	}
	if _, err := w.update(w.scopeA, old.ID, domain.UpdateInput{DisplayName: new("Đổi tên bản cũ")}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("renaming the replaced file: %v, want not found", err)
	}

	w.exec(t, `DELETE FROM app.media_assets WHERE id = $1`, newer.ID)
	if found := w.found(t, domain.ListInput{Scope: w.scopeA}); !slices.Equal(found, sorted(old.ID, picture)) {
		t.Errorf("with the newer row gone the library lists %v, want the older file back: %v", found, sorted(old.ID, picture))
	}
}

func TestTheSearchFoldsAccentsAndCaseAndTheTabsFollowIt(t *testing.T) {
	w := newMediaScope(t)
	audio, picture := domain.KindAudio, domain.KindImage
	calf := upload(t, w.svc, w.a, "Nghé.mp3")
	renamed := upload(t, w.svc, w.a, "rec-001.mp3")
	if _, err := w.update(w.scopeA, renamed.ID, domain.UpdateInput{DisplayName: new("Bài NGHE số 2")}); err != nil {
		t.Fatal(err)
	}
	sketch := w.image(t, w.a, "so-do-nghe.png", 10)
	other := w.image(t, w.a, "ban-do 100%.png", 20)
	w.question(t, w.a, sketch, picture, nil)
	upload(t, w.svc, w.b, "nghe-cua-b.mp3")

	hits := sorted(calf.ID, renamed.ID, sketch)
	for _, term := range []string{"nghe", "Nghé", "NGHE", "nghé", "  nghe  "} {
		if found := w.found(t, domain.ListInput{Scope: w.scopeA, Query: term}); !slices.Equal(found, hits) {
			t.Errorf("%q finds %v, want %v", term, found, hits)
		}
		for name, in := range map[string]domain.ListInput{
			"alone":           {Scope: w.scopeA, Query: term},
			"with audio":      {Scope: w.scopeA, Query: term, Kind: &audio},
			"with unused":     {Scope: w.scopeA, Query: term, Unused: true},
			"with both":       {Scope: w.scopeA, Query: term, Kind: &picture, Unused: true},
			"past every page": {Scope: w.scopeA, Query: term, Page: 9},
		} {
			if facets := w.facets(t, in); facets != (domain.Facets{All: 3, Audio: 2, Image: 1, Unused: 2}) {
				t.Errorf("%q %s counts %+v, want 3 files, 2 audio, 1 image and 2 unused", term, name, facets)
			}
		}
		if found := w.found(t, domain.ListInput{Scope: w.scopeA, Query: term, Kind: &picture}); !slices.Equal(found, []string{sketch}) {
			t.Errorf("%q among images finds %v, want %v", term, found, []string{sketch})
		}
		if found := w.found(t, domain.ListInput{Scope: w.scopeA, Query: term, Unused: true}); !slices.Equal(found, sorted(calf.ID, renamed.ID)) {
			t.Errorf("%q among unused files finds %v, want %v", term, found, sorted(calf.ID, renamed.ID))
		}
	}
	if total, err := w.svc.Queries.TotalBytes.Handle(context.Background(), query.TotalBytes{Scope: w.scopeA, Query: "nghe"}); err != nil || total != calf.Bytes+renamed.Bytes+10 {
		t.Errorf("the search totals %d bytes (%v), want %d", total, err, calf.Bytes+renamed.Bytes+10)
	}
	if total, err := w.svc.Queries.TotalBytes.Handle(context.Background(), query.TotalBytes{Scope: w.scopeA, Query: "nghe", Unused: true, Kind: &audio}); err != nil || total != calf.Bytes+renamed.Bytes {
		t.Errorf("the search among unused audio totals %d bytes (%v), want %d", total, err, calf.Bytes+renamed.Bytes)
	}

	for term, want := range map[string][]string{
		"rec-001": {renamed.ID},
		"số 2":    {renamed.ID},
		"so 2":    {renamed.ID},
		"100%":    {other},
		"%":       {other},
		"_":       {},
		"ban_do":  {},
		"video":   {},
	} {
		if found := w.found(t, domain.ListInput{Scope: w.scopeA, Query: term}); !slices.Equal(found, sorted(want...)) {
			t.Errorf("%q finds %v, want %v", term, found, want)
		}
	}
	if facets := w.facets(t, domain.ListInput{Scope: w.scopeA}); facets != (domain.Facets{All: 4, Audio: 2, Image: 2, Unused: 3}) {
		t.Errorf("with no search the tabs count %+v, want 4 files, 2 audio, 2 images and 3 unused", facets)
	}
	if usage := w.usage(t, w.scopeA); usage.AudioBytes != calf.Bytes+renamed.Bytes || usage.ImageBytes != 30 || usage.QuotaBytes != domain.DefaultOwnerQuotaBytes {
		t.Errorf("the library holds %+v, want %d of audio and 30 of images beside the default quota", usage, calf.Bytes+renamed.Bytes)
	}
}

func TestTheLibrarysFiguresHoldOnlyTheScopesOwnRows(t *testing.T) {
	w := newMediaScope(t)
	mine := upload(t, w.svc, w.a, "cua-a.mp3")
	theirs := upload(t, w.svc, w.b, "cua-b.mp3")
	theirPicture := w.image(t, w.b, "cua-b.png", 50)
	w.question(t, w.b, theirPicture, domain.KindImage, nil)
	own := upload(t, w.svc, w.admin, "cua-admin.mp3")

	for name, c := range map[string]struct {
		scope  access.Scope
		facets domain.Facets
		usage  domain.Usage
	}{
		"A":               {w.scopeA, domain.Facets{All: 1, Audio: 1, Unused: 1}, domain.Usage{AudioBytes: mine.Bytes}},
		"B":               {w.scopeB, domain.Facets{All: 2, Audio: 1, Image: 1, Unused: 1}, domain.Usage{AudioBytes: theirs.Bytes, ImageBytes: 50}},
		"the Admin's own": {w.anyone.Own(), domain.Facets{All: 1, Audio: 1, Unused: 1}, domain.Usage{AudioBytes: own.Bytes}},
		"the zero scope":  {access.Scope{}, domain.Facets{}, domain.Usage{}},
	} {
		c.usage.QuotaBytes = domain.DefaultOwnerQuotaBytes
		if facets := w.facets(t, domain.ListInput{Scope: c.scope}); facets != c.facets {
			t.Errorf("%s: the tabs count %+v, want %+v", name, facets, c.facets)
		}
		if usage := w.usage(t, c.scope); usage != c.usage {
			t.Errorf("%s: the library holds %+v, want %+v", name, usage, c.usage)
		}
	}
	if facets := w.facets(t, domain.ListInput{Scope: w.anyone}); facets.All < 4 {
		t.Errorf("scope.all counts %+v, want at least the four files here", facets)
	}
	if count := w.row(t, w.scopeB, theirPicture).QuestionCount; count != 1 {
		t.Errorf("B's picture counts %d questions, want 1", count)
	}
}

func TestRenameAndPlayLimitRoundTrip(t *testing.T) {
	w := newMediaScope(t)
	ctx := context.Background()
	clip := upload(t, w.svc, w.a, "cam15-t2-p1.mp3")
	picture := w.image(t, w.a, "ban-do.png", 10)

	if row := w.row(t, w.scopeA, clip.ID); row.DisplayName != "cam15-t2-p1.mp3" || row.DefaultMaxPlays != nil || row.Width != nil || row.Height != nil {
		t.Fatalf("a fresh upload lists as %q with limit %v and size %v by %v, want its filename and nothing else", row.DisplayName, row.DefaultMaxPlays, row.Width, row.Height)
	}

	updated, err := w.update(w.scopeA, clip.ID, domain.UpdateInput{DisplayName: new("  Cambridge 15 · Test 2 · Part 1  "), SetDefaultMaxPlays: true, DefaultMaxPlays: new(2)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != "Cambridge 15 · Test 2 · Part 1" || updated.DefaultMaxPlays == nil || *updated.DefaultMaxPlays != 2 {
		t.Errorf("the update answers %q with limit %v, want the trimmed name and 2", updated.DisplayName, updated.DefaultMaxPlays)
	}
	if updated.OriginalFilename != "cam15-t2-p1.mp3" || updated.StorageKey != clip.StorageKey || updated.URL == "" || updated.Bytes != clip.Bytes {
		t.Errorf("the update answers file %q at %q with URL %q, want the stored file untouched and signed", updated.OriginalFilename, updated.StorageKey, updated.URL)
	}
	var stored string
	if err := w.tx.QueryRow(ctx, `SELECT display_name FROM app.media_assets WHERE id = $1`, clip.ID).Scan(&stored); err != nil || stored != "Cambridge 15 · Test 2 · Part 1" {
		t.Errorf("the stored name is %q (%v)", stored, err)
	}
	if row := w.row(t, w.scopeA, clip.ID); row.DisplayName != updated.DisplayName || row.DefaultMaxPlays == nil || *row.DefaultMaxPlays != 2 {
		t.Errorf("the library lists %q with limit %v", row.DisplayName, row.DefaultMaxPlays)
	}
	if actors := w.audited(t, "media.updated", clip.ID); !slices.Equal(actors, []string{w.a}) {
		t.Errorf("the update is audited by %v, want once by its owner", actors)
	}

	if _, err := w.update(w.scopeA, clip.ID, domain.UpdateInput{DisplayName: new("Tên khác")}); err != nil {
		t.Fatal(err)
	}
	if row := w.row(t, w.scopeA, clip.ID); row.DisplayName != "Tên khác" || row.DefaultMaxPlays == nil || *row.DefaultMaxPlays != 2 {
		t.Errorf("a rename left %q with limit %v, want the limit kept at 2", row.DisplayName, row.DefaultMaxPlays)
	}
	if _, err := w.update(w.scopeA, clip.ID, domain.UpdateInput{SetDefaultMaxPlays: true, DefaultMaxPlays: new(0)}); err != nil {
		t.Fatal(err)
	}
	if row := w.row(t, w.scopeA, clip.ID); row.DisplayName != "Tên khác" || row.DefaultMaxPlays == nil || *row.DefaultMaxPlays != 0 {
		t.Errorf("unlimited plays left %q with limit %v, want the name kept and 0", row.DisplayName, row.DefaultMaxPlays)
	}
	if _, err := w.update(w.scopeA, clip.ID, domain.UpdateInput{SetDefaultMaxPlays: true}); err != nil {
		t.Fatal(err)
	}
	if row := w.row(t, w.scopeA, clip.ID); row.DefaultMaxPlays != nil {
		t.Errorf("a cleared limit lists as %d", *row.DefaultMaxPlays)
	}
	if actors := w.audited(t, "media.updated", clip.ID); len(actors) != 4 {
		t.Errorf("four updates left %d audit rows", len(actors))
	}

	if renamed, err := w.update(w.scopeA, picture, domain.UpdateInput{DisplayName: new("Bản đồ chỉ đường")}); err != nil || renamed.DisplayName != "Bản đồ chỉ đường" {
		t.Errorf("renaming an image: %q (%v)", renamed.DisplayName, err)
	}
	if _, err := w.update(w.scopeA, picture, domain.UpdateInput{SetDefaultMaxPlays: true, DefaultMaxPlays: new(1)}); !errors.Is(err, domain.ErrPlayLimitOnImage) {
		t.Errorf("a play limit on an image: %v, want it refused", err)
	}
	if actors := w.audited(t, "media.updated", picture); len(actors) != 1 {
		t.Errorf("the image has %d audit rows, want only its rename", len(actors))
	}
}

func TestAnUpdateReachesOnlyTheLibraryInTheCallersScope(t *testing.T) {
	w := newMediaScope(t)
	mine := upload(t, w.svc, w.a, "cua-a.mp3")
	gone := upload(t, w.svc, w.a, "da-xoa.mp3")
	w.exec(t, `UPDATE app.media_assets SET deleted_at = now() WHERE id = $1`, gone.ID)
	old := upload(t, w.svc, w.a, "ban-cu.mp3")
	newer := upload(t, w.svc, w.a, "ban-moi.mp3")
	w.exec(t, `UPDATE app.media_assets SET replaced_by = $2 WHERE id = $1`, old.ID, newer.ID)

	rename := domain.UpdateInput{DisplayName: new("Đã đổi"), SetDefaultMaxPlays: true, DefaultMaxPlays: new(1)}
	for name, c := range map[string]struct {
		scope access.Scope
		id    string
	}{
		"another owner's file":            {w.scopeB, mine.ID},
		"a deleted file":                  {w.scopeA, gone.ID},
		"a replaced file":                 {w.scopeA, old.ID},
		"a missing file":                  {w.scopeA, uuid.NewString()},
		"a deleted file, scope.all":       {w.anyone, gone.ID},
		"a replaced file, scope.all":      {w.anyone, old.ID},
		"a missing file, scope.all":       {w.anyone, uuid.NewString()},
		"the owner's file, no scope":      {access.Scope{}, mine.ID},
		"the Admin's own-rows list scope": {w.anyone.Own(), mine.ID},
	} {
		if _, err := w.update(c.scope, c.id, rename); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("updating %s: %v, want the not-found a missing file gets", name, err)
		}
	}
	var touched, audited int
	if err := w.tx.QueryRow(context.Background(), `SELECT count(*) FROM app.media_assets WHERE id = ANY($1::uuid[]) AND (display_name IS NOT NULL OR default_max_plays IS NOT NULL)`,
		[]string{mine.ID, gone.ID, old.ID, newer.ID}).Scan(&touched); err != nil {
		t.Fatal(err)
	}
	if err := w.tx.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_log WHERE action = 'media.updated' AND entity_id = ANY($1::uuid[])`,
		[]string{mine.ID, gone.ID, old.ID, newer.ID}).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if touched != 0 || audited != 0 {
		t.Errorf("the refused updates changed %d files and left %d audit rows", touched, audited)
	}

	updated, err := w.update(w.anyone, mine.ID, rename)
	if err != nil || updated.DisplayName != "Đã đổi" {
		t.Fatalf("scope.all renaming A's file: %q (%v)", updated.DisplayName, err)
	}
	if actors := w.audited(t, "media.updated", mine.ID); !slices.Equal(actors, []string{w.admin}) {
		t.Errorf("the Admin's update is audited by %v, want the Admin", actors)
	}
	if row := w.row(t, w.scopeA, mine.ID); row.DisplayName != "Đã đổi" || row.DefaultMaxPlays == nil || *row.DefaultMaxPlays != 1 {
		t.Errorf("A's library lists %q with limit %v after the Admin's update, want the file still A's", row.DisplayName, row.DefaultMaxPlays)
	}
	if _, err := w.update(w.scopeA, mine.ID, domain.UpdateInput{DisplayName: new("Của A")}); err != nil {
		t.Errorf("the owner renaming their own file: %v", err)
	}
}

func pictureOf(t *testing.T, width, height int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAnUploadRecordsAnImagesSizeAndAnAudiosPlayLimit(t *testing.T) {
	w := newMediaScope(t)
	ctx := context.Background()
	objects := newFakeStore()
	svc := application.New(repositories.NewPostgres(db.NewContext(w.tx)), objects, audioProbe{}).WithImageProbe(imageProbe{})
	send := func(name string, data []byte, limit *int) (domain.Asset, error) {
		return svc.Commands.Upload.Handle(ctx, command.Upload{Filename: name, Body: bytes.NewReader(data), UploaderID: w.a, DefaultMaxPlays: limit})
	}

	picture, err := send("ban-do.png", pictureOf(t, 320, 200), nil)
	if err != nil {
		t.Fatal(err)
	}
	if picture.Width == nil || picture.Height == nil || *picture.Width != 320 || *picture.Height != 200 {
		t.Errorf("the upload answers %v by %v, want 320 by 200", picture.Width, picture.Height)
	}
	if row := w.row(t, w.scopeA, picture.ID); row.Width == nil || row.Height == nil || *row.Width != 320 || *row.Height != 200 || row.DefaultMaxPlays != nil {
		t.Errorf("the library lists it as %v by %v with limit %v", row.Width, row.Height, row.DefaultMaxPlays)
	}

	extended := append([]byte("RIFF\x16\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x10\x00\x00\x00"), 0x3f, 0x06, 0x00, 0x83, 0x03, 0x00)
	if webp, err := send("bieu-do.webp", extended, nil); err != nil || webp.MimeType != "image/webp" || webp.Width == nil || *webp.Width != 1600 || webp.Height == nil || *webp.Height != 900 {
		t.Errorf("a WebP reads as %s %v by %v (%v), want image/webp 1600 by 900", webp.MimeType, webp.Width, webp.Height, err)
	}

	torn := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x7f}, 64)...)
	unread, err := send("hong.png", torn, nil)
	if err != nil {
		t.Fatalf("an image whose header cannot be read: %v, want it stored as before", err)
	}
	if unread.Width != nil || unread.Height != nil || unread.Kind != domain.KindImage {
		t.Errorf("it is stored as %s %v by %v, want an image with no size", unread.Kind, unread.Width, unread.Height)
	}

	clip, err := send("bai-nghe.mp3", fixture(t, "cbr-128k.mp3"), new(2))
	if err != nil {
		t.Fatal(err)
	}
	if row := w.row(t, w.scopeA, clip.ID); row.DefaultMaxPlays == nil || *row.DefaultMaxPlays != 2 || row.Width != nil || row.Height != nil {
		t.Errorf("the audio lists with limit %v and size %v by %v, want 2 and no size", row.DefaultMaxPlays, row.Width, row.Height)
	}
	unlimited, err := send("khong-gioi-han.mp3", fixture(t, "cbr-128k.mp3"), new(0))
	if err != nil {
		t.Fatal(err)
	}
	if row := w.row(t, w.scopeA, unlimited.ID); row.DefaultMaxPlays == nil || *row.DefaultMaxPlays != 0 {
		t.Errorf("unlimited plays list as %v, want 0", row.DefaultMaxPlays)
	}

	before := len(objects.puts)
	if _, err := send("ban-do-2.png", pictureOf(t, 8, 8), new(2)); !errors.Is(err, domain.ErrPlayLimitOnImage) {
		t.Errorf("an image uploaded with a play limit: %v, want it refused", err)
	}
	if len(objects.puts) != before || len(objects.deletes) != 0 {
		t.Errorf("the refused image was put (%d new) or deleted %v", len(objects.puts)-before, objects.deletes)
	}
	if found := w.found(t, domain.ListInput{Scope: w.scopeA}); len(found) != 5 {
		t.Errorf("the library holds %d files, want the five that were accepted", len(found))
	}
}

func TestAnUploadPastTheQuotaIsRefusedAndLeavesNothing(t *testing.T) {
	w := newMediaScope(t)
	ctx := context.Background()
	data := fixture(t, "cbr-128k.mp3")
	size := int64(len(data))
	objects := newFakeStore()
	repo := repositories.NewPostgres(db.NewContext(w.tx))
	svc := application.New(repo, objects, audioProbe{}).WithOwnerQuota(2 * size)
	send := func(owner string) (domain.Asset, error) {
		return svc.Commands.Upload.Handle(ctx, command.Upload{Filename: "nghe.mp3", Body: bytes.NewReader(data), UploaderID: owner})
	}

	first, err := send(w.a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := send(w.a); err != nil {
		t.Fatalf("the upload that fills the quota exactly: %v", err)
	}
	if _, err := send(w.a); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("a third file past the quota: %v, want it refused", err)
	}
	if len(objects.puts) != 2 || len(objects.objects) != 2 {
		t.Errorf("the refused file left %d puts and %d objects, want the two that fit", len(objects.puts), len(objects.objects))
	}
	if usage, err := svc.Queries.Usage.Handle(ctx, query.Usage{Scope: w.scopeA}); err != nil || usage.AudioBytes != 2*size || usage.QuotaBytes != 2*size {
		t.Errorf("the library holds %+v (%v), want %d of %d", usage, err, 2*size, 2*size)
	}
	if _, err := send(w.b); err != nil {
		t.Errorf("another owner's upload beside a full library: %v", err)
	}

	id := uuid.Must(uuid.NewV7()).String()
	checksum := sha256.Sum256([]byte(id))
	row := domain.InsertInput{ID: id, Kind: domain.KindImage, StorageKey: "image/" + id + ".png", MimeType: "image/png", Bytes: 1,
		OriginalFilename: "mot-byte.png", ChecksumSHA256: checksum[:], UploaderID: w.a, QuotaBytes: 2 * size, Now: time.Now()}
	if _, err := repo.Insert(ctx, row); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Errorf("one byte into a full library, past the command's own check: %v, want the repository to refuse it", err)
	}
	var written int
	if err := w.tx.QueryRow(ctx, `SELECT count(*) FROM app.media_assets WHERE id = $1`, id).Scan(&written); err != nil || written != 0 {
		t.Errorf("the refused row was written %d times (%v)", written, err)
	}

	if err := w.remove(w.scopeA, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := send(w.a); err != nil {
		t.Errorf("an upload into the room a deleted file left: %v", err)
	}
}

func TestTheQuotaCheckAndTheInsertRunInTheCallersTransaction(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	user := func(builtin string) string {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Hạn mức', (SELECT id FROM app.roles WHERE builtin_key = $2)) RETURNING id::text`,
			uuid.NewString()+"@example.test", builtin).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	uploader, owner := user("admin"), user("teacher")
	row := func(ownerID string, size int64) domain.InsertInput {
		id := uuid.Must(uuid.NewV7()).String()
		checksum := sha256.Sum256([]byte(id))
		return domain.InsertInput{ID: id, Kind: domain.KindImage, StorageKey: "image/" + id + ".png", MimeType: "image/png", Bytes: size,
			OriginalFilename: "so-do.png", ChecksumSHA256: checksum[:], UploaderID: uploader, OwnerID: ownerID,
			DisplayName: new("Sơ đồ lớp học"), Width: new(4), Height: new(3), Now: time.Now(), IP: new("203.0.113.5")}
	}

	old, err := repositories.Insert(ctx, tx, row(owner, 600))
	if err != nil {
		t.Fatalf("insert for another owner: %v", err)
	}
	if old.DisplayName != "Sơ đồ lớp học" || old.Width == nil || *old.Width != 4 || old.Height == nil || *old.Height != 3 || old.CreatedAt.IsZero() {
		t.Errorf("the insert answers %+v, want the name and the size it was given", old)
	}
	var ownedBy, uploadedBy, auditedBy string
	if err := tx.QueryRow(ctx, `SELECT owner_id::text, uploaded_by::text FROM app.media_assets WHERE id = $1`, old.ID).Scan(&ownedBy, &uploadedBy); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT actor_user_id::text FROM app.audit_log WHERE action = 'media.uploaded' AND entity_id = $1`, old.ID).Scan(&auditedBy); err != nil {
		t.Fatalf("the insert's audit row: %v", err)
	}
	if ownedBy != owner || uploadedBy != uploader || auditedBy != uploader {
		t.Errorf("the row is owned by %s, uploaded by %s and audited by %s, want owner %s and uploader %s", ownedBy, uploadedBy, auditedBy, owner, uploader)
	}

	for name, c := range map[string]struct {
		check   repositories.QuotaCheck
		refused bool
	}{
		"filling the library exactly":             {repositories.QuotaCheck{OwnerID: owner, Add: 400, Quota: 1000}, false},
		"one byte past":                           {repositories.QuotaCheck{OwnerID: owner, Add: 401, Quota: 1000}, true},
		"the old file left out":                   {repositories.QuotaCheck{OwnerID: owner, Add: 1000, Quota: 1000, LeaveOut: old.ID}, false},
		"the old file left out, one byte past":    {repositories.QuotaCheck{OwnerID: owner, Add: 1001, Quota: 1000, LeaveOut: old.ID}, true},
		"a file that is not there left out":       {repositories.QuotaCheck{OwnerID: owner, Add: 401, Quota: 1000, LeaveOut: uuid.NewString()}, true},
		"the uploader's own library, still empty": {repositories.QuotaCheck{OwnerID: uploader, Add: 1000, Quota: 1000}, false},
	} {
		err := repositories.RequireQuota(ctx, tx, c.check)
		if c.refused != errors.Is(err, domain.ErrQuotaExceeded) || (!c.refused && err != nil) {
			t.Errorf("%s: %v, want refused %v", name, err, c.refused)
		}
	}

	mine, err := repositories.Insert(ctx, tx, row("", 50))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT owner_id::text FROM app.media_assets WHERE id = $1`, mine.ID).Scan(&ownedBy); err != nil || ownedBy != uploader {
		t.Errorf("a row with no owner named is owned by %s (%v), want its uploader %s", ownedBy, err, uploader)
	}
	if err := repositories.RequireQuota(ctx, tx, repositories.QuotaCheck{OwnerID: uploader, Add: 951, Quota: 1000}); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Errorf("the uploader's library after its own insert: %v, want 50 bytes counted", err)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.media_assets WHERE id = ANY($1::uuid[])) + (SELECT count(*) FROM app.audit_log WHERE entity_id = ANY($1::uuid[]))`,
		[]string{old.ID, mine.ID}).Scan(&left); err != nil || left != 0 {
		t.Errorf("the rolled-back transaction left %d rows (%v)", left, err)
	}
}

func libraryOwner(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.Must(uuid.NewV7()).String()
	t.Cleanup(func() {
		for _, statement := range []string{
			`DELETE FROM app.audit_log WHERE actor_user_id = $1`,
			`DELETE FROM app.media_assets WHERE owner_id = $1`,
			`DELETE FROM app.users WHERE id = $1`,
		} {
			if _, err := pool.Exec(ctx, statement, id); err != nil {
				t.Errorf("cleaning up the library's owner: %v", err)
			}
		}
	})
	if _, err := pool.Exec(ctx, `INSERT INTO app.users (id, email, full_name, role_id) VALUES ($1, $2, 'Hạn mức', (SELECT id FROM app.roles WHERE builtin_key = 'teacher'))`,
		id, "quota-"+id+"@example.test"); err != nil {
		t.Fatal(err)
	}
	return id
}

func waitUntilBlockedBy(t *testing.T, pool *pgxpool.Pool, pid int, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var blocked bool
		if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked); err != nil {
			t.Fatalf("look for the waiting upload: %v", err)
		}
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the second upload never waited on the first one's quota lock")
		}
		select {
		case err := <-done:
			t.Fatalf("the second upload finished while the first still held the library's quota lock: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestTwoUploadsAtTheQuotaEdgeLeaveOneFileAndOneRefusal(t *testing.T) {
	pool := newPool(t)
	owner := libraryOwner(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data := fixture(t, "cbr-128k.mp3")
	size := int64(len(data))
	quota := size + size/2
	objects := newFakeStore()
	svc := application.New(repositories.NewPostgres(db.NewContext(pool)), objects, audioProbe{}).WithOwnerQuota(quota)

	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Rollback(context.Background()) })
	var pid int
	if err := first.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if err := repositories.RequireQuota(ctx, first, repositories.QuotaCheck{OwnerID: owner, Add: size, Quota: quota}); err != nil {
		t.Fatalf("the first upload fits alone: %v", err)
	}
	id := uuid.Must(uuid.NewV7()).String()
	checksum := sha256.Sum256(data)
	if _, err := repositories.Insert(ctx, first, domain.InsertInput{ID: id, Kind: domain.KindAudio, StorageKey: "audio/" + id + ".mp3", MimeType: "audio/mpeg",
		Bytes: size, DurationMs: new(10_005), OriginalFilename: "thu-nhat.mp3", ChecksumSHA256: checksum[:], UploaderID: owner, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	second := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Upload.Handle(ctx, command.Upload{Filename: "thu-hai.mp3", Body: bytes.NewReader(data), UploaderID: owner})
		second <- err
	}()
	waitUntilBlockedBy(t, pool, pid, second)
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-second:
		if !errors.Is(err, domain.ErrQuotaExceeded) {
			t.Fatalf("the second upload: %v, want it refused once the first is in the library", err)
		}
	case <-ctx.Done():
		t.Fatal("the second upload did not return after the first committed")
	}

	var rows []string
	found, err := pool.Query(ctx, `SELECT original_filename FROM app.media_assets WHERE owner_id = $1`, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer found.Close()
	for found.Next() {
		var name string
		if err := found.Scan(&name); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, name)
	}
	if err := found.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rows, []string{"thu-nhat.mp3"}) {
		t.Errorf("the library holds %v, want only the first upload", rows)
	}
	if len(objects.puts) != 1 || !slices.Equal(objects.deletes, objects.puts) || len(objects.objects) != 0 {
		t.Errorf("the refused upload put %v and deleted %v, want its one object removed", objects.puts, objects.deletes)
	}
}

func TestTheLibraryColumnsHoldTheirRules(t *testing.T) {
	w := newMediaScope(t)
	ctx := context.Background()

	clip := uuid.Must(uuid.NewV7()).String()
	checksum := sha256.Sum256([]byte(clip))
	w.exec(t, `
		INSERT INTO app.media_assets
		       (id, kind, storage_key, mime_type, bytes, duration_ms,
		        original_filename, checksum_sha256, uploaded_by, created_at, owner_id)
		VALUES ($1, $2::app.media_kind, $3, $4, $5, $6, $7, $8, $9, $10, $9)`,
		clip, "audio", "audio/"+clip+".mp3", "audio/mpeg", int64(11<<20), 252_000, "ban-truoc.mp3", checksum[:], w.a, time.Now())
	var untouched bool
	if err := w.tx.QueryRow(ctx, `SELECT display_name IS NULL AND default_max_plays IS NULL AND replaced_by IS NULL AND width IS NULL AND height IS NULL
		FROM app.media_assets WHERE id = $1`, clip).Scan(&untouched); err != nil || !untouched {
		t.Errorf("the previous release's insert left a new column set (%v)", err)
	}
	if row := w.row(t, w.scopeA, clip); row.DisplayName != "ban-truoc.mp3" || row.Bytes != 11<<20 || row.DefaultMaxPlays != nil || row.Width != nil {
		t.Errorf("the library lists the previous release's row as %+v, want 11 MiB named after its file", row)
	}
	picture := w.image(t, w.a, "hinh.png", 10)
	other := w.image(t, w.a, "hinh-khac.png", 10)

	attempt := func(statement string, args ...any) error {
		nested, err := w.tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = nested.Rollback(ctx) }()
		_, err = nested.Exec(ctx, statement, args...)
		return err
	}

	for _, c := range []struct {
		name, constraint, statement string
		args                        []any
	}{
		{"audio one byte over 50 MiB", "media_assets_bytes_by_kind", `UPDATE app.media_assets SET bytes = $2 WHERE id = $1`, []any{clip, int64(50<<20) + 1}},
		{"audio of 51 MiB", "media_assets_bytes_by_kind", `UPDATE app.media_assets SET bytes = $2 WHERE id = $1`, []any{clip, int64(51 << 20)}},
		{"an image one byte over 10 MiB", "media_assets_bytes_by_kind", `UPDATE app.media_assets SET bytes = $2 WHERE id = $1`, []any{picture, int64(10<<20) + 1}},
		{"an image of 11 MiB", "media_assets_bytes_by_kind", `UPDATE app.media_assets SET bytes = $2 WHERE id = $1`, []any{picture, int64(11 << 20)}},
		{"a file of no bytes", "media_assets_bytes_by_kind", `UPDATE app.media_assets SET bytes = 0 WHERE id = $1`, []any{clip}},
		{"a size in pixels on audio", "media_assets_dimensions_image_only", `UPDATE app.media_assets SET width = 10, height = 10 WHERE id = $1`, []any{clip}},
		{"a width alone", "media_assets_dimensions_paired", `UPDATE app.media_assets SET width = 10 WHERE id = $1`, []any{picture}},
		{"a height alone", "media_assets_dimensions_paired", `UPDATE app.media_assets SET height = 10 WHERE id = $1`, []any{picture}},
		{"a width of nothing", "media_assets_dimensions_positive", `UPDATE app.media_assets SET width = 0, height = 10 WHERE id = $1`, []any{picture}},
		{"a height below nothing", "media_assets_dimensions_positive", `UPDATE app.media_assets SET width = 10, height = -1 WHERE id = $1`, []any{picture}},
		{"a play limit on an image", "media_assets_default_max_plays_check", `UPDATE app.media_assets SET default_max_plays = 2 WHERE id = $1`, []any{picture}},
		{"unlimited plays on an image", "media_assets_default_max_plays_check", `UPDATE app.media_assets SET default_max_plays = 0 WHERE id = $1`, []any{picture}},
		{"four plays", "media_assets_default_max_plays_check", `UPDATE app.media_assets SET default_max_plays = 4 WHERE id = $1`, []any{clip}},
		{"a play limit below nothing", "media_assets_default_max_plays_check", `UPDATE app.media_assets SET default_max_plays = -1 WHERE id = $1`, []any{clip}},
		{"a file replaced by itself", "media_assets_replaced_by_check", `UPDATE app.media_assets SET replaced_by = id WHERE id = $1`, []any{clip}},
		{"an empty name", "media_assets_display_name_check", `UPDATE app.media_assets SET display_name = '' WHERE id = $1`, []any{clip}},
		{"a name of spaces", "media_assets_display_name_check", `UPDATE app.media_assets SET display_name = '   ' WHERE id = $1`, []any{clip}},
		{"a name of 201 characters", "media_assets_display_name_check", `UPDATE app.media_assets SET display_name = $2 WHERE id = $1`, []any{clip, strings.Repeat("ế", 201)}},
	} {
		if err := attempt(c.statement, c.args...); !db.IsCheckViolation(err, c.constraint) {
			t.Errorf("%s: %v, want it refused by %s", c.name, err, c.constraint)
		}
	}
	if err := attempt(`UPDATE app.media_assets SET replaced_by = $2 WHERE id = $1`, clip, uuid.NewString()); err == nil {
		t.Error("a file replaced by one that is not there was accepted")
	} else if constraint, ok := db.ForeignKeyViolation(err); !ok || constraint != "media_assets_replaced_by_fkey" {
		t.Errorf("a file replaced by one that is not there: %v, want media_assets_replaced_by_fkey", err)
	}

	for _, c := range []struct {
		name, statement string
		args            []any
	}{
		{"audio of exactly 50 MiB", `UPDATE app.media_assets SET bytes = $2 WHERE id = $1`, []any{clip, int64(50 << 20)}},
		{"an image of exactly 10 MiB", `UPDATE app.media_assets SET bytes = $2 WHERE id = $1`, []any{picture, int64(10 << 20)}},
		{"unlimited plays on audio", `UPDATE app.media_assets SET default_max_plays = 0 WHERE id = $1`, []any{clip}},
		{"three plays on audio", `UPDATE app.media_assets SET default_max_plays = 3 WHERE id = $1`, []any{clip}},
		{"an image of one pixel", `UPDATE app.media_assets SET width = 1, height = 1 WHERE id = $1`, []any{picture}},
		{"a name of 200 characters", `UPDATE app.media_assets SET display_name = $2 WHERE id = $1`, []any{clip, strings.Repeat("ế", 200)}},
		{"a file replaced by another", `UPDATE app.media_assets SET replaced_by = $2 WHERE id = $1`, []any{picture, other}},
	} {
		if err := attempt(c.statement, c.args...); err != nil {
			t.Errorf("%s: %v, want it accepted", c.name, err)
		}
	}
}
