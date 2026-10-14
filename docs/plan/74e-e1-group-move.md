# E1 — Dragging a question into and out of a shared-context group (v0.11.4)

An escalation release. After a teacher escalation, the owner decided on **2026-10-14** that a
teacher can drag a question into a shared-context group in the test builder, drag a member out of
one, move it between two groups of the same test, and reorder members inside a group. The decision
**overrides `74d-d4-deck-update.md` Q8**, whose default was "a bank question cannot be dragged
into a shared group" (DG-186). It ships as **v0.11.4**: v0.11.3 is a separate hotfix (the AI
Generate hotfix) already in flight, so E1 does not touch `web/package.json` until its own release
branch.

The principal engineer's design is the source of this file; its §0 to §11 are the sections and
tasks below, and its §13 is "Decided" at the end. Branches are `feature/t-e1-<nn>-<slug>` off the
integration branch `work/escalation-e1`, which was cut from `develop` at `6f502ab4` (main is at
v0.11.2) and is merged back `--no-ff` when the release is complete. As T-D4.21 shipped in v0.10.1,
E1 runs as its own small release from `develop`: `release/0.11.4` → `main` → back-merge. The
task ids are `T-E1.<n>`; a design-gap number, if the web lane needs one, comes from the Tech Lead
(`docs/team/README.md`, T-30).

**Deliverable:** in the builder's outline, a standalone question can be dropped onto a group's
header or between its members and becomes a member copy (the bank question stays); a member can be
dropped between a section's units and becomes an ordinary question of the bank, with the id it
had; a member can be dropped into another group of the same test or elsewhere in its own group;
each drag has a menu equivalent and a keyboard path. One new operation,
`POST /teacher/tests/{id}/questions/move`, does all of it in one transaction, guarded by the test
revision and the revision of every group it touches. No migration, no new permission, no change to
a published version, a deal or an attempt.

**Exit criteria:** the five canaries are green and their files are as they were at `v0.11.2`;
`moveTestQuestion` is the only operation added, `permissions.golden` changes in T-E1.1's pull
request and nowhere else, and the operation has its isolation-suite entry, with a kind on every
uuid it takes; the race tests of "Locks and races" pass on PostgreSQL 18, in both orders; a draft
moved by v0.11.4 is read unchanged by `v0.11.3` (an old pod answers the new route 404, and every
row the new binary writes is a row the old one already reads); the builder matches the deck at
360, 768, 1024, 1280 and 1440, light and dark, with every difference left a row in
`docs/design/gaps.md`; every CI step of `70-redesign-overview.md` §8 is green, `pnpm e2e:live`
included; released as `v0.11.4`.

**Depends on:** v0.11.2 on `main` and `develop` at `6f502ab4`. E1 builds on D4's builder
(T-D4.2a, T-D4.14: the group row, the Shared content dialog and `useGroupEditor`), on R2's
isolation suite and `x-resource` kinds, and on the two-sided locks of `20-data-model.md` §19. It
has no dependency on v0.11.3.

## The problem and what binds it

A section holds ordered units: standalone questions (bank rows, referenced by
`test_section_questions` and `test_section_units.question_id`) and shared-context groups
(`test_section_units.group_id`, `question_groups.owner_section_id` = the section). A group's
members are rows of `app.questions` the group owns exclusively:

- `context_group_id`, `context_ordinal` and `context_option_order` are one tuple
  (`00035`, `questions_context_complete`);
- a member is hidden from the bank (`questions_owner_bank_idx` is partial, on
  `context_group_id IS NULL`);
- it cannot be soft-deleted alone, and it is hard-deleted when its group drops it
  (`group_update.go`, `group_lifecycle.go`);
- a material's gaps reach it through `group_gap_bindings_member`, a deferred foreign key on
  `(context_group_id, id)` (`00037`).

Nothing moves a question across that boundary today. The outline writer (`mixed_outline.go`)
moves whole groups only; `updateQuestionGroup` replaces a graph without changing its owner;
`GroupBundle.Copy` gives every copied identity a fresh id.

What binds the design:

- the two-sided locks of AGENTS.md: `questions.LockForDraftUse` before any insert into
  `test_section_questions`, the media locks before a member insert, and `20-data-model.md` §19's
  lock order, test → group → members → media;
- the publish snapshot (`publish_snapshot_test.go`, a canary);
- "another teacher's id answers as a missing one", the isolation suite, and a kind on every uuid;
- Vietnamese first.

File names below are the design's reading of `develop` at `6f502ab4`; they name the code, and the
line numbers of that reading are left out because they drift.

## Decisions taken in this file

- **Copy on the way in, the same id on the way out, in place between groups.** Q1, Q3 and Q5
  below, with the reasons under "Identity".
- **One operation, not two.** Every case is "this question, to that place", and the three cases
  share the locks, the revisions and the response.
- **Scope is one test's draft.** A bank group is neither source nor destination.
- **A gap-bound member refuses to leave.** Auto-unbinding would edit the material's document on a
  drag, which is the content editor's job (T-D4.14, Q10).
- **The move is not optimistic.** The server decides the copy's id and both revisions.
- **No migration.** Every table and column exists.

## Identity

**Into a group: a copy.** Dragging a standalone question into a group creates a member copy,
with fresh question, option, blank and gap ids through the same remap `GroupBundle.Copy` does,
and removes the section's reference to the bank question. The bank question is not touched.

- Other drafts may reference the same bank row (`DraftReferences`). A same-id move would turn
  their reference into a member: their next autosave would fail `LockForDraftUse` (it filters
  `context_group_id IS NULL`) with "a question in the test has been deleted", and a later
  "Remove group" would hard-delete a row their `test_section_questions` foreign key still
  restricts (`group_reference`).
- The bank list is defined by `context_group_id IS NULL`. A same-id move would make a bank
  question vanish from the bank without a delete, and the data could not tell a "bank" question
  from a "test-only" one (D-14: a draft references a bank question, it does not own it).
- Published versions and grading history are indifferent: `attempt_answers` references
  `test_version_questions` (`00021`), and `source_question_id` is `ON DELETE SET NULL` (D-07).
  The snapshot canary edits a bank question after publish; a copy leaves that row alone, so the
  canary's premise holds.
- It is the product's rule for a group already: "The builder inserts bank groups as complete
  independent copies" (spec §7.3), "no editable source FK is retained" (`20-data-model.md` §19).

The teacher sees a toast after the drop: "Đã sao chép câu hỏi vào nhóm. Bản gốc vẫn ở ngân hàng
câu hỏi." / "The question was copied into the group. The original stays in the question bank."
There is no undo (an undo would be a second copy). The copy's `optionOrder` is `shuffle`; `fixed`
stays a choice in the group editor.

**Out of a group: the same id.** A member dragged out keeps its id. `context_group_id`,
`context_ordinal` and `context_option_order` are set to NULL in one statement (the check allows
the all-null tuple); the question is inserted into `test_section_units` and
`test_section_questions` at the drop position; the group's remaining members are renumbered under
`questions_context_ordinal_key DEFERRED`. Its `owner_id` is already the test's owner (a section
group's `owner_id` is the test's, and a member inherits the group's), so `NotOwnedBy` on the
next autosave passes. Nothing but its group can reference a member, so there is no second reader
to protect, and a copy would only churn ids. After the move the row is an ordinary bank question
of the teacher: listed, editable, and soft-deletable with the usual reference check.

**Gap bindings.** A member a material gap targets cannot leave. The move answers
`422 VALIDATION_FAILED` with `details.rule` `group_gap_bound` and the `stimulusId`, `gapId` and
`questionId`, and nothing changes. The deferred foreign key would refuse the commit anyway; the
explicit check gives the message. The web knows the bound members from the bundle's
`stimuli[].gaps`, disables their grip with the reason, and treats the server's refusal as the
backstop.

**An emptied group stays.** Empty draft groups are valid and already drawn ("Nhóm chưa có câu
hỏi", `OutlineGroupRow.tsx`); publish refuses them (`group_empty`), as today. The teacher removes
the group with "Remove group" if that is what they meant.

**Between groups: the same id.** A member dragged from group A to group B of the same test
re-parents in place: the same row, `context_group_id` A → B, both groups' ordinals renumbered,
both revisions bumped. The gap rule applies (a bound member does not leave A). Dropping a member
at another position inside its own group goes through the same operation and bumps one revision,
so the outline has one mechanism for a member's grip; the group editor keeps its own member order
control.

**Scope.** The source is a unit of a section of this test or a member of a group a section of
this test owns; the destination is a section or a section-owned group of this test. A bank group
(`owner_section_id IS NULL`) is neither: the builder never shows one, and the bank's group editor
already has member add and remove with the right ownership. Anything else answers `404` (a group
of another test is "missing"). Cross-test reuse stays "copy the group".

## The contract

`POST /teacher/tests/{id}/questions/move`, `operationId: moveTestQuestion`,
`x-permission: content.tests.write` (section-owned groups need exactly this key,
`domain.RequireGroupWrite`; no bank group is reachable), `x-max-body-bytes: 4096`.

`x-resource`: the path `id` is a `test`; `/questionId` a `question`; `/to/sectionId` a `section`;
`/to/groupId` and `/expectedGroupRevisions/-/groupId` a `question-group`.

```
MoveTestQuestionInput  (additionalProperties: false)
  expectedUpdatedAt: Timestamp                     the test revision
  questionId: Uuid                                 a standalone unit's question or a member
  to: MoveTestQuestionTarget
    sectionId: Uuid
    groupId: Uuid | null                           null: a unit of the section; set: a group the section owns
    index: integer, 0 to 32768                     position after the question has left its old place; clamped to the end
  expectedGroupRevisions: [MoveTestQuestionGroupRevision {groupId, revision}]
                                                   every group the move touches, once each; one or two items

MoveTestQuestionResult
  test: Test                                       the saved draft: new updatedAt, sections with units
  questionId: Uuid                                 the id the question has now (new after a copy-in)
  copied: boolean                                  true on a copy-in, so the client shows the toast
  groups: [StoredQuestionGroup]                    every touched group at its new revision, as getQuestionGroup builds one
```

Answers: `200` the result. `400` a body that fails its schema, a move between two standalone
places (that is `updateTest`'s job), a destination group not owned by `to.sectionId`, or an
`expectedGroupRevisions` list that repeats a group or names a reachable group the move does not
touch (the Tech Lead's decision of 2026-10-14: the list is exactly the touched groups). `403` the
key is missing. `404` the test, question, section or group is outside the caller's reach or not of
this test, an entry of `expectedGroupRevisions` included: each is a resource slot, and one the
caller cannot reach answers as a missing one does, before any revision is compared. `409
STALE_WRITE` the test revision or a touched group's revision is behind, or a touched group is
missing from `expectedGroupRevisions`; `409 TEST_ARCHIVED`. `413` over 4 KiB.
`422 VALIDATION_FAILED` with `details.rule`: `group_gap_bound`, or any existing `GroupError` rule
the destination graph fails after the insert (`group_limits` at 200 members or the 4 MiB budget,
`group_audio_scope` when the copied question's audio is the group's shared recording,
`group_asset`), and nothing is written. Errors map through `groupFailure` in
`group_response.go`, with the new rule's message added.

**Concurrency.** The test revision (`updated_at`, `checkVersion`) and each touched group's
`revision`, exactly as `updateQuestionGroup` checks both (`lockGroupMutation`). The response
carries the new values, and the builder goes on from them. There is no idempotency key: a replay
after a lost response meets `STALE_WRITE`, and the builder reloads the test, the path a stale
autosave takes (`version-guard.test.tsx`). **Rate limits:** a bearer operation that mints
nothing, so neither registry. **The `/admin/*` alias:** never for a new operation.

**Isolation suite.** One `moveTestQuestion` entry with a fixture `freshGroupedTest` (publish as
`freshPublishedTest` does, then `createQuestionGroup` into the section with one member), body
`{expectedUpdatedAt, questionId: own question, to: {sectionId, groupId, index: 0},
expectedGroupRevisions: [{groupId, revision: 1}]}`. Every slot substituted with another
teacher's id must answer `404`, as a missing id does (`TestAnotherTeachersIdsAnswerAsMissingOnes`),
and `resource_contract_test.go` sees a kind on each uuid.

## Locks and races

One transaction in `GroupsPostgres` (it already holds the question store and the media locks), in
`20-data-model.md` §19's order, which is `lockGroupMutation`'s:

1. `checkVersion`: `app.tests … FOR UPDATE`, revision and scope; then the archived check. This is
   the parent lock publish (`lockDraftContent` after the test lock), the outline autosave, every
   group write and "Remove group" take first, so the move serialises against all of them.
2. Resolve the source and the destination inside this test, or `ErrNotFound`.
3. Lock the touched groups `FOR UPDATE OF g` in id order (`readLockedGroup(groupExclusive)`), and
   check each revision against `expectedGroupRevisions`.
4. Lock the touched groups' members `FOR UPDATE` in id order (`lockGroupMembers`).
5. Copy-in only: `questions.LockForDraftUse(source)` on the bank row, the lock `SoftDelete`
   contends on; then read its content.
6. Copy-in only: `lockGroupAssets` on the destination bundle with the copied question's media,
   the media side of the two-sided rule. A media soft delete counts owned-member references under
   the asset row lock (§19), so the insert must hold it. `LockForVersionUse` itself belongs to
   publish; this is its draft-side counterpart, which `Create` and `Update` already use.
7. `SET CONSTRAINTS app.questions_context_ordinal_key, app.test_section_units_ordinal_key,
   app.test_section_questions_ordinal_key DEFERRED`.
8. Writes. The gap check comes before any write: the source group's `group_gap_bindings` naming
   the question give `group_gap_bound`.
   - Copy-in: delete the unit and the `test_section_questions` row, compact the section's
     ordinals, `CreateGroupMember` with the remapped input at `index`, renumber the members.
   - Move-out: `UPDATE app.questions SET context_group_id = NULL, context_ordinal = NULL,
     context_option_order = NULL WHERE id = $1 AND context_group_id = $2`; renumber the source
     group; `LockForDraftUse(id)` again (it is a bank row now; the row lock is already held from
     step 4, and this re-asserts "not deleted" before the insert AGENTS.md requires it for);
     insert the unit and the `test_section_questions` row at `index`, renumber.
   - Between groups, or within one: one `UPDATE` of the context tuple; renumber both.
9. `readGraph` on every touched group (it runs `Bundle.Validate`, so the destination's limits,
   audio scope and identity rules hold or the transaction rolls back); `revision = revision + 1`
   on each; `UPDATE app.tests SET updated_at = now`; audit `test.question_moved` (entity `test`)
   and `question_group.updated` per touched group (`recordGroupMutation`). An audit diff uses a
   data-modifying CTE (AGENTS.md).

Against each concurrent writer: a delete of the source bank question (step 5 and
`DraftReferences`: whichever commits first, the other sees a consistent world, 404 for the move
or no reference for the delete); publish (step 1: a version freezes the state before or after,
never between); the outline autosave (step 1 plus its own `lockQuestions`); a group save (steps 1
and 3: one `STALE_WRITE`); "Remove group" (steps 1 and 3); a media delete (step 6).

New integration tests, driven like `draftrace_test.go` and `group_media_race_test.go` (two
goroutines, both orders, a bounded number of attempts):

- `TestMoveIntoGroupSerialisesAgainstBankDelete`: copy-in against `SoftDelete` of the source; one
  of {move 404 and the row deleted, move done and the delete then succeeds}; never a member of a
  deleted question.
- `TestMoveOutSerialisesAgainstGroupSave`: move-out against `updateQuestionGroup` at the same
  revision; exactly one winner, the loser `ErrStaleWrite`.
- `TestMoveSerialisesAgainstOutlineAutosave`: against `updateTest` at the same `expectedUpdatedAt`.
- `TestMoveSerialisesAgainstPublish`: against `publish`; the version's units and members are the
  before-state or the after-state, never a mix.
- `TestMoveInContendsOnTheCopiedQuestionsAsset`: against a media soft delete of the copied
  question's asset; either the delete is refused as referenced or the move fails
  `ErrMediaNotFound`; no member points at a deleted asset.
- `TestMoveOutOfAGapBoundMemberIsRefused`: rule `group_gap_bound`, nothing written.
- `TestMoveInCopiesAndLeavesTheBankQuestion`, `TestMoveOutKeepsTheIdAndReturnsItToTheBank` (listed,
  `Get` answers, `SoftDelete` answers 409 while referenced), `TestMoveBetweenGroupsBumpsBothRevisions`,
  `TestMoveClampsIndexAndCompactsOrdinals`.
- `TestPublishAfterMoveFreezesTheNewUnitsAndOrdinals`, `TestMoveLeavesPublishedVersionsUntouched`
  (every `test_version_*` row byte-equal before and after).

## Migrations: none

Every table and column exists; the move is DML on `questions`, `test_section_units`,
`test_section_questions`, `question_groups` and `audit_log`. No index is missing: the writes hit
`questions_context_ordinal_key`, `test_section_units_ordinal_key` and the primary keys, and the gap
check hits `group_gap_bindings_question_idx`. No grant changes. So there is no numbering conflict
with R5 and nothing to expand or contract. During the rolling deploy an old pod answers the new
route `404 NOT_FOUND`, which the builder shows as its ordinary error toast.

## Publish, deal and attempts

- **Versions are untouched.** The operation writes no `test_version_*` row. Publish reads the live
  draft under the test lock (`publish_load.go`, `publish_groups.go`), and the next publish freezes
  the new structure. `TestEditingTheBankAfterPublishLeavesTheVersionUnchanged` keeps its premise.
  `TestMoveLeavesPublishedVersionsUntouched` pins it.
- **Ordinals.** `loadDraftUnits` renumbers a section's questions in unit order, so the frozen
  `test_version_questions` ordinals are contiguous and the moved question sits where the outline
  put it.
- **The deal.** `DealManager.Present` shuffles units within a section, a group as one unit with its
  members in `GroupOrdinal` order, from the frozen `GroupID` and `GroupOrdinal`. A copied-in
  question deals inside its group at its member ordinal; a moved-out one deals as its own unit.
  `DeliveryGroupV1` already describes this; there is no new delivery version.
- **Attempts.** Answers reference frozen rows only; started attempts keep their version.
- **The version diff** `against=draft` matches by source id, so a copy-in reads as "removed,
  added" and not "moved". Acceptable for the escalation; the spec line says so.

## The web

**Drag.** The outline's `DndContext` stays (`OutlineTree.tsx`). A member row
(`OutlineGroupRow.tsx`) becomes `useSortable({id: "question:" + memberId})` with the grip the deck
draws (DG-211 recorded "ours have none"; the decision reverses that); the member
`SortableContext` is the group's, nested under the section's. A group header is a droppable
(`useDroppable({id: "group:" + id + ":head", data: {groupId}})`), tinted `bg-brand-soft` while
over, as a section header is (`EmptySectionDrop`); a drop on it is `index 0`, which is the deck's
`finishDrop` (it unshifts and opens the group). The drop line stays the 2px primary line before
or after the row under the pointer (`trackDrop`'s midpoint rule), on member rows too.

`dropUnit` resolves `from` as a unit (`findUnit`) or a member (a new `findMember(groups, id)` in
`outlineUnits.ts`), and `to` as a unit position, a group head, or a member position. Standalone
to standalone keeps `onChange(moveUnit)` and the autosave. Any other pair calls a new
`onMove({questionId, to, groups})` prop the page implements. The collision filter keeps sections
apart from units; member rows and group heads join the unit set. A gap-bound member's grip is
`aria-disabled` with the tooltip "Đang gắn với chỗ trống trong ngữ liệu chung" / "Bound to a gap
in the shared content" and is not a drag source.

**Keyboard and menu: one for every drag (R4's rule).** `OutlineKeyboardSensor` walks the
droppables, so the new targets work with Space, arrows, Space. The menus: a standalone row's "…"
gains "Chuyển vào nhóm…" / "Move into group…", a submenu of the test's groups by section (a dialog
above eight); a member row gets a "…" menu like a question's with "Lên" / "Move up", "Xuống" /
"Move down" (inside the group), "Chuyển ra khỏi nhóm" / "Move out of group" (after the group, in
its section) and "Chuyển sang nhóm…" / "Move to group…". Below 768 the selected member row shows
the three at 44px in place of its points, as a question row does (DG-135).

**Not optimistic.** The server decides the copy's id and both revisions, and `useGroupEditor`
holds the active group's revision base. So the page flushes the question editor and
`outline.flush()`, then inside `writes.run` (`BuilderWrites`, as `insertGroup` does in
`TestBuilderPage.tsx`) calls `moveTestQuestion`. On success it sets
`version.current = test.updatedAt`, replaces the outline from the response (`editableOutline`),
writes each returned group to `["teacher-group", id]`, and selects the moved question in its new
place (`selectGroup(groupId, questionId)` or `selectQuestion(id)`), which remounts the pane on the
new revision; `copied` shows the toast. While the request runs the dragged row is at 40% opacity
with `aria-busy` and the outline refuses another drag.

**Errors.** The outline is unchanged on every one.

| Case | vi | en |
|---|---|---|
| `STALE_WRITE` | the existing reload prompt (`version-guard.test.tsx`) | the same |
| `group_gap_bound` | Câu hỏi đang gắn với chỗ trống trong ngữ liệu. Hãy gỡ liên kết trong Ngữ liệu chung trước. | The question is bound to a gap in the material. Unbind it in Shared content first. |
| `group_limits` | Nhóm đã đủ 200 câu hỏi | The group already has 200 questions |
| `group_audio_scope` | Câu hỏi dùng chính tệp ghi âm chung của nhóm | The question uses the group's own shared recording |
| `404` | Đề đã thay đổi. Tải lại để tiếp tục. | The test has changed. Reload to continue. |
| network | Chưa chuyển được. Thử lại. | Could not move. Try again. |

**Strings** (vi, then en, under `builder.*`): `moveIntoGroup`, `moveToGroup`, `moveOutOfGroup`,
`memberMoveUp`, `memberMoveDown`, `movedIntoGroupCopied`, `moveGapBound`, `moveGroupFull`,
`moveAudioScope`, `moveReload`, `moveFailed`, `moving` ("Đang chuyển…" / "Moving…"),
`reorderMember` (aria-label "Kéo câu {number}" / "Drag question {number}"), `groupDropHint`
("Thả vào nhóm" / "Drop into the group", on the tinted header for screen readers). The design
gave the Vietnamese alone for the gap-bound tooltip, `reorderMember`, `groupDropHint` and
`moving`; the English lines here are written to match and are the web lane's to confirm.

**Tests.** Units: `builder/group-move.test.tsx` (a drop on a head calls `onMove` with index 0; a
member out before and after a unit; a member to another group; a bound member not draggable; the
menus; the toast; every error), `builder/mixed-outline.test.ts` (`findMember`, target
resolution), `builder/group-row.test.tsx` (grip, menu, 44px below 768), and an MSW handler
validated by ajv against the contract. One live case in `builder-groups.live.spec.ts`: drag a bank
question in, publish, and the preview shows it inside the group's passage; the id remap and both
revision bumps are claims about the two halves meeting. The deck check is at 360, 768, 1024, 1280
and 1440, light and dark.

## Risks and the canaries

- `publish_snapshot_test.go`: untouched; the move writes no version row and never edits the bank
  row it copies. `TestMoveLeavesPublishedVersionsUntouched` adds a direct pin.
- `audio-player.test.tsx`, `client.refresh.test.ts`, `events_test.go`: not on the path.
- `router-chunks.test.ts`: the new code lives in `features/tests/`, already lazy; no entry import.
  The existing test checks it on every PR.
- `TestLockForDraftUseSerialisesAgainstDelete` and `TestLockForVersionUseSerialisesAgainstDelete`
  stay as they are; the race tests above extend them to the move.
- **A member becoming a bank row is new.** Mitigation: it is one `UPDATE` to the all-null tuple the
  check already allows, and the row's `owner_id` is already the test owner's; the tests cover
  list, get, delete and the next autosave.
- **Deferred ordinal constraints across three tables in one transaction.** Mitigation: the existing
  writers already defer the same three (`mixed_outline.go`, `group_update.go`); the renumbering is
  a `row_number()` update as `RemoveFromSection` does.
- **The builder's one-writer rule for the active group (T-D4.14).** Mitigation: the move never
  runs inside the group editor; it runs in `writes.run` after both flushes, and the pane remounts
  on the returned revision.
- **An old tab** with the previous SPA cannot drag into a group and still autosaves the outline; it
  meets `STALE_WRITE` after a move from another tab, as today.
- **The isolation entry's two group slots.** `to.groupId` and `expectedGroupRevisions[].groupId`
  name the same group in a real request, and the suite substitutes one slot at a time. Its `build`
  (`isolation_runner_test.go`) applies a `link` after setting the keyed slot and then lets every
  later slot overwrite its own pointer, so linking from `/expectedGroupRevisions/-/groupId` to
  `/to/groupId` loses the substitution of `/to/groupId`, and linking the other way leaves the pair
  unmirrored. Either way a request carries a foreign group in one slot and the caller's own in the
  other. So an `expectedGroupRevisions` entry is a resource slot like any other: a group the
  caller cannot reach, or that is not of this test, answers 404 as a missing one does, before any
  revision is compared, and a reachable entry the move does not touch is a 400 (decided by the
  Tech Lead, 2026-10-14). T-E1.2 builds that order; T-E1.3 proves it by running the suite.

## A. Contract and server

### T-E1.1 — Contract: `moveTestQuestion`
**Depends on:** none (the first task; everything else waits on it)
**Touches:** `api/openapi.yaml`, `server/gen/openapi/`, `web/src/lib/api/schema.d.ts`,
`server/internal/core/router/tests/testdata/permissions.golden`,
`server/internal/core/router/tests/{permissions_test,group_authoring_test}.go` (the pinned counts),
`server/internal/modules/tests/http/groups.go`, `server/tests/isolation_cases_test.go`
**Size:** S
**Done when:**
- [x] `POST /teacher/tests/{id}/questions/move` (`moveTestQuestion`) is in `api/openapi.yaml` with
      `x-permission: content.tests.write`, the five `x-resource` entries above and
      `x-max-body-bytes: 4096`; `MoveTestQuestionInput`, `MoveTestQuestionTarget`,
      `MoveTestQuestionGroupRevision` and `MoveTestQuestionResult` carry every constraint the
      schema can: closed objects, `expectedGroupRevisions` one to two items, `to.index` 0 to
      32768, `revision` at least 1.
- [x] The answers `200`, `400`, `403`, `404`, `409`, `413` and `422` are listed, and the `422`
      names `details.rule` `group_gap_bound` with the `stimulusId`, `gapId` and `questionId`.
- [x] `make gen` runs clean; the regenerated `server/gen/openapi/` and
      `web/src/lib/api/schema.d.ts` are committed on their own, and `make gen-check` shows no drift.
- [x] The server builds: `Tests.MoveTestQuestion` answers `501` until T-E1.3.
- [x] `permissions.golden` gains the one row and nothing else; the pinned counts in
      `TestEveryDeclaredValueIsKnown` (138 → 139 operations) and
      `TestGroupBodyBudgetIsBoundedBeforeJSONValidation` (12 → 13 body budgets, and the new one
      asserted) follow the contract.
- [x] `TestAnotherTeachersIdsAnswerAsMissingOnes` accepts the operation: without an entry it fails
      "the isolation table lacks [moveTestQuestion]", so the table has the smallest entry the
      suite accepts, with every slot `blind` for the stub. T-E1.3 replaces it.
- [x] `go build ./...`, `go vet`, `staticcheck`, the router and contract tests, and
      `pnpm typecheck` pass.
- [ ] **CI result** passes on the pull request.

**As built:** The contract is as above, with these differences from the design's sketch: the
nested objects are named schemas (`MoveTestQuestionTarget`, `MoveTestQuestionGroupRevision`) so
that the generated Go and TypeScript have no anonymous structs; `expectedGroupRevisions` has
`minItems: 1`, because a move that touches no group is a move between two standalone places, which
is `400`; `to.groupId` is required and nullable, so a client says "a unit of the section" with an
explicit null; `to.index` is capped at 32768, the units array's own cap; and its meaning is fixed
as the position after the question has left its old place, the convention of the web's `moveUnit`
(it removes, then splices), which is also the order of the writes in "Locks and races", step 8.
The stub is `func (Tests) MoveTestQuestion(...)` returning `httpx.ErrNotImplemented`, which the
router renders as `501` with the envelope's `INTERNAL` code ("This feature has not been built
yet."), as every unbuilt operation. The isolation entry uses `freshPublishedTest`, a schema-valid
body, and `blind` on the five slots, because a stub answers 501 to a foreign, a missing and the
caller's own id alike.

### T-E1.2 — Domain and repository: `MoveQuestion`
**Depends on:** T-E1.1
**Touches:** `server/internal/modules/tests/{domain,application/command,application/model,repositories}/`,
`server/internal/core/adapters/group_questions.go` (the port gains `BankQuestion`),
`server/internal/modules/tests/domain/tests/group_member_copy_test.go` (new),
`server/internal/modules/tests/application/tests/group_move_*_test.go` (new; the group tests'
fixtures live in that package)
**Size:** L. High risk: the test builder's draft tables, `LockForDraftUse`, the publish snapshot.
**Done when:**
- [x] `tests/domain` has `MoveQuestionInput` and its result, `GroupRepository.MoveQuestion`, and
      the `group_gap_bound` rule with its `stimulusId`, `gapId` and `questionId`; the repository
      takes `access.Scope` and never calls `Own()`. (`access.CanActOn` is the subset rule of role
      and user writes; a move touches neither, so it has no call to make.)
- [x] `repositories/group_move*.go` follows "Locks and races", steps 1 to 9, for the three cases:
      copy-in, move-out, and between or within groups. `LockForDraftUse` is called before the
      insert into `test_section_questions` on move-out and before reading the bank row on
      copy-in; `lockGroupAssets` before the member insert on copy-in.
- [x] The copy's identities are fresh through the same remap `GroupBundle.Copy` does
      (`GroupQuestion.Copy`); its `optionOrder` is `shuffle`; the bank row is never written.
- [x] A move-out inserts the unit and the `test_section_questions` row at `index`, compacts the
      source group, and leaves the group in place when it empties.
- [x] `index` is clamped to the end; ordinals are contiguous after every case.
- [x] A gap-bound member is refused before any write.
- [x] The audit rows are written in the transaction, and `question_groups.revision` and
      `app.tests.updated_at` move once.
- [x] `command/move_question.go` wires it through `application.New`; an unwired command leaves the
      handler at 501.
- [x] Every test named in "Locks and races" passes against PostgreSQL 18, in both orders and with
      a bounded number of attempts.
- [x] The canaries `publish_snapshot_test.go`, `TestLockForDraftUseSerialisesAgainstDelete`,
      `TestLockForVersionUseSerialisesAgainstDelete`, `TestTheSameClientSeqFromTwoSessionsBothPersist`,
      `audio-player.test.tsx`, `client.refresh.test.ts` and `router-chunks.test.ts` pass before and
      after.
- [x] `make lint`'s three Go tools; the unit tests of every package touched; the module's
      integration tests.
- [ ] **CI result** passes on the pull request.

**As built:** `GroupsPostgres.MoveQuestion` is `group_move.go` (the flow and the three writers),
`group_move_plan.go` (what the request names, resolved inside the test) and `group_move_units.go`
(the section and member ordinals). It takes the locks in the design's order: `checkVersion` on the
test, then the archived check and the key; then the source, the destination and every revision the
request lists are resolved inside this test, so a foreign or missing id answers `ErrNotFound`
before any revision is compared; then the touched groups `FOR UPDATE` and their members, each in
id order; then, on a copy-in, the bank row (`LockForDraftUse`), its content (the port gains
`BankQuestion`) and its media (`lockGroupAssets`); the three ordinal constraints are deferred for
the writes and made immediate before the read-back, so a violation surfaces inside the move.
`readGraph` validates every touched group, which is where `group_limits` (200 members),
`group_audio_scope` and `group_asset` refuse a destination; the member count is checked first so
a full group answers `group_limits` before its `CHECK` on the ordinal can fire.

What the design left open, as built. The request's refusals are `GroupError` rules, so
`groupFailure` already carries them: `move_outline_only` (a standalone question to a standalone
place), `move_group_section` (the destination group is not the section's), `move_revisions` (the
list repeats a group or names one the move does not touch), and `move_source_ambiguous` (a
standalone question held by two sections of the test, which the contract cannot tell apart). A
touched group missing from the list is `ErrStaleWrite`, as the contract's `409` says. A section the
move touches that has no `test_section_units` yet (a legacy outline) is seeded first, as mounting
a group seeds it, so the unit and the standalone ordinals can be rewritten together;
`insertStandaloneQuestion` and `removeStandaloneQuestion` then rewrite the units and recompute the
`test_section_questions` ordinals from them. A move-out checks `NotOwnedBy` against the test's
owner, as the outline's `lockQuestions` does. The audit diff of `test.question_moved` holds facts
the transaction already knows under its locks (the ids, the sections, the index), so it needs no
data-modifying CTE; the groups' `question_group.updated` rows carry none.

Tests (`application/tests/group_move_*_test.go`, integration): `TestMoveInCopiesAndLeavesTheBankQuestion`,
`TestMoveOutKeepsTheIdAndReturnsItToTheBank` (which also saves the outline afterwards),
`TestMoveBetweenGroupsBumpsBothRevisions`, `TestMoveClampsIndexAndCompactsOrdinals`,
`TestMoveOutOfAGapBoundMemberIsRefused`, `TestMoveSeedsALegacySectionItTouches`,
`TestMoveByAnAdminKeepsTheTeachersOwnership`, `TestMoveRefusalsWriteNothing` (eighteen refusals,
each checked against an unchanged draft), `TestMoveRefusesAnArchivedTest`,
`TestMoveRefusesAFullGroupAndStillReordersIt`, `TestMoveRefusesAQuestionWhoseAudioIsTheGroupsSharedRecording`,
`TestMoveLeavesPublishedVersionsUntouched` (all thirteen `test_version_*` tables, row by row),
`TestPublishAfterMoveFreezesTheNewUnitsAndOrdinals`; and the races, which pause one writer inside
its lock and watch `pg_blocking_pids` for the other: `TestMoveIntoGroupSerialisesAgainstBankDelete`
(the move holds the bank row; a deletion commits first), `TestMoveOutSerialisesAgainstGroupSave`
(and six free-running attempts with exactly one winner), `TestMoveSerialisesAgainstOutlineAutosave`
and `TestMoveSerialisesAgainstPublish` (each holder first, both ways),
`TestMoveInContendsOnTheCopiedQuestionsAsset` (the media delete waits, then is refused as
referenced; an asset deleted beforehand fails the move). The deletion that commits first is a
direct `UPDATE`: a real `SoftDelete` is refused while the draft references the question, so the
only deletion a copy-in can meet is one that raced past the reference check. The domain's unit
tests cover `GroupQuestion.Copy`, and `TestMoveQuestionWithoutGroupAuthoringIsUnavailable` the 501.
`wiring/tests.go` needed no change: `application.New` builds the handler and `WithGroups` gives it
the repository. The integration tests ran on a database created for the purpose
(`qa_e1_move`, migrated to version 125), never the dev one.

After the independent review (2026-10-14), three changes. **The copy-in's lock order.** A section
with no `test_section_units` is seeded by `seedSectionUnits`, which takes `LockForDraftUse` on every
question of the section in id order. The first build locked the moved question Q and then seeded, so
the move held Q and waited on an earlier question R, while another test that shares R and Q takes
them in id order (autosave's `lockQuestions`, publish, duplicate): a `40P01` deadlock, which
`groupWriteError` does not map, so a teacher would have met a 500. `copyIntoGroup` now seeds the
source section first and locks Q after, so every writer takes questions in ascending order.
`TestMoveIntoGroupLocksQuestionsInIdOrderAgainstAnotherTestsAutosave` holds one writer inside its lock
while the other waits, in both orders (an autosave holds the earlier question; the move holds the
later one), and requires both to finish; against the first build it deadlocked every time (six runs of
six, `40P01` on the move in one order and on the autosave in the other), and with the fix it passes
four runs in a row. **The owner of a member that leaves.** The `UPDATE` that turns the member into a
bank question now names the owner, the test's, and the `NotOwnedBy` refusal is gone: a member whose
`owner_id` predates ownership (backfilled from `created_by` by 00061 and 00062) no longer answers 404
for a member the caller can see, and comes out owned by the test's owner
(`TestMoveOutNamesTheTestsOwnerWhoseverRowTheMemberWas`). **The deal.**
`TestMovedQuestionsDealInsideAndOutsideTheirGroups` publishes after a copy-in and a move-out, reads
the version through the attempts store and deals it with `PresentVersion(DeliveryGroupV1, …)` for
thirty seeds: the copied member is in its group's run in member order, the question that left deals as
its own unit, and the shuffle moves the units. The race fixtures' author is a Teacher, not an Admin, so
their cleanup is not stopped by the last-Admin guard on a database without a seeded Admin.

### T-E1.3 — HTTP, errors, isolation
**Depends on:** T-E1.2
**Touches:** `server/internal/modules/tests/http/{groups.go,group_response.go}`,
`server/tests/isolation_cases_test.go`
**Size:** M
**Done when:**
- [ ] `MoveTestQuestion` reads the body, builds the command from the principal and
      `ScopeFromContext`, and answers the result; it answers 501 only when the command is unwired.
- [ ] `groupFailure` maps `group_gap_bound` and the existing destination rules to `422
      VALIDATION_FAILED` with `details.rule`, and the three `400` cases to `400`; every message is
      written vi first and en second through `httpx.Text`.
- [ ] A foreign or missing test, question, section or group answers 404, and so does a group in
      `expectedGroupRevisions` the caller cannot reach or that is not of this test, before any
      revision is compared (see "Risks"); a reachable entry the move does not touch, or a repeated
      one, is a `400` (`move_revisions`), and the contract's description of the `400` says so.
- [ ] The isolation table's `moveTestQuestion` entry is the design's: `freshGroupedTest`, the body
      above, T-E1.1's `blind` marks removed, and any `link` checked against how `build` applies it
      (see "Risks"), so that each of the five slots' substitution reaches the server;
      `TestAnotherTeachersIdsAnswerAsMissingOnes` passes with the provider-call count unchanged.
- [ ] A student payload never carries `isCorrect`, `sampleAnswer`, `acceptedAnswers` or
      `transcript`; the move's answer is teacher-only and is read by the payload walk as such.
- [ ] `make lint`, the router and contract tests, the module's tests, and `go test -tags e2e
      ./tests/...` pass.

## B. The web

### T-E1.4 — Outline drag
**Depends on:** T-E1.1 (runs in parallel with T-E1.2 against MSW)
**Touches:** `web/src/features/tests/components/{OutlineTree,OutlineGroupRow}.tsx`,
`web/src/features/tests/{outlineUnits,api}.ts`,
`web/src/features/tests/pages/teacher/TestBuilderPage.tsx`, `web/tests/support/` (MSW),
`web/tests/units/builder/{group-move.test.tsx (new),mixed-outline.test.ts,group-row.test.tsx}`
**Size:** L. High risk: `features/tests/` builder, autosave flushes.
**Done when:**
- [ ] A member row is a sortable with the deck's grip; a group header is a droppable tinted while
      over; the drop line shows on member rows.
- [ ] `findMember` resolves a member; `dropUnit` calls `onMove` for every pair that is not
      standalone to standalone, and `onChange(moveUnit)` stays for that pair.
- [ ] `TestBuilderPage.moveQuestion` flushes the question editor and `outline.flush()`, runs the
      call in `writes.run`, replaces the outline and the groups from the response, selects the
      moved question, and sets `version.current`.
- [ ] While the request runs the dragged row is at 40% opacity with `aria-busy` and the outline
      refuses another drag; the outline is unchanged on every error.
- [ ] A gap-bound member's grip is `aria-disabled` and not a drag source.
- [ ] The MSW handler is validated by ajv against the contract; unit cases as in "The web".
- [ ] `pnpm typecheck`, `pnpm lint:changed`, `pnpm format:changed`, `pnpm test:changed`,
      `pnpm test:integration`; `autosave-unmount.test.tsx` and `router-chunks.test.ts` pass.

### T-E1.5 — Menus, keyboard, strings, error states, the toast
**Depends on:** T-E1.4 (same files)
**Touches:** `OutlineTree.tsx`, `OutlineGroupRow.tsx`, `OutlineKeyboardSensor.ts` (only if a target
needs a coordinate), `web/src/locales/{vi,en}.json`,
`web/tests/units/builder/{group-move,outline-keyboard-activation,question-menu}.test.tsx`
**Size:** M
**Done when:**
- [ ] The standalone row's menu gains "Move into group…", and a member row gets its menu: Move up,
      Move down, Move out of group, Move to group…; below 768 the selected member row shows them at
      44px (DG-135).
- [ ] Every drag has a keyboard path through `OutlineKeyboardSensor` (Space, arrows, Space), and a
      menu equivalent (R4's rule).
- [ ] Every string of "The web" is in `vi.json` and `en.json`, vi first; the toast shows after a
      copy-in and says the original stays; every error state of the table is drawn.
- [ ] `aria-label` and the group header's drop hint are announced; focus lands on the moved
      question's row after the pane remounts.
- [ ] The locale files are Tech-Lead-serialised: the change goes in one commit the lead merges.

## C. Verification and documents

### T-E1.6 — QA
**Depends on:** T-E1.3, T-E1.5
**Touches:** `web/tests/e2e/builder-groups.live.spec.ts`
**Size:** M
**Done when:**
- [ ] One live case: drag a bank question into a group, publish, and the preview shows it inside
      the group's passage; the id remap and both revision bumps are asserted.
- [ ] The builder matches the deck at 360, 768, 1024, 1280 and 1440, light and dark; each
      difference left is a row in `docs/design/gaps.md`.
- [ ] The five canaries pass before and after; `make test-api` and `pnpm e2e` pass; `pnpm
      e2e:live` passes with the new case.

### T-E1.7 — Documents
**Depends on:** with T-E1.3 (server behaviour) and T-E1.5 (the screen); never batched after
**Touches:** the files below
**Size:** S
**Done when:**
- [ ] `docs/quizzivy-spec-v0.3.md`: bumped to v0.107 (the v0.11.3 hotfix takes v0.106) with a
      "Changes since v0.106" entry. §7.3, after "Members belong to one group in one
      section": a standalone question dragged into a section's group becomes a member copy and
      the bank row stays; a member dragged out keeps its id and becomes a bank question; a member
      moves between a test's groups in place; a gap-bound member does not leave; both the test
      revision and each touched group's revision guard it; the next publish freezes the new
      structure and the diff reads a copy-in as removed and added. §8 builder row: the outline's
      member grips, the group header as a drop target and the menu equivalents. §15:
      `POST /teacher/tests/:id/questions/move` with its answers.
- [ ] `docs/plan/20-data-model.md`: a new §48 "Moving a question across the group boundary": no
      schema change; the DML, the lock order, the deferred constraints, and why a copy on the way
      in and the same id on the way out (the §12 register needs no row: no table changes).
- [ ] `docs/design/gaps.md`: DG-186 gains "**Reversed 2026-10-14 (owner):** a bank question can be
      dragged into a shared group and a member out, built in v0.11.4"; DG-211's "ours have none"
      for the member grip is marked built, and the request to the design team to remove the grip is
      withdrawn.
- [ ] `docs/plan/74d-d4-deck-update.md`: Q8 gets "**Overridden 2026-10-14 by the owner, after a
      teacher escalation:** drag in and out is built as v0.11.4, design in `74e-e1-group-move.md`";
      the "Decisions taken in this file" bullet for Q8 and the DG-186 row point to it.
- [ ] `docs/plan/70-redesign-overview.md` §2: a row for the escalation release, as v0.9.1 has one.
- [ ] This file: every task's boxes ticked and its "As built" written in the task's own PR.
- [ ] `AGENTS.md` needs no rule change for the inserts: the two-sided lock paragraphs already cover
      them. Its "Git workflow" paragraph gains the line for `work/escalation-e1` and
      `feature/t-e1-<nn>-<slug>` (the Tech Lead's, decided 2026-10-14).

## Order of work

1. T-E1.1, alone. Everything else waits on it, and `api/openapi.yaml` and the generated files are
   Tech-Lead-serialised (`docs/team/README.md`, "Coordinated changes").
2. Two lanes in parallel once T-E1.1 is merged into `work/escalation-e1`: {T-E1.2 → T-E1.3} and
   {T-E1.4 → T-E1.5}. The web lane builds against MSW; the locale files are serialised.
3. T-E1.7 lands with T-E1.3 (server behaviour) and T-E1.5 (the screen), never after.
4. T-E1.6, after T-E1.3 and T-E1.5.
5. The release: `work/escalation-e1` → `develop` (`--no-ff`), `release/0.11.4`, the checklist
   below.

## Release checklist

**v0.11.4, dragging a question into and out of a group** (decided 2026-10-14)

- [ ] Every task is ticked on `work/escalation-e1`. `develop` is merged into it, and verification
      is re-run after that merge. Then `work/escalation-e1` → `develop` (`--no-ff`), and
      `release/0.11.4` is cut from `develop` at a named commit. v0.11.3 has shipped before it, or
      its fix is already on `develop`.
- [ ] `web/package.json` is bumped to 0.11.4 in the release branch only.
- [ ] Every CI step runs on the release pull request, in order, judged by exit code (70 §8.1):
      `make lint`, `make gen-check`; `make test-api` and goose up/down/up (no migration, so the
      round trip is unchanged); `pnpm lint`, `pnpm typecheck`, `pnpm format:check`;
      `pnpm test:unit`, `pnpm test:integration`, `pnpm build`; `pnpm e2e`; `pnpm e2e:live`,
      required because the builder and the API meet.
- [ ] The five canaries pass, and their files are as they were at `v0.11.2`.
- [ ] Security: `moveTestQuestion` is the only operation added; `permissions.golden` changed once
      (T-E1.1) and holds `content.tests.write` for it; R2's isolation suite passes with the
      operation's entry; no student payload changes.
- [ ] Migrations: none. The deploy is a rolling one; an old pod answers the route 404.
- [ ] The documents of T-E1.7 are merged with the code they describe.

## Open items

**Decided** by the owner, 2026-10-14. These are the design's five questions with the owner's
answers; each had the default shown, and the answers differ from the design's defaults only where
noted.

**Q1. Copy on the way in?** Decided: **copy**, the bank row untouched. The alternative, move the
same id when nothing else references it, is invisible to the teacher and touches the invariant
that a member is born in its group.

**Q2. Version number?** Decided: **v0.11.4**, because v0.11.3 is the Generate hotfix. The design's
default was v0.11.3, as T-D4.21 shipped in a patch (v0.10.1), with R5 keeping v0.12.0.

**Q3. Reorder members by grip inside a group?** Decided: **yes**, through the same operation.
Otherwise members would drag only across the boundary and the group editor would keep member
order.

**Q4. A gap-bound member?** Decided: **refuse**, with guidance. The alternative edits the
material's document on a drag.

**Q5. Between groups of the same test?** Decided: **yes**.

**Raised while building, and answered**

- **The index's meaning.** The design says "position among units, or among members; clamped to
  the end". Built, and agreed by the Tech Lead on 2026-10-14: the position after the question has
  left its old place, as the web's `moveUnit` counts it (T-E1.1). A client that counts before the
  removal is off by one when it moves a question later within the same list.
- **`expectedGroupRevisions`.** Each entry is a resource slot; a group the caller cannot reach, or
  that is not of this test, answers 404 before any revision is compared; a reachable group the
  move does not touch, and a repeated one, are 400; a touched group missing from the list is
  `409 STALE_WRITE`, as the contract says (the Tech Lead, 2026-10-14).
- **`AGENTS.md`'s "Git workflow"** names R and D4 branches only. The Tech Lead adds the line for
  `work/escalation-e1` in T-E1.7 (2026-10-14).
- **The spec number.** E1's entry is v0.107; the v0.11.3 hotfix takes v0.106 (the Tech Lead,
  2026-10-14).
- **A standalone question held by two sections.** The request names no source section, so a
  question that two sections of one test hold cannot be told apart; the move answers `400`
  (`move_source_ambiguous`). The builder's picker does not let a test hold a bank question twice,
  so this is a guard for data written some other way. If it ever matters, the contract gains an
  optional source section.
