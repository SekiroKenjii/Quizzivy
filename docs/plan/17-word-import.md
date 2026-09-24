# Word exam import — milestone delivery plan

Status: delivery plan accepted by Thuong on 2026-09-22; implementation started.
The planning change itself implements no import functionality. Product owner:
Thuong. Engineering owner:
the implementing developer. Teacher pilot acceptance belongs to Thuong and the
participating teachers, not to automated tests.

## 1. Scope, authority and baseline

The supplied [Word import specification](../quizzivy-word-import-spec.md), version
1.0, defines the target. Its copy is unchanged, including its older reviewed
baseline. This plan reconciles that target with released **v0.5.0**, main
`954fbec83b12a77d845174e3a55e47dd426f41b1`, and develop
`1e9e7a2710b9c610c2ac53c7ae1cbad87fc965d7`. Their application trees include the
recent authoring fixes, safe version lifecycle and refreshed UI.

The [current product spec](../quizzivy-spec-v0.3.md), [architecture](60-backend-architecture.md)
and `api/openapi.yaml` still describe implemented behavior. Proposals below do
not silently supersede them. At each accepted contract gate, update the relevant
product sections/version and plan, then OpenAPI, then run `make gen`, before
implementing either side. No new dependency or migration is selected by this
planning change.

Delivery means: real Word file → durable review → one coherent draft → manual
authoring → immutable publication → assignment → student attempt → grading.
Both free-form `.docx` and legacy `.doc` are required for general availability.
The six stages sequence implementation; they do not reduce the release scope.
OCR/scans, PDF input, solving missing answers, new interactions such as matching,
automatic publication and automatic assignment remain outside this milestone.
Internally rendered PDF/page images are comparison artifacts, not PDF import.

On 2026-09-22 Thuong requested consideration of **both cloud API and privately
hosted AI**. No vendor, model, external data transfer or infrastructure purchase
has been approved. Compare both using the same evaluation protocol (§8).

### 1.1 Implementation checkpoint — source evidence

W-01 now has an authorized local corpus supplied by Thuong: eight DOCX files
(five related exams, two companion-key documents and one annotated homework),
49 two-page exam PDFs and one reference/advertising PDF. The five new exams have
31 top-level labels and 35 answer targets each, including a five-child cloze
group. The matching key document contains all 175 target labels. This is a
structural correspondence check, not independent verification of answer truth.
The PDF family has keys for 50 tests but only 49 exam files; the missing exam is
excluded from coverage calculations. Private sources, extracted text, answer
data, hashes and detailed local reports are not committed.

PDFs are authorized **layout/semantic evaluation references**, not Word parser
fixtures or a change to the supported upload formats. A conversion from PDF
cannot validate original OOXML style/numbering behavior. The corpus is clustered
in a few English-exam families; it does not yet establish broad format coverage,
legacy `.doc` compatibility, listening coverage or a teacher-effort baseline.
All files are inventory data until expected semantic outputs are reviewed. Any
tuning/holdout split must account for related templates and shared key files.

The W-03 source spike is `server/internal/platform/word`, with a local-only
`cmd/word-inspect` command. It retains paragraph/run locations, explicit marks,
table properties, relationships and unresolved object evidence; bounds ZIP/XML
and locator growth; and does not infer questions or answers. See its README for
limits and reproducible checks. It uses no additional dependency and is not
wired into the API or student application. A separate bounded resolver now
derives supported inherited semantic marks and automatic list labels, retaining
property provenance and explicit findings for unsupported or ambiguous cases.
It handles main-body list instances, nested levels, starts/overrides/restarts,
style associations and common number formats without rewriting source text.
Conditional table styles, ambiguous toggles, non-Latin marks, numbering style
links, custom/symbol numbering and revision-dependent counters remain unresolved.
Full style/numbering fidelity, asset decoding, semantic recognition and the W-12
coverage gate remain open; the package README defines the supported subset.

All eight supplied DOCX files pass inventory with unresolved findings retained.
One sequential local measurement with two Go scheduler threads took 12–130 ms
per file and 13–133 MB peak process RSS, including optional JSON output; these
are source-inventory measurements, not end-to-end import SLOs. A synthetic
50-question inventory benchmark took about 0.6 ms/iteration and 0.53 MB allocated
per iteration on the development machine. A local LibreOffice rendition of one
exam completed with a sampled process-group peak near 204 MiB. Production
converter isolation and capacity are still unproven; this local render is not
the W-13 security gate.

Corpus-driven cases to carry into semantic validation and review:

- Instructions refer to underlines missing from the five new DOCX documents
  and their style definitions. A rendered sample confirms the missing marks.
  Missing evidence must ask for a teacher correction, never a guessed underline.
- Compact keys may concatenate question labels without spaces; decimal labels
  such as `23.1` are identities, not decimal numbers. Some child keys omit the
  `Question` prefix. Bind within the selected test, not across the whole key file.
- Annotated homework interleaves original questions, learner responses and
  teacher corrections. These must not be treated as interchangeable answer keys.
- PDF references include semantic underlines, shared passages, page continuations,
  option-label references, and three-way true/false/not-given exercises. Preserve
  meaning or surface an explicit unsupported/mapping decision; never reduce
  three choices to a binary true/false interaction.

W-01–04 are still in progress: the complete editor evaluation, remaining domain/API
contracts, reviewed expected outputs, converter isolation, cloud/private model
evaluation and teacher walkthrough are not satisfied by this extraction spike.

### 1.2 Implementation checkpoint — content editor experiment

W-03 now includes a candidate application-owned semantic AST, bounded strict
validation, plain-text projection, a read-only renderer and a Tiptap adapter.
The synthetic [editor harness](../../web/tests/support/content-editor/README.md)
exercises editing, undo, document switching, stable gaps, merged tables, rejected
rich paste and a 320 px layout without changing production routes or storage.
Its browser checks run in CI; a real-build test separates editor code from the
reader and enforces transfer budgets. Changes exist only in the harness session.

Tiptap 3.31.3 and Lexical 0.51.0 both preserved the tested model semantics on the
same synthetic fixtures. Tiptap is the current interactive candidate, not a final
selection based on teacher or native-IME acceptance. The comparison procedure,
dependency rationale and remaining limitations are documented with the harness.
This does not complete W-05 or authorize storing editor JSON: the domain/API
contracts, server validation, real asset bindings and integration with immutable
versions must precede production use. No teacher document was sent to a service.

### 1.3 Implementation checkpoint — content contract foundation

W-02 now has additive OpenAPI components, generated Go/TypeScript types and an
editor-independent Go content value object. Shared synthetic conformance cases
pin structure, Unicode, projection, table/gap rules and unsafe-data rejection
against the frontend validator and JSON Schema. The Go parser also bounds raw
JSON and rejects duplicate keys and malformed encodings before validation.
[The content contract](19-content-contract.md) names ownership, recovery,
integration and rollback requirements. That checkpoint added no question write,
schema migration, asset delivery or durable browser storage; §1.4 now connects
the bounded inline option subset. D-02 and the recovery
policy within D-04 are approved. Thuong also approved D-03 group ordering and
shared recording allowances. Final editor acceptance, the complete graph/revision
API and implementation of these approved policies remain open.

### 1.4 Implementation checkpoint — inline option slice (W-05a)

Option formatting now connects bank/builder to normalized storage and immutable
snapshot/restore, previews, attempts/results and teacher review. The bounded
profile is one paragraph of marked text and breaks; it preserves pronunciation
underlines without exposing answer metadata or enabling unbound media/gaps.
`text` remains an exact literal/plain projection. Migration 00032 is additive;
no old snapshot is rewritten. See `19-content-contract.md` for legacy writes and
rollback constraints. The pilot formatting affordance is opt-in with
`VITE_RICH_OPTION_EDITOR`; existing rich readers/editors remain available.

This is a reviewable W-05/W-06 sub-slice, not completion of either package.
Question prose is integrated in §1.5; structured paste, grouped materials, shared
playback, revision recovery and the import journey remain later work packages.

### 1.5 Implementation checkpoint — question prose (W-05b)

Optional semantic prompts and explanations now use bounded paragraphs, headings,
lists and tables with marks, breaks and safe links. Migration 00033 preserves
these documents through bank duplication, publication, restoration, previews,
attempts, results and teacher review. Existing Markdown stays unchanged. Prose
cannot bind assets or gaps; fill-blank prompts retain their historical path.
The new editor affordance is opt-in with `VITE_RICH_QUESTION_EDITOR`; existing
rich content remains editable when the flag is off.

Explicit Markdown conversion previews the supported subset and refuses unknown
structures without modifying the original. Removing rich structure is an explicit
teacher action with a warning; the literal projection is escaped before it goes
back through the Markdown reader. Editor code remains lazy and absent from the
learner dependency graph. Explanation delivery follows the existing result
policy in SQL, while active papers and previews never select either explanation
column. Legacy writes cannot silently discard these documents.

This remains a partial W-05/W-06 slice. Structured clipboard conversion, stable
blank bindings, shared material graphs, revision recovery and final editor/pilot
acceptance are not implied by these integrations. The two new direct dependencies,
`unified` and `remark-parse`, were already transitive dependencies of the Markdown
reader; declaring them directly lets the conversion reuse the same parser instead
of approximating Markdown with regular expressions.

### 1.6 Implementation checkpoint — shared question validation (W-06a)

Bank/manual/internal writes and publication now share `questions.Input.Validate`.
Publication adapts resolved draft questions to that input and keeps violations
anchored to the affected question/section. This closes true/false key cardinality,
empty option/blank answers, unsupported types, blank correspondence and invalid
content gaps between the write and snapshot paths. Question points use exact
hundredths without silent rounding; publication rejects empty exams and total
overflow before creating a snapshot. Frontend form checks mirror the interaction
rules. Historical published versions are not rewritten.

This is the validation portion of W-06. Rich blank bindings, structured paste,
asset authoring and group/revision integration remain separate work.

### 1.7 Implementation checkpoint — stable blank bindings (W-06b)

Rich fill-blank prompts now bind gaps to answer metadata independently of node
order or labels, including table cells and nested lists. Migration 00034 preserves
these bindings through bank writes, duplication, immutable publication, restored
drafts, learner attempts, results and teacher review. Copies remap both ends of
the relation. Answer submissions keep their frozen blank UUIDs and existing grading.

The opt-in authoring flow previews legacy conversion, preserves orphaned answers
for undo, requires explicit discard and blocks incomplete bindings before save.
Legacy Markdown is unchanged. This advances W-06; structured paste, asset bindings,
groups, revision recovery, import processing/review and teacher acceptance remain.

### 1.8 Implementation checkpoint — structured clipboard (W-06c)

Rich authoring now previews supported clipboard prose before changing a field.
The local converter preserves inline semantic marks, list starts, safe links and
table spans, using the existing application-owned content contract. It rejects
unsupported/ambiguous source structure as a whole and never derives keys from
underlines or other marks. Input and resulting documents have bounded budgets.
Captured selection, unchanged-document checks and transaction history protect
cancel, concurrent edits and undo. The parser loads only when formatted paste is
requested, outside the learner reader and the initial editor dependency closure.

The pinned `parse5` dependency parses inert HTML without DOM mounting, scripts or
resource fetching. Files, externally dropped HTML, unbound assets/gaps, stylesheet
rules, tracked changes and Word-specific list conventions remain explicit refusals.
This is a bounded clipboard subset, not a claim of arbitrary Word conversion.
Group/media authoring, revision-safe persistence, local recovery and native IME/
teacher acceptance remain subsequent gates.

### 1.9 Implementation checkpoint — group graph contract (W-07a)

The additive `QuestionGroup` contract now distinguishes actual shared context
from existing sections. Tests-domain validation resolves the complete member
catalog, validates interactions, binds material gaps to choice questions or
stable rich-blank identities, and checks recording coverage and option-order
constraints. Limits cover the complete resolved context, not just one material.

Detached copy remaps every editable identity and both ends of each gap binding,
retains immutable media references and exact grading/text values, and rejects
reused or invalid generated identities without mutating the source. Tests cover
independent copies, repeated local gap names, changed catalog order, repeated
audio nodes, separate member audio and mixed-scope rejection. The contract is
structurally checked against generated types and shared content schemas.

This is a domain foundation, not enabled group authoring. W-07 still requires
relational ownership/bindings, lifecycle locks, context-aware bank operations
and the builder. W-08/09 must supply immutable graph snapshots and versioned
delivery before group writes become available. No learner payload or existing
test/section semantics changes in this checkpoint.

### 1.10 Implementation checkpoint — relational group foundation (W-07b)

Migrations 35–37 add independent bank/section group ownership, nullable ownership
on existing questions, ordered section units, material/gap bindings and explicit
recording scopes. Composite foreign keys prevent cross-group response and audio
links. Stable blank targets survive transactional answer-row replacement; member
ordinals support deferred reordering. Required reverse-reference indexes protect
lifecycle query paths. Existing populated question indexes are built concurrently.

PostgreSQL integration checks cover ownership, kind/scope restrictions, guarded
rollback and both full and intermediate migration round trips. New group rows
block destructive schema rollback; no old content is reinterpreted or backfilled.
This is storage groundwork only: repository lifecycle commands, legacy-writer
barriers, bank/builder integration and W-08/09 snapshots/readers remain required
before any group endpoint or write affordance is enabled.

### 1.11 Implementation checkpoint — atomic group storage (W-07c)

The internal group repository creates and reloads complete independent graphs in
one transaction, including owned interactions and audit entries. Section-owned
creation checks the enclosing draft revision and preserves standalone order when
introducing ordered units. Invalid late media bindings roll back the entire graph.
Bulk member reads validate relational media mirrors against the semantic content.

Existing question get, update, delete, duplicate, tagging, outline references,
listing and facet paths hide or reject owned children. Active and archived groups
protect shared and member media; creation and deletion contend on the same asset
locks. Admin media-delete conflicts can name both published tests and groups.
Database tests cover independent copies after source deletion, rollback, stale
outline writes and both orders of the media create/delete race.

This repository is not connected to a group HTTP endpoint. Revision-safe group
editing, bank/lifecycle commands, media usage UI, full draft readers and W-08/09
snapshot/delivery support are still required before enabling group authoring.
No new migration or dependency is introduced beyond W-07b migrations 35–37.

### 1.12 Implementation checkpoint — revision-safe group lifecycle (W-07d)

Internal full-group edits now check the aggregate revision and enclosing test
revision before changing any graph rows. The lock order is parent test, group,
owned members by ID, then incoming media by ID. Two writers of the same revision
produce one successful edit; the stale writer changes neither content nor audit.
Member reordering and question-type/option-policy changes are atomic, including
replacement of stable blank targets and material bindings.

Bank archive, restore and permanent deletion are revision checked; deletion
requires archival and preserves audit history. Test-owned groups cannot use the
bank lifecycle. Explicit section removal deletes the complete owned graph and
compacts ordered units. Copy reads a coherent source revision and materializes
fresh editable identities under the destination's revision and media locks.
Source edits, archival and deletion cannot alter these independent copies.

Docker checks cover these operations, late-failure rollback, parent archival,
concurrent edits and released media references. No endpoint is exposed yet:
full draft readers, bank/builder UI and W-08/09 snapshots and delivery remain
required. This checkpoint adds no migration, dependency or learner payload.

## 2. Current code and the actual gaps

| Area | Verified current behavior | Required work |
| --- | --- | --- |
| Questions | `questions/domain/question.go` stores legacy Markdown plus additive semantic prose/options and five supported types | Versioned semantic content for prompts, options, explanations and materials; keep historical text readers |
| Validation | Shared question validation covers manual/internal writes and publication, including true/false cardinality and exact points | Extend the same path to rich gap/group/asset graph invariants and import findings |
| Draft outline | `tests/domain/test.go` has sections with question IDs; the UI currently calls these groups | Separate section and actual shared-context group without reinterpreting old sections |
| Shared materials | Instructions are a small text note in `SectionInstructions.tsx` | First-class rich passage/table/image/audio material and group editing/delivery |
| Rendering | `Markdown.tsx` sanitizes Markdown; `QuestionBody.tsx` and `StudentPreview.tsx` render options as text | One allowlisted content renderer used by builder, preview, engine, review and results |
| Randomization | `attempts/domain/deal.go` shuffles individual questions inside sections and all eligible options | Versioned group-aware deal policy, fixed member order where required and explicit option-reference constraints |
| Audio | Counters and audio-play requests are per version question | Shared playback scope that survives group navigation, reload and takeover, with existing optimistic-play semantics |
| Lifecycle | Duplicate/archive/permanent-delete, version restoration and current-version selection exist | Extend all copying, reference queries, deletion locks and restoration to the complete new graph |
| Transactions | `platform/db.Context` can bind a pool or transaction; publish/autosave use transactions | A transaction-aware cross-module import materialization port, not sequential HTTP calls |
| Autosave | `useAutosave.ts` serializes/flushes pending writes and exposes stale/save failure | Reuse behavior with revision control, route-leave blocking and durable import edits; unmount alone cannot guarantee browser-crash recovery |
| Jobs | `core/jobs/prune.go` schedules token cleanup in memory | Persistent import queue, fenced worker claims, checkpoints, cancellation and recovery |
| Infrastructure | Go API, PostgreSQL 18, private object storage; no Word converter or model runtime in package manifests | Isolated bounded document worker and optional model adapter; no heavy jobs inside API requests |

This is a domain/platform milestone with an import workflow, not an upload
button added to the current question editor.

## 3. UX contract and evidence of success

Detailed screens, interaction states, responsive behavior and acceptance scripts
are in [18-word-import-ux.md](18-word-import-ux.md). The primary journey is:

1. **Choose files:** exam, optional answer key, optional listening assets. Automatic
   recognition is the default; templates/profiles are optional conveniences.
2. **Process:** real stages and persistent history; the teacher may leave safely.
3. **Review:** linked source/result views, issue-first navigation, inline fixes,
   explicit missing answers and visible save state.
4. **Confirm:** learner preview and a current validated summary; create one draft.
5. **Continue authoring:** normal builder, with publication remaining separate.

The UI must answer: What was found? What needs my decision? Where is the source?
Has my edit been saved? What happens next? Avoid confidence percentages and a
large technical processing dashboard in the normal teacher flow.

Measure total teacher effort, including upload, processing wait, correction,
preview and builder corrections. A visually attractive importer that transfers
cleanup work to the builder does not pass. Compare matched document families
with the same teacher's manual authoring baseline, controlling for order effects.

## 4. Proposed architecture and content contracts

### 4.1 Boundaries

Add `modules/imports/{domain,application,repositories,http}` and
`web/src/features/imports/`. Imports owns sources, jobs, candidates, review,
evidence, issue resolution and commit identity. It does not own grading or write
other modules' tables directly. Tests owns assessment structure and snapshots;
questions owns supported interactions/keys; media owns publishable assets;
attempts owns delivery, playback counters and grading.

Keep document parsing/conversion and provider clients behind ports. Wire adapters
in `core/adapters` and modules in `core/wiring`; extend architecture tests. A
dedicated worker command can use the same Go module/deployment repository while
running separately from the API. Do not introduce a broker or microservice
framework unless queue/capacity measurements show a need.

The converter child process has no network, database or object-store credentials.
The worker stages input/output through a controlled temporary directory and
handles storage itself. The provider client is a distinct adapter with bounded
network access. A private GPU inference endpoint is a possible deployment choice,
not a requirement to run a model on the developer laptop.

### 4.2 Versioned content

The W-02 foundation defines an application-owned `ContentDocument` discriminated union
([contract details](19-content-contract.md)):

- `legacy_markdown_v1`: preserve the original string and existing rendering.
- `semantic_v1`: typed blocks/runs, paragraphs, ordered/bullet lists, safe links,
  tables with header/read-order semantics, image/audio references, marks and
  stable gap nodes. Include bold, italic, underline, superscript and subscript.

Define node count, nesting, text/URL/table/image bounds in the contract. Unknown
node/mark/attribute types produce a finding or a rejected write; they are not
silently dropped. Raw Word HTML never becomes trusted content. Reject unsafe
URL schemes and external-resource loads. Teacher answer annotations/evidence
belong outside learner content, not in hidden CSS or data attributes.

Persist the small content AST as validated JSONB where appropriate, while keeping
question options, keys, blank answers, memberships and media references relational.
This does not overturn the existing decision to normalize assessment snapshots.
Mirror asset references into relational bindings so deletion checks and FK locks
do not depend on scanning arbitrary JSON. Maintain a deterministic plain-text
projection for Vietnamese search; never index serialized markup as question text.

Evaluate a Tiptap/ProseMirror editor adapter against a second candidate before
selection. The stored schema remains Quizzivy's, not a plugin's undocumented
internal shape. Rich editor code stays admin-only; students load a small read-only
renderer. Pin marks/tables/gaps, paste/undo/IME behavior, keyboard accessibility,
Vietnamese input, license and bundle budgets in the editor spike.

### 4.3 Section, group, stimulus and bank reuse

Approved semantics with W-07a graph validation/copy foundations:

- A **section** organizes the exam; old sections remain sections.
- A **group** has ordered members, shared instructions and one or more stimulus
  bindings. Standalone questions remain possible without synthetic visible groups.
- A **stimulus** is a passage, table, image, audio or composite content. Stable IDs
  link cloze gaps and question/blank targets, independently of printed numbering.
- Approved by Thuong: test-owned editable groups/materials. Copying into another
  test or the bank creates an independent graph with all required members and
  materials. Source edits/deletion cannot change the copy. Immutable asset bytes
  may be reused through separately protected relational bindings.
- Context-dependent bank questions must expose their required group/materials.
  The picker offers **copy with context**, copying all required members and
  remapping gaps/assets; a lone child cannot be inserted without its dependencies.
  Copies do not mutate the source bank item or source test. The exact dependency
  relations and behavior after source-test deletion are part of this contract
  gate, not a deferred cleanup issue.
- Snapshot all materials, group memberships, ordering rules, semantic content and
  audio policies. The source import is never needed to render a published exam.

`dealVersion`/equivalent distinguishes legacy and group-aware delivery. Old
attempts retain the existing algorithm, seeds, question IDs and answer binding.
Approved D-03: when question shuffling is enabled, shuffle groups and standalone
questions as units within a section; always preserve authored member order inside
shared-context groups. Keep authored unit order when shuffling is disabled. Mark questions with semantic option-label references as non-shufflable
unless references are represented structurally. Reject incompatible assignment
configurations with a named group/question, rather than silently weakening them.

Approved D-03: a shared recording's play allowance belongs to an explicit
versioned group-audio binding, not merely the underlying asset ID. All children
share that allowance; different groups may use the same file independently.
Existing per-question behavior remains readable; shared scopes do not grant new
plays on child navigation, reload or takeover of the same attempt. Preserve synchronous
user-gesture `.play()`, optimistic accounting and post-limit reporting. Do not
silently change audio policy into hard network authorization before playback.

### 4.4 Import persistence and identity

Final names/DDL come after §4.3, following PostgreSQL 18 conventions and the Neon
skill. Proposed responsibilities:

| Relations | Integrity and access expectations |
| --- | --- |
| Imports / source sets | UUIDv7 identity, creator, current source/draft revision, status, resulting test; single-organization admin scope with checks on every resource |
| Sources / artifacts | Role exam/key, checksum, detected MIME, bytes, immutable private key, lineage, source revision; originals and answer files never become student media |
| Runs / stage artifacts | Fencing token, lease, attempt count, source/component versions, bounded cost/time, cancellation intent and outcome |
| Blocks / source references | Stable IDs per source revision, part/paragraph/table-cell/run offsets, marks and coverage; rendered page coordinates only when actually available |
| Candidates / review revisions | Immutable schema-versioned candidate; bounded reviewed graph with revision and digest; known/unknown answer is a union, not all-false flags |
| Issues / decisions | Entity/field target, reason, evidence, severity, resolver, affected revision/digest; invalidate relevant decisions after substantive edits |
| Asset bindings | Validated asset ID plus explicit learner placement and dependency; independently guarded access for source artifacts |
| Commit records | One resulting test per import, approved revision/digest, idempotency key and outcome stored with the created graph |
| Profile revisions | Declarative numbering/answer conventions, creator/scope and version; no executable regex/code supplied unchecked by users |

Use `timestamptz`, exact numeric points and indexed foreign keys. Plan indexes
around import history filters, runnable jobs, expired leases and source blocks.
Do not partition prematurely. Enforce graph ownership with relational constraints
where possible and full domain validation where not. Audit/events remain
append-only; no new UPDATE/DELETE privileges on those tables.

Source coordinates do not survive a replacement by assumption: create a new
revision and an explicit mapping/diff. Coverage is many-to-many when reuse is
intentional; every meaningful block needs an assignment, metadata role or reasoned
exclusion. Duplicate text is not sufficient evidence of duplicate questions.

### 4.5 Durable work, saving and committing

Lifecycle: `awaiting_sources → queued → processing → needs_review → committing
→ committed`, with failed/cancelled branches. Store stages separately. A technically
successful run always enters review. Cancelled imports remain inspectable until
retention/deletion; retryable failures preserve completed artifacts and edits.

Queue claims use bounded PostgreSQL transactions, lease expiry and a monotonically
changing fencing token. Late worker writes must match both source revision and
claim token. Cancellation and commit contend on the import state, so only one can
win. Do not hold a database transaction during parsing, conversion or provider calls.

Review saves use a single-flight queue plus expected revision. A `409` never
auto-overwrites another tab's changes. A draft-level coordinator flushes before
changing questions/routes, reprocessing, preview and commit. Saved means server
acknowledged. For crashes with unsent edits, design a bounded per-user IndexedDB
outbox with visible recovery/conflict handling. Thuong approved account-isolated
local recovery with a maximum seven-day lifetime from unsent revision creation
and clearing on logout; source/answer files are excluded. Enforce expiry before
any read/replay, surface quota/storage failures, and never report local-only
changes as server-saved. Implementation and recovery tests remain pending.
Browser unload requests alone are not durability. Server-acknowledged edits must survive all browser failures.

Prepare immutable validated media before commit. Commit runs complete domain
validation against the approved revision/digest, then creates bank content, test,
groups/materials, bindings, audit entries and commit record in **one transaction**.
Use transaction-bound application adapters; existing handlers that start isolated
transactions are not sufficient. Take reference locks in one documented order on
all participating insert/delete paths. A unique import-result constraint and
idempotency key make double click, concurrent commit and lost response safe.

Same import + same commit identity returns the existing test; a mismatched digest
conflicts. Intentional “create another exam” creates a new import identity.
Reprocessing produces a separate candidate and a three-way comparison against the
previous machine candidate and current manual draft. Teacher edits win by default;
conflicts require per-field decisions and new validation.

## 5. API and screen inventory

Use the operations in source spec §10 as the minimum, not the final OpenAPI.
All import operations live under `/admin/`; every nested artifact/run route must
derive scope from its parent and check it. A UUID is not a capability token.

Resolve these contract gaps in W-02 before implementation:

| Addition/detail | Reason |
| --- | --- |
| Explicit source upload/finalization, byte/content validation and orphan handling | Create-record success must not imply a complete file upload; retries must not duplicate source sets |
| List/get processing runs and cancellation result | History and recovery must be observable rather than inferred from one status |
| Paginated block views and rendition manifest | Large sources must not be downloaded/rendered as a single giant DOM |
| Candidate comparison and revision-guarded apply | Reprocess must not overwrite manual work |
| Revision-scoped issue decisions and explicit exclusions | Bulk confirmation must name the visible set, decision and target digest |
| Save/list/select/delete declarative profile revisions | Saved profiles in the required journey need real persistence and scope |
| Artifact download authorization, expiry and asset binding operations | Keys/source pages are private even when the teacher previews learner content |
| New group/stimulus, snapshot, delivery and shared-play contracts | The existing tests/questions APIs do not describe the required assessment graph |

Pin request/response size limits, `202` job identities, `409` stale/state conflicts,
`422` content findings, `413` limits, `415` unsupported containers, quota responses,
idempotency behavior and localized errors. Define ETag/expected revision on every
mutation; retain both tokenized source IDs and human labels. Validation tokens
bind revision, digest, validator version and referenced assets; commit rechecks.

History filters, pagination, selected issue/question/source view and panel widths
survive child navigation. Use existing saved-filter and side-column conventions.
No streaming transport required initially: adaptive polling while visible is enough.

## 6. Six stages and reviewable work packages

Each W-ID is a bounded PR into develop, with code + contract + focused tests and
relevant document updates. Split further if a package cannot be reviewed coherently.
Use `work/word-import-<id>-<slug>` branches. Feature flags keep incomplete paths
hidden; a merged foundation does not claim a usable import release.

| Stage / ID | Deliverable and main surfaces | Depends on | Exit evidence |
| --- | --- | --- | --- |
| 1 / W-01 | Authorized corpus inventory, expected outputs, document families, holdout split, manual effort baseline | Teacher sample access | Rights/access recorded; missing corpus families visible; synthetic fixtures distinguished from teacher data |
| 1 / W-02 | Content/group/reuse/audio/revision contracts, threat model, API design and document updates | This plan; representative fixtures | D-01–D-04 below resolved; migration/deletion/rollback paths reviewed; AC matrix has no unowned requirement |
| 1 / W-03 | Editor/converter/extractor spikes and cloud/private recognition benchmark harness | W-01; approved data policy | Reproducible fidelity/resource results; dependency choices justified; no real exam leaves approved scope |
| 1 / W-04 | Interactive review prototype and teacher usability walkthrough | UX plan; representative scenarios | Teacher can find source, resolve ambiguity and recover edits without coaching; layout decisions recorded |
| 2 / W-05 | Versioned content schemas, safe renderer, legacy adapters and search projection | W-02, W-03 | Old/new fixtures render; option underlines survive; unsafe nodes rejected; student entry chunk stays small |
| 2 / W-06 | Rich editor in bank/builder, content validation shared by direct/internal writes | W-05 | Full editing/paste/undo and five types; true/false and publish cardinality regressions pass |
| 2 / W-07 | Draft groups/materials, context-aware bank insertion, copying/reference lifecycle | W-02, W-06 | Empty groups, reorder, cloze remap, dependency-aware copy and deletion races pass |
| 2 / W-08 | Snapshot/restore/duplicate/preview for full graph | W-07 | Editing/deleting sources cannot change published content; old versions still preview/restore |
| 2 / W-09 | Group-aware attempts, assignment validation, shared audio scope, grading/result rendering | W-08 | Manually authored passage/cloze/listening test completes; legacy deal remains identical; leak/audio canaries pass |
| 3 / W-10 | Imports module, private source uploads, ownership/quota/state contracts | W-02 | Type mismatch, invalid roles, unauthorized downloads and upload retries covered |
| 3 / W-11 | Durable queue/worker, leases, fencing, cancellation, metrics | W-10 | Kill/restart worker and reject late result; no source/edit loss; API remains responsive |
| 3 / W-12 | OOXML inventory/extraction with locators, numbering/styles/tables/assets | W-03, W-11 | Golden coverage corpus including repeated numbering, nested tables, marks and unsupported objects |
| 3 / W-13 | Isolated `.doc` normalization and source rendition | W-03, W-12 | Originals retained; converted lineage; losses visible; blocked network, timeout and memory limits verified |
| 4 / W-14 | Deterministic recognizer, declarative profiles, explicit answer reconciliation | W-12, W-13 | Restarts and companion keys correctly map; unknown/conflicting keys remain findings |
| 4 / W-15 | Assisted recognizer adapters, budgets, kill switch and versioned evaluation | W-03, W-14 | Cloud and private paths compared; schema/provenance validated independently; outage is recoverable |
| 4 / W-16 | Coverage/fidelity/domain validator and invalidation rules | W-05–W-09, W-14, W-15 | Every meaningful block accounted for; unsupported interaction cannot pass by relabeling |
| 5 / W-17 | Import history, upload/setup and durable progress UI | W-04, W-10, W-11 | Leave/return, retry/cancel and URL filter state work in Vietnamese/English |
| 5 / W-18 | Linked review, field editing, answers, split/merge/move, issues and autosave/conflicts | W-06, W-16, W-17 | Source-linked correction; keyboard-only workflow; save failure and browser recovery tested |
| 5 / W-19 | Source replacement, candidate comparison, selective apply, revalidation | W-18 | Manual edits protected; new source coordinates reconciled; equivalent confirmations only |
| 5 / W-20 | Learner-safe preview, final summary, atomic idempotent commit and builder handoff | W-08, W-16, W-18 | Failure after each persistence step leaves no partial test; duplicate/lost-response commit returns one result |
| 6 / W-21 | Capacity/security/a11y evaluation, retention/cleanup and operator runbooks | W-09–W-20 | Limits/SLOs approved and exercised; restore/drain/kill-switch/rollback drills pass |
| 6 / W-22 | Teacher pilot, correction-time evaluation, fixes and versioned release | W-21 | All AC-01–17 pass, critical escaped defects cleared, teacher acceptance and monitored rollout |

Critical path: contracts → content → group/snapshot/delivery parity → full
validation → review/commit → pilot. Extraction/worker work can proceed separately
after contracts stabilize, but no simultaneous heavy local processes are assumed.
Do not estimate a release date from task count: W-01–04 must establish document
complexity, converter/runtime fit and domain scope. At that gate provide remaining
effort ranges, uncertainties and the next checkpoint; re-estimate after W-09 and
the recognizer holdout run. No staffing or calendar commitment is implied here.

## 7. Extraction policies to make explicit

| Source class | Proposed treatment |
| --- | --- |
| Paragraphs/runs/automatic numbering | Resolve inherited style/numbering; retain original labels and stable structural positions |
| Option tables, same-line options, columns | Preserve cell/column order and geometry evidence; ambiguity blocks mapping until a teacher chooses |
| Headers/footers | Inventory and suggest metadata/exclusion only with reason; do not discard possible instructions |
| Footnotes/endnotes | Preserve referenced content and source links; unresolved semantic attachment is review-required |
| Text boxes/floating shapes | Inventory text/anchor/order; show actual rendition when order is layout-dependent; semantic loss blocks commit |
| Comments/hidden text | Keep in private source evidence; never automatically render to learners; teacher classifies relevance |
| Tracked changes | Show revision ambiguity and require accepted/current vs original interpretation; keep both evidence branches |
| Underline/bold/color | Preserve semantic marks. Treat them as keys only through an explicit confirmed convention; remove key-only marks from learner content |
| Embedded images | Validate/decode bounds, promote usable assets, request alt text when necessary; unsupported vector/OLE objects become findings |
| Active content/macros/external relationships | Reject unsafe source classes or isolate unsupported objects without executing/fetching; no silent successful empty import |
| Scans/encryption/corruption | Actionable unsupported/invalid state; no OCR or lossy fallback disguised as success |

## 8. Cloud API versus private inference

Both approaches use **the same local extraction, source inventory, validation,
review and transactional commit**. AI receives a bounded evidence bundle and
returns untrusted candidates; it never has application-write tools. Deterministic
recognition runs in either deployment. No automatic cloud fallback from private
processing without a separately approved data policy.

| Criterion | Cloud API path | Private inference path | Evidence required |
| --- | --- | --- | --- |
| Quality | Candidate provider/model chosen from benchmark | Candidate model/runtime/quantization chosen from benchmark | Same holdout documents and schema; correct answers/coverage/grouping per family, not model confidence |
| Data handling | Explicit allowed payload, region, retention/training controls, contractual terms | Private network, controlled model/log/storage access; infrastructure provider still matters | Document flow map and approved processing policy; no automatic claim of complete privacy |
| Capacity | Provider limits, throughput, outages and variable token usage | GPU/CPU RAM/VRAM, context/KV cache, throughput, cold starts and OOM behavior | Measured concurrent workloads, queue wait and recovery; no laptop RAM guess used for sizing |
| Cost | Input/output usage, retries, extraction/storage and operational time | Runtime idle/active cost, storage, hardware/hosting and operational time | Cost per successfully reviewed exam and monthly scenarios from actual usage/quotes |
| Operations | Credentials, rate limits, request budgets and provider degradation | Deployment, model weights/license, upgrades, monitoring and availability | Named owner, kill switch, rollback and maintenance estimate |
| Portability | Provider-specific strict schema may differ | Runtime/model strict schema may differ | Canonical candidate validator and adapter conformance tests |

Benchmark sequence: (1) deterministic baseline, (2) small tuning corpus per
candidate, (3) frozen settings on holdout, (4) repeated difficult families to expose
variance, (5) correction-time pilot, (6) maximum-input and outage tests. Record
component versions, tokens/cost, latency, peak RAM/VRAM, omissions, key association,
format/group fidelity, unsupported detection and escaped defects. Define corpus
family weights before scoring. Mandatory invariants veto a candidate regardless
of an attractive mean accuracy or price.

Recommendation at this point: keep both behind a narrow recognizer port and
delay the model/vendor decision until these measurements. Prefer the path that
meets the quality/privacy gates with the lowest **total teacher and operating
effort**, rather than token price alone. Do not build a generic plugin system or
multiple production integrations before this gate.

## 9. Quality gates, capacity and release

### 9.1 Traceability to the supplied acceptance criteria

| Criteria | Responsible packages | Evidence at release |
| --- | --- | --- |
| AC-01, 03 | W-12–15, W-22 | Real `.doc`/`.docx` and companion-key golden + upload-to-attempt E2E |
| AC-02 | W-06, W-14–16, W-18 | Unknown/conflicting/ambiguous keys; no generated key accepted without source/teacher evidence |
| AC-04 | W-05, W-06, W-08, W-09 | Pronunciation/options/tables survive edit → snapshot → student render |
| AC-05 | W-07–09 | Stable cloze links, grouped shuffle and shared playback; legacy replay fixture |
| AC-06 | W-12, W-16, W-18 | Complete coverage ledger; intentional reuse and exclusions reflected in totals |
| AC-07 | W-08–10, W-20 | Authorized asset delivery and recursive payload/content leakage checks |
| AC-08 | W-18, W-19 | Reload/save failure/two-tab conflict/reprocess/manual-merge scenarios |
| AC-09, 10 | W-20 | Duplicate/concurrent/lost-response commit and injected transactional failure tests |
| AC-11 | W-11, W-13, W-15, W-21 | Worker kill, stale fencing, cancellation races, provider outage and deployment drain |
| AC-12, 14 | W-10–13, W-21 | Authorization matrix, malicious archive/XML/markup/relationship/active-object fixtures |
| AC-13 | W-05, W-08, W-09, W-21 | Historical snapshot/attempt corpus, version restore/delete and rollback compatibility |
| AC-15 | W-04, W-17–21 | Keyboard/screen reader/zoom, 768px admin, 320px learner, Vietnamese/English |
| AC-16 | W-03, W-21 | Approved supported envelope, retention, numeric SLO/quality targets and measured results |
| AC-17 | W-01, W-22 | Teacher baseline/pilot, correction effort and resolved critical escaped defects |

Unit tests target parsers/validators/identity and decision invalidation. Integration
tests exercise real PostgreSQL reference locks, transaction atomicity and job
recovery. Browser tests exercise user actions/states, including editable tables,
IME, autosave and source navigation. Live E2E must include real file upload through
published learner rendering/grading; stubbed browser tests alone cannot prove it.
Keep all existing snapshot, audio-gesture, event-session, refresh-single-flight and
router-chunk canaries. New tests must validate outcome, not mirror implementation.

### 9.2 Proposed measurement targets, not current capabilities

Initial benchmark envelope candidates: 25 MiB compressed exam/key each, 200 MiB
expanded package each, 100 rendered pages, 200 questions, 100 embedded images.
These are starting hypotheses; image pixels, XML depth, archive entries, nested
object count, conversion time/memory and review payload limits must also be set.
Reject over-limit inputs before consuming unbounded resources and offer a clear
split/reduce-file instruction. No arbitrary silent truncation is allowed.

Proposed SLO hypotheses for a 50-question editable-text exam under the approved
load: autosave acknowledgment p95 ≤1s excluding the 1.5s debounce; commit p95 ≤3s;
processing p95 ≤120s excluding queue wait; expired-worker recovery ≤2min. Measure
queue wait separately, plus maximum-input latency and time to interactive review.
Tune/approve numbers using W-03/W-21; do not present them as a production promise.

Proposed pilot outcome: at least 50% median reduction in active authoring/correction
time against matched manual work, with no critical escaped answer/context defects
in the release corpus/pilot. This is a proposed product target, not an accuracy
claim. Agree probabilistic per-family thresholds and completion-rate targets after
the baseline, then freeze them before holdout evaluation. Report sample sizes,
uncertainty and the difficult families separately. A small corpus cannot establish
zero real-world failure risk.

Local capacity: use one conversion/model/test worker at a time, inspect available
RAM before starting and reserve at least 2 GiB for the user's applications. Stop
task-owned heavy work if that reserve is breached. Do not load model weights or
launch GPU services on this laptop by default. Larger benchmarks need separately
sized infrastructure; check capacity before provisioning or running them.

### 9.3 Retention, observability and rollout

Propose separate retention classes: original/key files and committed provenance;
temporary conversions/renditions; superseded machine candidates; abandoned reviews;
reusable published media. Decide durations with Thuong using pilot needs/storage
measurements before release. The existing 13-month integrity policy is **not** an
implicit import-retention policy. Never cascade import deletion into a committed
test or referenced asset. Preserve append-only audit history and minimal commit
identity required to prevent duplicate materialization.

Operator tooling reports IDs, stage timing, backlog, error codes, retries,
stale claims, save conflicts, cleanup failures and cost. It must not log document
text/answers/transcripts/signed URLs. Test actual alert delivery, not just the
presence of an alert config. Runbooks cover converter/provider outages, stuck
claims, partial uploads/orphans, lost commit responses, leakage response, storage
failure and coordinated database/object restoration. Existing open production
alert/PITR verification work remains a prerequisite where this milestone relies
on it; do not mark those dependencies done without operational proof.

Expand-contract deployment: ship readers first, then new writes behind flags,
then pilot, then general availability. W-21 must exercise stale cached browser
bundles and enforce a reader capability/reload boundary before enabling interactive
new-format writes; deployment of a new HTML entry alone is not that proof.
Keep a release capable of reading all new
content as the rollback floor; an old binary that cannot read `semantic_v1` is not
a safe rollback after new-format publication. Disable new imports/provider calls
without removing revisions or breaking existing attempts. Drain/fence workers on
deploy and verify backup/restore before enabling production writes.

## 10. Decision register and first implementation checkpoint

| ID | Decision | Proposed direction | Owner / deadline |
| --- | --- | --- | --- |
| D-01 | Content/editor | Application-owned versioned AST, lightweight student renderer, evaluated editor adapter | Engineering + Thuong; W-02/03 before W-05 |
| D-02 | Group/material ownership and bank reuse | **Approved by Thuong:** independent copies; full context copied into bank/other tests; source deletion cannot affect copies | Engineering; reference/DDL review remains before W-07 |
| D-03 | Audio/deal semantics | **Approved by Thuong:** shuffle groups as units within a section, keep child order; a shared recording has one allowance per group/attempt, preserved on navigation/reload/takeover, independent across groups using the same file | Engineering; snapshot/API/ledger integration and legacy regressions before W-08/09 |
| D-04 | Access and unsent recovery | **Recovery approved by Thuong:** per-account local drafts, at most seven days, logout clearing, explicit local/server save states. Existing admin authorization still applies | Engineering; recovery and access implementation/tests pending |
| D-05 | Converter/extractor dependencies | Benchmark structured OOXML extraction and isolated LibreOffice normalization; Mammoth is a comparison candidate only | Engineering; W-03 before dependency addition |
| D-06 | Cloud/private model and data policy | Evaluate both, no silent provider fallback or external upload | Thuong + engineering; before real external benchmark/assisted processing |
| D-07 | Limits/SLOs/cost | Measure §9.2 hypotheses and approve supported envelope and spending cap | Thuong + engineering; W-21 before release |
| D-08 | Retention/cleanup | Separate source/evidence/transient classes; reference-safe deletion | Thuong; before cleanup implementation and W-21 |
| D-09 | Quality and pilot | Verified real corpus, holdout, matched manual baseline and explicit release thresholds | Thuong + pilot teachers; W-01 baseline, thresholds before holdout |

First checkpoint deliverables are W-01–04: source family/coverage inventory,
proposed contracts with concrete examples, extraction/editor/provider comparisons,
and a tested teacher review flow. The initial planning input had no real exam
documents; the subsequently supplied local corpus is inventoried in §1.1.
Synthetic tests and successful source inventory do not close W-01 or the
teacher-quality gate. Review expected outputs and confirm answer truth before
making recognition-quality or release-time claims.

## 11. Primary technical references checked during planning

- [Mammoth documentation](https://github.com/mwilliamson/mammoth.js): underline
  needs deliberate mapping; conversion is not sanitization and direct Markdown
  output is deprecated. Useful benchmark candidate, insufficient as a complete
  evidence/coverage architecture by itself.
- [LibreOffice conversion filters](https://help.libreoffice.org/latest/en-US/text/shared/guide/convertfilters.html):
  command-line conversion is available. Fidelity, isolation and resource envelopes
  still require tests on our files.
- [Tiptap content output](https://tiptap.dev/docs/guides/output-json-html): JSON
  editing is available; this is not evidence that all Word semantics survive.
- [vLLM structured outputs](https://docs.vllm.ai/en/latest/features/structured_outputs/):
  schema-constrained output is possible for the private-inference candidate;
  schema validity alone does not establish recognition quality.

Verify versions/licenses/security configuration again at dependency selection.
