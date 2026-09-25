# Private Word conversion runtime

This image supports the W-13 converter behind `platform/wordconvert`. It is not
wired to public upload/processing routes yet. Original DOCX and binary Word DOC
remain private sources; conversion never creates learner media or a test.

Build locally from this directory's narrow context:

```sh
docker build -t quizzivy-word-converter:development docker/word-converter
docker image inspect quizzivy-word-converter:development --format '{{.Id}}'
```

The Go adapter requires the resulting immutable `sha256:…` image ID and uses
`--pull=never`. Record that ID with the rendition manifest. Rebuilding for package
security updates creates a new processing version; a mutable image tag is not a
runtime identity. Debian's supported Writer package supplies native legacy Word
filters. Runtime additions are LibreOffice Writer, Python UNO (explicit safe load
properties), Poppler (bounded page rasterization) and Liberation/DejaVu fonts.
No Python/UNO object is exposed to the API process or document text.

## Boundary

The adapter copies at most 25 MiB into a new `0700` disk job with `0600` input.
Native DOCX is inspected before conversion; normalized legacy DOCX is inspected
again before use. The source mount is read-only. The writable work and fallback
`/tmp` mounts are job-specific disk directories, never the host's `/tmp` or tmpfs.
No home directory, repository, credential file or Docker socket enters the container.

The converter runs as the worker's non-root UID with a read-only root filesystem,
no capabilities, no-new-privileges, no network, one CPU, 512 MiB RAM with no extra
swap, 64 processes, 256 file descriptors and a 128 MiB per-file ceiling. One fixed
Docker name allocates a physical converter slot atomically across worker processes.
An occupied slot returns busy; cleanup uses the invocation's journalled container
ID or unique owner label and cannot kill another worker. Completed owned slots can
be reclaimed after a crash. A crashed worker cannot start overlapping conversion
containers just because its database lease expired.

The Go timeout/cancellation path force-removes its container. The image also has
an independent 120-second process-group kill deadline, which remains effective if
the host worker disappears. A disk watchdog cancels jobs exceeding 256 MiB or 4,096
entries. This is a polling threshold, not a filesystem quota. A cleanup failure
retains the private job and reports an operator-action error rather than pretending
cleanup succeeded. Broader orphan-file retention remains W-21/D-08 work.

UNO loads with [MacroExecutionMode=NEVER_EXECUTE](https://api.libreoffice.org/docs/idl/ref/namespacecom_1_1sun_1_1star_1_1document_1_1MacroExecMode.html)
and [UpdateDocMode=NO_UPDATE](https://api.libreoffice.org/docs/idl/ref/namespacecom_1_1sun_1_1star_1_1document_1_1UpdateDocMode.html).
An interaction handler aborts password/repair prompts. Do not replace this with a
bare `--convert-to` call that relies on a user's profile. Corrupt/locked inputs
fail without a successful empty result or a lossy text fallback.

## Artifacts and limitations

The runtime emits a private PDF, at most 60 PNG pages (long edge at most 1,600 px),
and a normalized DOCX only for legacy input. The host checks allowed names,
sequence, regular-file status, byte totals, PNG decoding and native package safety;
`os.Root` confines artifact access. Total accepted artifacts are at most 128 MiB.
Results record source checksum, image ID, renderer version and artifact checksums.
Callers must Close the result after durable storage to remove transient files.

All renditions require layout review. Legacy normalization additionally requires
conversion review. Font substitution, desktop layout differences, fields and other
losses are not silently asserted equivalent to Microsoft Word. Page images are a
comparison aid; they do not provide fabricated question bounding boxes. Original
files and source answer annotations never become learner assets automatically.

Local command, using an existing private disk workspace and a new output directory:

```sh
GOMAXPROCS=2 go run ./cmd/word-convert \
  -image sha256:THE_LOCAL_IMAGE_ID \
  -work-dir /absolute/private/work \
  -output /absolute/private/new-artifacts source.docx
```

Run from `server/`. Output contains private source material and must stay outside
Git. The image is a runtime dependency, not a new Go module dependency. Production
capacity, real legacy-family fidelity, durable artifact wiring and retention remain
gates; these development limits are not an approved throughput promise.

## Checks

Set `TEST_WORD_CONVERTER_IMAGE` to the local image ID and run:

```sh
go test -tags integration ./internal/platform/wordconvert/...
```

Set `TMPDIR` and `GOTMPDIR` to a private disk directory for local checks. The suite
uses synthetic files and one container at a time. It covers real binary DOC
normalization/underlines, encrypted sources, PNG rendition, timeout/cancellation,
physical-slot collision/reclaim, disk limits and output confinement. A runtime
probe checks actual network reachability, mount writes, capabilities, cgroup limits
and disk-backed temporary mounts. Docker Desktop may create down tunnel devices;
network isolation is checked through routes, interface state and failed egress,
not an assumption that loopback is the only named device.
