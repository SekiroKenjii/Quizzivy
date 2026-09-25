# DOCX source inspection

`word.Inspect` reads a bounded OOXML package into **private source evidence**.
It is the extraction spike for W-03, not a completed importer, document renderer,
answer recognizer or a general OOXML schema validator. It uses the Go standard
library and does not execute objects, resolve external URLs or write files.

Paragraph/run locators refer to the original package part and namespaced XML
path. The caller must bind them to an immutable source revision. Parts and
relationships have stable ordering independent of ZIP entry order.

The inventory retains explicit formatting, table/container properties, revision
context, drawing/equation subtrees, ancillary XML and source relationship IDs.
Run text is an evidence convenience: inspect `Fragments` and findings before
using it. Field instructions and deleted/hidden/alternate text are **not** safe
display content. `Resolve` adds separate, conservative mark and numbering evidence.
Likewise, table cell order is source order, not a claim about page reading order.

`Assets` records declared types and sizes only. The media pipeline still needs
bounded decoding, sniffing, validation and reference promotion; an inventory
entry is not evidence that an image is valid or publishable. Original source,
answer files and full inspection JSON must never be sent to a student endpoint.

## Derived evidence

`word.Resolve(ctx, inspection)` consumes an unmodified result from `Inspect`.
It never changes the raw paragraphs, text or findings. Property paths, resolved
run locators and contributing style/list locators bind derived evidence back to
the original XML. Follow the document's relationships to styles and numbering;
an unreferenced conventional filename is not an authority.

Supported semantic marks are `b`, `i`, `u`, `strike`, `dstrike`, `vertAlign`,
`vanish`, `webHidden`, `caps` and `smallCaps` for Latin-script text. The resolver
walks document defaults, paragraph `basedOn` chains, explicit character styles
and direct formatting. Underline styles and sub/superscript retain their values;
direct on/off formatting can clear inherited emphasis. Paragraph-mark and list
glyph properties do not become body-run marks. `Complete` applies only to this
subset, not colors, fonts, layout, visibility selection or learner suitability.
An absent mark is unspecified; an unresolved mark has no asserted value.

Ambiguous style toggles across multiple contributing definitions are deliberately
unresolved until Word-version compatibility is characterized. Cycles, missing or
duplicate styles, conditional table styling, complex-script overrides and
non-Latin text also produce findings rather than guessed marks. Existing source
findings for revisions, fields and alternate content still require review even
when individual source-branch properties can be read.

Automatic labels stay separate from run text. The supported main-body subset
includes independent numbering instances, nine levels, explicit/default starts,
level/start overrides, restart rules, style-associated levels, `numId=0`, legal
numbering, decimal/zero-padded decimal, A–Z/a–z, Roman 1–3999 and Unicode bullets.
Missing `start` means zero and missing `numFmt` means decimal. A style's `ilvl`
does not override its abstract level's `pStyle` association. The suffix records
`tab`, `space` or `nothing`; this is source semantics, not measured geometry.

Unsupported number formats/ranges, custom formats, symbol-font/picture bullets,
numbering style links and section-break restart extensions remain findings.
Headers, notes and textboxes do not advance main-body counters. Revisions or
alternate branches in the body leave body numbering unresolved. An ambiguous
reference stops trusting subsequent counters; the resolver never silently skips
an unknown item and confidently labels the next one.

Implementation references:

- [Word character-style hierarchy](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.runstyle?view=openxml-3.0.1)
  and [Word toggle compatibility](https://learn.microsoft.com/en-us/openspecs/office_standards/ms-oi29500/f7130225-2368-48f3-acae-a9d278d0fb25).
- [Numbering properties](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.numberingproperties?view=openxml-3.0.1),
  [level text](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.leveltext?view=openxml-3.0.1),
  [start values](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.startnumberingvalue?view=openxml-3.0.1),
  [formats](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.numberingformat?view=openxml-3.0.1),
  [restart rules](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.levelrestart?view=openxml-3.0.1)
  and [start overrides](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.startoverridenumberingvalue?view=openxml-3.0.1).

## Local inspection

From `server/`, with an authorized local file:

```sh
GOMAXPROCS=2 go run ./cmd/word-inspect /absolute/path/exam.docx
GOMAXPROCS=2 go run ./cmd/word-inspect -json /private/new-evidence.json /absolute/path/exam.docx
```

The command runs inspection and resolution. Default output contains counts,
source/resolution finding codes and elapsed time, not exam text or labels.
Full JSON is opt-in, created with mode `0600`, and refuses an existing output
path. The command has a 30-second context deadline. Neither source files nor
private evidence belong in Git. Synthetic XML bodies under `tests/testdata` are
deliberately small, independently authored regression cases; the tests build
their ZIP containers at runtime.

## Limits and verification

Initial limits are 25 MiB compressed, 200 MiB declared expanded size, 4,096 ZIP
entries, 16 MiB per XML part, 32 MiB total XML, 32 MiB cumulative locator strings,
200,000 XML elements and 64 levels. Limits apply across parts where relevant.
Resolution separately caps style ancestry at 64, work at two million units,
retained locator bytes at 32 MiB and a generated label at 1,024 bytes. Context
cancellation is checked during indexing and resolution. Cycles become findings;
exhausting an expansion budget returns `ErrLimit`, never partial success.
XML reads verify decompression/checksum errors; assets are not decompressed by
this stage. A worker still needs an enforced memory/CPU/time boundary: Go context
cancellation and parser budgets do not replace operating-system isolation.

```sh
GOMAXPROCS=2 go test ./internal/platform/word/...
GOMAXPROCS=2 go test ./internal/platform/word/tests -run '^$' -fuzz FuzzInspection -fuzztime=15s -parallel=1
GOMAXPROCS=2 go test ./internal/platform/word/tests -run '^$' -bench BenchmarkInspect -benchmem
GOMAXPROCS=2 go test ./internal/platform/word/tests -run '^$' -bench BenchmarkResolve -benchmem
```

The regression corpus covers pronunciation marks, options in nested tables,
shared cloze text, tracked/hidden text, styles/numbering, textboxes, fields,
equations, compact keys and interleaved manual corrections. Security cases cover
invalid packages, active objects, XML directives, relationships, duplicate
attributes and cumulative expansion/complexity bounds. Real teacher files remain
a separate local evaluation corpus; passing these tests does not establish
recognition accuracy, `.doc` compatibility or teacher acceptance.

## Ordered extraction (W-12a)

`word.Extract(ctx, reader, size, sourceID, limits)` builds `ooxml-blocks-v1` private
source blocks from inventory and conservative resolution. The caller supplies the
immutable source record ID, never a mutable filename. Block IDs bind that identity
to the package part and XML locator; equal text in two locations stays distinct.
XML preorder intervals establish mixed paragraph/table/object order and ancestry.
Sorting XML paths would incorrectly group different element types and `p[10]`.
Order is local to a part, not a claimed visual/page reading order.

Paragraphs retain every source fragment and resolved mark with its evidence.
Offsets are Unicode code points across original paragraph fragments, not UTF-8
bytes or JavaScript UTF-16 indices. Generated numbering stays separate. Hidden,
inserted/deleted/moved text, field instructions/results (including fields spanning
paragraphs), ancillary parts and unresolved objects remain private evidence with
review reasons. The extractor never selects a tracked-change or alternate branch,
assigns correctness from formatting, or claims these blocks are learner-safe.

Nested tables keep separate grids. Cell coordinates are zero-based; column spans
and supported vertical merge origins retain their source properties. Duplicate,
overflowing, legacy horizontal or orphan/mismatched merge evidence remains
unresolved. The initial grid bound is 128 columns. Conditional table style fidelity,
visual columns, floating layout and page coordinates still require review/rendition.
See Microsoft's [grid spans](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.gridspan?view=openxml-3.0.1)
and [vertical merges](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.verticalmerge?view=openxml-3.0.1).

`word.ReadImage` reads one selected embedded asset without following a URL. It
checks dimensions before full decoding, following Go's [untrusted-image guidance](https://pkg.go.dev/image#hdr-Security_Considerations),
and re-encodes static PNG/JPEG as PNG without source metadata. Initial limits are
10 MiB input, 16 million pixels, 8,192 pixels per edge and 20 MiB output. Animated
PNG and EXIF-bearing images require an explicit later normalization decision;
unsupported vector/other formats are not displayed or silently flattened. A
normalized image is still private: extraction does not create a learner asset,
choose its placement, or authorize source/answer graphics for student delivery.

Local-only evaluation:

```sh
GOMAXPROCS=2 go run ./cmd/word-extract source.docx
GOMAXPROCS=2 go run ./cmd/word-extract -json /private/new-blocks.json source.docx
```

The CLI derives a local identity from the source digest; its default output contains
counts/codes/timing only. Optional evidence is a new `0600` file. It is not a
committable fixture or a public API payload. Large source evidence must be stored
as private artifacts and paged for review, not placed in the queue's candidate
result envelope. Original source and extracted text remain outside Git.

Synthetic regressions cover mixed/nested order, stable identities, merges,
Unicode offsets, inherited marks, automatic numbering, field/revision ambiguity,
image normalization/resource limits and invalid images. A bounded fuzz check
asserts distinct identities and valid ancestry. A local independent XML walk of
the eight authorized DOCX files matched all 13,489 source fragments; this is text
coverage evidence, not recognition accuracy or an independently reviewed answer
benchmark. The private corpus/check output is not part of the repository.
