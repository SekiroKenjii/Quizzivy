# Phase 2 — Admin authoring

**Deliverable (§16):** tests list, builder (all five types), question bank,
media upload + validation, publish + version snapshot.

**Exit criteria — corrected.** §16 states "E2E 1 passes". E2E 1 is *"Admin logs
in → creates a test with one of each question type including audio → publishes →
**assigns**"*. Assignment creation is Phase 3. The phase exits on **E2E 1a** —
identical up to and including publish, stopping before assign. E2E 1 in full
becomes a Phase 4 exit criterion.

Ordered media → bank → drafts → versions, so each layer is usable before the
next depends on it.

---

### T-2.1 — Add the media migration
**Depends on:** T-1.1
**Touches:** `migrations/`
**Size:** S
**Migrations:** `00009_create_media_assets.sql`
**Done when:**
- [ ] Matches `20-data-model.md` §6, including `UNIQUE (id, kind)` (D-05), the
      non-unique checksum index (D-06), and the byte and duration `CHECK`s that
      encode §11.1's 10 MB / 5 minute limits
- [ ] Test: `constraints_test.go` — an audio row with `duration_ms IS NULL` is
      rejected; an 11 MB row is rejected
- [ ] Test: `migrate_test.go` up/down/up clean
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-2.2 — Build the pure-Go audio duration probe
**Depends on:** none
**Touches:** `server/internal/media/probe/`
**Size:** L
**Done when:**
- [ ] `probe.Audio(r io.ReaderAt, size int64) (mime string, durationMs int, err error)`
      identifies MP3 and MP4/M4A **by magic bytes**, never by extension or the
      `Content-Type` header (§11.1)
- [ ] MP3: parses frame headers, honours a Xing/Info or VBRI header when present,
      and falls back to counting frames when absent. Skips an ID3v2 tag first
- [ ] MP4/M4A: walks the atom tree to `moov/mvhd` and computes
      `duration / timescale`; handles both 32-bit (version 0) and 64-bit
      (version 1) `mvhd`
- [ ] Anything that is not `audio/mpeg`, `audio/mp4` or `audio/aac` is rejected
      with a plain message (§11.1)
- [ ] A file that sniffs correctly but cannot be probed is **rejected**, not
      stored with a null duration — the `media_assets` `CHECK` would reject it
      anyway, and a 500 is a worse message than a refusal
- [ ] Bounded work: the prober reads at most N frames / atoms and returns an
      error rather than looping on a crafted file
- [ ] Test corpus committed under `server/internal/media/probe/testdata/`, each
      ≤200 KB: CBR MP3, VBR MP3 **with** a Xing header, VBR MP3 **without** one,
      MP3 with a large ID3v2 tag, iTunes-produced M4A, ffmpeg-produced M4A,
      a WAV renamed to `.mp3`, and a truncated MP3
- [ ] Test: `probe/probe_test.go` — each corpus file's duration is within 1% of
      its known value, and the two rejects are rejected
- [ ] No new Go dependency; if one is proposed, the PR states why (§14)

> Chosen over shelling out to `ffprobe` (see `00-overview.md` §5). VBR MP3
> without a Xing header is the case most likely to be wrong — hence its own
> fixture. Residual risk is R-08 in `30-risks.md`.

---

### T-2.3 — Implement media upload
**Depends on:** T-2.1, T-2.2
**Touches:** `server/internal/media/`, `server/internal/storage/`
**Size:** M
**Done when:**
- [ ] `POST /admin/media` accepts multipart, streams to a bounded temp buffer,
      sniffs magic bytes, probes duration, then writes to object storage
- [ ] Validation order is size → magic bytes → duration, so a 50 MB upload is
      cut off before anything is parsed
- [ ] Object key is `audio/{asset_id}.{ext}` (§11.2); the bucket is private
- [ ] The row is written **after** the object lands, so a failed upload leaves no
      dangling row; a failed row insert deletes the object
- [ ] Assets are immutable — re-upload creates a new row and a new key, never
      overwrites (§11.1)
- [ ] Local development targets MinIO through the same `aws-sdk-go-v2/s3` client
      as R2 (`00-overview.md` §4.7)
- [ ] Test: `media/upload_test.go` — a WAV renamed `.mp3` is rejected with the
      mime error, not stored
- [ ] Test: `media/upload_test.go` — a 6-minute MP3 is rejected on duration
- [ ] Test: `media/upload_test.go` — uploading the same file twice yields two
      rows with two keys and equal checksums

---

### T-2.4 — Implement signed URLs, media listing, and delete
**Depends on:** T-2.3
**Touches:** `server/internal/media/`
**Size:** M
**Done when:**
- [ ] `GET /app/media/:assetId/url` and the admin equivalent mint a **10-minute**
      signed URL per request (§11.2)
- [ ] Response carries `Cache-Control: private, max-age=600` so the cache entry
      cannot outlive the signature (§11.2)
- [ ] A student may only mint a URL for an asset reachable from an attempt they
      own — checked against `test_version_questions`, not by asset ID alone
- [ ] `GET /admin/media?kind=&cursor=` uses keyset pagination (§13.8)
- [ ] `DELETE /admin/media/:id` returns **409** when `tvq_media_idx` finds a
      reference from any published version; otherwise sets `deleted_at` and
      writes an audit row (§15, §13.4)
- [ ] Test: `media/signed_url_test.go` — a student cannot mint a URL for an
      asset used only by another student's assignment
- [ ] Test: `media/delete_test.go` — deleting a referenced asset is 409 and the
      row is untouched
- [ ] Note in the PR: `controlsList="nodownload"` is not protection and no
      further investment is planned (§11.2)

---

### T-2.5 — Add the question bank migrations
**Depends on:** T-2.1
**Touches:** `migrations/`
**Size:** M
**Migrations:** `00010_create_questions.sql`,
`00011_create_question_options.sql`, `00012_create_question_blanks.sql`
**Done when:**
- [ ] Matches `20-data-model.md` §7, including nullable audio policy columns
      (D-04), the composite media FK (D-05), the trigram index (D-11), partial
      indexes on `deleted_at IS NULL` (D-12), and deferrable ordinal uniques
      (D-13)
- [ ] Test: `constraints_test.go` — a non-audio question with
      `audio_allow_seek` set is rejected, and an audio question without it is
      rejected (D-04's biconditional)
- [ ] Test: `constraints_test.go` — `sample_answer` on a `single_choice`
      question is rejected
- [ ] Test: `constraints_test.go` — attaching an **image** asset and setting an
      audio policy is rejected by the composite FK + `CHECK` (D-05)
- [ ] Test: `reorder_test.go` — swapping two option ordinals in one transaction
      succeeds with `SET CONSTRAINTS … DEFERRED` (D-13)
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-2.6 — Implement question CRUD and search
**Depends on:** T-2.5
**Touches:** `server/internal/questions/`
**Size:** L
**Done when:**
- [ ] `GET /admin/questions?type=&tag=&q=&cursor=`, `POST`, `PATCH`, `DELETE`
      per §15; `DELETE` is a soft delete (§13.2)
- [ ] Search combines the tsvector index for word queries and the
      accent-folded trigram index for substring matching (D-11), in one query.
      The query must use `app.immutable_unaccent(lower(prompt))` **verbatim** on
      both sides or the planner will not match the index
- [ ] Options and blanks are written in the same transaction as the question,
      with ordinals normalized to a dense 0..n-1 (1..n for blanks)
- [ ] Explicit column lists everywhere; no `SELECT *` (§13.8)
- [ ] Test: `questions/search_test.go` — searching `nghe` matches a prompt
      containing `nghé`, `phat am` matches `phát âm`, and `duong` matches
      `Đường`. Without D-11's `unaccent` folding these all fail: `pg_trgm` is
      case-insensitive but **not** accent-insensitive
- [ ] Test: `questions/search_test.go` — `EXPLAIN` confirms the trigram index is
      actually used, so a later refactor of the query expression cannot silently
      fall back to a seq scan
- [ ] Test: `questions/crud_test.go` — a soft-deleted question is absent from
      list results but still resolvable by ID for version snapshots
- [ ] Test: `questions/crud_test.go` — reordering options round-trips

---

### T-2.7 — Add the tests and draft-structure migration
**Depends on:** T-2.5
**Touches:** `migrations/`
**Size:** S
**Migrations:** `00013_create_tests.sql`
**Done when:**
- [ ] Creates `tests`, `test_sections`, `test_section_questions` per
      `20-data-model.md` §8, including the draft tables added as D-14
- [ ] Test: `constraints_test.go` — a `published` test with
      `current_version = 0` is rejected
- [ ] Test: `constraints_test.go` — deleting a question still referenced by a
      draft section is blocked by `RESTRICT`
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-2.8 — Implement test CRUD and builder autosave
**Depends on:** T-2.7, T-2.6
**Touches:** `server/internal/tests/`
**Size:** M
**Done when:**
- [ ] `GET /admin/tests?status=&q=&cursor=`, `POST`, `GET /:id`, `PATCH /:id`,
      `POST /:id/duplicate` per §15
- [ ] `PATCH` accepts a whole-outline write (sections + question ordering) in one
      transaction, using deferred ordinal constraints (D-13)
- [ ] Autosave is **last-write-wins with a version guard**: the request carries
      the `updated_at` it read, and a mismatch is a 409. §1.3 says one admin
      edits at a time; this makes a stale second tab fail loudly instead of
      silently reverting the outline
- [ ] `duplicate` copies the draft structure, never the versions, and resets
      status to `draft` with `current_version = 0`
- [ ] Test: `tests/autosave_test.go` — a stale `updated_at` is rejected with 409
- [ ] Test: `tests/duplicate_test.go` — the copy has no `test_versions` rows

---

### T-2.9 — Add the version-table migrations
**Depends on:** T-2.7
**Touches:** `migrations/`
**Size:** M
**Migrations:** `00014_create_test_versions.sql`,
`00015_create_test_version_content.sql`
**Done when:**
- [ ] Creates all six version tables per `20-data-model.md` §8, including
      `UNIQUE (id, test_id)` (D-17), `tvq_media_idx`, and
      `source_question_id ON DELETE SET NULL` (D-07)
- [ ] Test: `constraints_test.go` — hard-deleting a bank question nulls
      `source_question_id` and leaves the version row intact (D-07)
- [ ] Test: `migrate_test.go` up/down/up clean
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-2.10 — Implement publish validation and the version snapshot
**Depends on:** T-2.9, T-2.8
**Touches:** `server/internal/tests/publish/`
**Size:** L
**Done when:**
- [ ] `POST /admin/tests/:id/publish` validates, snapshots, bumps
      `current_version`, sets status `published`, and writes an audit row — all
      in **one transaction** (§15, §13.4)
- [ ] Validation is §8's list exactly: `points > 0`; choice questions have ≥1
      correct option; `fill_blank` has ≥1 accepted answer per blank; audio
      questions have an attached audio asset; no empty sections
- [ ] Plus one validation §8 implies but does not state: for `fill_blank`, the
      set of `{{n}}` placeholders in the prompt equals the set of blank ordinals.
      A prompt with `{{3}}` and only two blanks is unrenderable
- [ ] Validation failures return **all** problems at once, each with a question
      ID, so the builder can mark them inline rather than surfacing one at a time
- [ ] The snapshot copies content into normalized rows; `media_asset_id` points
      at the same immutable asset and the file is never copied (§13.3)
- [ ] `total_points` is computed and frozen on `test_versions`
- [ ] Republishing an unchanged test still creates a new version — versions are
      an append-only history, not a diff
- [ ] Test: `publish/validate_test.go` — one case per rule, each asserting the
      returned question ID
- [ ] Test: `publish/snapshot_test.go` — after publish, editing the bank
      question's prompt leaves the version's prompt unchanged. **This is §7's
      core invariant; if this test is ever deleted the versioning is decorative**
- [ ] Test: `publish/snapshot_test.go` — the snapshot's option ordinals and
      `is_correct` flags match the draft exactly

---

### T-2.11 — Build the media upload widget and asset picker
**Depends on:** T-2.4
**Touches:** `web/src/features/media/`
**Size:** M
**Done when:**
- [ ] Upload widget with drag-and-drop and a file picker, restricted to
      `.mp3`/`.m4a` (§11.1)
- [ ] **Client-side pre-check** reads duration via an `<audio>` element and
      rejects over-length files before uploading, so the teacher is not made to
      wait for 10 MB to be told no (§11.1)
- [ ] The client pre-check is advisory; the server re-validates regardless
- [ ] Progress and cancel; a failed upload leaves no half-state in the editor
- [ ] Asset picker lists existing assets with filename, duration and size
- [ ] `/admin/media` library shows where each asset is used and blocks delete
      when referenced (§8)
- [ ] Test: `media/upload-widget.test.tsx` — a 6-minute file is rejected client
      side with a Vietnamese message and no network request is made
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-2.12 — Build the question editor for all five types
**Depends on:** T-2.6, T-2.11
**Touches:** `web/src/features/question-bank/`
**Size:** L
**Done when:**
- [ ] Editors for `single_choice`, `multiple_choice`, `true_false`,
      `fill_blank`, `short_answer` (§7)
- [ ] Prompt is Markdown rendered with react-markdown + rehype-sanitize; never
      `dangerouslySetInnerHTML` (§2)
- [ ] `fill_blank` uses the `{{1}}`-style placeholder from `40-open-items.md`,
      with an inline hint and live validation that placeholders and blanks agree
- [ ] Audio questions expose the `AudioPolicy` controls with §11.1's defaults —
      `maxPlays` 2, `allowSeek` **false**, `showTranscriptAfterSubmit` true — plus
      the transcript textarea
- [ ] `short_answer` exposes `sampleAnswer`, labelled clearly as admin-only (§7)
- [ ] zod schemas carry the type-level contract assertion from
      `00-overview.md` §3
- [ ] Test: one renderer/editor test per `QuestionType` (§14's component level)
- [ ] Test: `fill-blank-editor.test.tsx` — a `{{3}}` with two blanks shows an
      inline error before submit
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-2.13 — Build the test builder
**Depends on:** T-2.12, T-2.8
**Touches:** `web/src/features/tests/`
**Size:** L
**Done when:**
- [ ] Left outline with drag-to-reorder sections and questions via
      `@dnd-kit/core` + `@dnd-kit/sortable`, **lazy-loaded** (§2)
- [ ] Right pane is the question editor from T-2.12, including audio attach
- [ ] Autosave debounced 1.5s (§8), with a visible saved/saving/failed indicator
- [ ] A 409 from the version guard surfaces as "mở ở nơi khác" rather than
      silently discarding the user's edits
- [ ] Publish surfaces T-2.10's validation errors inline against the offending
      questions
- [ ] Drag-and-drop has a keyboard alternative — dnd-kit's keyboard sensor, with
      move-up/move-down buttons as the accessible path (§14 keyboard-operable)
- [ ] Test: `builder/autosave.test.tsx` — edits within 1.5s coalesce into one
      request
- [ ] Test: `builder/reorder.test.tsx` — reordering is achievable with the
      keyboard alone
- [ ] Loading / error / empty states; both locales (§14)

---

### T-2.14 — Build the question bank list
**Depends on:** T-2.12
**Touches:** `web/src/features/question-bank/pages/`
**Size:** M
**Done when:**
- [ ] Type and tag filters plus a search box, backed by T-2.6 (§8)
- [ ] Audio badge with inline preview using the T-3.12 player in a read-only mode
- [ ] Dense table (~40px rows) per §12; keyset pagination, not offset
- [ ] CSV import is **not** built — it is P1 (§8, §16)
- [ ] Test: `question-bank/list.test.tsx` — filter + search compose into one
      query and the empty state renders one sentence and one action (§12)
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-2.15 — Build the tests list and test detail
**Depends on:** T-2.13, T-2.10
**Touches:** `web/src/features/tests/pages/`
**Size:** M
**Done when:**
- [ ] `/admin/tests` — title, status, question count, total points, updated;
      filter by status; create / duplicate / archive (§8)
- [ ] `/admin/tests/:id` — read-only student-eye preview plus version history
      (§8), using the version-history endpoint added in T-0.7
- [ ] The preview renders from the **published version**, not the draft, so what
      the admin checks is what a student would receive
- [ ] Test: `tests/detail.test.tsx` — the preview of a published test does not
      change after the draft is edited
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-2.16 — Close the phase with E2E 1a
**Depends on:** T-2.15, T-2.14
**Touches:** `web/e2e/`
**Size:** M
**Done when:**
- [ ] **E2E 1a** passes: admin logs in → creates a test with one of each
      question type **including audio** → publishes. Stops before "assigns",
      which is Phase 3
- [ ] The E2E uploads a real fixture MP3 from T-2.2's corpus, so the probe runs
      in the loop rather than being mocked
- [ ] E2E 2a, 3 and 4 from Phase 1 still pass
- [ ] §1.2's goal is sanity-checked by hand: authoring a mixed test takes under
      15 minutes without reading docs. Record the actual time in the PR
- [ ] `release/phase-2` merges to `main` and back to `develop`; deployable (§16)
