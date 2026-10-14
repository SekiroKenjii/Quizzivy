import { http } from "msw";
import type { components } from "@/lib/api/schema";
import { assertContractRequest, contractJson } from "./contractResponse";

type Test = components["schemas"]["Test"];
type StoredGroup = components["schemas"]["StoredQuestionGroup"];
type StoredGroupQuestion = components["schemas"]["StoredGroupQuestionInput"];
type MoveInput = components["schemas"]["MoveTestQuestionInput"];
type ErrorCode = components["schemas"]["ErrorCode"];

const BASE = "http://localhost:8080";
const PATH = "/teacher/tests/{id}/questions/move";
const REQUEST_ID = "018f0000-0000-7000-8000-00000000ffff";

/**
 * CopyOrigin is the bank question a member copy was made from, and whether the
 * copy has been edited since; a test sets `edited` where the server would find
 * the copy's content changed.
 */
export interface CopyOrigin {
  questionId: string;
  edited: boolean;
}

/**
 * GroupMoveDraft is the draft a fake `moveTestQuestion` acts on: the test,
 * its section-owned groups by id, how a bank question becomes a member copy,
 * the origin each copy remembers, and every request body it was sent.
 */
export interface GroupMoveDraft {
  test: Test;
  groups: Map<string, StoredGroup>;
  copy: (questionId: string) => StoredGroupQuestion;
  requests: MoveInput[];
  origins?: Map<string, CopyOrigin>;
}

/**
 * moveQuestionHandler answers `POST /teacher/tests/{id}/questions/move` on
 * `draft` as the contract describes it: it checks the request against the
 * contract, the test revision and every touched group's revision, refuses a
 * gap-bound member's leaving with `group_gap_bound`, copies a standalone
 * question into a group and remembers where the copy came from, moves a
 * member by its id, puts the original back when an unedited copy leaves its
 * group, bumps the test and every touched group, and answers with the
 * contract-checked result.
 */
export function moveQuestionHandler(draft: GroupMoveDraft) {
  return http.post(`${BASE}/teacher/tests/:id/questions/move`, async ({ request }) => {
    const body = (await request.json()) as MoveInput;
    assertContractRequest(PATH, "post", body);
    draft.requests.push(body);
    return answer(draft, body);
  });
}

/** moveFailure answers the move with the contract's error envelope. */
export function moveFailure(
  status: number,
  code: ErrorCode,
  message: string,
  details?: Record<string, unknown>,
) {
  return contractJson(PATH, "post", status, {
    error: { code, message, requestId: REQUEST_ID, ...(details ? { details } : {}) },
  });
}

interface Checked {
  source: Source;
  destination: StoredGroup | null;
  touched: string[];
}

function check(draft: GroupMoveDraft, body: MoveInput): Checked | Response {
  if (body.expectedUpdatedAt !== draft.test.updatedAt)
    return moveFailure(409, "STALE_WRITE", "Đề đã được sửa ở nơi khác.");
  const source = sourceOf(draft, body.questionId);
  const section = draft.test.sections.find(({ id }) => id === body.to.sectionId);
  const destination =
    body.to.groupId === null ? null : draft.groups.get(body.to.groupId);
  if (!source || !section || destination === undefined)
    return moveFailure(404, "NOT_FOUND", "Không tìm thấy.");
  if (destination && destination.ownerSectionId !== body.to.sectionId)
    return moveFailure(400, "VALIDATION_FAILED", "Nhóm không thuộc phần này.");
  if (source.kind === "unit" && destination === null)
    return moveFailure(400, "VALIDATION_FAILED", "Dùng updateTest để chuyển câu.");
  const touched = [
    ...new Set([
      ...(source.kind === "member" ? [source.group.bundle.group.id] : []),
      ...(destination ? [destination.bundle.group.id] : []),
    ]),
  ];
  const behind = touched.some(
    (groupId) =>
      body.expectedGroupRevisions.find((item) => item.groupId === groupId)?.revision !==
      draft.groups.get(groupId)?.revision,
  );
  if (behind) return moveFailure(409, "STALE_WRITE", "Nhóm đã được sửa ở nơi khác.");
  const leaving =
    source.kind === "member" &&
    destination?.bundle.group.id !== source.group.bundle.group.id;
  const binding = leaving ? gapBinding(source.group, body.questionId) : undefined;
  if (binding)
    return moveFailure(422, "VALIDATION_FAILED", "Câu hỏi đang gắn với chỗ trống.", {
      rule: "group_gap_bound",
      ...binding,
      questionId: body.questionId,
    });
  return { source, destination, touched };
}

function gapBinding(group: StoredGroup, questionId: string) {
  return group.bundle.group.stimuli.flatMap((stimulus) =>
    stimulus.gaps
      .filter((gap) => gap.questionId === questionId)
      .map((gap) => ({ stimulusId: stimulus.id, gapId: gap.gapId })),
  )[0];
}

function answer(draft: GroupMoveDraft, body: MoveInput): Response {
  const checked = check(draft, body);
  if (checked instanceof Response) return checked;
  const landing = land(draft, body, checked);
  const updatedAt = new Date(Date.parse(draft.test.updatedAt) + 1000).toISOString();
  draft.test = { ...draft.test, updatedAt };
  for (const groupId of checked.touched) {
    const group = draft.groups.get(groupId)!;
    draft.groups.set(groupId, {
      ...group,
      revision: group.revision + 1,
      testUpdatedAt: updatedAt,
    });
  }
  return contractJson(PATH, "post", 200, {
    test: draft.test,
    ...landing,
    groups: checked.touched.map((groupId) => draft.groups.get(groupId)!),
  });
}

interface Landing {
  questionId: string;
  copied: boolean;
  restored: boolean;
  editedCopy: boolean;
}

function land(draft: GroupMoveDraft, body: MoveInput, checked: Checked): Landing {
  const { source, destination } = checked;
  const taken = take(draft, source, body.questionId);
  if (source.kind === "unit") return copyIn(draft, body, destination!);
  if (!destination) return leave(draft, body);
  insertMember(draft, destination.bundle.group.id, taken!, body.to.index);
  return {
    questionId: body.questionId,
    copied: false,
    restored: false,
    editedCopy: false,
  };
}

function copyIn(draft: GroupMoveDraft, body: MoveInput, into: StoredGroup): Landing {
  const copy = draft.copy(body.questionId);
  draft.origins ??= new Map();
  draft.origins.set(copy.id, { questionId: body.questionId, edited: false });
  insertMember(
    draft,
    into.bundle.group.id,
    { question: copy, optionOrder: "shuffle" },
    body.to.index,
  );
  return { questionId: copy.id, copied: true, restored: false, editedCopy: false };
}

function leave(draft: GroupMoveDraft, body: MoveInput): Landing {
  const origin = draft.origins?.get(body.questionId);
  draft.origins?.delete(body.questionId);
  const original = origin && !origin.edited ? origin.questionId : undefined;
  const questionId = original ?? body.questionId;
  insertUnit(draft, body.to.sectionId, questionId, body.to.index);
  return {
    questionId,
    copied: false,
    restored: original !== undefined,
    editedCopy: origin?.edited === true,
  };
}

type Source =
  { kind: "unit"; sectionId: string } | { kind: "member"; group: StoredGroup };

function sourceOf(draft: GroupMoveDraft, questionId: string): Source | null {
  const section = draft.test.sections.find((item) =>
    (item.units ?? []).some(
      (unit) => unit.kind === "question" && unit.id === questionId,
    ),
  );
  if (section) return { kind: "unit", sectionId: section.id };
  const group = [...draft.groups.values()].find((item) =>
    item.bundle.group.members.some((member) => member.questionId === questionId),
  );
  return group ? { kind: "member", group } : null;
}

interface Taken {
  question: StoredGroupQuestion;
  optionOrder: "shuffle" | "fixed";
}

function take(
  draft: GroupMoveDraft,
  source: Source,
  questionId: string,
): Taken | undefined {
  if (source.kind === "unit") {
    setUnits(draft, source.sectionId, (units) =>
      units.filter((unit) => !(unit.kind === "question" && unit.id === questionId)),
    );
    return undefined;
  }
  const group = source.group;
  const question = group.bundle.questions.find((item) => item.id === questionId);
  const member = group.bundle.group.members.find(
    (item) => item.questionId === questionId,
  );
  draft.groups.set(group.bundle.group.id, {
    ...group,
    bundle: {
      group: {
        ...group.bundle.group,
        members: group.bundle.group.members.filter(
          (member) => member.questionId !== questionId,
        ),
      },
      questions: group.bundle.questions.filter((item) => item.id !== questionId),
    },
  });
  return question && member ? { question, optionOrder: member.optionOrder } : undefined;
}

function insertMember(
  draft: GroupMoveDraft,
  groupId: string,
  taken: Taken,
  index: number,
) {
  const group = draft.groups.get(groupId)!;
  const members = [...group.bundle.group.members];
  const questions = [...group.bundle.questions];
  const at = Math.min(index, members.length);
  members.splice(at, 0, {
    questionId: taken.question.id,
    optionOrder: taken.optionOrder,
  });
  questions.splice(at, 0, taken.question);
  draft.groups.set(groupId, {
    ...group,
    bundle: { group: { ...group.bundle.group, members }, questions },
  });
}

function insertUnit(
  draft: GroupMoveDraft,
  sectionId: string,
  questionId: string,
  index: number,
) {
  setUnits(draft, sectionId, (units) => {
    const next = [...units];
    next.splice(Math.min(index, next.length), 0, { kind: "question", id: questionId });
    return next;
  });
}

function setUnits(
  draft: GroupMoveDraft,
  sectionId: string,
  change: (
    units: NonNullable<Test["sections"][number]["units"]>,
  ) => NonNullable<Test["sections"][number]["units"]>,
) {
  draft.test = {
    ...draft.test,
    sections: draft.test.sections.map((section) => {
      if (section.id !== sectionId) return section;
      const units = change(section.units ?? []);
      return {
        ...section,
        units,
        questionIds: units
          .filter((unit) => unit.kind === "question")
          .map((unit) => unit.id),
      };
    }),
  };
}
