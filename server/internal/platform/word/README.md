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
display content. Numbering and inherited styles are retained but not resolved.
Likewise, table cell order is source order, not a claim about page reading order.

`Assets` records declared types and sizes only. The media pipeline still needs
bounded decoding, sniffing, validation and reference promotion; an inventory
entry is not evidence that an image is valid or publishable. Original source,
answer files and full inspection JSON must never be sent to a student endpoint.

## Local inspection

From `server/`, with an authorized local file:

```sh
GOMAXPROCS=2 go run ./cmd/word-inspect /absolute/path/exam.docx
GOMAXPROCS=2 go run ./cmd/word-inspect -json /private/new-evidence.json /absolute/path/exam.docx
```

Default output contains counts, finding codes and elapsed time, not exam text.
Full JSON is opt-in, created with mode `0600`, and refuses an existing output
path. The command has a 30-second context deadline. Neither source files nor
private evidence belong in Git. Synthetic XML bodies under `tests/testdata` are
deliberately small, independently authored regression cases; the tests build
their ZIP containers at runtime.

## Limits and verification

Initial limits are 25 MiB compressed, 200 MiB declared expanded size, 4,096 ZIP
entries, 16 MiB per XML part, 32 MiB total XML, 32 MiB cumulative locator strings,
200,000 XML elements and 64 levels. Limits apply across parts where relevant.
XML reads verify decompression/checksum errors; assets are not decompressed by
this stage. A worker still needs an enforced memory/CPU/time boundary: Go context
cancellation and parser budgets do not replace operating-system isolation.

```sh
GOMAXPROCS=2 go test ./internal/platform/word/...
GOMAXPROCS=2 go test ./internal/platform/word/tests -run '^$' -fuzz FuzzInspection -fuzztime=15s -parallel=1
GOMAXPROCS=2 go test ./internal/platform/word/tests -run '^$' -bench BenchmarkInspect -benchmem
```

The regression corpus covers pronunciation marks, options in nested tables,
shared cloze text, tracked/hidden text, styles/numbering, textboxes, fields,
equations, compact keys and interleaved manual corrections. Security cases cover
invalid packages, active objects, XML directives, relationships, duplicate
attributes and cumulative expansion/complexity bounds. Real teacher files remain
a separate local evaluation corpus; passing these tests does not establish
recognition accuracy, `.doc` compatibility or teacher acceptance.
