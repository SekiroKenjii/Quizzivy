import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import type { components } from "@/lib/api/schema";
import { loadSpec, propertyNames } from "@tests/support/openapi";

const spec = loadSpec();
const ajv = new Ajv({ strict: false, validateFormats: true });
addFormats(ajv);
const shape = ajv.compile({
  $ref: "#/components/schemas/QuestionGroup",
  components: spec.components,
});
const id = "019535d9-3df7-79fb-b466-fa907fa17f9e";

function fixture(): components["schemas"]["QuestionGroup"] {
  return {
    id,
    title: "Đọc và trả lời",
    members: [{ questionId: id, optionOrder: "fixed" }],
    stimuli: [
      {
        id,
        title: "Bài đọc",
        content: {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: [{ type: "gap", id: "gap-1", label: "1" }],
            },
          ],
        },
        gaps: [{ kind: "question", questionId: id, gapId: "gap-1" }],
      },
    ],
    recordings: [
      {
        id,
        assetId: id,
        policy: {
          maxPlays: 2,
          allowSeek: false,
          showTranscriptAfterSubmit: false,
        },
        transcript: "Nội dung chỉ dành cho giáo viên",
      },
    ],
  };
}

test("group structural schema and generated types include the approved context contract", () => {
  const group = fixture();
  expect(shape(group), JSON.stringify(shape.errors)).toBe(true);
  group.stimuli[0]!.gaps = [
    { kind: "blank", questionId: id, gapId: "gap-1", blankGapId: "answer-1" },
  ];
  expect(shape(group), JSON.stringify(shape.errors)).toBe(true);
  expect(
    shape({ id, title: "Nhóm mới", members: [], stimuli: [], recordings: [] }),
  ).toBe(true);
});

test.each([
  ["unknown group field", { ...fixture(), sourcePath: "/private/source.docx" }],
  ["empty title", { ...fixture(), title: "" }],
  ["oversized title", { ...fixture(), title: "ế".repeat(201) }],
  [
    "unknown option order",
    { ...fixture(), members: [{ questionId: id, optionOrder: "random" }] },
  ],
  [
    "too many members",
    {
      ...fixture(),
      members: Array.from({ length: 201 }, () => ({
        questionId: id,
        optionOrder: "shuffle",
      })),
    },
  ],
  ["missing members", { id, title: "Nhóm mới", stimuli: [], recordings: [] }],
  ["null members", { ...fixture(), members: null }],
  [
    "unsafe instructions",
    {
      ...fixture(),
      instructions: {
        format: "semantic_v1",
        blocks: [{ type: "audio", assetId: id, label: "Bài nghe" }],
      },
    },
  ],
])("rejects %s structurally", (_, value) => {
  expect(shape(value)).toBe(false);
});

test.each([
  { kind: "blank", questionId: id, gapId: "gap-1" },
  { kind: "question", questionId: id, gapId: "gap-1", blankGapId: "answer-1" },
  { kind: "question", questionId: id, gapId: "gap-1", isCorrect: true },
  { kind: "choice", questionId: id, gapId: "gap-1" },
  { kind: "question", questionId: "printed question 1", gapId: "gap-1" },
  { kind: "blank", questionId: id, gapId: "gap-1", blankGapId: " " },
])("rejects malformed gap binding %#", (gap) => {
  const group = fixture();
  expect(shape({ ...group, stimuli: [{ ...group.stimuli[0], gaps: [gap] }] })).toBe(
    false,
  );
});

test("teacher-only recording metadata is separate from learner material and active payloads", () => {
  const material = propertyNames(spec, spec.components.schemas.GroupStimulus);
  expect(material.has("transcript")).toBe(false);
  expect(material.has("acceptedAnswers")).toBe(false);
  expect(material.has("isCorrect")).toBe(false);
  const recording = propertyNames(spec, spec.components.schemas.GroupRecording);
  expect(recording.has("transcript")).toBe(true);
  expect(recording.has("isCorrect")).toBe(false);
});

test("editable groups carry complete member inputs and retryable media state", () => {
  const storedShape = ajv.compile({
    $ref: "#/components/schemas/StoredQuestionGroup",
    components: spec.components,
  });
  const stored: components["schemas"]["StoredQuestionGroup"] = {
    bundle: {
      group: fixture(),
      questions: [
        {
          id,
          input: {
            type: "single_choice",
            prompt: "Chọn từ",
            points: 1,
            options: [
              { text: "Một", isCorrect: true },
              { text: "Hai", isCorrect: false },
            ],
          },
        },
      ],
    },
    ownerSectionId: null,
    revision: 1,
    archivedAt: null,
    createdAt: "2026-09-24T00:00:00Z",
    updatedAt: "2026-09-24T00:00:00Z",
    assets: [],
    unavailableAssetIds: [id],
  };
  expect(storedShape(stored), JSON.stringify(storedShape.errors)).toBe(true);
  expect(storedShape({ ...stored, unavailableAssetIds: undefined })).toBe(false);
  expect(
    storedShape({ ...stored, bundle: { ...stored.bundle, questions: [{ id }] } }),
  ).toBe(false);
  expect(storedShape({ ...stored, revision: 0 })).toBe(false);
  expect(storedShape({ ...stored, testUpdatedAt: "not a revision timestamp" })).toBe(
    false,
  );
});

test("bank summaries do not contain group content, answer keys or source documents", () => {
  const summary = propertyNames(spec, spec.components.schemas.QuestionGroupSummary);
  for (const field of [
    "bundle",
    "questions",
    "stimuli",
    "recordings",
    "transcript",
    "sourcePath",
    "sourceUrl",
  ]) {
    expect(summary.has(field)).toBe(false);
  }
  const unitShape = ajv.compile({
    $ref: "#/components/schemas/DraftSectionUnit",
    components: spec.components,
  });
  expect(unitShape({ kind: "group", id })).toBe(true);
  expect(unitShape({ kind: "question", id })).toBe(true);
  expect(unitShape({ kind: "material", id })).toBe(false);
});
