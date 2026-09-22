# Versioned content contract — W-02 foundation

Status: additive domain/API foundation; no production question writer, data
migration, group persistence or import endpoint is enabled by this change.
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
Old snapshots can remain read-only through that rollback floor. D-03 shared audio
and group-deal contracts and final D-01 editor acceptance are still open.
