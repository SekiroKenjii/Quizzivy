# Quizzivy — Frontend Portal & Data Model Specification

**Version:** 0.28 · **Owner:** Thuong · **Audience:** AI coding agent + future contributors
**Scope:** web frontend (admin + student portals) and the PostgreSQL data model. Go backend implementation is a separate spec; the API surface in §15 is the contract both sides implement.

**Changes since v0.27**

- W-07e exposes teacher-only complete-group read/write/copy/archive operations,
  bounded bank summaries and ordered draft units. Writes check aggregate and
  enclosing-test revisions. Media-link outages return a retryable asset state
  without obscuring committed content. Builder integration remains a release gate.

**Changes since v0.26**

- Draft totals/tag filters include group members. Legacy question-only outline
  writes refuse grouped drafts atomically. Archived tests must be restored before
  draft restoration/default-version changes. Assignment introductions include
  shared recording policies from the assigned version and explain their scope.

**Changes since v0.25**

- W-09f supplies frozen shared context in learner results and both teacher
  grading modes. Transcript release follows each recording's policy independently
  of score/key/explanation flags. Review playback does not add attempt plays.

**Changes since v0.24**

- W-09e renders shared learner materials beside the question on wide screens and
  above it on phones with remembered collapse state. Stable recording players and
  idempotent gesture recovery preserve shared counts across member navigation,
  reload and takeover. Result/review context and authoring remain gated.

**Changes since v0.23**

- W-09d freezes the delivery algorithm on each published version. Historical
  standalone papers keep `section_v1`; group-aware snapshots use `group_v1`.
  Attempt reload, takeover and results select the frozen marker, never the
  current test default. Unsupported formats fail closed.

**Changes since v0.22**

- W-09c persists shared-recording counters and gesture receipts, deduplicates
  retries atomically with timeline events, and includes shared excess plays in
  teacher monitoring. Group authoring/player/result UI remain gated.

**Changes since v0.21**

- W-09b adds learner-safe shared context to start/resume/read attempt responses.
  Frozen media bindings require an attempt owned by the learner. Shared playback,
  result/review context and group authoring remain gated.

**Changes since v0.20**

- W-08c previews frozen group context and authorized assets without grading keys,
  follows the selected default version, and offers a 320px learner preview inside
  the supported admin shell. Group authoring and live learner delivery stay gated.

**Changes since v0.19**

- W-08b restores and duplicates complete independent context graphs, preserves
  mixed unit order and removes owned draft graphs when an unreferenced archived
  test is deleted. Grouped preview/draft UI and delivery remain gated.

**Changes since v0.18**

- W-08 defines independent frozen group membership, materials, gap targets and
  recording policies alongside existing version questions. Historical snapshots
  are not backfilled; group authoring stays unavailable until readers are ready.

**Changes since v0.17**

- W-07d adds revision-checked full-group editing, independent copy materialization,
  bank archive/restore/delete and section removal. These remain internal operations
  until complete draft, snapshot and learner readers are available.

**Changes since v0.16**

- W-07c adds atomic internal group persistence, owned-member barriers and protected
  media references. No group endpoint or authoring affordance is enabled yet.

**Changes since v0.15**

- W-07b adds relational group ownership, ordered units, stable cloze targets and
  protected material/recording bindings. Existing question ownership remains
  null; new group authoring is still unavailable until lifecycle and readers ship.

**Changes since v0.14**

- W-07a defines the bounded, independent group graph: ordered members, rich
  materials, explicit choice/blank gap targets and shared recording identities.
  Validation and detached copying precede persistence and delivery; group writes
  remain unavailable until snapshots, protected references and readers are ready.

**Changes since v0.13**

- Rich authoring supports bounded structured clipboard conversion with an explicit
  preview, atomic insertion and undo. Unsupported source content is refused in full;
  it is never silently reduced to plain text. Original clipboard HTML stays local.

**Changes since v0.12**

- Rich fill-blank prompts bind stable gap identities to answer rows. Moving gaps
  keeps answers attached; copies remap both ends; published versions preserve the
  frozen graph. Legacy Markdown and answer payloads remain compatible.

**Changes since v0.11**

- Manual/internal question writes and publication share interaction validation.
  Single-choice and true/false require one key; true/false requires two options.
  Question points are positive exact hundredths within numeric(8,2); publication
  sums exact hundredths and rejects empty exams or an overflowing total.

**Changes since v0.10**

- W-05b adds bounded semantic prompts and explanations with exact plain projections,
  immutable snapshots and policy-gated explanation delivery. Legacy Markdown remains
  on its existing reader; no old content is rewritten. Fill-blank prompts retain
  Markdown until stable gap binding is integrated.

**Changes since v0.9**

- Inline option formatting now has additive storage, snapshot/restore and learner
  readers (§7.1, §13.3). New authoring is a pilot opt-in; legacy options remain
  literal text and existing published rows are not backfilled.
- Option documents contain only marked text and line breaks; their plain text
  must match the stored `text`. Missing rich content in a legacy write cannot
  silently erase formatting. See `docs/plan/19-content-contract.md`.

**Changes since v0.8**

- The Word foundation defines an additive `ContentDocument` contract (§7.1).
  Existing question writes, stored Markdown and published versions are unchanged.
- Thuong approved independent copies of groups/materials and per-account local
  recovery for unsent edits, with seven-day expiry and logout clearing (§16).
- New grouped exams will shuffle whole groups within their section, retain member
  order, and share a recording's play allowance across its group (§7.2, §11.5).

**Changes since v0.7**

- The Word exam import milestone is accepted for implementation (§16). Its full
  scope includes semantic content, shared materials and groups, durable review,
  safe publication/delivery and operational release gates. It is not yet released.
- Both cloud and private AI processing remain candidates for measured evaluation;
  provider selection and external exam-data processing are not enabled by default.

**Changes since v0.6**

- Dashboard work cards lead into a full-width assignment table and compact activity below.
- Student discovery has counted status filters and prominent resume cards.
- Teacher and student settings share Profile, Security and Preferences routes,
  persistent forms, section navigation and separated form rows.
- Subtle shadows, hover feedback and 150ms section transitions respect reduced motion.

**Changes since v0.5**

- Admin lists retain filters, support bulk removal and duplicate in place. The
  question bank exposes attached outlines in a nested table (§8).
- Version history supports safe draft restoration, default selection and unused
  version deletion without changing assigned snapshots or reusing version numbers.
- Assignment limits support custom durations, attempt counts and immediate
  integrity submission with retained answers and recorded violations (§10).

**Changes since v0.4**

- The approved independent UX review replaces the student mockup's sparse right
  panels with fluid discovery pages and centred reading/form pages (§9, §12).
- All live attempts remain discoverable; class/status filters, result empties,
  explicit selection/audio/save states and direct submitted-paper navigation are
  part of the student flow. Responsive changes preserve local form/filter state.
- Phone actions use 44px touch targets, including navigation and dialog controls;
  the final-question footer must fit at 320px in Vietnamese and English.

**Changes since v0.3**

- Pending student answers survive a failed save/reload within the same session;
  manual submission waits for the final save. Keyboard editing/navigation rules
  are explicit in §9.
- §12 pins the mockup type scale and phone input/touch exceptions.
- Approved O-23 defines thirteen-month integrity retention, retained audit logs
  and manual structured-identity anonymization in §13.3.

**Changes since v0.2**
- **OQ-2 answered: yes** — students self-join with a class code. New §6, new `/join` flow, join-code lifecycle in the schema. This adds a public, unauthenticated surface that did not exist before; read §6.5 on the security consequences.
- **OQ-3 answered: yes** — audio (listening) questions ship in v1, with upload and a custom player. New §11, new `media_assets` table, object storage added to the stack.
- **OQ-4 answered: yes** — `sample_answer` on `short_answer` questions, visible to the admin during grading only.
- All open questions are now closed. §17 lists the decisions that were made *inside* these answers and are worth a second look.

---

## 1. Product context

Quizzivy is a web app supporting a private English-teaching practice. Students take tests assigned by their teacher; the teacher/admin builds tests, assigns them, monitors attempts, grades open answers, and reviews results.

**v1 ships one capability end-to-end: test-taking.** Later versions add vocabulary practice, homework, pronunciation drills, and class management. v1 must not paint us into a corner for those, but must not build them either.

### 1.1 Personas

| Persona | Role enum | Description |
|---|---|---|
| Teacher / owner | `admin` | Builds tests, manages students and classes, grades, reads results. **Desktop/tablet only.** Data-dense UI is fine. |
| Student | `student` | Joins a class with a code, takes assigned tests, sees own results. Often on a phone. Needs a calm, focused UI. |

Roles are an enum (`admin`, `student`). Design guards and permission checks so a third role (`teacher`, limited admin) can be added later without restructuring.

### 1.2 Goals (v1)

1. A student can join a class and complete an assigned test on a phone without losing work on refresh, tab close, or brief network loss.
2. The admin can author a test with mixed question types — including listening — in under 15 minutes without reading docs.
3. Every attempt is reviewable: answers, timing, score breakdown, and an integrity timeline.
4. Codebase is structured so the next feature (e.g., vocabulary sets) is a new folder under `features/`, not a rewrite.

### 1.3 Non-goals (v1)

- **Payments and subscriptions.**
- **Open public sign-up.** Self-signup exists but is gated behind a class join code (§6). There is no "create an account" entry point without one.
- **Multi-tenant / multiple schools.** Single teacher, single organization. No org/tenant scoping in the schema.
- **Hard proctoring** — webcam, screen recording, screen-lock, remote inspection. §10 covers browser-signal monitoring only; see §10.5 on its limits.
- **Audio transcoding, trimming, or waveform editing.** Upload, validate, serve. Nothing more (§11.2).
- **Speaking/recording questions.** Students listen in v1; they do not record. That is a separate feature with its own storage, consent, and grading model.
- **Rich analytics dashboards.** Per-test and per-student score tables only.
- **Native mobile apps.** Responsive web only.
- **Real-time collaborative authoring.** One admin edits at a time.

---

## 2. Tech stack

Fixed unless Thuong approves a change. Do not swap libraries silently.

| Concern | Choice | Notes |
|---|---|---|
| Build | Vite + React 19 + TypeScript (strict) | `pnpm` |
| Routing | React Router v7 (SPA mode) | Route-level code splitting via `lazy` |
| Server state | TanStack Query v5 | All API reads/writes go through query/mutation hooks |
| Client state | Zustand (minimal) | Auth session, test-taking engine, UI prefs only |
| Forms | react-hook-form + zod | Zod schemas are the single source of validation and TS types |
| Styling | Tailwind CSS v4 + shadcn/ui (neutral/zinc base) | See §12 |
| Icons | lucide-react | One icon set only |
| i18n | i18next + react-i18next | `vi` default, `en` secondary. All strings via `t()` from day one. |
| Dates | date-fns + date-fns-tz | `Asia/Ho_Chi_Minh` default |
| HTTP | native `fetch` wrapped in `src/lib/api/client.ts` | No axios |
| Google auth | Google Identity Services | Authorization Code + PKCE, exchanged server-side (§5.3) |
| Audio | native `<audio>` element + custom React controls | No wavesurfer.js, no howler. See §11.3 |
| Object storage | Cloudflare R2 (S3-compatible) | Audio and image assets. `aws-sdk-go-v2/s3` on the backend |
| Markdown | react-markdown + rehype-sanitize | Never `dangerouslySetInnerHTML` with server content |
| Drag & drop | `@dnd-kit/core` + `@dnd-kit/sortable` | Builder reordering only; lazy-loaded |
| Testing | Vitest + Testing Library; Playwright e2e; MSW mocks | See §14 |
| Lint/format | ESLint (typescript-eslint, react-hooks, jsx-a11y) + Prettier | CI fails on lint errors |

**Single SPA, three route trees.** `/admin/*`, `/app/*`, and a small public tree (`/login`, `/join/*`). Split at the route level so a student never downloads admin code and an anonymous visitor downloads neither.

---

## 3. Repository layout

```
web/src/
├─ app/                      # shell: providers, router, error boundaries, guards/
├─ features/
│  ├─ auth/
│  ├─ join/                  # NEW: public class-code join flow
│  ├─ tests/                 # admin: authoring
│  ├─ question-bank/
│  ├─ media/                 # NEW: upload widget + asset picker
│  ├─ assignments/
│  ├─ attempts/              # admin: monitoring + grading
│  ├─ integrity/
│  ├─ students/
│  ├─ classes/
│  ├─ take-test/             # student: the engine
│  └─ results/
├─ components/
│  ├─ ui/                    # shadcn primitives
│  └─ shared/
├─ layouts/                  # AdminLayout, StudentLayout, FocusLayout, PublicLayout
├─ lib/                      # api/, i18n/, utils/, config.ts
├─ hooks/  stores/  styles/  main.tsx
```

**Feature folder convention** (`src/features/<name>/`): `api.ts`, `schemas.ts`, `components/`, `pages/`, `store.ts` (optional), `index.ts` (public exports only). Features import each other only via `index.ts`. `components/shared` never imports from `features/`.

---

## 4. Naming and branding

Display name **Quizzivy**, sentence case. Package `quizzivy-web`; Go module `quizzivy`. DB `quizzivy`, schema `app` (not `public`, §13.2). R2 bucket `quizzivy-media`. The literal string appears once, in `.env`; everything else reads `config.appName`.

---

## 5. Auth

### 5.1 Methods

Two, both landing on the same `users` row keyed by verified email:

1. **Email + password** — admin-created accounts, and the admin's own login.
2. **Google Sign-In** — the primary path for students, and the **only** path for self-join (§6.3).

A user may have both. Linking rule: a Google sign-in whose ID token carries `email_verified: true` matching an existing user links to that user. An **unverified** email is rejected outright — no link, no create. This closes an account-takeover path.

### 5.2 Session model

- Access token: JWT, ~15 min, held **in memory** (Zustand). Never localStorage, never sessionStorage.
- Refresh token: opaque, rotating, `httpOnly; Secure; SameSite=Lax; Path=/auth` cookie. Stored server-side as a hash (§13.5).
- On 401 the client calls `POST /auth/refresh` once and retries. A second 401 logs out.
- App load: `GET /auth/me`. 401 → `/login`.
- Reuse detection: presenting an already-rotated token revokes the whole family and forces re-login.

### 5.3 Google flow

1. Frontend loads GIS and requests an **authorization code** (not implicit ID-token) with PKCE.
2. `POST /auth/google` `{ code, codeVerifier, redirectUri, joinCode? }`.
3. Backend exchanges with Google, verifies the ID token (`iss`, `aud`, `exp`, signature via JWKS), reads `sub`, `email`, `email_verified`, `name`, `picture`.
4. Resolution order:
   - identity exists → log in;
   - verified email matches a user → link identity, log in;
   - no match **and** a valid `joinCode` is present → create account + enroll (§6.3);
   - no match, no join code → `403 ACCOUNT_NOT_PROVISIONED`.
5. Returns `{ accessToken, user }` and sets the refresh cookie.

`VITE_GOOGLE_CLIENT_ID` is public config. The client secret lives only in the backend.

### 5.4 Guards and edge cases

- `RequireAuth` → `/login?next=<path>` when unauthenticated.
- `RequireRole` — `admin` on `/app/*` redirects to `/admin`; `student` on `/admin/*` gets a **403 page**, not a redirect (a redirect hides the misconfiguration).
- `mustChangePassword: true` → all routes redirect to `/change-password`. Google-only users never hit this.
- Logout: `POST /auth/logout` (revokes refresh token), clear store, `queryClient.clear()`, → `/login`.
- Password reset in v1: admin sets a temporary password from the student detail page. No self-service email flow (§17.1).

---

## 6. Class join codes (self-signup)

### 6.1 Model

A join code belongs to a class and is a **bearer secret**: whoever holds it can enrol. Treat it accordingly.

- Format: 8 characters from an unambiguous alphabet `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` (no `0/O`, `1/I/L`). Displayed grouped `XXXX-XXXX`; accepted with or without the dash, case-insensitive.
- Generated from a CSPRNG. Never sequential, never derived from the class ID.
- Per-code controls: `expires_at` (default 30 days), `max_uses` (default null = unlimited), `uses_count`, `revoked_at`.
- One **active** code per class at a time. Rotating issues a new code and revokes the old one; previously enrolled students are unaffected.

### 6.2 Student flow

```
/join                → enter code
/join/:code          → deep link (QR / message), code prefilled
/join/:code/confirm  → shows class name + teacher name, "Tiếp tục với Google"
                     → GIS → POST /auth/google {code, joinCode}
                     → account created + enrolled → /app
```

The confirm step exists so the student sees **which class they are joining** before authenticating. Never create an account and enrol in one blind tap.

Already-authenticated students hitting `/join/:code` skip straight to enrolment via `POST /app/classes/join`.

### 6.3 Why Google-only for self-join

Self-signup with email + password requires a verified email, which requires transactional email infrastructure (provider, domain auth, deliverability, bounce handling) — a real dependency v1 does not have. Google hands us a verified email for free and is one tap on a phone, which is exactly the self-join case.

So: **self-join requires Google.** Password accounts remain admin-created. If Thuong later wants password self-signup, add an email provider and a `email_verifications` table; the join flow itself does not change. This is a v1 scoping decision, not a permanent constraint (§17.1).

### 6.4 Admin controls

On `/admin/classes/:id`:
- Show the active code, with copy button, QR code, expiry, and uses count.
- **Rotate code** (confirm dialog: "Mã cũ sẽ ngừng hoạt động ngay").
- **Disable self-join** toggle — revokes the code without issuing a new one.
- Member list shows `joined_via` (`admin` / `join_code`) and `joined_at`, so the teacher can spot unexpected enrolments.
- Remove member (revokes access; attempts are retained, not deleted).

### 6.5 Security requirements — non-optional

A leaked code lets a stranger into the class. Mitigations, all required:

- **Rate limit** `POST /join/preview` and `POST /auth/google` with a `joinCode`: per-IP (e.g. 10/min, 60/hour) and per-code (e.g. 30/hour). Return `429` with `Retry-After`. Without this, an 8-char code space is still worth probing at scale.
- **Constant-time comparison** on code lookup; look up by a hash of the normalized code, not by plaintext equality.
- `POST /join/preview` returns **only** class name and teacher display name — never student names, never counts, never IDs. It is an unauthenticated endpoint.
- Log every enrolment (`class_id`, `user_id`, `ip`, `user_agent`, `at`) to the audit table.
- Expiry defaults to 30 days precisely so an abandoned code stops working on its own.

**Deliberately not built:** an admin approval queue. For a single-teacher practice, rotate-and-remove is sufficient and one less state machine. Revisit if enrolment volume grows (§17.2).

---

## 7. Domain model (frontend types)

Mirrors §13. IDs are UUID strings; timestamps ISO 8601 UTC.

```ts
type Role = 'admin' | 'student';

interface User {
  id; email; fullName; role: Role;
  hasPassword: boolean;                 // false for Google-only accounts
  linkedProviders: ('google')[];
  mustChangePassword: boolean;
  createdAt;
}

interface Class {
  id; name; description?;
  studentCount: number;
  selfJoinEnabled: boolean;
  joinCode?: { code: string; expiresAt: string; maxUses: number | null; usesCount: number }; // admin only
  createdAt;
}

interface MediaAsset {
  id; kind: 'image' | 'audio';
  url: string;                          // short-lived signed URL
  mimeType: string; bytes: number;
  durationMs?: number;                  // audio only
  originalFilename: string;
}

type QuestionType =
  | 'single_choice' | 'multiple_choice' | 'true_false'
  | 'fill_blank' | 'short_answer';

interface AudioPolicy {
  maxPlays: number | null;              // null = unlimited
  allowSeek: boolean;                   // default false for listening
  showTranscriptAfterSubmit: boolean;
}

interface Question {
  id; type: QuestionType;
  prompt: string;                       // Markdown, rendered sanitized
  media?: MediaAsset;
  audio?: AudioPolicy;                  // present iff media.kind === 'audio'
  transcript?: string;                  // admin-authored; student sees it only per policy
  options?: { id; text; isCorrect: boolean }[];
  blanks?: { id; ordinal: number; acceptedAnswers: string[]; caseSensitive: boolean }[];
  points: number;
  explanation?: string;
  sampleAnswer?: string;                // short_answer; ADMIN ONLY, never in a student payload
  tags: string[];
}

interface Section { id; title; instructions?; questionIds: string[] }

interface Test {
  id; title; description?;
  sections: Section[];
  totalPoints: number;                  // server-computed
  status: 'draft' | 'published' | 'archived';
  currentVersion: number;
  createdAt; updatedAt;
}

interface IntegrityPolicy {
  requireFullscreen: boolean;
  blockCopyPaste: boolean;
  maxFocusLoss: number;                 // -1 = none allowed; 0 = unlimited
  onLimitExceeded: 'warn' | 'flag' | 'auto_submit';
}

interface Assignment {
  id; testId; testVersionId; testVersion: number;
  targets: { classIds: string[]; studentIds: string[] };
  window: { opensAt: string; closesAt: string };
  durationMinutes: number;              // server-enforced
  maxAttempts: number;
  shuffleQuestions: boolean; shuffleOptions: boolean;
  review: { showScore: boolean; showCorrectAnswers: boolean; showExplanations: boolean };
  integrity: IntegrityPolicy;
  status: 'scheduled' | 'open' | 'closed';
}

type AttemptStatus = 'in_progress' | 'submitted' | 'timed_out' | 'graded' | 'voided';

interface Attempt {
  id; assignmentId; studentId; testVersionId; attemptNo: number;
  status: AttemptStatus;
  startedAt; deadlineAt; submittedAt?; gradedAt?;
  answers: Record<string, Answer>;
  audioPlays: Record<string /*questionId*/, number>;   // server-authoritative (§11.4)
  score?: { earned: number; total: number; pendingManual: number };
  integrity?: { focusLossCount: number; flagged: boolean };
}

type Answer =
  | { type: 'choice'; optionIds: string[] }
  | { type: 'true_false'; value: boolean }
  | { type: 'fill_blank'; values: Record<string, string> }
  | { type: 'text'; value: string };
```

**Invariant:** an attempt references `testVersionId`. Editing a published test creates a new version; in-flight attempts keep rendering the version they started with. In the student flow, test content is fetched **only** via `GET /app/attempts/:id`.

---

### 7.1 Word milestone content contract (foundation)

Every question write and publication uses the same interaction invariants:
nonempty prompt and option text, supported type, choice/key cardinality,
nonempty accepted blank answers, unique blank ordinals matching the prompt,
media/audio consistency and content projection validity. Single-choice and
true/false require exactly one correct option; true/false has exactly two options.
Points must be greater than zero, no greater than 999999.99 and have no more than
two decimal places. Do not silently round source or teacher-entered points.
Publication requires a nonempty exam and an exact total within numeric(8,2).

`ContentDocument` is an application-owned discriminated union: `legacy_markdown_v1`
retains the exact historical Markdown string; `semantic_v1` contains typed
paragraphs, three heading levels, lists, tables, asset references and inline
text/marks, safe links, line breaks and stable gaps. Its normative structural
schema is in `api/openapi.yaml`; aggregate and cross-node rules are described in
[the content contract](plan/19-content-contract.md) and enforced by both domain
validators. Unknown fields, answer metadata, raw HTML nodes and editor-specific
JSON are refused. Text is not rewritten or Unicode-normalized during validation.

The general content components and value object provide the foundation for the
inline option and question prose profiles below. Each new profile needs matching
persistence, publication, restoration and delivery before enabling its writes.
Media/group integration is pending.
The renderer never resolves an asset ID
without an authorized media binding. Existing question payloads above remain the
active contract with the additive option and prose rollouts below.

**Inline option rollout (W-05a).** Choice options may carry optional `content`
using the `OptionContent` subset: exactly one semantic paragraph of text marks
and line breaks. Existing option text is literal, not Markdown, and stays exact.
`text` is the deterministic plain projection when `content` is present. Preserve
both fields in bank copies, version snapshots, restored drafts, previews,
attempts, results and teacher review. The document cannot contain links, gaps,
assets, source metadata or answer keys; grading remains in normalized columns.
An omitted document on update may retain existing formatting only when the
current option ID and text match. Stale or changed legacy writes fail atomically;
explicit `content: null` removes formatting. `VITE_RICH_OPTION_EDITOR=true`
enables the pilot affordance to format plain options; readers and editing
existing rich options remain available. This is not an import release or final
editor acceptance. Grouped content and shared audio remain on their existing paths until their
separate integration gates pass.

**Question prose rollout (W-05b).** Optional `promptContent` and
`explanationContent` support semantic paragraphs, headings, lists and tables with
text marks, breaks and safe links. `QuestionContent` excludes assets and gaps;
`QuestionPromptContent` additionally allows bound gaps for fill-blank questions. `prompt`/`explanation`
are exact plain projections when their document exists; otherwise they retain
legacy Markdown semantics. Rich fill-blank prompts require a bijection between gap node IDs and blank
`gapId` values. Labels never bind answers. Legacy `{{n}}` prompts retain ordinal
binding; rich explanations support all five types.
Snapshots, restoration, bank duplication and all relevant readers preserve these
fields. Active attempts and learner previews never contain explanations; results
release both explanation fields only when `review.showExplanations` permits it.
Legacy writes omitting a rich field must leave its companion string unchanged;
otherwise the update fails atomically. Explicit null clears a document. New
rich authoring is opt-in via `VITE_RICH_QUESTION_EDITOR`; existing documents stay
editable. Conversion from Markdown is explicit, validates the supported subset
and refuses unsupported structures without changing the original.

Formatted clipboard content follows the same principle: parse locally into the
allowlisted semantic vocabulary, preview the complete resulting field, and apply
only after confirmation. Preserve supported marks, list starts, table spans and
safe links; adapt fonts, colors and spacing to the application's design. Reject
files, active/hidden content, unbound gaps, unsupported styles or incomplete
structure as one paste. No fallback to text without the teacher explicitly using
plain-text paste. The converter cannot infer answer keys from visual formatting.
Cancel/stale preview leaves current edits unchanged; one undo reverses insertion.
Field profiles and aggregate budgets apply to the whole resulting document.

### 7.2 Word milestone group ordering (approved, not yet enabled)

For the new group-aware delivery version, `shuffleQuestions` shuffles complete
question groups and standalone questions as units within each section. Sections
retain their authored order. Members of a shared passage/listening/cloze group
retain their authored order even when the groups move. With shuffle disabled,
all units retain authored order. These rules were approved by Thuong as D-03.

Snapshot group membership and member order at publication. Persist the delivery
algorithm version with the assigned snapshot/attempt so reload, takeover and
restoration cannot choose a new deal; preserve stable answer IDs. Historical
versions/attempts continue using their existing section-scoped algorithm and
seeds. `test_versions.delivery_version` stores this marker: existing standalone
versions default to `section_v1`; prerelease group snapshots are classified as
`group_v1` without changing their content or identities. New publications
explicitly write `group_v1`, whose standalone ordering matches `section_v1`.
Rollback refuses to remove the marker while group-aware snapshots exist.
A choice member with semantic references to its option labels declares
`optionOrder: fixed`; assignment with option shuffling is rejected for that member.
Other members use `shuffle`; this never changes the member order inside a group.

### 7.3 Word milestone group graph (contract foundation, not yet enabled)

`QuestionGroup` owns ordered members and materials, optional rich instructions,
and explicit recording bindings. A material's gap targets either a choice member
or the stable `blankGapId` of a rich fill-blank member. Printed labels and mutable
answer-row IDs cannot bind responses. Each material gap has exactly one target;
a response appears only once across that group's material gaps. Gap names are
local to their material or question. Empty groups are valid drafts but cannot
publish; existing sections are not reinterpreted as groups.

The complete resolved context is bounded to 200 members, 16 materials, 16 shared
recordings and 4 MiB, with per-document content limits still enforced. Materials
may repeat an audio asset but resolve it to one recording binding in the group;
the same asset cannot also grant a per-question allowance inside that group.
Distinct member recordings can keep their existing per-question policy. Asset
kind, existence, authorization and deletion locks remain required at persistence.

Copying remaps group, question, material, answer, gap and recording identities,
preserving all grading data, labels, order and content. Immutable media IDs are
reused through new protected bindings. Group authoring endpoints remain gated;
the safe preview and attempt readers below are additive. Relational ownership uses a nullable section owner for each group
(null means an independent bank group) and explicit ownership/order on its member questions.
Section units distinguish standalone questions from groups. Material gaps and
media references have relational bindings; cross-group response/playback links
are rejected. Owned children cannot be archived separately. Restrictive owner
foreign keys require explicit whole-graph cleanup; existing section deletion
cannot leave questions detached from their context. Internal graph creation and
copy materialization are atomic with their audits. Legacy question operations
hide or refuse owned children; active and archived groups protect referenced
media under the same asset lock used by deletion. Full-group editing checks an
aggregate revision; section groups also check the enclosing test revision. Edits
replace content and member order atomically. Bank archive/restore/delete checks
the same revision; permanent deletion requires archival and keeps audit history.
Removing a section-owned group deletes its complete draft graph and compacts unit
order. Copy materialization checks an observed source revision and creates new
editable identities. Complete draft readers, snapshots, delivery and UI integration
remain required before enabling group authoring.

Draft totals and tag filters include owned members alongside standalone questions.
Listening counts count each question once when it has its own audio or a shared
group recording. Independent bank groups are not part of a test's totals. The
legacy question-only whole-outline writer refuses a draft containing groups with
`GROUP_OUTLINE_REQUIRED` before changing metadata or structure; metadata-only
updates remain supported. Group-aware outline editing must ship before enabling
group authoring.

Publication freezes the entire group into version-owned rows, including ordered
units, member order, materials, stable gap targets and explicit recording policy.
Frozen context points only to frozen question/blank identities and immutable media;
editing or deleting a source group cannot alter it. The app role cannot update
these new snapshot graph rows. Deleting an unreferenced version removes its owned
graph together. Restoring a version replaces the current owned draft graph with
fresh editable copies and remaps both ends of material/blank gap links. Duplicating
a draft preserves mixed unit order and independently copies every group; standalone
bank references retain their existing duplication semantics. Permanent test deletion
removes its owned draft groups after the assigned-version reference checks pass.
Published previews resolve the selected default when no version is specified,
read one coherent frozen version, and expose ordered sections/groups plus safe
material bindings. Their projection never selects answer keys or transcripts;
media URLs come only from relational bindings on that version. Group material
appears before its first member, and material gaps link to the corresponding
question. Teacher playback consumes no student allowance. Desktop and 320px phone
preview modes run inside the existing admin shell (minimum supported width 768px).
Existing version questions and historical attempts are unchanged.

Archived tests must be restored before creating a draft from a version or changing
their default version. Both flat and grouped tests return `TEST_ARCHIVED` without
rewriting content. An unused, non-current version may still be deleted while its
parent is archived; existing reference checks continue protecting assigned history.

Start, resume and read-attempt responses include the same safe frozen group
projection. Membership comes from the attempt's version, regardless of the test's
current default. The tests module owns the reader, invoked through an application
port after attempt authorization. Shared media is reachable only through protected
relational bindings on a version the learner has an attempt on; assignment
targeting alone grants no media access. A missing context reader fails explicitly
instead of serving grouped questions without their materials. Learner material
uses the preview renderer: side-by-side with answers when space permits, above
answers on phones with remembered collapse state. Shared audio controls remain
available while material text is collapsed. Stable gap targets navigate to the
question or blank input without losing pending answers. Group players stay mounted
across child navigation.

Learner results retain the frozen material when filtering questions and expose
only transcripts released by each recording's `showTranscriptAfterSubmit` flag.
The score, answer-key and explanation flags do not override that release policy.
Teacher paper review includes the complete shared transcript and that attempt's
recording counts. Grading by question includes only the selected question's
group, without an aggregate playback count across students. Material links focus
the matching question where navigation is available. Review playback is unlimited
and does not append attempt playback events or change recorded counts. Legacy
flat results/reviews keep their existing payload and require no group reader.


---

## 8. Screens & routes — admin (`/admin/*`)

`AdminLayout`: sidebar + top bar. Collapsible sidebar ≤1280px. Minimum supported width 768px.

| Route | Screen | Key behaviour |
|---|---|---|
| `/admin` | Dashboard | Open assignments, attempts awaiting grading, active students, flagged attempts, recent attempts. |
| `/admin/tests` | Tests list | Title, status, #questions, total points, updated. Filter by status. Create / duplicate / archive; permanent delete for unreferenced archived tests. |
| `/admin/tests/new`, `/admin/tests/:id/edit` | Test builder | Left: outline with drag-to-reorder. Right: question editor incl. **audio attach** (§11.1). Autosave debounced 1.5s. **Publish** validates: `points > 0`; choice questions have ≥1 correct option; `fill_blank` has ≥1 accepted answer per blank; audio questions have a processed asset; no empty sections. |
| `/admin/tests/:id` | Test detail | Student-eye preview of any version, with history in the shared right sidebar. Restore a snapshot into a draft, select the default for future assignments, or delete an unused non-default version. |
| `/admin/question-bank` | Question bank | Type/tag filters + full-text search. CRUD. Audio badge + inline preview. CSV import (P1). |
| `/admin/media` | Media library | Uploaded audio/images: filename, duration, size, where used. Delete blocked if referenced by any published version. |
| `/admin/assignments` | Assignments list | Test, targets, window, status, `submitted/total`, flagged count. |
| `/admin/assignments/new` | Create assignment | Published test, targets, window, duration, attempts, shuffle, review policy, integrity policy (§10.3). |
| `/admin/assignments/:id` | Monitor | Per-student: not started / in progress (live remaining, live focus-loss) / submitted / graded. Poll 15s while `open`. Extend deadline, reset, void — all with confirm + reason. |
| `/admin/attempts/:id` | Review & grading | Per question; auto-graded shown; `short_answer` gets points input + comment + **sample answer panel**. Audio questions show plays used vs allowed. **Integrity timeline tab** (§10.4). "Finish grading" → `graded`. |
| `/admin/students` | Students | Table + create/edit. Linked providers, `joined_via`. Reset password. CSV import (P1). |
| `/admin/classes`, `/admin/classes/:id` | Classes | CRUD, members, **join-code panel** (§6.4). |
| `/admin/settings/:section?` | Settings | Profile (default), security (password and Google) and preferences (language), with desktop section navigation and a mobile select. |

Admin list behaviour (approved change request, 2026-09-22):

- Tests, questions, media, classes, students and assignments support explicit multi-selection and confirmed bulk removal. Class rosters support bulk membership removal. Failed items remain selected with an individual explanation; successful items are not retried. Attempt history, grading records and integrity/audit events remain retained.
- Tests and questions expose duplication beside the row menu and inside it. A duplicate leaves the teacher on the list and marks the source and newly created row until navigation.
- List search, filters and pagination survive navigation to a child and back for the signed-in session. Explicit shared URLs take precedence. The question bank shows updated time and orders newest-created questions first.
- The question bank's usage count expands a nested table of currently attached test outlines, fetched on demand, with links to their builders. Frozen published snapshots are excluded from this count and remain unaffected by bank edits.
- Archive tests/classes or disable student accounts before permanent deletion. Assignment deletion requires a draft or closed assignment without attempts. Foreign-key references to assigned work block deletion. Student actor references in retained audit history also block deletion; the existing manual anonymization policy remains separate.
- Published snapshots are never edited in place. Editing an old version copies its frozen content into new bank questions and replaces the editable draft after confirmation. Existing assignments retain their version. Switching the default does not reset the monotonically increasing publication counter. Only unused, non-default versions can be deleted.
- Empty builder groups accept question drops; double-clicking a group title opens rename. Switching questions flushes the current editor and uses the returned saved record. Fresh starter text disappears on focus without scheduling an invalid empty save. Tags suggest recent and matching existing names.
- Assignment duration offers 30, 45 and 60 minutes plus a custom minute input. Attempt count is a number input defaulting to 1. Focus loss offers unlimited, none allowed, or a custom positive count.

## 9. Screens & routes — student and public

`StudentLayout`: minimal top bar, no sidebar, mobile-first, safe-area padding.
Navigation branches at 1024px; the route outlet stays mounted across that
breakpoint. Home and classes fill the available width with adaptive card grids.
Intro and results are centred at a maximum 720px reading width, with
facts and primary actions in the same flow. The focus engine retains a separate
resizable question navigator (256px default, 224–384px), with a preference
independent of the teacher panel. `PublicLayout`: logo + centred content.

| Route | Layout | Key behaviour |
|---|---|---|
| `/join`, `/join/:code`, `/join/:code/confirm` | Public | §6.2. Invalid/expired/exhausted code → distinct, plain messages, no hints about which classes exist. |
| `/login` | Public | Password form + "Tiếp tục với Google". |
| `/app` | Student | All in-progress attempts ordered by deadline, then available / upcoming / completed (score if allowed). Class and status filters in the URL, clear-filter empty state. Available cards say "Xem chi tiết"; only the intro starts the clock. |
| `/app/assignments/:id` | Student | Intro: title, instructions, duration, attempts used/allowed, review policy, **integrity rules stated plainly**, audio rules if any (the allowed plays and that additional plays are recorded, per §11.4). Start / Resume. |
| `/app/attempts/:id` | Focus | The engine. §10, §11.3. |
| `/app/attempts/:id/result` | Student | Score (if allowed), per-question review honoring `review.*`, transcript if `showTranscriptAfterSubmit`. "Pending grading" notice when the score is published and `pendingManual > 0`. Summary above answers, full paper title on phones. Wrong-answer filters exist only when scores are published; empty filters explain why and offer all questions. |
| `/app/classes` | Student | Classes joined with assignment counts and links to `/app?classId=…`; one join action → `/join`. |
| `/app/settings/:section?` | Student | Profile (default), security and preferences share section navigation with teacher settings. Forms remain mounted while changing section or viewport; mobile uses a section select. |

Shared: `/change-password`, `/403`, `/404`, global error boundary with reload + copyable error ID.

---

Student answers awaiting server confirmation are cached locally per student,
attempt and session until saved or closed. They may be restored before the
server deadline after a reload; a superseded session must not overwrite a newer
session's answers. Explicit sign-out clears the local cache. Manual submission
waits for pending saves and stays open if the final answer cannot be saved.
Keyboard A–D and F work after selecting an option; arrows navigate questions
except inside text editors or other controls that own those keys. The last
right-arrow opens review. Dialogs suspend shortcuts.

## 10. Integrity monitoring (proctoring-lite)

First-class requirement. Lives in `src/features/integrity/`, consumed by `take-test` (capture) and `attempts` (review).

### 10.1 Signals

Every signal produces an append-only event `{ kind, occurredAt, clientSeq, questionId?, meta? }`.

| Kind | Source | Notes |
|---|---|---|
| `tab_hidden` / `tab_visible` | `document.visibilitychange` | Primary tab-switch signal; pair to compute away-duration. |
| `window_blur` / `window_focus` | `window` blur/focus | Catches alt-tab to another **application** — the Visibility API alone misses this. |
| `fullscreen_enter` / `fullscreen_exit` | Fullscreen API | Only when `requireFullscreen` is on. |
| `copy` / `cut` / `paste` | listeners on the question container | Always recorded; **blocked** only when `blockCopyPaste` is on. |
| `context_menu` | `contextmenu` | Recorded; blocking off by default (it breaks assistive tooling). |
| `network_offline` / `network_online` | `navigator.onLine` + fetch failures | Distinguishes cheating from bad wifi. Matters for fairness. |
| `audio_play` / `audio_ended` / `audio_blocked` | player (§11.4) | Drives `maxPlays` and gives the teacher listening behaviour. |
| `resume` | server-side | Re-entry into an `in_progress` attempt: reload, crash, device change. |
| `session_takeover` | server-side | Attempt opened in another tab/device. |
| `page_hide` | `pagehide` | Best-effort final flush via `navigator.sendBeacon`. |

**Deliberately excluded:** devtools-detection heuristics (window-size deltas, `debugger` timing). They false-positive on zoom, split-screen, and extensions, and are bypassed in seconds. Do not implement them.

**Away-duration over count.** A 2-second blur is a notification; a 90-second blur is a search. Store both endpoints so duration is visible, and only count a strike when an away episode exceeds `minAwayMs` (default 3000ms).

### 10.2 Student-facing behaviour

Announced, visible, never silent.

- The intro page states the active rules in plain Vietnamese before starting. If `requireFullscreen` is on, the "Bắt đầu" click is what enters fullscreen (browsers require a gesture).
- First violation: a non-dismissible dialog — what happened, strikes remaining, what happens at zero. The timer keeps running.
- A small persistent indicator shows remaining strikes when a limit is set. `maxFocusLoss = 0` retains the unlimited default; `-1` permits no counted departure; positive values permit that many departures.
- `onLimitExceeded`: `warn` = dialog only; `flag` = attempt marked for the admin, student told; `auto_submit` = immediate submission on exceeding the count, retaining answers for grading and recording the violation. There is no cancellation or extra strike. The final answer/event batch is saved before the server grades and closes. While offline, the attempt is locked locally, pending answers are retained and submission is retried with a visible notice.
- Fullscreen exit shows a "Quay lại toàn màn hình" button. Never trap the student: `Esc` always works and there is always a visible way to leave and submit.

### 10.3 Policy defaults (per assignment)

`requireFullscreen: false`, `blockCopyPaste: true`, `maxFocusLoss: 0`, `onLimitExceeded: 'flag'`. Conservative on purpose — the teacher opts into stricter modes per test.

### 10.4 Admin review

`/admin/attempts/:id` → **Integrity** tab: chronological timeline with event kind, wall-clock time, offset from attempt start, duration for paired events, and the question on screen. Summary strip: total away-time, away episodes ≥ `minAwayMs`, paste count, resume count, audio replays. Neutral text — no red banners, no "CHEATING DETECTED". The teacher judges; the app reports.

### 10.5 Honest limits — say this in the UI help text

Browser monitoring detects *this tab* losing focus. It cannot see a second device, a phone beside the laptop, a person in the room, or a printed sheet. The signals are **evidence for a conversation**, not proof. Do not build auto-zero or auto-ban features that assume otherwise.

### 10.6 Client implementation

- One `useIntegrityMonitor(attemptId, policy)` hook owns all listeners, registered and torn down in a single `useEffect`. No scattered listeners.
- Events buffer in memory + `sessionStorage`, flush with the autosave batch, and immediately on `pagehide` via `sendBeacon`.
- `clientSeq` is monotonic so the server can order events despite clock skew.
- Failed background event flushes do not block answering or manual submission.
  The immediate `auto_submit` policy retries its final answer/event batch before
  confirming submission so the violation is retained with the answers.
- An event transport failure alone does not block input. The explicit `auto_submit` policy locks further answering once exceeded, while preserving and retrying the pending answers.

---

## 11. Audio / listening questions

### 11.1 Admin: upload and attach

- In the question editor, an audio question has: file upload, transcript textarea (admin-authored, optional), and the `AudioPolicy` controls (`maxPlays` default 2, `allowSeek` default **false**, `showTranscriptAfterSubmit` default true).
- Accepted: `audio/mpeg` (.mp3) and `audio/mp4` / `audio/aac` (.m4a). **Reject everything else** with a plain message. These two cover every current browser without transcoding; supporting `.ogg`/`.wav`/`.webm` means either transcoding or a Safari support matrix, and neither is worth it in v1.
- Limits: 10 MB, 5 minutes. Validate **server-side** by sniffing magic bytes and probing duration — never trust the `Content-Type` header or the file extension.
- Upload goes **through the Go backend** in v1 (small files, low volume, and the backend must validate anyway). Presigned direct-to-R2 upload is the P1 optimisation; design the API so switching does not change the client contract beyond the upload call.
- Assets are **immutable**. Re-uploading creates a new `media_assets` row; it never overwrites an existing key. This is what lets `test_version_questions` reference an asset without copying the file.
- Client-side pre-check before upload: read duration via an `<audio>` element and reject early, so a student's teacher does not wait 10 MB to be told no.

### 11.2 Storage and delivery

- Bucket `quizzivy-media`, key `audio/{asset_id}.{ext}`. Bucket is **private**; no public listing, no public read.
- Served via **short-lived signed URLs** (10 min), minted per request by the backend. The frontend treats `MediaAsset.url` as expiring and refetches on `403`.
- `Cache-Control: private, max-age=600` — long enough to survive a replay, short enough that the signed URL does not outlive its cache entry.
- Honest note: `controlsList="nodownload"` and hiding the URL are **not** protection. A determined student can save the file. Signed short-lived URLs raise the cost slightly; do not invest further. If a listening file must never leak, do not put it online.

### 11.3 Player component

`features/media/AudioPlayer.tsx`. Custom controls over a native `<audio>`; no third-party audio library.

- Controls: play/pause, elapsed/total time, a progress bar that is **display-only when `allowSeek` is false**, and a plays-remaining indicator.
- `preload="metadata"` so duration renders without downloading the file.
- **Autoplay is impossible** — browsers block audio without a user gesture. The first play is always a tap. Do not attempt to auto-start; do not treat the block as an error state.
- iOS Safari: only one audio element plays at a time, and playback must originate from a gesture handler (not an async continuation). Call `.play()` synchronously in the click handler; do not `await` anything before it.
- `allowSeek: false` implementation: no `<input type="range">`, and an `onSeeking` handler that resets `currentTime` to the last known position. Note that OS-level media controls can still seek in some browsers — record an `audio_seek` event rather than pretending it cannot happen.
- Accessibility: real `<button>` elements, `aria-label` on each, keyboard-operable, `aria-live` announcement of plays remaining. When `showTranscriptAfterSubmit` is on, the transcript appears on the result page — this is also the accessibility fallback for hard-of-hearing students.
- One player instance per question. Navigating away pauses and releases it.

### 11.4 `maxPlays` — server-authoritative

The obvious client-side counter resets on reload, which makes the limit meaningless. So:

- Each `play` sends an `audio_play` event; the server increments `attempt_audio_plays (attempt_id, question_id, plays)` and the count is returned in `GET /app/attempts/:id` as `audioPlays`.
- Client renders remaining plays from the server value, optimistically decrements on play, and reconciles on the next fetch.
- Playback is **optimistic**: a failed event POST does not block the audio. A student who goes offline to farm replays will show a gap in the event log, which is exactly what the integrity timeline is for. Blocking playback on a network round-trip would punish bad wifi far more often than it would catch anyone.
- On submit, the server rejects nothing based on play count. Over-limit plays are reported to the teacher, not enforced retroactively.

### 11.5 Word milestone shared recordings (approved, not yet enabled)

D-03 extends the existing audio contract to a recording shared by a group. All
child questions consume the same configured play allowance. Navigating between
children, reloading or resuming the same attempt on another device never grants
a fresh allowance. Two groups may reference the same immutable file and still
have independent counters. The counter identity therefore includes the versioned
group recording binding; the asset ID alone is insufficient.

Freeze that binding and its audio policy in the published version, and keep its
counter scoped to the attempt. Preserve synchronous gesture playback, optimistic
accounting, retry deduplication, teacher reporting and transcript visibility rules
from §11.3–11.4. The future shared player must use this scope consistently in
navigation, event writes and state reconciliation. Its API, relational references,
concurrency tests and student-payload tests must precede enabling shared audio.
Existing per-question audio remains a separately supported historical contract.

W-09c adds a separate shared-play endpoint with `recordingId`, current `sessionId`
and a stable per-gesture `playId`. It follows the existing autosave writable-session
checks and verifies the recording's material binding belongs to the attempt's
version before writing. Retrying the same gesture/recording returns its current
counter; reusing the gesture ID for another recording is a conflict. The counter,
append-only receipt and server `audio_play` event commit together. Counts survive
reload/takeover and are returned as `groupAudioPlays`, keyed by recording ID.
Teacher monitor and timeline excess-play totals include this ledger. New attempts
start with their own allowance. Network acknowledgements and `maxPlays` never gate
the actual browser play call. The browser persists unconfirmed gesture IDs per
learner/attempt until the attempt deadline, clears them on logout and serializes
bounded retries. Confirmed extensions update the recovery deadline. A pending
sync label is distinct from answer save state. A response lost after commit does
not consume a second allowance on retry; late responses cannot mutate another
session. Counts remain monotonic during refetch. Submission attempts a bounded
three-second telemetry flush but remains available during network accounting
failure. Closed/expired attempts reject late telemetry; offline closure cannot
guarantee complete listening evidence. Results and teacher paper review report
the confirmed counts without creating new playback receipts.

The assignment introduction derives listening presence, transcript permission and
the strictest finite allowance from both question audio and shared recordings on
the assigned version. A newer default does not change these facts. The displayed
minimum is not presented as every recording's allowance; individual players show
their own limits. A shared-audio notice explains that member navigation and reload
do not grant a new allowance.

---

## 12. Design guidelines

Deliberate. Do not "improve" them with trendy defaults.

- **Neutral and unstyled by default.** Zinc scale, white surfaces, 1px borders, `shadow-sm` at most. Text `zinc-900`, secondary `zinc-500`.
- **Primary action: dark charcoal (`zinc-900`) buttons, white text.** Not blue, not purple, not indigo.
- **Semantic color only where it carries meaning:** green = correct/success, red = incorrect/error/destructive, amber = warning/time-low. Never decorative.
- **Forbidden:** decorative gradients, glassmorphism/backdrop blur, pulsing rings, glow effects, oversized radii (max `rounded-md` controls, `rounded-lg` cards), emoji in UI chrome.
- **Typography:** system UI stack or Inter. Match the mockup scale: xs 12px, sm 13px, base 14px, lg 17px, xl 20px, with proportional line heights. Phone text inputs stay at 16px to avoid input zoom; student phone buttons, icon controls and question navigation cells are at least 44px in both dimensions; seek tracks have a 44px hit area. `leading-relaxed` in the test view.
- **Icons:** lucide-react, 16px dense / 20px nav, consistent stroke, `aria-hidden` unless standalone.
- **Density:** admin tables dense (~40px rows). Student discovery grids use one column on phones, two from 768px, three from 1536px. Reading pages are centred at 720px; settings use a 192px local navigation column beside a form column capped at 768px, with divided rows and light shadows. Student test view stays spacious, one question centred at max-width ~720px beside the navigator. This approved review supersedes S-13–S-17's placement of ordinary page content in a right panel.
- **Action hierarchy:** resume is primary; opening assignment details and entering a class are secondary. Deadline badges turn amber only within 24 hours. The 320px test footer has previous, an icon-only question-list control and a flexible next/review action. Review submission remains outside the scrolling summary. Submission confirmation offers the submitted paper directly.
- **Explicit states:** single- and multiple-choice instructions identify selection behaviour without revealing the key. Restored answers say they were loaded; exhausted audio allowances explain continued playback is recorded. Login exposes class joining and a reversible password visibility toggle. Signed-in join screens identify the current account and provide a way back.
- **Motion:** 150ms ease-out for control feedback and settings section changes. Cards use subtle shadows and a 2px hover lift on pointer devices. Disable these transitions under `prefers-reduced-motion`; exam inputs remain stationary.
- **Audio player:** monochrome. A filled `zinc-900` play button, a thin `zinc-200` track with a `zinc-900` fill. No waveform visualisation, no equaliser animation, no colored accents.
- **Join screens:** single centered card, class name large, one primary button. This is the first thing a new student sees — it should look calm and legitimate, not like a marketing page.
- **Dark mode:** not in v1, but theme via CSS variables / Tailwind tokens so it can be added without touching components.
- **Empty states:** one short sentence + one primary action. No illustrations.
- **Vietnamese first.** Design for longer Vietnamese strings; avoid fixed-width labels.
- **Integrity UI is calm.** Plain dialogs, plain text. No alarm iconography, no shame.

---

## 13. Database design (PostgreSQL 18)

### 13.1 Binding references

Two authorities govern all schema and query work. Where they conflict, the PostgreSQL docs win.

1. **Neon `postgres-best-practices` agent skill** — <https://github.com/neondatabase/postgres-skills>. Install before writing any DDL:
   ```
   npx skills add neondatabase/postgres-skills
   ```
   An Agent Skill package (`skills/postgres-best-practices/SKILL.md` + `references/schema-design.md`), curated by Postgres practitioners including a PostgreSQL Core Team member. Its schema-design, indexing, and migration guidance is binding for this project.
2. **PostgreSQL 18 official documentation** — <https://www.postgresql.org/docs/18/index.html>. Version-specific behaviour is checked here, not against training-data recollection. Target **PostgreSQL 18**; do not write DDL that silently degrades on 16/17.

**Rule for the agent:** load the skill before proposing DDL or non-trivial SQL, and cite the specific docs section for any PG18-specific construct used.

### 13.2 Conventions

- Schema `app`, not `public`. `REVOKE CREATE ON SCHEMA public FROM PUBLIC`.
- Tables plural snake_case; columns snake_case; no Hungarian prefixes.
- PKs: `uuid PRIMARY KEY DEFAULT uuidv7()` — PG18's built-in time-ordered UUID avoids the B-tree fragmentation random v4 causes on insert-heavy tables. Use `uuidv4()` only where hiding creation time genuinely matters.
- `timestamptz` everywhere; never bare `timestamp`. Store UTC, render `Asia/Ho_Chi_Minh`.
- `text` over `varchar(n)` unless a real business constraint exists; enforce with `CHECK` when it does.
- Scores: `numeric(8,2)`. Never `float`.
- PG enums where the set is genuinely closed (`role`, `attempt_status`); lookup tables otherwise. Adding an enum value is easy, removing one is not.
- Every FK has an explicit `ON DELETE`. Default `RESTRICT`; `CASCADE` only for true owned children.
- Every FK column used in a join or filter gets an index — Postgres does not index FKs automatically.
- `created_at timestamptz NOT NULL DEFAULT now()`; `updated_at` via trigger, not application code.
- Soft delete only where history matters (`questions`, `tests`, `media_assets`); everything else deletes for real.

### 13.3 Core tables

Sketch, not final DDL. The agent produces real migrations after loading the skill.

```sql
CREATE SCHEMA app;

CREATE TYPE app.user_role      AS ENUM ('admin','student');
CREATE TYPE app.test_status    AS ENUM ('draft','published','archived');
CREATE TYPE app.question_type  AS ENUM ('single_choice','multiple_choice','true_false','fill_blank','short_answer');
CREATE TYPE app.attempt_status AS ENUM ('in_progress','submitted','timed_out','graded','voided');
CREATE TYPE app.media_kind     AS ENUM ('image','audio');
CREATE TYPE app.join_source    AS ENUM ('admin','join_code');

CREATE TABLE app.users (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  email         text NOT NULL,
  full_name     text NOT NULL,
  role          app.user_role NOT NULL DEFAULT 'student',
  password_hash text,                              -- NULL = Google-only account
  must_change_password boolean NOT NULL DEFAULT false,
  disabled_at   timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_key ON app.users (lower(email));

CREATE TABLE app.user_identities (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id          uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  provider         text NOT NULL CHECK (provider IN ('google')),
  provider_user_id text NOT NULL,                  -- Google `sub`, immutable
  email_at_link    text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_user_id)
);
CREATE INDEX ON app.user_identities (user_id);
```

**Classes and join codes** (§6):

```sql
CREATE TABLE app.classes (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  name              text NOT NULL,
  description       text,
  self_join_enabled boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.class_join_codes (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  class_id   uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  code_hash  bytea NOT NULL,                       -- sha256(normalized code); never plaintext
  code_hint  text NOT NULL,                        -- last 4 chars, for admin display
  expires_at timestamptz NOT NULL,
  max_uses   integer CHECK (max_uses IS NULL OR max_uses > 0),
  uses_count integer NOT NULL DEFAULT 0,
  revoked_at timestamptz,
  created_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (code_hash)
);
-- one active code per class
CREATE UNIQUE INDEX class_join_codes_one_active
  ON app.class_join_codes (class_id)
  WHERE revoked_at IS NULL;

CREATE TABLE app.class_members (
  class_id   uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  joined_via app.join_source NOT NULL,
  joined_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (class_id, user_id)
);
CREATE INDEX ON app.class_members (user_id);
```

The plaintext code is shown to the admin **once at generation** and thereafter only as `code_hint`. If they lose it, they rotate. Storing it hashed means a database dump does not hand over class access.

**Media** (§11):

```sql
CREATE TABLE app.media_assets (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  kind              app.media_kind NOT NULL,
  storage_key       text NOT NULL UNIQUE,          -- R2 object key; immutable
  mime_type         text NOT NULL,
  bytes             bigint NOT NULL CHECK (bytes > 0),
  duration_ms       integer CHECK (duration_ms IS NULL OR duration_ms > 0),
  original_filename text NOT NULL,
  checksum_sha256   bytea NOT NULL,                -- dedupe identical re-uploads
  uploaded_by       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CHECK (kind <> 'audio' OR duration_ms IS NOT NULL)
);
CREATE INDEX ON app.media_assets (kind, created_at DESC);
```

**Question bank** — normalize; do not stuff options into `jsonb`. Options and blanks are ordered, queried, and graded against.

Migration `00032_add_option_content.sql` adds nullable `content jsonb` to
`question_options` and `test_version_options` for the bounded inline AST only.
Option identity, order, text and correctness stay relational. No grading key is
placed in the AST, and no legacy row is rewritten. Read-compatible binaries are
the rollback floor after rich writes; rolling back the migration discards rich
formatting and is only a pre-rollout development operation.

Migration `00033_add_question_content.sql` adds nullable `prompt_content jsonb`
and `explanation_content jsonb` to `questions` and `test_version_questions`.
These columns hold the bounded prose AST from §7.1; companion strings remain
exact plain projections. Migration `00034_add_question_gap_bindings.sql` enables
rich fill-blank prompts and adds nullable, question-scoped unique `gap_id` to
`question_blanks` and `test_version_blanks`. Null identifies legacy Markdown.
Bank copies and restored drafts remap gap identities and answer bindings together;
snapshots freeze them. Answer submissions remain keyed by frozen blank UUID.
Down refuses existing rich fill-blank rows instead of silently discarding bindings.
No legacy rows are rewritten, no grading keys move, and the same read-compatible rollback floor
applies after prose writes.

```sql
CREATE TABLE app.questions (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  type            app.question_type NOT NULL,
  prompt          text NOT NULL,
  media_asset_id  uuid REFERENCES app.media_assets(id) ON DELETE RESTRICT,
  audio_max_plays integer CHECK (audio_max_plays IS NULL OR audio_max_plays > 0),
  audio_allow_seek boolean NOT NULL DEFAULT false,
  audio_show_transcript_after boolean NOT NULL DEFAULT true,
  transcript      text,
  points          numeric(8,2) NOT NULL CHECK (points > 0),
  explanation     text,
  sample_answer   text,                            -- OQ-4; admin-only, never in a student payload
  tags            text[] NOT NULL DEFAULT '{}',
  created_by      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX ON app.questions USING gin (tags);
CREATE INDEX ON app.questions USING gin (to_tsvector('simple', prompt));
CREATE INDEX ON app.questions (media_asset_id) WHERE media_asset_id IS NOT NULL;

CREATE TABLE app.question_options (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal     smallint NOT NULL,
  text        text NOT NULL,
  is_correct  boolean NOT NULL DEFAULT false,
  UNIQUE (question_id, ordinal)
);
```

`question_blanks` and `question_blank_answers` mirror this for `fill_blank`.

**Tests and versioning** — the load-bearing decision. On publish, snapshot resolved content into version tables so editing a test can never mutate an in-flight or historical attempt:

```
tests(id, title, description, status, current_version, last_published_version, ...)
test_versions(id, test_id, version, published_at, total_points, UNIQUE(test_id, version))
test_version_sections(id, test_version_id, ordinal, title, instructions)
test_version_questions(id, test_version_section_id, ordinal, source_question_id,
                       type, prompt, media_asset_id, audio_max_plays, audio_allow_seek,
                       audio_show_transcript_after, transcript, points, explanation, sample_answer)
test_version_options(id, test_version_question_id, ordinal, text, is_correct)
test_version_blanks / test_version_blank_answers
```

Snapshotting into **normalized rows** rather than one `jsonb` blob keeps per-question analytics a plain SQL query later. `source_question_id` preserves the bank link without coupling to it. `media_asset_id` points at the same immutable asset — the file is never copied.

**Assignments and attempts:**

```sql
CREATE TABLE app.attempts (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  assignment_id   uuid NOT NULL REFERENCES app.assignments(id) ON DELETE RESTRICT,
  test_version_id uuid NOT NULL REFERENCES app.test_versions(id) ON DELETE RESTRICT,
  student_id      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  attempt_no      smallint NOT NULL CHECK (attempt_no > 0),
  status          app.attempt_status NOT NULL DEFAULT 'in_progress',
  session_id      uuid NOT NULL,                   -- takeover detection
  started_at      timestamptz NOT NULL DEFAULT now(),
  deadline_at     timestamptz NOT NULL,            -- authoritative, server-computed
  submitted_at    timestamptz,
  graded_at       timestamptz,
  score_earned    numeric(8,2),
  score_total     numeric(8,2),
  focus_loss_count integer NOT NULL DEFAULT 0,
  flagged         boolean NOT NULL DEFAULT false,
  UNIQUE (assignment_id, student_id, attempt_no),
  CHECK (deadline_at > started_at)
);
CREATE UNIQUE INDEX attempts_one_live
  ON app.attempts (assignment_id, student_id) WHERE status = 'in_progress';
CREATE INDEX ON app.attempts (assignment_id, status);
CREATE INDEX ON app.attempts (student_id, started_at DESC);

CREATE TABLE app.attempt_answers (
  attempt_id   uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id  uuid NOT NULL REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  payload      jsonb NOT NULL,
  auto_score   numeric(8,2),
  manual_score numeric(8,2),
  final_score  numeric(8,2) GENERATED ALWAYS AS (coalesce(manual_score, auto_score)) VIRTUAL,
  grader_comment text,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (attempt_id, question_id)
);

CREATE TABLE app.attempt_audio_plays (               -- §11.4
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  plays       integer NOT NULL DEFAULT 0 CHECK (plays >= 0),
  last_played_at timestamptz,
  PRIMARY KEY (attempt_id, question_id)
);

CREATE TABLE app.attempt_events (                    -- append-only, §10
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  kind        text NOT NULL,
  occurred_at timestamptz NOT NULL,                  -- client time, offset-corrected
  received_at timestamptz NOT NULL DEFAULT now(),
  client_seq  integer NOT NULL,
  question_id uuid,
  meta        jsonb,
  UNIQUE (attempt_id, client_seq)
);
CREATE INDEX ON app.attempt_events (attempt_id, occurred_at);
```

`final_score` uses PG18 **virtual generated columns** — computed on read, no storage, never stale (the default for generated columns in 18). Grading precedence is a pure function of two columns and must never drift.

`attempt_events` uses `bigint IDENTITY` rather than a UUID: a high-volume append-only log read only by `attempt_id` is better served by a narrow sequential key. The application never updates or deletes it. An explicit privileged retention operation may delete events only after both assignment closure and server receipt are more than thirteen calendar months old, using server `received_at` and a UTC cutoff; each batch is audited. The audit log remains retained. Disabled accounts are not automatically erased. Manual requested student anonymization removes structured identity and credentials while preserving historical IDs and records; free text, old audit entries and backups require separate review (see `docs/setup/operations.md`).

### 13.4 Audit log

`audit_log(id, actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff jsonb)`. Required entries: class enrolment (§6.5), join-code generation and rotation, attempt reset/void/extend, password reset, media deletion, test publish.

Use PG18's `OLD`/`NEW` in `RETURNING` to capture the diff in the same statement as the update, rather than a read-then-write.

### 13.5 Security in the data layer

- Refresh tokens stored as SHA-256 hashes: `token_hash`, `family_id`, `user_id`, `expires_at`, `revoked_at`, `replaced_by`, `user_agent`, `ip`.
- Join codes stored as SHA-256 hashes (§13.3). Lookup by hash, compared in constant time.
- Passwords: Argon2id (bcrypt cost ≥ 12 if unavailable).
- `sample_answer`, `transcript`, `is_correct`, and `accepted_answers` must never reach a student response. Enforce with explicit column lists — **no `SELECT *` in student-facing paths.** Add a test that asserts these keys are absent from `GET /app/attempts/:id`.
- Least-privilege DB roles: the app connects with DML on `app` only, not as owner. Migrations run as a separate role.

### 13.6 PG18 features — use and skip

| Feature | Use? | Why |
|---|---|---|
| `uuidv7()` | **Yes**, all PKs | Time-ordered → index locality; avoids v4 fragmentation |
| Virtual generated columns | **Yes**, `final_score` | Computed on read, no storage, never stale |
| `OLD`/`NEW` in `RETURNING` | **Yes**, audit log | One statement instead of read-then-write |
| B-tree skip scan | Passive | One multicolumn index serves more shapes — do not add redundant prefix indexes preemptively |
| `NOT NULL NOT VALID` + `VALIDATE` | **Yes**, in migrations | Adds `NOT NULL` without a full-table lock |
| Async I/O | Passive | Server-side; no schema impact |
| Temporal constraints (`WITHOUT OVERLAPS`) | **No** in v1 | No business rule needs non-overlapping validity periods. Revisit for class scheduling. |
| OAuth DB authentication | **No** | That is Postgres *connection* auth, unrelated to §5's application-level Google Sign-In. Do not confuse the two. |

### 13.7 Migrations

- **goose**, SQL, forward-only, one concern per migration.
- Expand-contract for anything breaking: add nullable → backfill → start writing → `NOT NULL` → drop old. Never in one migration.
- `CREATE INDEX CONCURRENTLY` runs outside a transaction — mark those `-- +goose NO TRANSACTION`.
- Test every migration against a copy of real data before production. On Neon, a branch per migration, reset from parent between runs.
- Seed data in `seed/`, never in migrations.

### 13.8 Query discipline

- No `SELECT *` in application code.
- Keyset pagination everywhere (`WHERE (created_at, id) < ($1,$2) ORDER BY created_at DESC, id DESC LIMIT $3`), not `OFFSET`. `uuidv7()` PKs make this natural.
- `EXPLAIN (ANALYZE, BUFFERS)` any query touching `attempts`, `attempt_answers`, or `attempt_events` before merging.
- N+1 is the default failure mode of the grading and monitor screens — one query for an attempt's answers, one query for an assignment's monitor rows.

---

## 14. Testing & definition of done

**Unit (Vitest):** zod schemas, utils, take-test store (timer math, resume merge, submit idempotency), integrity buffering and strike counting, join-code normalization, audio plays-remaining reconciliation.

**Component (Testing Library):** each form validates and submits; guards redirect correctly; a renderer test per `QuestionType`; integrity dialog states; audio player states (idle / playing / plays exhausted / seek blocked / load error).

**E2E (Playwright + MSW or a seeded backend):**
1. Admin logs in → creates a test with one of each question type **including audio** → publishes → assigns.
2. Student logs in with **password** → starts → answers → reloads mid-test → answers persist → submits → sees result.
3. **Join flow:** anonymous visitor opens `/join/:code` → sees class name → Google sign-in (mocked GIS) → account created, enrolled, lands on `/app`.
4. **Expired code** → plain error, no account created, nothing leaked about the class.
5. Timer expiry auto-submits.
6. Tab-switch fires `tab_hidden`, warning dialog appears, event lands on the admin integrity timeline.
7. Second tab on the same attempt supersedes the first; the first goes read-only.
8. **Audio `maxPlays`:** play twice → play button disabled → **reload** → still disabled (server-authoritative).
9. Student payload assertion: `GET /app/attempts/:id` contains no `isCorrect`, `sampleAnswer`, `transcript`, or `acceptedAnswers`.

**Definition of done for any task:**
- [ ] TypeScript strict passes; no `any` without a comment
- [ ] Lint clean
- [ ] Loading / error / empty states present
- [ ] All strings via `t()`, keys in both `vi` and `en`
- [ ] Keyboard-operable
- [ ] Tests added/updated per the levels above
- [ ] Any DDL reviewed against §13 and the Neon skill
- [ ] Any new public (unauthenticated) endpoint is rate-limited and leaks nothing (§6.5)
- [ ] No new dependency without a stated reason in the PR description

---

## 15. Assumed API surface

Base `VITE_API_BASE_URL`, JSON, `Authorization: Bearer <access>`.

```
# public (unauthenticated — rate-limited, §6.5)
POST   /join/preview                    {joinCode} → {classId, className, teacherName}
POST   /auth/login                      {email,password} → {accessToken,user}
POST   /auth/google                     {code,codeVerifier,redirectUri,joinCode?} → {accessToken,user}
POST   /auth/refresh                    (cookie) → {accessToken}

# authenticated
POST   /auth/logout
GET    /auth/me                         → User
POST   /auth/change-password
POST   /auth/google/link                link Google to current account
DELETE /auth/google/link                rejected if it would leave no login method

# admin
GET    /admin/tests?status=&q=&cursor=
POST   /admin/tests | GET /:id | PATCH /:id
POST   /admin/tests/:id/publish         → new version
POST   /admin/tests/:id/duplicate
GET    /admin/questions?type=&tag=&q=&cursor=
POST   /admin/questions | PATCH /:id | DELETE /:id
POST   /admin/media                     multipart → MediaAsset (validates mime, size, duration)
GET    /admin/media?kind=&cursor=
DELETE /admin/media/:id                 409 if referenced by a published version
GET    /admin/assignments | POST | GET /:id | PATCH /:id
GET    /admin/assignments/:id/attempts  → rows incl. integrity + audio summary
POST   /admin/attempts/:id/extend       {minutes,reason}
POST   /admin/attempts/:id/reset        {reason}
POST   /admin/attempts/:id/void         {reason}
GET    /admin/attempts/:id | GET /admin/attempts/:id/events
POST   /admin/attempts/:id/grade        {items:[{questionId,points,comment}]}
POST   /admin/attempts/:id/finish-grading
GET    /admin/students | POST | GET /:id | PATCH /:id
POST   /admin/students/:id/reset-password
GET    /admin/classes | POST | GET /:id | PATCH /:id
POST   /admin/classes/:id/members | DELETE /admin/classes/:id/members/:userId
POST   /admin/classes/:id/join-code     rotate → {code}  (plaintext returned once)
DELETE /admin/classes/:id/join-code     revoke, disable self-join

# student
POST   /app/classes/join                {joinCode} → Class      (already-authed path)
GET    /app/classes
GET    /app/assignments                 → {dueNow,upcoming,completed}
GET    /app/assignments/:id
POST   /app/assignments/:id/attempts    → create or resume → Attempt + ordered questions + sessionId
GET    /app/attempts/:id                → Attempt + questions + serverTime + audioPlays
PATCH  /app/attempts/:id/answers        {sessionId,answers:[...],events:[...]}
POST   /app/attempts/:id/events         standalone flush (sendBeacon path)
POST   /app/attempts/:id/audio-play     {questionId} → {plays, maxPlays}
POST   /app/attempts/:id/submit         idempotent; 409 if already closed
GET    /app/attempts/:id/result
GET    /app/media/:assetId/url          → short-lived signed URL
```

List endpoints return `{ items, nextCursor }` (keyset, §13.8). Student payloads never include `isCorrect`, `sampleAnswer`, `acceptedAnswers`, or `transcript` (the last only per `showTranscriptAfterSubmit`, on the result endpoint).

---

## 16. Delivery phases

Each phase ends deployable. Do not start the next with the current one red.

| Phase | Deliverable | Exit criteria |
|---|---|---|
| **0 — Scaffold** | Vite app, Tailwind, shadcn (neutral), router, i18n, API client, lint/test/CI. **Neon skill installed**; `app` schema + users/identities/classes migrations. R2 bucket + credentials. | `pnpm dev/test/build` green; `/login` renders; migrations clean |
| **1 — Auth, join, shells** | Password login, Google login, refresh + rotation, **join-code flow end-to-end**, guards, both layouts, settings | E2E 2, 3, 4 pass |
| **2 — Admin authoring** | Tests list, builder (all 5 types), question bank, **media upload + validation**, publish + version snapshot | E2E 1 passes |
| **3 — Assign & take** | Assignment create/list, student home, intro, **take-test engine**, **audio player**, **integrity capture** | E2E 5, 6, 7, 8 pass |
| **4 — Grade & results** | Monitor, attempt review + integrity timeline, grading with sample answers, result page honoring review + transcript flags | Full E2E suite green, incl. 9 |
| **5 — Hardening** | Perf budgets, a11y pass, empty/error audit, mobile QA at 360px, rate-limit verification, `EXPLAIN` review of hot queries | Lighthouse a11y ≥ 95; budgets met; no seq scans on hot paths |

P1 after v1: presigned direct-to-R2 upload, CSV import (students, questions), password self-signup with email verification, dark mode, per-student time accommodations, per-question analytics.

The next accepted milestone is [production Word exam import](quizzivy-word-import-spec.md),
implemented through [the six-stage delivery plan](plan/17-word-import.md) and
[teacher workflow](plan/18-word-import-ux.md). The supplied document defines the
target rather than current availability. Existing free-form `.docx` and `.doc`
documents, explicit answer evidence, shared context, semantic formatting,
recoverable review and atomic draft creation are required before general release.
Imported and manually authored exams must share rendering, publication and
grading contracts. Missing answers remain unknown; PDF input, OCR, answer
generation and automatic publication are excluded. Each accepted detailed
contract updates the relevant sections above and OpenAPI before implementation.

Thuong approved two ownership/recovery policies for this milestone:

- Groups and shared materials belong to independent editable copies. Saving a
  dependent group to the bank or copying it into another test carries every
  required member/material and remaps identities; subsequent source edits or
  deletion cannot affect the copy. Relational asset references protect reused
  immutable files from deletion; reuse of bytes is not shared editable content.
- Unsent edits may be stored locally, isolated by account, for at most seven days
  from the unsent revision's creation, and cleared on logout. Recovery is explicit
  and revision-aware; local-only changes never display as server-saved. Local
  storage failure or quota exhaustion must remain visible and cannot silently
  discard unsent changes. Source/answer files are not stored in this recovery
  outbox. This is approved policy, not an implemented durability guarantee yet.

---

## 17. Decisions taken inside the answers — worth a second look

All original open questions are closed. These three sub-decisions were made while implementing the answers, and are the ones most likely to want changing.

1. **Self-join is Google-only** (§6.3). Password self-signup needs verified email, which needs an email provider. If Thuong wants that in v1, add Resend/SES and an `email_verifications` table — the join flow itself is unchanged. *Decide before Phase 1.*
2. **No admin approval queue for self-join** (§6.5). Rotate-and-remove is judged sufficient for a single-teacher practice. If enrolments become high-volume or a code leaks in practice, add a `pending` membership state. *Non-blocking.*
3. **mp3 and m4a only** (§11.1). Covers every current browser with no transcoding. Adding `.ogg`/`.wav` means a transcoding pipeline or a Safari support matrix. *Decide before Phase 2.*

---

## 18. Working agreement for the AI agent

- Read this spec fully before starting any phase. Where §5, §6, §7, §10, §11, §12, or §13 is ambiguous, ask **one** precise question rather than guessing.
- Install and consult the Neon `postgres-best-practices` skill before writing any DDL or non-trivial SQL. Cite the PG18 docs section for any version-specific construct.
- Propose before adding a dependency or deviating from §2, §12, or §13.
- Any endpoint reachable without a session gets a rate limit and a leak review in the same PR.
- Small, reviewable PRs scoped to one feature folder.
- Write the `vi` string first, then `en`. Never leave English-only user-facing text.
- When touching `features/take-test/`, `features/integrity/`, or `features/media/`, run their unit tests before and after every change.
- Never remove a failing test to make CI green; fix it or flag it.
- Keep this document current: when a decision changes, edit the section and bump the version at the top.
