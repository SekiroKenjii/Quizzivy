import Ajv from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import type { components } from "@/lib/api/schema";
import { loadSpec } from "@tests/support/openapi";

const spec = loadSpec();
const ajv = new Ajv({ strict: false, validateFormats: true });
addFormats(ajv);
const validate = ajv.compile({
  $ref: "#/components/schemas/GradingQueue",
  components: spec.components,
});
const id = "019535d9-3df7-79fb-b466-fa907fa17f9e";

function item(): components["schemas"]["GradingQueueItem"] {
  return {
    attemptId: id,
    questionId: id,
    assignmentId: id,
    assignmentTitle: "Bài kiểm tra",
    studentId: id,
    studentName: "Nguyễn An",
    questionNumber: 2,
    type: "fill_blank",
    prompt: "Chọn từ",
    answer: { type: "text", value: "Câu trả lời" },
    points: 12345.67,
    score: null,
    comment: null,
    promptContent: {
      format: "semantic_v1",
      blocks: [
        { type: "paragraph", content: [{ type: "text", text: "Chọn từ", marks: [] }] },
      ],
    },
    options: [{ id, ordinal: 0, text: "Đúng", isCorrect: true }],
    blanks: [
      {
        id,
        ordinal: 1,
        gapId: "gap-1",
        acceptedAnswers: ["secret"],
        caseSensitive: true,
      },
    ],
    sampleAnswer: "Teacher sample",
    transcript: "Teacher transcript",
    sharedContext: { groups: [], transcripts: { [id]: "Shared teacher transcript" } },
  };
}
function fixture(): components["schemas"]["GradingQueue"] {
  return {
    groups: [
      {
        key: id,
        kind: "student",
        label: "Nguyễn An",
        sub: "Bài kiểm tra",
        remaining: 201,
      },
    ],
    items: [item()],
    answersRemaining: 201,
    studentsWaiting: 2,
  };
}

test("queue operation records historical filtering, default mode and grading authority", () => {
  const op = spec.paths["/teacher/grading/queue"]!.get;
  expect(op.operationId).toBe("listGradingQueue");
  expect(op["x-permission"]).toBe("teaching.grading");
  expect(op["x-resource-list"]).toBe("attempt");
  expect(op["x-resource"]).toEqual([
    { in: "query", name: "assignmentId", kind: "assignment" },
    { in: "query", name: "studentId", kind: "student" },
  ]);
  expect(op.parameters.find((p: { name: string }) => p.name === "mode").schema).toEqual(
    { type: "string", enum: ["student", "question"], default: "student" },
  );
  expect(op.description).toContain("BOTH AssignmentIDs and Papers reach");
});

test("closed typed queue preserves flat rich teacher keys and complete counts", () => {
  expect(validate(fixture()), JSON.stringify(validate.errors)).toBe(true);
  expect(
    validate({ groups: [], items: [], answersRemaining: 0, studentsWaiting: 0 }),
    JSON.stringify(validate.errors),
  ).toBe(true);
  const question = fixture();
  question.groups[0] = {
    key: `${id}:${id}`,
    kind: "question",
    label: "2",
    sub: "Bài kiểm tra",
    remaining: 201,
  };
  question.items = Array.from({ length: 200 }, item);
  expect(validate(question), JSON.stringify(validate.errors)).toBe(true);
});

test.each([
  ["missing null score", { ...item(), score: undefined }],
  ["missing null comment", { ...item(), comment: undefined }],
  ["bad question number", { ...item(), questionNumber: 0 }],
  ["private account field", { ...item(), email: "private@example.com" }],
  ["flattened accepted answers", { ...item(), acceptedAnswers: ["secret"] }],
  [
    "unknown answer",
    { ...item(), answer: { type: "text", value: "x", isCorrect: true } },
  ],
  [
    "unknown rich field",
    {
      ...item(),
      promptContent: { format: "semantic_v1", blocks: [], sourcePath: "private" },
    },
  ],
  [
    "missing blank key",
    { ...item(), blanks: [{ id, ordinal: 1, caseSensitive: false }] },
  ],
])("rejects %s", (_, bad) => {
  expect(validate({ ...fixture(), items: [bad] })).toBe(false);
});

test.each([
  ["201 item prefix", { ...fixture(), items: Array.from({ length: 201 }, item) }],
  ["null groups", { ...fixture(), groups: null }],
  ["negative full count", { ...fixture(), answersRemaining: -1 }],
  [
    "zero represented group",
    { ...fixture(), groups: [{ ...fixture().groups[0], remaining: 0 }] },
  ],
  [
    "display label as key",
    { ...fixture(), groups: [{ ...fixture().groups[0], key: "Nguyễn An" }] },
  ],
  ["unknown root field", { ...fixture(), completionCandidates: [] }],
])("rejects %s", (_, bad) => {
  expect(validate(bad)).toBe(false);
});
