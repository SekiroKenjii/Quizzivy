# E1 — Dragging a question into and out of a shared-context group (v0.11.6)

An escalation release. After a teacher escalation, the owner decided on **2026-10-14** that a
teacher can drag a question into a shared-context group in the test builder, drag a member out of
one, move it between two groups of the same test, and reorder members inside a group. The decision
**overrides `74d-d4-deck-update.md` Q8**, whose default was "a bank question cannot be dragged
into a shared group" (DG-186). It ships as **v0.11.6**: three hotfixes took the numbers before it,
v0.11.3 (the AI Generate hotfix), v0.11.4 (the AI check reads group members) and v0.11.5 (a
phone-layout fix), so E1 does not touch `web/package.json` until its own release branch.

The principal engineer's design is the source of this file; its §0 to §11 are the sections and
tasks below, and its §13 is "Decided" at the end. Branches are `feature/t-e1-<nn>-<slug>` off the
integration branch `work/escalation-e1`, which was cut from `develop` at `6f502ab4` (main is at
v0.11.2) and is merged back `--no-ff` when the release is complete. As T-D4.21 shipped in v0.10.1,
E1 runs as its own small release from `develop`: `release/0.11.6` → `main` → back-merge. The
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
moved by v0.11.6 is read unchanged by `v0.11.5` (an old pod answers the new route 404, and every
row the new binary writes is a row the old one already reads); the builder matches the deck at
360, 768, 1024, 1280 and 1440, light and dark, with every difference left a row in
`docs/design/gaps.md`; every CI step of `70-redesign-overview.md` §8 is green, `pnpm e2e:live`
included; released as `v0.11.6`.

**Depends on:** v0.11.2 on `main` and `develop` at `6f502ab4`. E1 builds on D4's builder
(T-D4.2a, T-D4.14: the group row, the Shared content dialog and `useGroupEditor`), on R2's
isolation suite and `x-resource` kinds, and on the two-sided locks of `20-data-model.md` §19. It
has no dependency on the hotfixes v0.11.3 to v0.11.5.

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
- [x] **CI result** passes on the pull request.

**As built** (#610): The contract is as above, with these differences from the design's sketch: the
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
- [x] **CI result** passes on the pull request.

**As built** (#612): `GroupsPostgres.MoveQuestion` is `group_move.go` (the flow and the three writers),
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
**Touches:** `api/openapi.yaml` (the 400 and 404 descriptions), `server/gen/openapi/`,
`web/src/lib/api/schema.d.ts`, `server/internal/modules/tests/http/{groups,group_move,group_response}.go`,
`server/internal/modules/tests/http/tests/group_move_test.go` (new),
`server/tests/{isolation_cases_test,group_move_flow_test}.go`
**Size:** M
**Done when:**
- [x] `MoveTestQuestion` reads the body, builds the command from the principal and
      `ScopeFromContext`, and answers the result; it answers 501 only when the command or its
      repository is unwired.
- [x] `groupFailure` maps `group_gap_bound` to `422 VALIDATION_FAILED` with `details.rule`, the
      stimulus, gap and question, and the four `move_*` rules to `400`; the destination rules the
      read-back finds (`group_limits`, `group_audio_scope`, `group_asset`) keep their `422`; every
      message is written vi first and en second through `httpx.Text`.
- [x] A foreign or missing test, question, section or group answers 404, and so does a group in
      `expectedGroupRevisions` the caller cannot reach or that is not of this test, before any
      revision is compared (see "Risks"); a reachable entry the move does not touch, or a repeated
      one, is a `400` (`move_revisions`), and the contract's description of the `400` says so.
- [x] The isolation table's `moveTestQuestion` entry is the design's: `freshGroupedTest`, the body
      above, T-E1.1's `blind` marks removed; `TestAnotherTeachersIdsAnswerAsMissingOnes` passes
      with the provider-call count unchanged.
- [x] A student payload never carries `isCorrect`, `sampleAnswer`, `acceptedAnswers` or
      `transcript`; the move's answer is teacher-only, a `/teacher/*` operation under
      `content.tests.write`, and reaches no student surface.
- [x] `make lint`'s three Go tools, `make gen-check`, the router and contract tests, the
      module's tests, and `go test -tags e2e ./tests/...` pass.
- [x] **CI result** passes on the pull request.

**As built** (#613): The handler is `groups.go`'s `MoveTestQuestion`, with `group_move.go` for the request
reading, the answer and the move's messages; `group_response.go` gains the `Visit` method and one
case for the five rules. `ErrNotFound` is answered by `moveFailure` with a message that names no
particular kind ("The test, question, section or group was not found."), because the move's
missing thing is any of four; every other error goes through `groupFailure`. The contract's 400
and 404 descriptions were widened (the Tech Lead's decision of 2026-10-14), regenerated and
committed on their own. The isolation entry needs no `link`: the suite substitutes one slot at a
time, the base body carries the caller's own group in both group slots, and every substitution
reaches the server as the only foreign id in the request (the risk written under "Risks" is why
none is used). `freshGroupedTest` publishes a test as `freshPublishedTest` does and mounts a group
of one member, with the caller's own image as its material, in the section over HTTP.
`TestATeacherMovesQuestionsAcrossAGroupsBoundary` (e2e) drives the operation through the real
stack: a copy-in whose answer carries the new id, the group at revision 2 and the units; the same
body replayed at the old test revision answering `409 STALE_WRITE`; a move-out; the `400`, `404` and
`422` envelopes with their `details.rule`, and a refused move that leaves the group as it was.
The handler's unit tests (`http/tests/group_move_test.go`) pin what the command receives (ids,
index, the nullable group, the revisions, the actor, scope and grants), the answer, each refusal's
status, code and rule in both languages, and the 501.

After the independent review (2026-10-14): the 409 description also names `GROUP_CONFLICT` and the 422
description `group_reference`, beside `STALE_WRITE` and `TEST_ARCHIVED`, since `groupWriteError` can
emit both (a unique violation raced by another writer; a foreign key or check the write met), as
`updateQuestionGroup` lists them; the handler's table pins both. The branch carries the T-E1.2
review fixes (the copy-in's lock order, the move-out's owner) through a merge of that branch.

## B. The web

### T-E1.4 — Outline drag
**Depends on:** T-E1.1 (runs in parallel with T-E1.2 against MSW)
**Touches:** `web/src/features/tests/components/{OutlineTree,OutlineGroupRow}.tsx`,
`web/src/features/tests/{outlineUnits,api}.ts`,
`web/src/features/tests/pages/teacher/TestBuilderPage.tsx`, `web/tests/support/` (MSW),
`web/tests/units/builder/{group-move.test.tsx (new),mixed-outline.test.ts,group-row.test.tsx}`
**Size:** L. High risk: `features/tests/` builder, autosave flushes.
**Done when:**
- [x] A member row is a sortable with the deck's grip; a group header is a droppable tinted while
      over; the drop line shows on member rows.
- [x] `findMember` resolves a member; `dropUnit` calls `onMove` for every pair that is not
      standalone to standalone, and `onChange(moveUnit)` stays for that pair.
- [x] `TestBuilderPage.moveQuestion` flushes the question editor and `outline.flush()`, runs the
      call in `writes.run`, replaces the outline and the groups from the response, selects the
      moved question, and sets `version.current`.
- [x] While the request runs the dragged row is at 40% opacity with `aria-busy` and the outline
      refuses another drag; the outline is unchanged on every error.
- [x] A gap-bound member's grip is `aria-disabled` and not a drag source.
- [x] The MSW handler is validated by ajv against the contract; unit cases as in "The web".
- [x] `pnpm typecheck`, `pnpm lint:changed`, `pnpm format:changed`, `pnpm test:changed`,
      `pnpm test:integration`; `autosave-unmount.test.tsx` and `router-chunks.test.ts` pass.

**As built** (#615): The grip is `OutlineGroupRow`'s: a member row is `useSortable({id: "question:" + id})`
inside the group's own `SortableContext`, with the grip 20 by 24px at the standalone rows' 33px
pitch. The group's header, with the shared-content line under it, is `useDroppable("group:" + id +
":head")`. While a question is over it, it is tinted `--accent-soft`; a drop there lands at index 0
and opens a closed group, as the deck's `finishDrop` does. The 2px `--primary` line shows before or
after the member row under the pointer. The collision filter (`targetsFor`, `accepts`) keeps a whole
group from being dropped into a group, and screen readers hear `builder.groupDropHint` over a
header. A member a material gap is bound to has an `aria-disabled` grip with the reason as its
tooltip and description, and is not a drag source.

Every drop is decided by `dropRequest` and `findMember` in `outlineUnits.ts`, pure functions that
read the outline and the groups and answer a `units` move or a `question` move or nothing; the design
sketched this inside `dropUnit`, which now only calls them. Standalone to standalone is still
`onChange(moveUnit)` and the outline's autosave; every other pair is `onMove`. The page's
`moveQuestion` is not optimistic, as the design has it: it flushes the open question and the outline
(`flushBeforeMove`), reads each touched group's revision from the group reads the page already holds
or from the server, and calls `moveTestQuestion` inside `writes.run`. `acknowledgeMove` then sets
`version.current` from the answer, writes the returned groups into `["teacher-group", id]` (and
re-stamps the untouched ones with the new test revision), replaces the outline from the answer,
selects the moved question in its new place and remounts the pane, so the next group save carries
`expectedRevision` 2. Focus returns to the moved row's grip. While a move runs the row is at 40% with
`aria-busy`, the sensors are off, and a screen-reader status announces "Đang chuyển…". The outline is
unchanged on every refusal.

`tests/support/groupMove.ts` is a stateful fake of the move for the unit tests, and
`assertContractRequest` (new, in `contractResponse.ts`) checks the request it receives against the
contract with ajv, as `contractJson` does the answer. The four strings are `builder.reorderMember`,
`memberGapBound`, `groupDropHint` and `moving`; `memberGapBound` says "nội dung dùng chung", the name
the builder gives the Shared content dialog, where the design wrote "ngữ liệu chung". Tests:
`builder/group-move.test.tsx` (new), `mixed-outline.test.ts` and `group-row.test.tsx`; the author's run
over `units/{builder,question-groups,ai,i18n,styles,tests}` was 70 files and 439 tests. The grips,
the tint and the drop line were compared with `Quizzivy Teacher.dc.html` in Chromium at 360, 768,
1024, 1280 and 1440, light and dark, with no horizontal overflow before or after a drop; the
differences are DG-213 to DG-216 and the two rows T-E1.7 changed. **CI result** was green on the
pull request.

### T-E1.5 — Menus, keyboard, strings, error states, the toast
**Depends on:** T-E1.4 (same files)
**Touches:** `OutlineTree.tsx`, `OutlineGroupRow.tsx`, `OutlineKeyboardSensor.ts` (only if a target
needs a coordinate), `web/src/locales/{vi,en}.json`,
`web/tests/units/builder/{group-move,outline-keyboard-activation,question-menu}.test.tsx`
**Size:** M
**Done when:**
- [x] The standalone row's menu gains "Move into group…", and a member row gets its menu: Move up,
      Move down, Move out of group, Move to group…; below 768 the selected member row shows them at
      44px (DG-135).
- [x] Every drag has a keyboard path through `OutlineKeyboardSensor` (Space, arrows, Space), and a
      menu equivalent (R4's rule).
- [x] Every string of "The web" is in `vi.json` and `en.json`, vi first; the toast shows after a
      copy-in and says the original stays; every error state of the table is drawn.
- [x] `aria-label` and the group header's drop hint are announced; focus lands on the moved
      question's row after the pane remounts.
- [x] The locale files are Tech-Lead-serialised: the change goes in one commit the lead merges.

**As built** (#617): A member row has its own "…" menu (`MemberActions` in `OutlineGroupRow.tsx`) with
Move up, Move down, Move out of group and Move to group…, the first member's Move up and the last's
Move down disabled. It takes the place of the points on hover and focus from 768px, and below 768 it
shows at 44px on the selected member. A gap-bound member keeps Move up and Move down, and its Move out
of group and Move to group… are disabled with the reason printed under them and named by
`aria-describedby`.

Two things differ from the design's text. **Where "Move into group…" lives.** A standalone question's
outline row has no "…" menu: it keeps the deck's ↑ ↓ ×. The question's only "…" menu is the editor's,
in its header, so "Move into group…" joined Duplicate and Move to section… there
(`BuilderQuestionEditor`), and shows only while the test has a group. **The picker.** The design
sketched a submenu of groups by section and a dialog above eight; both entries open one dialog
(`FormDialog` in `TestBuilderPage`) with a "Nhóm" select whose entries read "{group} · {section}",
every group of the test but the member's own. "Move into group…" and "Move to group…" send the
question to the end of the chosen group, and Move to group… is disabled while the test has no other
group.

Every drag has a keyboard path with no change to `OutlineKeyboardSensor`: the new droppables are
reached by Space, the arrows and Space, and `outline-keyboard-activation.test.tsx` pins a member to the
next member's place and out to the next section's header, each asking the page once. After a move
the page puts focus on the moved question's grip. The errors are `moveFailure` and `refuseMove` in the
page: `group_gap_bound`, `group_limits` and `group_audio_scope` each have a message; a 404 says the
test changed and offers Reload; a `STALE_WRITE` turns on the builder's stale prompt; any other failure
says "Chưa chuyển được. Thử lại.". The outline is unchanged on each. After a copy-in the toast is
`builder.movedIntoGroupCopied`.

The strings are one commit (`dfb9de50`), `builder.memberActions`, `memberMoveUp`, `memberMoveDown`,
`moveIntoGroup`, `moveToGroup`, `moveOutOfGroup`, `moveGroupTitle`, `moveGroupField`,
`moveGroupOption`, `movedIntoGroupCopied`, `moveGapBound`, `moveGroupFull`, `moveAudioScope`,
`moveReload` and `moveFailed`. Their wording departs from "The web" in four places, and the locale
files are what the product says: the Shared content dialog is "Nội dung dùng chung", not "Ngữ liệu
chung"; the member menu says "Di chuyển lên" and "Di chuyển xuống", as the group's own menu beside it
does, not "Lên" and "Xuống"; `moveGroupFull` names both halves of `group_limits`, 200 questions and
4 MB; and the English `moveFailed` is "Could not move the question. Try again.". Tests:
`group-move.test.tsx` (the member menu, the errors, the toast, the reload offers),
`question-menu.test.tsx` (the editor menu's entry) and `outline-keyboard-activation.test.tsx`. The
deck comparison of the whole builder was made here, at 360, 768, 1024, 1280 and 1440, light and dark;
its differences are DG-213 to DG-216. No contract change, no migration and no new dependency. **CI
result** was green on the pull request.

## C. Verification and documents

### T-E1.6 — QA
**Depends on:** T-E1.3, T-E1.5
**Touches:** `web/tests/e2e/builder-groups.live.spec.ts`
**Size:** M
**Done when:**
- [x] One live case: drag a bank question into a group, publish, and the preview shows it inside
      the group's passage; the id remap and both revision bumps are asserted.
- [x] The builder matches the deck at 360, 768, 1024, 1280 and 1440, light and dark; each
      difference left is a row in `docs/design/gaps.md`.
- [x] The five canaries pass before and after; `make test-api` and `pnpm e2e` pass; `pnpm
      e2e:live` passes with the new case.

**As built** (#620): One new case in `web/tests/e2e/builder-groups.live.spec.ts`, against the real API;
the file's two existing cases are untouched and nothing else changed. It signs in as the seeded Admin,
creates a bank question worth 3 points and a shared-context group with a passage and one resident
member, and drags the question into the group by the outline's keyboard path (grip, Space, ArrowDown,
Space), because that path is deterministic; a real mouse drag in a scratch spec, since deleted, also
worked three times in three. The header is checked tinted while the question is over it.

It asserts the request body is exactly `{expectedUpdatedAt, questionId, to: {sectionId, groupId, index:
0}, expectedGroupRevisions: [{groupId, revision}]}`. It asserts the id remap: 200, `copied` true and a
`questionId` that is neither the source's nor the resident's. It asserts both bumps: `groups` holds
only the touched group at its revision plus one, `test.updatedAt` moves forward, and a re-read over the
API returns both. The group's members are `[copy, resident]`, the copy has the source's prompt, points
and options, and the section's units are now just the group. The bank row is deep-equal to before,
`updatedAt` included, and only the derived `usedIn` and `usedInTests` change, from 1 to 0; a bank
search for the prompt returns only the source. The next group write uses the returned revision and
answers 200, the revision becoming the baseline plus two. Publish answers 201, the preview's group
lists `[edited copy, resident]`, and the student frame shows the copy beside the passage's heading and
text.

The author's runs: the case 3 of 3 on a private stack at `ffd6823c` and 9 of 9 over three runs on the
earlier merge; the canaries before and after (web: `audio-player`, `client.refresh`, `router-chunks`;
Go, with `-tags integration`: `publish_snapshot`, `TestTheSameClientSeqFromTwoSessionsBothPersist`,
`TestLockForDraftUseSerialisesAgainstDelete`, `TestLockForVersionUseSerialisesAgainstDelete`);
`tests/units/builder`, 31 files and 235 tests. **CI result** was green on the pull request, with E2E
and E2E (live API) among its jobs; its Server tests job was skipped, because no file it reads had
changed since it last passed. The seeded Admin's sign-in is limited to 20 an hour per email, so about
20 local reruns on one stack end on a 429; CI runs each case once.

Two notes. The deck comparison was made at T-E1.5, not here: 152 captures, at 360, 768, 1024, 1280 and
1440, light and dark, and its differences are DG-213 to DG-216 in `docs/design/gaps.md`, with the
changes to DG-186 and DG-211 (T-E1.7). The sync of `develop` (v0.11.3, v0.11.4) into the branch is #618.

### T-E1.7 — Documents
**Depends on:** with T-E1.3 (server behaviour) and T-E1.5 (the screen); never batched after
**Touches:** the files below
**Size:** S
**Done when:**
- [x] `docs/quizzivy-spec-v0.3.md`: bumped to v0.108 (the v0.11.3 hotfix took v0.106 and the
      v0.11.4 hotfix v0.107) with a "Changes since v0.107" entry. §7.3, after the paragraph on
      the builder's group inserts and copies ("Members belong to one group in one section" is
      the contract's sentence, not the spec's): a standalone question dragged into a section's
      group becomes a member copy and the bank row stays; a member dragged out keeps its id and
      becomes a bank question; a member moves between a test's groups in place; a gap-bound
      member does not leave; both the test revision and each touched group's revision guard it;
      the next publish freezes the new structure and the diff reads a copy-in as removed and
      added. §8 builder row: the outline's member grips, the group header as a drop target and
      the menu equivalents. §15: `POST /teacher/tests/:id/questions/move` with its answers.
- [x] `docs/plan/20-data-model.md`: a new §48 "Moving a question across the group boundary": no
      schema change; the DML, the lock order, the deferred constraints, and why a copy on the way
      in and the same id on the way out (the §12 register needs no row: no table changes).
- [x] `docs/design/gaps.md`: DG-186 gains "**Reversed 2026-10-14 (owner):** a bank question can be
      dragged into a shared group and a member out, built in v0.11.6"; DG-211's "ours have none"
      for the member grip is marked built, and the request to the design team to remove the grip is
      withdrawn. The differences T-E1.5's deck check found are four new rows, DG-213 to DG-216.
- [x] `docs/plan/74d-d4-deck-update.md`: Q8 gets "**Overridden 2026-10-14 by the owner, after a
      teacher escalation:** drag in and out is built as v0.11.6, design in `74e-e1-group-move.md`";
      the "Decisions taken in this file" bullet for Q8 and the DG-186 row point to it.
- [x] `docs/plan/70-redesign-overview.md` §2: a row for the escalation release, as v0.9.1 has one.
- [x] This file: every task's boxes ticked and its "As built" written in the task's own PR.
- [x] `AGENTS.md` needs no rule change for the inserts: the two-sided lock paragraphs already cover
      them. Its "Git workflow" paragraph gains the line for `work/escalation-e1` and
      `feature/t-e1-<nn>-<slug>` (the Tech Lead's, decided 2026-10-14).

**As built** (this pull request): The spec is v0.108. The data model's section is §48, the next free
one. The four new gap rows took DG-213 to DG-216, the next free ids on `develop` and on this branch;
each was checked against `OutlineGroupRow.tsx` and `OutlineTree.tsx`. `AGENTS.md` gains one sentence
in its branch-workflow paragraph, beside D4's. The plan's own corrections are the version (E1 is
v0.11.6 throughout, since v0.11.3, v0.11.4 and v0.11.5 are hotfixes that took the numbers before it)
and the spec numbers. T-E1.4 to T-E1.6 did not write their own "As built" in their pull requests, so
this one writes them, from the merged pull requests and the code on the branch, and ticks the boxes
the merged code meets.

## Order of work

1. T-E1.1, alone. Everything else waits on it, and `api/openapi.yaml` and the generated files are
   Tech-Lead-serialised (`docs/team/README.md`, "Coordinated changes").
2. Two lanes in parallel once T-E1.1 is merged into `work/escalation-e1`: {T-E1.2 → T-E1.3} and
   {T-E1.4 → T-E1.5}. The web lane builds against MSW; the locale files are serialised.
3. T-E1.7 lands with T-E1.3 (server behaviour) and T-E1.5 (the screen), never after.
4. T-E1.6, after T-E1.3 and T-E1.5.
5. The release: `work/escalation-e1` → `develop` (`--no-ff`), `release/0.11.6`, the checklist
   below.

## Release checklist

**v0.11.6, dragging a question into and out of a group** (decided 2026-10-14)

- [x] Every task is ticked on `work/escalation-e1`. `develop` is merged into it, and verification
      is re-run after that merge. Then `work/escalation-e1` → `develop` (`--no-ff`), and
      `release/0.11.6` is cut from `develop` at a named commit. The hotfixes v0.11.3 to v0.11.5
      have shipped before it, or their fixes are already on `develop`.
- [x] `web/package.json` is bumped to 0.11.6 in the release branch only.
- [x] Every CI step runs on the release pull request, in order, judged by exit code (70 §8.1):
      `make lint`, `make gen-check`; `make test-api` and goose up/down/up (no migration, so the
      round trip is unchanged); `pnpm lint`, `pnpm typecheck`, `pnpm format:check`;
      `pnpm test:unit`, `pnpm test:integration`, `pnpm build`; `pnpm e2e`; `pnpm e2e:live`,
      required because the builder and the API meet.
- [x] The five canaries pass, and their files are as they were at `v0.11.2`.
- [x] Security: `moveTestQuestion` is the only operation added; `permissions.golden` changed once
      (T-E1.1) and holds `content.tests.write` for it; R2's isolation suite passes with the
      operation's entry; no student payload changes.
- [x] Migrations: none. The deploy is a rolling one; an old pod answers the route 404.
- [x] The documents of T-E1.7 are merged with the code they describe.

Evidence (the Tech Lead, 2026-10-14):

- **Branches.** develop was merged into `work/escalation-e1` twice: v0.11.3 and v0.11.4 in #618,
  v0.11.5 in #623. Typecheck, the builder, question-groups and question-bank units, and
  `builder-phone-layout.spec.ts` were re-run on the second merge. `work/escalation-e1` reached
  develop in #624 (`d3ede6e4`), and `release/0.11.6` is cut there. v0.11.3 to v0.11.5 shipped
  first (`5849da00`, `2beb7dcf`, `5231b258`).
- **Canaries.** `git diff 170285a2` (v0.11.2) is empty for the five canary files. T-E1.6 ran them
  before and after.
- **Security.** `permissions.golden` gained one line, in T-E1.1 (`8c5646b5`):
  `POST /teacher/tests/{id}/questions/move content.tests.write`. The isolation suite holds
  `moveTestQuestion` with a grouped test of the second teacher (`freshGroupedTest`, T-E1.3). No
  `/app/*` payload changes.
- **Migrations.** `git diff 5231b258 -- migrations` is empty.
- **Documents.** T-E1.7 (#622) is on develop with the code. Rolling out v0.11.6 is in
  `docs/setup/deploy.md`.
- **CI.** The release pull request (#625) ran every job, judged by exit code: Plan, Contract,
  Server lint, Server tests (unit, integration, end-to-end, goose up/down/up), Web checks (lint,
  typecheck, format, integration, build), Web unit (1) and (2), E2E, E2E (live API), Production
  image and Deck. It was re-run in full (attempt 2), so that none was skipped by its file hash.
  `main` took the merge at `9821c7fd`, and the deploy went green. Production reports 0.11.6, and
  the move route answers 401 without a token, where v0.11.5 answered 404.

## Open items

**Decided** by the owner, 2026-10-14. These are the design's five questions with the owner's
answers; each had the default shown, and the answers differ from the design's defaults only where
noted.

**Q1. Copy on the way in?** Decided: **copy**, the bank row untouched. The alternative, move the
same id when nothing else references it, is invisible to the teacher and touches the invariant
that a member is born in its group. (What the decision left behind, a second bank question after
every round trip, is answered in "After release (v0.11.7)" below; the decision itself stands.)

**Q2. Version number?** Decided: **v0.11.6**. The owner first decided v0.11.4, because v0.11.3 was
the Generate hotfix; v0.11.4 then went to a hotfix (the AI check reads group members) and v0.11.5 to
another (a phone-layout fix), so E1 takes v0.11.6 (the Tech Lead, 2026-10-14). The design's default
was v0.11.3, as T-D4.21 shipped in a patch (v0.10.1), with R5 keeping v0.12.0.

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
- **The spec number.** E1's entry is v0.108. The v0.11.3 hotfix took v0.106 and the v0.11.4 hotfix
  v0.107, so the number the plan first named, v0.107, was not free (the Tech Lead, 2026-10-14).
- **A standalone question held by two sections.** The request names no source section, so a
  question that two sections of one test hold cannot be told apart; the move answers `400`
  (`move_source_ambiguous`). The builder's picker does not let a test hold a bank question twice,
  so this is a guard for data written some other way. If it ever matters, the contract gains an
  optional source section.

## After release (v0.11.7)

Decided by the owner on 2026-10-14, the day v0.11.6 reached production, after a teacher saw duplicates
piling up in the bank. It changes Q1's consequence and does not reverse Q1: a bank question dragged into a
group is still copied, and the bank row is still never written by that move.

**The defect.** A drag in copies, and a drag out keeps the member's id, so a round trip turned one bank
question into two with the same content, and every further round trip added one. The copy was an
independent question the moment it left, and nothing said it had once been the original.

**What was decided.**

1. **A copy remembers its origin.** `questions.copied_from_id`, nullable, references `questions (id)` with
   `ON DELETE SET NULL` (migration 00126; its partial index is 00127; D-50 in
   `20-data-model.md`). The copy into a group sets it to the bank question's id. Nothing else sets it.
2. **An unedited copy dragged out restores the original.** When the member has an origin, the origin is a
   live bank question in the caller's reach (and owned by the test's owner, and not in the test already),
   and the member's content equals its content, the member is removed and the original is placed where
   the member would have landed, in the same section at `to.index`. No bank row is added. The answer says
   `restored: true` and `questionId` is the original's.
3. **An edited copy stays a new bank question**, as before, with its origin cleared, because it is now an
   independent question. The answer says `editedCopy: true` and the builder says it was saved as a new
   question because it had been edited. That signal is the contract's second flag: a member that was
   never a copy, or whose original is gone, is neither restored nor an edited copy, and the builder says
   nothing for it.
4. **A move between groups, or within one, keeps the origin.**
5. **Locking.** Inserting the original into `test_section_questions` calls `questions.LockForDraftUse`
   first. The origin is locked in one ascending sweep with the questions the destination section's
   seeding locks, so no writer holds a later question while it waits on an earlier one; a concurrent
   delete of the origin makes the move leave the member as a bank question and never restores a deleted
   row (`20-data-model.md` §48, "After release").
6. **Removing the copy** is the hard `DELETE` a group save uses for a member it drops, not a soft delete (a
   member cannot be soft-deleted). A published version that froze the copy and any attempt on it keep
   every frozen row; only the copy's informational `source_question_id` becomes NULL (D-07).

**What changed in the code.** `GroupQuestion.SameContent` (`tests/domain/group_content.go`) is the one
comparison; `GroupsPostgres.MoveQuestion` decides in `group_move_origin.go`; `moveTestQuestion` answers
`restored` and `editedCopy` beside `copied`; the builder shows two toasts (`builder.movedOutRestored`,
`builder.movedOutEditedCopy`). `permissions.golden`, the isolation entry and every `x-resource` are
unchanged.

**Tests it moved.** `TestMoveClampsIndexAndCompactsOrdinals` dragged a copy out and expected it under its
own id at the drop position; the copy now goes back as the original, so the test expects the bank
question there. It still pins the clamp and the compaction. No other test of v0.11.6 changed.

**Deploy.** Two migrations run in the release command, before the new pods take traffic. A v0.11.6 pod
still running writes copies with a NULL origin, which never restore; that is harmless. Rolling back leaves
the column unused (`docs/setup/deploy.md`, "Rolling out v0.11.7").
