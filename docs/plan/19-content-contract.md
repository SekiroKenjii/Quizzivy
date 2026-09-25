# Versioned content contract — W-02 foundation

Status: content foundation plus W-05a options and W-05b question prose, with
pilot authoring affordances, stable blank bindings and bounded clipboard conversion.
Group graph validation/copy foundations are additive; group persistence and import
endpoints remain pending.
`api/openapi.yaml` is the structural authority. Go's `shared/content.Parse` and
the frontend validator enforce the cross-node rules below. They share synthetic
accept/reject fixtures; generated TS types are the frontend model. Go's domain
value object stays independent of HTTP/generated types. The generator retains
these standalone components intentionally until the rich endpoints consume them.

## Vocabulary and readers

`ContentDocument` discriminates by `format`. `legacy_markdown_v1` contains exactly
`markdown` and preserves that string (including whitespace and Unicode form);
its reader is the existing sanitized Markdown renderer. `semantic_v1` contains
one or more `blocks`. It never contains editor JSON or raw HTML nodes.

Blocks: paragraph, heading (levels 1–3), ordered/unordered list, table, image and
audio. Inline nodes: text with allowlisted marks, break, stable gap and HTTPS link
containing text only. Marks are bold, italic, underline, strike, superscript and
subscript; duplicates and simultaneous super/subscript are invalid. A blank
paragraph is valid. Each list item begins with a paragraph; unordered start is 1.

Tables contain source-order rows/cells, with explicit `header`, `rowSpan` and
`colSpan`. Place each cell at the first unoccupied column, skipping rowspans.
Spans cannot overlap, escape the grid or leave holes; every row has the same
nonzero occupied width. A row may have no new cells only when prior spans cover
it completely. Tables may contain lists, but never another table, even through
a list. Unsupported source structure remains a review finding, not silent loss.

A gap has a stable case-sensitive ASCII identity (1–64 characters, first
alphanumeric, then alphanumeric/underscore/hyphen) and a 1–32 scalar display
label. IDs are unique per document. Binding gaps to question/blank identities is
an enclosing graph invariant, not an inferred relationship from displayed labels.

Image/audio nodes carry UUID asset references and required alt/label text. These
are inert in the foundation reader. References are deduplicated case-insensitively
in first-use order; one UUID cannot claim both kinds. The enclosing application
must validate existence, authorization, kind, ownership and relational bindings,
and take the existing media reference locks before persistence/publication. No
parser/renderer fetches a URL from these nodes. Private source/key files cannot
be used as learner media merely because their IDs are known.

Links permit explicit HTTPS, ASCII DNS-style host labels (international domains
must be punycoded), optional decimal port 1–65535, and Unicode path/query/fragment.
IPv6 literals are outside this initial portable subset. Reject credentials,
whitespace, C0/C1 controls, backslashes and malformed percent escapes. Never fetch
links during import or validation. Render with the existing safe external-link
attributes. The validator does not assert the destination's reputation or reachability.

## Budgets and strictness

Limits are per content document, not a whole exam; larger exam/group/job budgets
remain W-02/W-21 work. `ContentDocument.x-content-limits` pins these values:

| Budget | Limit / counting rule |
| --- | --- |
| JSON input | 1 MiB before decoding; also enforce enclosing HTTP/worker input limits |
| Nesting | 16 JSON property/array edges; root depth 0, including scalar leaves |
| Values | 24,576 JSON values; object property names are not values |
| Width | 2,048 elements/properties per array/object before shape validation |
| All string values | 200,000 Unicode scalar values including type/format names, IDs and URLs |
| Content nodes | 2,048 blocks, inline nodes (including linked text) and table cells |
| Visible text | 100,000 Unicode scalars across text, gap labels, image alt and audio labels; legacy Markdown counts in full |
| URL / table | 2,000 scalars per URL; 50 rows, 12 occupied columns |

JSON Schema lengths and both semantic validators count Unicode scalar values,
not UTF-16 code units or UTF-8 bytes. Reject malformed UTF-8, unpaired escaped
surrogates and NUL; do not replace them with U+FFFD or normalize Vietnamese text.
Transport parsing rejects duplicate object keys (including escaped aliases),
trailing JSON, non-JSON numbers and oversized/deep/wide values before constructing
a recursive domain graph. Frontend validation of already-parsed objects cannot
recover duplicate-key evidence: future server writes must feed original raw JSON
to this parser, not marshal a permissively decoded object back into it.

Every object is closed. Unknown nodes/marks/properties—including grading keys,
source coordinates, arbitrary style/classes and event handlers—are rejected,
never stripped. Structural JSON Schema validation is necessary but insufficient;
it does not express graph, Unicode, URL or aggregate invariants. The Go error
contains no input text. This validation cannot infer whether teacher-entered
ordinary text discloses an answer; review and learner-preview policy still apply.

Plain-text projection retains text and link labels, emits breaks as newlines and
gaps as `[label]`, separates top-level blocks by two newlines, list items/cell
blocks/table rows by one newline, and table cells by tabs. Spans do not duplicate
text. Asset labels participate; URLs, IDs, marks and provenance do not. The
projection is deterministic search input, not a faithful rendered exam or a
canonical review digest. Legacy strings remain exact.

## Integration, ownership and recovery gates

Thuong approved independent group/material graphs when copying to another test
or saving to the bank. Copies retain all required members/materials, remap gap and
membership identities, and survive source edits/deletion. Reused immutable asset
bytes have distinct relational bindings. Source provenance is bookkeeping, never
a live content dependency. DDL, deletion locks, version snapshots, copy/restore
and concurrent-reference tests remain W-07/W-08 work.

Thuong also approved account-isolated recovery for unsent edits for at most seven
days, cleared on logout. Implement a bounded local outbox only with explicit
local/server save states, revision-based conflict handling, expiry before reads
and replay, visible quota/storage failures and account-switch/logout tests.
Do not renew expiry merely by reading an old revision. Exclude uploaded original
and answer files. Server acknowledgements alone define server durability. Storage
and UI implementation remain W-18 work; this foundation adds no browser storage.

Readers ship before new writes. Add rich question fields and their relational
asset bindings with explicit contract/migrations, then builder, preview,
publication/restore, attempts/results and answer-leak tests. Preserve the complete
historical snapshot/deal/audio behavior. Do not backfill or rewrite old published
versions as part of adding readers. After new-format writes begin, rollback must
retain a binary capable of reading them; a flag disables new writes, not readers.
Old snapshots can remain read-only through that rollback floor. Final D-01 editor
acceptance and the complete group/revision endpoint contracts remain open.

## Approved delivery policy — D-03

When question shuffling is enabled, groups and standalone questions move as units
within their section. Sections and members within a group keep authored order;
with shuffling disabled, all units keep authored order. Group membership, order
and delivery version are frozen in the snapshot and retained for attempt reload
and takeover. Historical snapshots continue using the existing algorithm/seeds.
Option-label dependencies must be checked against option shuffling separately.

A recording shared by a group has one configured play allowance across its
children for that attempt. Child navigation, reload and takeover cannot reset it;
another group using the same file has a separate allowance. Thus an explicit
versioned group-recording binding identifies the playback scope, not the asset
alone. Publication freezes the binding/policy; restoration/copy creates an
independent draft binding. Preserve synchronous user-gesture playback, optimistic
accounting, replay deduplication, reporting and transcript visibility policies.

Required integration evidence: grouped/standalone deal ordering with shuffling
on/off; fixed child/cloze order; stable answers after reload/takeover; counts
shared across children but separate across two groups using the same asset;
idempotent playback retries and session handover; immutable published bindings;
and unchanged legacy deal/audio/leak canaries. This is an approved product policy,
not an enabled player, new ledger schema or implemented group endpoint.


## W-07a — independent group graph

`QuestionGroup` is an additive admin contract; current section/test writes do not
accept it. The tests domain resolves a `GroupBundle` against exactly its member
questions, validates existing question invariants and rejects missing, duplicate
or foreign members. The catalog's fetch order and UUID letter case do not define
authored member order. Empty drafts are allowed; publication rejects emptiness.
Fixed option order applies only to choice members and rejects a conflicting
shuffle configuration rather than silently changing the assignment policy.

A stimulus owns a content document plus one binding per gap. `question` targets
a choice member; `blank` targets a rich fill-blank member's stable prompt gap.
Printed labels may repeat. Binding IDs are local to a document, while response
targets are unique across the group's materials. Blank row IDs are not stable
targets because ordinary question writes may replace those rows. Unsupported
targets, orphan gaps, missing keys, invalid content and mixed asset kinds fail
before copy. These are graph checks in addition to the structural OpenAPI schema.

Each audio asset appearing in materials has exactly one explicit recording
binding, even when repeated across materials. It cannot also carry a member-level
allowance in that group. Other member audio retains its own policy. Different
groups may reuse the same file under independent recording identities. Group
recording transcripts are teacher metadata: future learner/review projections
must be explicit and disclosure-policy gated, never JSON casts of this graph.

The complete resolved group is limited to 4 MiB, 200 members, 16 materials and
16 recordings. Titles allow 200 Unicode scalars and recording transcripts
100,000; the content-document limits still apply independently. Persistence must
validate original bounded raw input, authorized media kinds and relational
references, not treat successful domain validation as an access grant.

`GroupBundle.Copy` first validates and deep-copies all data, then generates fresh
group, member, option/blank, material, local-gap and recording identities. Both
ends of cloze links are remapped together; exact text, keys, scores, order and
audio policy remain unchanged. Asset IDs stay stable; future storage creates
independent protected bindings. An invalid/repeated ID provider returns no
partial copy and cannot mutate its source. Source provenance is not a live link.

Relational ownership, group-only bank insertion, deletion races, optimistic
revisions, snapshot/restore and learner delivery are not supplied by this value
model. Keep new writes disabled until those lifecycle paths and readers ship;
do not claim W-07 complete from these foundation tests.

## W-05a — inline option integration

`OptionContent` is a deliberately smaller semantic document: exactly one
paragraph, with text/marks and breaks only. It inherits all `ContentDocument`
budgets. It cannot bind media/gaps or carry links, tables, source evidence or
answer keys. `content.ParseOption` checks original raw JSON, including duplicate
keys; generated Go request fields use `json.RawMessage` so typed decoding does
not destroy that evidence. Frontend validation uses the same content validator
plus this profile. The stored `text` must equal the exact plain projection.
Legacy options are literal text, unlike legacy Markdown prompts. Conversion
wraps text/breaks without interpreting Markdown, HTML or normalization.

The additive nullable option field travels through bank CRUD/duplication,
publication, restore, published/draft preview, attempts, results and teacher
review. Migration 00032 stores it per normalized option; it does not rewrite
historical versions. Absent content retains existing rich formatting only with
a matching current option ID and unchanged text, checked while the parent write
lock is held. Explicit null clears it. Stale or changed legacy writes roll back
instead of dropping marks. New clients send the complete document or explicit
null. Existing full-question concurrency semantics are otherwise unchanged;
revision-based authoring conflicts remain a later work package.

`VITE_RICH_OPTION_EDITOR` defaults to false and controls the pilot affordance to
format plain options in both bank and builder. It is a UI rollout flag, not an
API authorization boundary. Authenticated question writes validate rich content
regardless of the flag. Existing rich options always remain readable/editable.
The option editor loads on demand, accepts the inline profile only, maps Enter
to a line break, rejects formatted/file paste visibly, and updates the existing
bank save/builder autosave coordinator immediately. No local recovery claim is
made. Disabling the flag must not remove readers or roll back migration 00032.

W-05b below extends this checkpoint to prompts/explanations. W-05/W-06 remain
partial: broader content adapters, five-type authoring, safe structured paste,
final D-01 acceptance and complete shared validation are still pending.
W-07–W-09 group/media behavior is unchanged.


## W-05b — question prose integration

The next profile, `QuestionContent`, carries semantic prose in nullable
`promptContent` and `explanationContent`. It allows paragraphs, headings, lists
and tables with text, breaks and safe links; recursively rejects assets and gaps.
This preserves the existing media/reference lock paths and leaves graph binding
to W-07/W-08. Prompts for `fill_blank` retain legacy Markdown until stable gap
bindings are integrated. Explanations support every question type.

Each non-null document must project exactly to its companion string. Raw JSON
strictness and aggregate budgets are checked on question writes and publication.
Absent rich fields in legacy updates preserve stored documents only when the
companion text is unchanged, under the question row lock. Explicit null clears
the document. The new UI sends explicit document/null values. Existing concurrency
semantics remain; this is not revision-conflict resolution or durable recovery.

Migration 00033 adds nullable prompt/explanation JSONB to bank and snapshot
questions, without backfill or new privileges. Publication, restoration, copying
and reads carry the fields independently of grading keys. Active paper/preview
queries never read explanations. Result SQL gates both explanation columns with
the existing review policy. Rollback after rich writes retains these columns
and capable readers; destructive Down is only for disposable/pre-rollout data.

The lazy authoring affordance is controlled by `VITE_RICH_QUESTION_EDITOR`
(default false); existing rich documents remain editable. Markdown conversion
is explicit and refuses unsupported nodes rather than dropping them. Media,
gap bindings, structured clipboard import, IME/teacher acceptance and full
five-type rich prompt authoring remain later gates.


## W-06b — stable question gap bindings

`QuestionPromptContent` extends the prose profile with gaps inside paragraphs,
headings, lists and table cells. `QuestionContent` remains gap-free for explanations.
A rich fill-blank prompt needs at least one gap, and its gap IDs must match the
non-null `gapId` set on blank metadata one-to-one. No inference from labels,
ordinals or plain projection is permitted. Other interaction types reject gaps;
legacy Markdown rejects non-null gap IDs. All document budgets remain unchanged.

Migration 00034 stores nullable `gap_id` on bank and snapshot blanks, with format
checks and uniqueness scoped to the parent question. Legacy rows stay null.
A gap identity survives draft edits even though normalized answer-row UUIDs may
be replaced. Publication freezes identities and answers together. Learner answers
continue to use frozen blank UUIDs, and existing grading logic remains unchanged.
Duplication and version restoration allocate fresh local gap IDs and replace both
AST references and metadata in the same write transaction. Source edits do not
mutate a copied graph or snapshot. Student payloads contain no new grading keys.

Explicit Markdown conversion accepts only unique supported markers with matching
answer rows and previews before applying. Repeated/unsupported markers block
conversion. In rich authoring, newly inserted gaps receive empty answer rows;
moving a node preserves its key. Removing a node retains orphaned answers for
undo and blocks save until they are restored or explicitly discarded. Removing
formatting restores ordinal markers before escaping the prose projection.
New authoring uses the existing opt-in flag; stored rich blank prompts stay
editable when disabled. This is not structured clipboard import or pilot acceptance.

Migration Down first restores the old prompt restriction. It therefore refuses
rich fill-blank rows without losing data. After such writes, retain the schema
and these readers as the rollback floor and disable only new authoring.

## W-06c — structured clipboard conversion

The editor intercepts formatted/file paste at the DOM-event boundary, before
ProseMirror's HTML parser runs. `parse5` 8.0.1, MIT, is an explicit dependency for
inert standards-based HTML parsing. Raw clipboard HTML is never mounted, saved,
sent to a service, or included in diagnostics. The converter loads on demand.

The accepted subset maps paragraphs, headings 1–3, ordinary lists with explicit
starts, tables/spans, six marks and safe HTTPS links to the existing semantic AST.
Allowlisted inline CSS supplies marks. Fonts, size, color and layout decoration
are replaced by the product design; the preview explains that normalization.
Unknown tags/attributes, active content, images/files, unbound gap metadata,
stylesheets, conditional Word markup, tracked changes, ambiguous CSS/list rules,
nested/malformed tables and unsafe URLs cause whole-paste refusal. Parsed source
locations must cover the input: HTML tree repair cannot silently discard unknown
source tags. This is deliberately narrower than arbitrary Word clipboard HTML.

Input is at most 256 KiB UTF-8, with valid Unicode and no NUL. Parse inspection
allows at most 6,144 source nodes and 48 tree levels before semantic conversion;
existing content limits and the narrower field profile apply to both the candidate
and the complete replacement. No mark implies an answer key or gap binding.

Preview renders only the validated resulting semantic document, with accessible
cancel/apply controls and focus restoration. The original document/selection is
captured before opening it. Cancellation and a late cancelled conversion do not
change content. If the document changes before confirmation, the user must paste
again. Applying uses one validated transaction with history boundaries on both
sides; undo restores the previous content and answer bindings remain governed by
W-06b. Existing autosave/server rules apply after the confirmed edit.
