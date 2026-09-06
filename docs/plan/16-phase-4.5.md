# Phase 4.5 — The student at a desk

**Deliverable:** every `/app/*` screen designed and built for 1024px and up,
and the take-test engine brought to parity with the deck at every width
(S-05, S-06, S-08, S-12, and the new S-15), including the section dimension
the engine has been missing since Phase 3.
**Exit criteria:** each student route at 1280 and 1440 matches S-13–S-17
board by board; the engine matches S-08 at 1024 and S-12 at 360; #87, #88
and the take-test half of #91 close; every CI step green, E2E chromium and
live green; released to production as `v0.4.0-phase-4.5`.

**Why a half phase.** Phase 3 built the student side from a sheet drawn at
390 and stretched it: on a laptop the home was a 768px column of full-width
buttons, the intro kept its phone back-arrow chrome, the result page stacked
its score above a single column, and the engine kept the sticky footer and
the separate save strip that S-08 removes above 1024. Thuong measured this
after the Phase 4 release (2026-09-06) and asked for it to be fixed before
Phase 5 rather than folded into it: "giao diện của học viên đang focus hết
vào mobile … giao diện trên desktop rất tệ và kể cả phần giao diện làm bài
thi cũng không giống với mockup ở nhiều điểm."

Design first, because the deck is the contract (`docs/design/mockups`): the
student sheet gains section 6 with five boards, and nothing is built that a
board does not draw. Then the contract change the engine needs, then the
engine, then the shell and the four pages, then release.

**Decisions taken in this file, worth a second look.**

- **Sections reach the student through the session payload.** `AttemptSession`
  gains `sections[]` (`id`, `title`, `instructions`) and `StudentQuestion`
  gains `sectionId`. Nothing revealing is added — instructions are written
  *for* the student — and E2E 9's key list is unchanged.
- **Shuffle stays inside a section.** `DealManager.Present` shuffles the
  questions of each section with the section id as salt and keeps section
  order, so S-08's rail can group by section and still count 1…n in
  presentation order. A cross-section shuffle would put "Phần 1" at
  questions 1, 4, 7. An attempt in flight across the deploy keeps every
  answer (they are keyed by question id) and sees its questions reordered
  once; the release note says so.
- **`caseSensitive` joins `StudentBlank`.** The deck's fill-blank line
  ("Không phân biệt hoa thường. Viết đúng chính tả.") is a promise about
  grading, and a promise the student cannot check is worse than none. The
  flag is a rule, not a key; it is the one field of the blank that is safe
  to show.
- **Section instructions are shown on the first question of the section**
  in presentation order, not on every question. S-05 draws them on Q11 and
  Q16, which are firsts; a ten-question section repeating its instruction
  ten times is noise. The navigator's section label carries the title on
  every question.
- **Below 1024 nothing changes visually except the column cap**: the S-03
  layouts keep their phone shape, with the content column capped at 40rem
  rather than the current 48rem (S-13's note).
- **One student layout, not two.** `StudentDetailLayout` (back arrow +
  title) becomes a mode of `StudentLayout` selected by a route handle,
  because S-13/S-14 keep the nav bar on detail pages from 1024 and only the
  phone wants the arrow. The take-test route keeps `FocusLayout`.
- **#94 ("Hồ sơ" card) is not decided here.** S-17 draws the card as S-10
  does; whichever way #94 goes applies to both widths.

---

### T-4.5.1 — Deck: the student at a desk (S-13–S-17)
**Depends on:** —
**Touches:** `docs/design/mockups/sheets/10-student.html`, `docs/design/mockups/index.html`
**Size:** M
**Done when:**
- [x] Section 6 with S-13 (shell + home), S-14 (intro), S-15 (review at
      1024), S-16 (result), S-17 (classes + settings), each with a board
      note stating the rule it adds and callouts for the states it does not draw
- [x] Every rule is derived from an existing board (F-04 densities, F-11
      panel width, S-08's 720px column) — no new tokens, no new components
- [x] `node docs/design/mockups/check.mjs` clean; index counts updated
- [ ] Thuong has looked at the five boards in the deck and said go

---

### T-4.5.2 — Sections in the attempt session
**Depends on:** —
**Touches:** `api/openapi.yaml`, `server/gen/openapi/`, `server/internal/modules/attempts/{domain,repositories,http}/`, `web/src/lib/api/schema.d.ts`, `web/src/features/take-test/`
**Size:** M
**Done when:**
- [ ] `AttemptSession.sections: StudentSection[]` (`id`, `title`,
      `instructions: string | null`, in test order) and
      `StudentQuestion.sectionId`; `StudentBlank.caseSensitive`
- [ ] The session query projects `test_version_sections` (already joined
      for ordering) into the payload; the domain `Question` carries
      `SectionID`, the paper carries `Sections`
- [ ] `DealManager.Present` shuffles within a section (salt = section id)
      and keeps section order; `deal_test` pins that a two-section version
      never interleaves
- [ ] The contract test over `openapi.yaml` and E2E 9 still assert none of
      `isCorrect`, `sampleAnswer`, `acceptedAnswers`, `transcript` under
      `/app/*`; `make gen-check` and `pnpm gen:check` clean
- [ ] `make test-api` green (needs Docker)

---

### T-4.5.3 — Engine parity: S-05, S-06, S-08, S-12, S-15
**Depends on:** T-4.5.2
**Touches:** `web/src/features/take-test/`, `web/src/lib/i18n/locales/{vi,en}.json`
**Size:** L
**Done when:**
- [ ] From `lg`: one header row — Thoát · test title · save badge ("Đã lưu
      09:41" / "Đang lưu…" / unsaved) · "Còn n lần rời trang" · timer — over
      the 4px progress bar; the save strip and the sticky footer render only
      below `lg`
- [ ] From `lg`: "Câu trước" (outline) and "Câu sau" (primary) inline under
      the answer at their own width, the last question's "Xem lại & nộp" in
      the same slot, and the S-08 shortcut hint to their right
- [ ] From `lg`: the meta line above the stem reads "Phần 1 · Ngữ pháp — Câu
      3 / 24 · 1 điểm"; below `lg` the header keeps "Câu 3/24" and the points
      stay under the answer (S-05)
- [ ] Section instructions render above the body of the section's first
      question: an `alert-muted` with a headphones icon when the question has
      audio, otherwise the muted line S-05 draws on the fill-blank frame
- [ ] Navigator rail and sheet group the dots under "Phần n · Title" (S-06,
      S-08); the review screen at `lg` is S-15 — header row unchanged, first
      control "Quay lại bài", rail without its button, the two actions inline
- [ ] Fill-blank closes with the matching rule (case-sensitive or not) and
      short-answer's meta is one `justify-between` row (#91)
- [ ] Unit tests: section grouping, first-of-section detection, the `lg`
      branch of the header (render at 1280 and 360 with a matchMedia stub)
- [ ] E2E chromium (1280): the smoke spec asserts the S-08 header row and the
      absence of the footer; a new `mobile-chromium` project (Pixel 5) runs
      the same spec and asserts the S-12 footer

---

### T-4.5.4 — Student shell at 1024 and up (S-13)
**Depends on:** T-4.5.1 sign-off
**Touches:** `web/src/layouts/StudentLayout.tsx`, `web/src/layouts/StudentDetailLayout.tsx` (removed), `web/src/app/router.tsx`, `web/src/components/shared/`
**Size:** M
**Done when:**
- [ ] Top bar: logo, "Bài của tôi", "Lớp", and from `lg` a user menu (avatar
      initial + given name + chevron → Cài đặt, Đăng xuất); below `lg` the
      S-03 bar with its icon button, unchanged
- [ ] Page column `max-w-5xl` with `px-6 py-8` from `lg`; below `lg` the
      phone padding and a 40rem cap
- [ ] Detail routes (`assignments/:id`, `attempts/:id/result`, `settings`)
      declare `handle.detail`; below `lg` the layout shows the back arrow +
      title, from `lg` the nav bar with the page drawing its own "← Bài của
      tôi" link (S-14, S-16)
- [ ] `/app/settings` lights no nav item; `/app/classes` lights "Lớp"
- [ ] Unit test: the layout renders the arrow at 360 and the bar at 1280 for a
      detail route

---

### T-4.5.5 — Home at 1280 (S-13)
**Depends on:** T-4.5.4
**Touches:** `web/src/features/assignments/pages/StudentHomePage.tsx`
**Size:** M
**Done when:**
- [ ] From `lg`: `minmax(0,1fr) 20rem` grid with a 2rem gap; due and resume
      cards lay out sideways with the button at its own width on the right;
      completed rows add the class name before "Nộp dd/mm"
- [ ] Right column: "Sắp tới · n" then "Lớp của tôi" (names, teacher, "Tham
      gia lớp"); absent when both are empty; empty states stay in the left column
- [ ] Below `lg`: S-03 exactly as today

---

### T-4.5.6 — Assignment intro at 1280 (S-14)
**Depends on:** T-4.5.4
**Touches:** `web/src/features/assignments/pages/AssignmentIntroPage.tsx`
**Size:** S
**Done when:**
- [ ] From `lg`: back link, provenance and title over "Khi làm bài" and "Sau
      khi nộp" on the left; facts + start button + note in one card on the
      right; blocked states and resume take the button's place
- [ ] Below `lg`: S-04 exactly as today

---

### T-4.5.7 — Result at 1280 (S-16)
**Depends on:** T-4.5.4
**Touches:** `web/src/features/results/pages/ResultPage.tsx`
**Size:** M
**Done when:**
- [ ] From `lg`: back link, title, and the "Nộp lúc … · Lượt n/m" line on the
      left with the filters and the paper at 720px; the score tile on the
      right with "x đúng · y sai · z chờ chấm" under the bar
- [ ] The S-09b subtractions hold in both columns; a withheld score leaves
      the right column empty except the one muted sentence
- [ ] Below `lg`: S-09 exactly as today

---

### T-4.5.8 — Classes, settings, join and login at 1280 (S-17, S-01, S-02)
**Depends on:** T-4.5.4
**Touches:** `web/src/features/classes/pages/StudentClassesPage.tsx`, `web/src/features/auth/pages/StudentSettingsPage.tsx`, `web/src/features/auth/pages/LoginPage.tsx`, `web/src/layouts/PublicLayout.tsx`
**Size:** S
**Done when:**
- [ ] Classes: 42rem column, title row, "Lớp" lit; settings: 36rem column,
      "Cài đặt" title, sign-out at its own width, nothing lit
- [ ] Login shows S-02's brand panel from `lg`; join is S-01's single card at
      every width — both measured in the browser, fixed if off

---

### T-4.5.9 — Browser verification against S-13–S-17 and S-08/S-12
**Depends on:** T-4.5.3 … T-4.5.8
**Touches:** —
**Size:** M
**Done when:**
- [ ] Every `/app/*` route captured at 1440, 1280, 1024, 768 and 360 as the
      seed student, with the engine on a two-section test (the dev seed gains
      one: `seed/04-dev-e2e.sql`), compared board by board with the deck open
      beside it
- [ ] Deviations fixed in the same PR, or filed as issues if outside this
      phase's scope

---

### T-4.5.10 — Release
**Depends on:** T-4.5.9
**Touches:** `docs/plan/`, `AGENTS.md`, `server/README.md` if the contract note moves
**Size:** S
**Done when:**
- [ ] `release/phase-4.5` off `develop`, merged `--no-ff` to `main` with
      Thuong's go, tagged `v0.4.0-phase-4.5`; Fly and Cloudflare Pages green;
      `/healthz` and a student login checked on production
- [ ] #87, #88 closed; #91 updated to leave only the papers back button
- [ ] AGENTS.md records the student layout rule (one layout, detail mode by
      handle, 1024 breakpoint) and the within-section shuffle
