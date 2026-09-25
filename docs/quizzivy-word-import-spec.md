# Quizzivy — Production Word Exam Import Specification

| Field | Value |
| --- | --- |
| Version | 1.1 |
| Date | 2026-09-25 |
| Status | Proposed production product specification; not implemented |
| Repository | https://github.com/SekiroKenjii/Quizzivy |
| Reviewed baseline | `main` at `4b84de3bf468fcb4b0f477caacbf47071b18f2d3` |
| Audience | Product, engineering, design, QA, and operations |
| Documentation language | English, matching the repository convention; product UI remains Vietnamese-first |

This specification describes a complete production capability. Core correctness, content fidelity, review, recovery, security, and operations are release requirements. Delivery stages sequence engineering work; they do not define a reduced-scope product.

Repository facts refer to the previously reviewed commit. All new components, contracts, targets, and models below are proposals. No claim is made that they already exist, have been benchmarked, or reflect later repository changes.

## 1. Product objective

A teacher uploads an existing English exam in Microsoft Word format and obtains a correctly structured, editable Quizzivy test. The system reconstructs sections, shared materials, question groups, questions, answer keys, media associations, and scoring. The teacher reviews uncertainty and confirms the result before it enters the normal authoring and publishing workflow.

The product succeeds when teachers spend substantially less time creating exams without sacrificing correctness or control. Successful parsing, valid JSON, or an attractive preview alone does not constitute success.

### 1.1 Non-negotiable outcomes

- Existing free-form Word documents are a first-class input. A template is an optional convenience, never a prerequisite.
- Content must not silently disappear, change meaning, acquire invented answers, or attach to the wrong passage.
- Uncertainty is represented explicitly, linked to source evidence, and resolvable in the product.
- A saved review survives reloads, worker failures, deployment restarts, and transient provider outages.
- Committing an import creates one coherent draft test; retries do not create duplicates or partial exams.
- Imported exams use the same rendering, validation, publication, and grading contracts as manually authored exams.
- A feature is supported only when it works end to end: extraction, review, editing, publication, learner delivery, and grading where applicable.

### 1.2 Product boundaries

This feature imports existing exam content. Generating new questions, solving missing answers, OCR of scans, and automatic assignment or publication are separate products and are outside this specification.

Version 1.1 (2026-09-25, decision D-10): PDF files with a text layer are accepted as exam and key sources. A PDF gives no reliable underline, bold or colour marks, so questions that depend on them are completed by the teacher in review. Scans remain out of scope.

Word document support is not a promise to preserve arbitrary desktop publishing layouts pixel for pixel. Preserve all exam semantics and provide a faithful source view for comparison. Unsupported document objects must produce visible, actionable findings.

## 2. Baseline and required platform changes

| Reviewed baseline | Required production design |
| --- | --- |
| Go modular monolith, React/TypeScript, PostgreSQL, R2-compatible storage | Implement within the current architecture, with isolated document processing where required. |
| Five existing question types | Import those types faithfully; classify other types explicitly and prevent misleading conversion. |
| Bank creation requires valid answer data | Keep incomplete imported content in a separate durable review model. |
| Draft outlines reference bank questions | Commit through a coordinated application use case with atomic persistence. |
| Publish freezes question content and grading data | Extend snapshots to include shared materials, groups, formatting, and required asset references. |
| Markdown prompts; no explicit underline support in the inspected renderer | Introduce a safe, versioned semantic content representation and one rendering contract. |
| Section instructions displayed as a small note | Add proper passage/material authoring and learner presentation; do not overload instructions as the permanent passage model. |
| Media kinds are audio and image | Keep source documents separate and support explicit asset placement and shared media references. |

Read the current `AGENTS.md`, product specification, architecture plan, and API contract before implementation. Update the authoritative documents when this feature changes their assumptions. Follow the existing dependency rules, reference-locking rules, localization conventions, and contract generation process.

## 3. Supported input and content contract

### 3.1 File formats

| Input | Required behavior |
| --- | --- |
| `.docx` with editable text | Native structured extraction, including paragraphs, runs, numbering, tables, relationships, and embedded images. |
| Legacy `.doc` | Conversion in a resource-limited isolated process, preserving the original and converted artifact; then the same pipeline and review guarantees. |
| Password-protected documents | Reject with a clear request to provide an unprotected copy. |
| Corrupt or mislabeled documents | Detect from actual package/content structure and return an actionable error. |
| Macro-enabled documents, active embedded objects, or executable payloads | Reject or isolate unsupported objects without executing them; block completion if required content is unavailable. |
| PDF with a text layer | Read the text in an isolated, resource-limited reader, never by conversion. Keep running headers, footers and page numbers out of the exam and list them for review. State in the review that formatting marks are unavailable. |
| Scanned or image-only PDF | Detect the missing text layer and ask for the Word file or a PDF exported from Word; do not attempt OCR. |
| Image-only/scanned Word documents | Detect lack of extractable exam text and report the unsupported source class; do not present an empty import as success. |

Do not silently fall back from failed legacy conversion to a lossy text extraction path. Conversion findings must be included in the review report.

### 3.2 Document structures

The supported corpus must cover paragraphs, manual and automatic numbering, options on separate lines or in tables, multiple options on one line, nested tables where safely interpretable, multi-column layouts, shared passages, numbered cloze gaps, images, and answer tables.

Headers, footers, footnotes, text boxes, floating objects, comments, hidden text, and tracked changes must be inventoried. Define extraction policies for each class. Relevant content that cannot be interpreted must be surfaced for review. Tracked-change ambiguity requires a visible decision about which revision constitutes the exam.

Every meaningful source block is assigned to an exam element, recorded as metadata, or explicitly excluded. Reused blocks may have multiple justified references. Coverage validation must not mistake intentional reuse for duplication.

### 3.3 Question compatibility

| Source question | Target behavior |
| --- | --- |
| One correct option | `single_choice`; exactly one correct option. |
| Several correct options | `multiple_choice`; preserve the explicit key and existing grading semantics. |
| True/false | `true_false`; exactly two options and one correct option. |
| Typed gaps with accepted answers | `fill_blank`; explicit slots, answer variants, and case policy. |
| Teacher-graded written response | `short_answer` when its response and grading requirements fit the existing type. |
| Pronunciation or error-identification question | Supported choice type with preserved semantic marks in prompt and options. |
| Cloze passage with separate options per gap | Grouped choice questions tied to stable gap IDs in a shared material. |
| Matching, ordering, long essays with richer rubrics, or another unsupported interaction | Classify accurately and retain all source content. Require an explicit teacher transformation or exclusion; do not relabel merely to pass validation. |

Adding a new question interaction requires a separate end-to-end domain change. The import feature must not create data that the learner renderer or grading engine cannot interpret.

### 3.4 Answer sources

Support explicit answers inside the question, a final section/table in the same file, and an optional separate answer-key document. Match answers using section identity, question labels, source location, and explicit option labels. Printed question numbers alone are insufficient when numbering restarts.

Bold, color, or underline may mark an answer only when an explicit template convention or a teacher-confirmed rule establishes that meaning. Preserve pronunciation marks separately from answer-key markings. Teacher-only answer annotations must not leak into learner-visible formatting.

An answer absent from the source remains unknown. Short-answer questions may legitimately have no sample answer; other answer requirements depend on their grading contract.

## 4. User journeys

### 4.1 Import a complete exam

1. Open Import Word from test authoring.
2. Upload the exam and optionally an answer-key file; attach or select listening assets if available.
3. Choose automatic recognition, a saved import profile, or a published template profile.
4. See durable processing progress and actionable failures.
5. Review the source and reconstructed exam with issue navigation.
6. Resolve blocking findings; confirm inferred structure, defaults, and exclusions.
7. Preview the learner-facing result, including groups, media, and formatting.
8. Create the draft test and continue in the existing builder.
9. Publish separately through the normal validation and snapshot flow.

### 4.2 Resume or correct an import

The teacher can leave and return to an import, inspect processing history, resume saved edits, retry a recoverable failure, replace a source with a new revision, and compare reprocessed candidates. Reprocessing must not silently overwrite manual edits.

### 4.3 Unsupported or incomplete input

The teacher sees the exact affected source, why it cannot be represented, and available actions. The teacher may repair the mapping, supply an explicit missing answer or asset, transform the question intentionally, or exclude it with a recorded reason. Exclusion changes the included question count and scoring visibly.

### 4.4 Import history

Provide a searchable list of imports with filename, creator, creation time, status, issue counts, and resulting test. Distinguish retrying the same operation from intentionally creating another test from the same source.

## 5. Review and authoring experience

### 5.1 Source and result comparison

Provide a source pane and reconstructed exam pane linked through source references. Support document outline navigation, source highlighting, issue filters, and jumping between a question and its answer evidence.

Generate a source rendition where needed for layout-dependent documents. Label the difference between a rendered page view and semantic extracted content. The original file remains available to authorized teachers. Page numbers are valid source coordinates only when produced by an actual renderer; stable block/run coordinates are always retained.

### 5.2 Editing requirements

Teachers can edit prompts, option content, question types, keys, accepted answers, scoring, explanations, groups, instructions, passages, and media placements. Support splitting or merging recognition candidates and moving questions between groups without losing provenance.

Every substantive edit invalidates affected automated validation and any relevant prior confirmation. Edits use revision-based concurrency control. Autosave flushes before navigation and commit, and unresolved failures remain visible.

### 5.3 Review semantics

Differentiate explicit source facts, inferred structure, suggested defaults, and teacher edits. Show concrete reasons for uncertainty rather than a bare confidence percentage.

Blocking issues cannot be dismissed by a blanket confirmation. Required-review findings need a concrete decision. Batch confirmation is allowed for equivalent low-risk findings when all affected items are visible and the action is recorded.

The final summary shows included/excluded questions, sections, groups, total points, applied defaults, answer completeness, media availability, and unresolved findings. The Create draft test action is enabled only for a current valid revision.

### 5.4 Accessibility and localization

Use Vietnamese-first strings with English translations under the existing localization system. All review operations must be keyboard-accessible, have labeled controls, and avoid conveying status by color alone. Screen-reader announcements must communicate progress and save errors without overwhelming the user.

## 6. Canonical content and assessment model

### 6.1 Semantic content representation

Introduce a versioned, application-owned structured content model for paragraphs, text runs, supported marks, lists, tables, links, and asset references. Marks include bold, italic, underline, and supported superscript/subscript where meaningful.

Use typed content nodes and an allowlisted renderer; never store arbitrary imported HTML as trusted application content. The representation must cover prompts, options, shared materials, and explanations as appropriate.

Persist stable media asset IDs rather than expiring URLs. Resolve URLs at delivery time. Define table semantics and accessible reading order. Unsupported Word styling is discarded only when it is demonstrably decorative; semantic loss creates a finding.

Existing Markdown content needs a backward-compatible migration/adapter path. Preserve the rendering of existing tests and published versions. Version the new content contract and validate both supported historical and new representations during rollout.

### 6.2 Shared materials and groups

Add first-class concepts rather than permanently embedding passages in section instructions:

| Concept | Responsibility |
| --- | --- |
| `Section` | Exam organization, title, and section instructions. |
| `Stimulus` | Shared passage, audio, image, table, or composite material. |
| `QuestionGroup` | Ordered questions sharing instructions, stimuli, or response dependencies. |
| `Question` | Supported response interaction and grading contract. |

Groups and stimuli require editable draft representations and immutable published snapshots. A group can reference multiple stimuli. Material placement must not depend on printed numbering remaining unchanged.

For cloze, use stable gap IDs linked to the corresponding question/blank. For listening, define group-level playback policy when audio is shared; enforce play limits at the correct scope rather than granting a fresh limit for each child question.

### 6.3 Ordering and randomization

Represent ordering constraints explicitly. A group is an indivisible unit where context requires it; member order is preserved when questions refer to each other or to ordered gaps. Option shuffling is permitted only when labels, keys, and references remain semantically valid.

The existing assignment/deal path must honor these constraints. If a requested randomization policy is incompatible, block that configuration with an explanation. Import correctness must not be undone at attempt delivery.

### 6.4 Authoring and publication parity

Manually authored content and imported content share the same editing and rendering capabilities. Publication freezes the complete assessment graph: sections, groups, stimuli, questions, grading keys, ordering constraints, and immutable asset references.

Student payloads exclude answer keys, answer provenance, teacher notes, restricted sample answers, and transcripts not allowed by the existing disclosure policy. Source answer markings must not remain in student content.

## 7. Processing architecture

### 7.1 Responsibilities

| Component | Responsibility |
| --- | --- |
| Upload gateway | Authenticate, enforce limits, validate declared input, and create source records. |
| Document normalizer | Safely convert legacy inputs and produce renderable/parseable artifacts. |
| Document extractor | Extract ordered blocks, marks, numbering, tables, assets, and stable source locators. |
| Recognition orchestrator | Select profiles, deterministic parsers, and AI-assisted recognition stages. |
| Exam recognizer | Identify sections, groups, questions, options, explicit keys, and scoring with evidence. |
| Answer reconciler | Match same-file and companion-file keys, detecting conflicts and ambiguity. |
| Semantic validator | Check coverage, structure, fidelity, domain compatibility, and renderability. |
| Review application | Persist edits, issue resolutions, revisions, and confirmation history. |
| Import committer | Materialize the approved assessment in one coordinated database transaction. |
| Job runner | Execute persistent bounded work with leases, retry policy, cancellation, and recovery. |

Go retains authorization, orchestration, validation, and commit responsibilities in the modular monolith. Conversion and extraction can run as isolated processes or workers behind ports. An additional runtime is justified only by measured fidelity and operability; no microservice or generic plugin framework is required by default.

### 7.2 Document extraction

Read actual document structure, including automatic numbering and inherited run styles. Preserve original labels separately from normalized identifiers. Record text boxes, revisions, and unsupported objects even when they cannot be mapped.

Apply limits to compressed size, expanded size, archive entries, XML complexity, image dimensions, conversion duration, memory, and process concurrency. Disable external relationships and network access in converters. Validate embedded asset types and keep untrusted objects out of the application renderer.

The extractor returns warnings and partial findings, never an unexplained success with missing blocks. Extraction failure is distinct from semantic ambiguity.

### 7.3 Hybrid recognition

Use deterministic rules for explicit template conventions and recognizable structures. Use AI for free-form structural interpretation where it adds measured value. Both produce the same versioned candidate schema and evidence model.

Chunk by section/group boundaries with shared context. Maintain a document-wide identity map and a separate reconciliation pass for answer keys, restarted numbering, and cross-references. Do not split a passage from its question group merely to meet a token limit.

AI output must be schema-constrained, validated against allowed question types, and independently checked against source content. Treat document text as data, not instructions. Recognition has no authority or tools to write directly to LMS entities.

Pin and record model/provider, prompt, schema, parser profile, and extractor versions. Bound tokens, elapsed time, retries, and cost. Provider failure must produce a recoverable job state, not an empty successful draft.

### 7.4 Extraction is not answer generation

The recognizer must not solve unanswered questions, invent missing options, rewrite passages to make them easier, or silently correct source errors. Source contradictions and suspected typos become findings. A teacher may explicitly correct them and the revision records that change.

## 8. Import data model

The following are conceptual contracts, not final SQL definitions. DDL must follow repository-specific database review requirements.

| Model | Required information |
| --- | --- |
| `Import` | Owner/access scope, status, current source set, current review revision, timestamps, resulting test ID. |
| `ImportSource` | Role (exam/key), immutable object key, filename, detected type, checksum, size, original/converted relationship. |
| `ProcessingRun` | Run ID, source revision, stages, worker claim, attempt count, component versions, cost/duration, outcome. |
| `DocumentBlock` | Stable block ID within its source revision, kind, order, content, marks, structural metadata, source locator, asset references. |
| `RecognitionCandidate` | Parsed assessment graph, evidence, findings, and producing run/version. |
| `ImportDraft` | Schema version, revision, assessment graph, asset bindings, findings, exclusions, teacher edits, confirmations. |
| `AnswerEvidence` | Known/unknown state, typed key, origin, question association, source locators, reconciliation outcome. |
| `ImportIssue` | Stable code, severity, affected fields/entities, evidence, resolution, resolver and revision. |
| `CommitRecord` | Import ID, approved revision, idempotency identity, content digest, created test ID, outcome. |

Use local stable IDs before commit. Printed labels and bank IDs are separate fields. Reordering options must not break answer references; reprocessing must not confuse source identity with persistence identity.

Field provenance distinguishes `source_explicit`, `inferred_structure`, `defaulted`, and `teacher_entered`. An unknown key is not a collection of false `isCorrect` flags. Missing values and known empty values must remain distinguishable.

Retain both the extracted source view and the reviewed representation so teacher corrections can be explained. Changes to a source set create a new revision and invalidate dependent processing results.

## 9. Validation and issue policy

Validation runs after extraction, recognition, teacher edits, and immediately before commit. Reuse existing domain rules and add constraints required for generated inputs; HTTP request validation alone does not protect internal calls.

| Check | Required result |
| --- | --- |
| Source coverage | Every meaningful block has an assignment or explicit exclusion. |
| Question boundaries | No unexplained missing, duplicate, or merged questions. |
| Choice structure | Valid option count/content and answer cardinality for the target type. |
| Blank structure | Stable slots, prompt/gap correspondence, accepted answers, and case policy. |
| Key matching | No dangling labels, conflicting keys, or ambiguous cross-section assignments. |
| Formatting fidelity | All meaning-bearing marks survive final rendering. |
| Shared context | Group membership, gap links, instructions, and stimuli are consistent. |
| Scoring | Positive valid points, explicit defaults, and total consistent with included items. |
| Assets | Required assets are present, usable, accessible, and correctly placed. |
| Learner delivery | Snapshot, rendering, grading, disclosure, and shuffle policies can represent the assessment. |

Typical issue codes include `MISSING_ANSWER`, `CONFLICTING_ANSWER_KEYS`, `AMBIGUOUS_ANSWER_MAPPING`, `UNASSIGNED_SOURCE_BLOCK`, `UNSUPPORTED_INTERACTION`, `SEMANTIC_FORMATTING_LOSS`, `BROKEN_GAP_REFERENCE`, `AMBIGUOUS_GROUP`, `MISSING_REQUIRED_MEDIA`, `SCORING_DEFAULTED`, and `UNSUPPORTED_DOCUMENT_OBJECT`.

Severity is `blocking`, `review_required`, or `informational`. Resolution must be a concrete edit, evidence-backed mapping, confirmed default, or explicit exclusion. A numeric model confidence score cannot override a blocking invariant.

## 10. API contract

Proposed operations must be finalized in `api/openapi.yaml`, with generated clients and handlers updated through the normal tooling. Use teacher-only routes and enforce resource-level access on every operation.

| Operation | Purpose |
| --- | --- |
| `POST /admin/imports` | Create import and source uploads; return persistent identity and upload/processing state. |
| `GET /admin/imports` | Paginated import history with supported filters. |
| `GET /admin/imports/{id}` | Status, stage, source summary, progress, errors, and resulting test. |
| `POST /admin/imports/{id}/sources` | Add/replace exam or companion key as a new source revision. |
| `POST /admin/imports/{id}/process` | Start recognition for a selected source revision and profile. |
| `GET /admin/imports/{id}/source-view` | Authorized source rendition and semantic blocks. |
| `GET /admin/imports/{id}/draft` | Current review model and revision. |
| `PUT /admin/imports/{id}/draft` | Save a bounded review model with expected revision. |
| `POST /admin/imports/{id}/validate` | Validate the selected revision and return structured findings. |
| `POST /admin/imports/{id}/preview` | Produce the learner-safe preview for a revision. |
| `POST /admin/imports/{id}/retry` | Retry an eligible failed run under defined policy. |
| `POST /admin/imports/{id}/cancel` | Cancel work or abandon an uncommitted import. |
| `POST /admin/imports/{id}/commit` | Commit the approved revision idempotently. |
| `DELETE /admin/imports/{id}` | Apply authorized retention/deletion semantics without deleting an already created test implicitly. |

Long-running operations return `202` and a pollable identity. Define request limits, revision/ETag semantics, error codes, and retry guidance. Use `409` for state/revision conflicts and a documented validation response for blocking content issues. Polling is sufficient unless measured requirements justify push updates.

All modifying operations record the actor. Source downloads and preview assets must use the same access scope as the import; identifiers are not authorization tokens.

## 11. Durable lifecycle and concurrency

Persist lifecycle states such as `awaiting_sources`, `queued`, `processing`, `needs_review`, `committing`, `committed`, `failed`, and `cancelled`. Track detailed processing stages independently of lifecycle status. Technical completion still enters review; it does not imply teacher approval.

Workers acquire exclusive leases with expiry and fencing/version checks. Heartbeats extend active claims. After a worker loses its claim, its late result must be rejected. Recovery finds expired work and applies bounded retry policy.

Distinguish transient provider/storage failures from invalid documents and unsupported content. Respect cancellation at safe boundaries. Stage outputs are immutable and reusable only when their source and component versions match.

Reprocessing creates a candidate revision. Provide a compare/apply operation for reviewed imports, preserving manual changes or explicitly reporting conflicts. Never replace a teacher-edited draft with fresh model output automatically.

Use optimistic concurrency for review and a guarded transition for commit. A successful cancel cannot race into a commit, and a cancelled run cannot later overwrite the import status.

## 12. Atomic commit and immutable publication

Commit performs the following as one coherent application use case:

1. Authorize the caller and claim the approved import revision.
2. Resolve the idempotency identity; return the existing result for the same successful request.
3. Verify the current revision and content digest match what the teacher confirmed.
4. Revalidate all content, answer, media, group, scoring, and compatibility rules.
5. Create the test, bank content, stimuli/groups, options/blanks, memberships, audit records, and commit record in one database transaction.
6. Mark the import committed and store the resulting test ID in that transaction.

Use transaction-aware application ports and composition adapters, preserving module boundaries. Sequential HTTP calls to existing create endpoints are not an acceptable substitute. Honor reference locks for draft and published media use.

Do not parse documents, call AI, or upload objects inside the transaction. Prepare immutable media first; garbage-collect abandoned unreferenced objects according to retention policy. Object storage operations are not covered by the database transaction.

A connection failure after commit must be safely recoverable by querying or replaying the commit identity. Intentional reuse of a file creates a new import identity, so checksum deduplication does not prevent a teacher from creating another exam.

Publish through the existing test workflow, extended for the complete content graph. Existing published versions and attempts must remain unchanged after subsequent edits, migrations, or parser upgrades.

## 13. Security and data handling

- Enforce teacher authorization and resource-level access on imports, runs, originals, previews, drafts, answers, and assets.
- Keep originals and keys private. Source documents may themselves contain answers and are never learner assets.
- Detect type from content; validate archives and images; bound expansion, object count, XML depth, and execution resources.
- Isolate converters with minimal permissions, no external network resolution, controlled temporary directories, and process timeouts.
- Disable macros and active content. Prevent path traversal, external entity/resource fetching, and unsafe HTML/URL rendering.
- Treat all AI output as untrusted candidate data. Document instructions cannot alter system permissions or initiate LMS writes.
- Make provider use visible in product configuration and define exactly which content may be sent. Select retention and regional handling based on deployment requirements before enabling the provider.
- Redact exam text, answers, transcripts, credentials, and signed URLs from ordinary logs. Record identifiers, durations, counts, and issue codes instead.
- Define retention for originals, converted artifacts, processing candidates, abandoned drafts, and committed-import evidence. Deletion must respect references and existing audit requirements.
- Provide quotas and concurrency limits per actor/access scope and globally to prevent resource exhaustion and uncontrolled provider cost.

## 14. Operations, performance, and support

### 14.1 Required observability

Correlate import, source revision, processing run, draft revision, and resulting test IDs. Record queue wait, stage duration, retries, lease recovery, extraction coverage, issue counts, AI usage/cost, save conflicts, and commit outcomes.

Dashboards and alerts cover queue backlog, repeated stage failures, exhausted retries, stalled jobs, provider degradation, conversion crashes, storage cleanup failure, and abnormal cost growth. Support tooling must show useful processing history without exposing unnecessary exam content.

### 14.2 Capacity and responsiveness

Define and enforce a supported envelope: compressed/expanded document size, pages, questions, assets, concurrent imports, and provider budget. Test maximum supported inputs and concurrent review sessions.

Set measurable service objectives for processing latency, review autosave latency, commit latency, and recovery time using representative workloads. These numeric values must be approved and load-tested before release; they cannot remain undefined at launch. No numerical performance claim is asserted by this unbenchmarked specification.

Large documents must not block API request workers. The review UI must progressively load or virtualize large source/result views while preserving source navigation and edit integrity.

### 14.3 Required runbooks

Provide operational procedures for converter failures, provider outages, queue recovery, stale leases, suspected answer leakage, storage failure, interrupted commits, orphan cleanup, and restoring metadata/object consistency from backups.

A provider kill switch stops new assisted runs without destroying pending imports. Parser/model rollback preserves existing draft revisions and published exams. Deployments drain or safely relinquish active jobs.

## 15. Quality measurement and release gates

### 15.1 Evaluation corpus

Build a versioned, appropriately authorized corpus of real teacher documents and verified expected outputs. Begin collection with representative samples, then expand until every supported input/content class and known edge case is covered. A small fixed sample count is not evidence of production readiness.

Include automatic numbering, repeated numbering, option tables, same-line options, multiple columns, passages, cloze, pronunciation marks, shared listening, images, companion answer files, contradictory keys, malformed inputs, and documents without answers.

Maintain a holdout set independent of prompt/rule tuning. Compare changes by document family so improvements to common files do not hide regressions on difficult ones. Use teacher-reviewed ground truth and explicit adjudication for ambiguous originals.

### 15.2 Metrics

Measure question omission/duplication, boundary accuracy, answer association, shared-material grouping, semantic formatting preservation, source coverage, unsupported-object detection, scoring correctness, teacher correction time, and successful completion rate.

Track defects that escape review separately from findings surfaced correctly. Report confidence intervals and corpus composition when publishing aggregate accuracy. Model-reported confidence is not an accuracy measurement.

### 15.3 Blocking acceptance criteria

| ID | Required evidence |
| --- | --- |
| AC-01 | Supported `.docx` and `.doc` families complete the full import-to-test workflow; conversion losses are surfaced. |
| AC-02 | Missing answers remain unknown, explicit keys are preserved, and ambiguous/conflicting keys block completion until resolved. |
| AC-03 | Restarted numbering and companion keys reconcile correctly on verified fixtures. |
| AC-04 | Meaning-bearing formatting survives prompt and option editing, preview, snapshot, and learner rendering. |
| AC-05 | Shared passages, cloze references, media groups, ordering, and shuffle constraints remain correct during attempts. |
| AC-06 | All meaningful source content is accounted for; exclusions are explicit and reflected in totals. |
| AC-07 | Required assets are usable and disclosure rules protect keys/transcripts and teacher-only annotations. |
| AC-08 | Saved edits survive reloads; stale writes fail safely; reprocessing cannot silently overwrite manual work. |
| AC-09 | Duplicate commit requests, concurrent commits, and post-commit connection loss produce one coherent resulting test. |
| AC-10 | Injected persistence failures produce no partial assessment and can be retried safely. |
| AC-11 | Worker termination, expired leases, provider failure, cancellation, and deployment restarts preserve recoverability. |
| AC-12 | Unauthorized access to every source/draft/key/preview path is rejected. |
| AC-13 | Old tests and attempts retain their original behavior through content migrations and publication changes. |
| AC-14 | Malformed packages, expansion attacks, external relationships, unsafe markup, and active objects remain contained. |
| AC-15 | Keyboard and screen-reader review flows are usable; Vietnamese and English states are complete. |
| AC-16 | Supported input limits, numeric service objectives, retention rules, and quality thresholds are documented and verified before release. |
| AC-17 | Teacher pilot results demonstrate reduced correction/authoring effort against the manual baseline, with critical escaped defects resolved. |

Release thresholds for probabilistic recognition require measured baselines and product agreement. Transactional integrity, access control, answer non-fabrication, and immutable publication are mandatory invariants, not trade-offs against average recognition accuracy.

### 15.4 Verification strategy

Use golden extraction fixtures, recognizer/reconciler evaluation suites, domain tests, integration tests for transactional and access behavior, and end-to-end tests covering real upload through learner delivery. Add concurrency and fault-injection scenarios for leases, revision conflicts, and commit recovery.

Validate semantic content and relationships rather than unstable generated IDs. Test final learner rendering, not just import preview. Require regression evaluation before changing parser profiles, models, prompts, schemas, or converters.

## 16. Rollout, migration, and compatibility

Use expand-contract migrations for new content, groups, and stimuli. Provide adapters for existing Markdown and existing snapshots. Do not rewrite historic exam meaning or grading keys as an incidental migration.

Feature flags can control rollout exposure and provider use while the complete product is verified. They do not excuse omitted correctness or operational requirements. Pilot with authorized teachers and real workflows before general availability.

Roll back code/provider selection without deleting original files, teacher revisions, committed tests, or immutable published versions. Versioned readers must remain compatible with data written during the rollout window. Define a rollback procedure before enabling new writes.

## 17. Engineering delivery sequence

| Stage | Deliverables | Required exit evidence |
| --- | --- | --- |
| 1 — Discovery and contracts | Source corpus, compatibility matrix, content/group semantics, UI flows, threat model, architecture decisions, OpenAPI design | Coverage of actual teacher workflows; explicit unresolved decisions and owners. |
| 2 — Domain foundation | Structured content, stimuli/groups, ordering policy, backward compatibility, snapshot and learner support | Manual authoring and published delivery support the full target content model. |
| 3 — Document pipeline | Upload, legacy normalization, extraction, source rendition, asset handling, durable jobs | Supported input families preserve required evidence and handle failures safely. |
| 4 — Recognition and reconciliation | Rules, template profiles, free-form AI, companion keys, provenance, semantic validation | Corpus-based quality measurements and explicit uncertainty handling. |
| 5 — Review and commit | Complete review UI, revision control, compare/reprocess, preview, atomic idempotent commit | Teacher workflow, concurrent edits, recovery, and publication tests pass. |
| 6 — Operational validation | Capacity tests, service objectives, retention, security verification, accessibility, runbooks, rollback, pilot | All release gates satisfied before general availability. |

These stages are implementation dependencies, not independent promises to release incomplete behavior. Estimate staffing and duration after sizing the domain migration, extraction fidelity, and source corpus; do not assign arbitrary dates in lieu of that work.

## 18. Decisions to finalize before implementation or release

| Decision | Required basis | Gate |
| --- | --- | --- |
| Canonical content schema and editor | Existing authoring/learner needs, marks, tables, media, compatibility | Before content-domain implementation. |
| Group/Stimulus persistence and ownership | Reuse semantics, bank behavior, snapshot and audio policy | Before domain/API contracts stabilize. |
| Converter and extractor/runtime | Measured fidelity, numbering/style support, licensing, maintenance, isolation cost | Before selecting dependencies. |
| AI provider/model and fallback policy | Corpus results, schema support, data handling, cost, latency, outage behavior | Before enabling assisted processing. |
| Supported document envelope | Actual files and capacity tests | Before release. |
| Retention and deletion policy | Teacher needs, storage cost, reference integrity, deployment requirements | Before release. |
| Quality and service objectives | Ground-truth corpus, manual authoring baseline, load tests | Before release. |

Mammoth is one candidate for DOCX conversion, not a selected production solution. The previously reviewed documentation notes default underline omission, deprecated direct Markdown conversion, and no automatic sanitization. Benchmark any candidate against the complete extraction contract rather than selecting it solely for plain-text conversion convenience.

## 19. Evidence and references

Repository links are pinned to the reviewed baseline:

- [Repository stack and purpose](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/README.md)
- [Agent and architecture conventions](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/AGENTS.md)
- [Product specification](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/docs/quizzivy-spec-v0.3.md)
- [Architecture overview](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/docs/plan/00-overview.md)
- [Question model](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/server/internal/modules/questions/domain/question.go)
- [Question validation](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/server/internal/modules/questions/domain/input.go)
- [Draft model](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/server/internal/modules/tests/domain/draft.go)
- [Publish rules](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/server/internal/modules/tests/domain/publish.go)
- [Published content schema](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/migrations/00017_create_test_version_content.sql)
- [Markdown renderer](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/web/src/components/shared/Markdown.tsx)
- [Section instructions renderer](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/web/src/features/take-test/components/SectionInstructions.tsx)
- [Media model](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/server/internal/modules/media/domain/asset.go)
- [Draft update transaction](https://github.com/SekiroKenjii/Quizzivy/blob/4b84de3bf468fcb4b0f477caacbf47071b18f2d3/server/internal/modules/tests/repositories/autosave.go)
- [Mammoth documentation](https://github.com/mwilliamson/mammoth.js#readme); reverify the selected version before implementation.

## 20. Definition of done

The product is done when supported real Word exams can be imported, reviewed, corrected, committed, published, and taken by students with preserved assessment meaning; teachers can recover from failures without losing work; operations can monitor and support the system; and the full release criteria have objective evidence.

This document specifies target behavior and does not claim completed implementation. Implementation starts by reconciling the current repository with this baseline, finalizing the necessary contracts, and planning the work against these production requirements.
