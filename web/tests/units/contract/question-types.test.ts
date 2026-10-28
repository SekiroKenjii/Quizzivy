import { describe, expect, it } from "vitest";
import Ajv from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import type { components } from "@/lib/api/schema";
import { loadSpec } from "@tests/support/openapi";

/**
 * R6's question types on the wire (T-R6.2): each new shape accepts what the
 * server will send, and refuses the values its rules exclude. Fails when a
 * schema loses a field or a bound, never on a handler.
 */

const spec = loadSpec();
const ajv = new Ajv({ strict: false, allErrors: true });
addFormats(ajv);
ajv.addSchema(spec, "openapi");
const schema = (name: string) =>
  ajv.compile({ $ref: `openapi#/components/schemas/${name}` });
const gradeBody = ajv.compile({
  $ref: "openapi#/paths/~1teacher~1attempts~1%7Bid%7D~1grade/post/requestBody/content/application~1json/schema",
});

const [a, b, c, d] = [
  "019535d9-3df7-79fb-b466-fa907fa17f9a",
  "019535d9-3df7-79fb-b466-fa907fa17f9b",
  "019535d9-3df7-79fb-b466-fa907fa17f9c",
  "019535d9-3df7-79fb-b466-fa907fa17f9d",
];
const at = "2026-10-21T00:00:00Z";

function studentMatching(): components["schemas"]["StudentQuestion"] {
  return {
    id: a,
    sectionId: b,
    type: "matching",
    prompt: "Match each heading to a paragraph.",
    points: 2,
    options: [
      { id: c, key: "i", text: "Causes" },
      { id: d, key: "ii", text: "Effects" },
    ],
    matchItems: [
      { id: a, key: "A", text: "Paragraph A" },
      { id: b, key: "B", text: "Paragraph B" },
    ],
    matchAllowReuse: false,
  };
}

function adminEssay(): components["schemas"]["AdminQuestion"] {
  return {
    id: a,
    type: "essay",
    prompt: "Write about a city you know.",
    points: 9,
    tags: [],
    level: null,
    skill: "writing",
    sampleAnswer: "A model answer.",
    rubric: [
      { id: b, ordinal: 0, label: "Task response" },
      { id: c, ordinal: 1, label: "Coherence" },
    ],
    createdAt: at,
    updatedAt: at,
  };
}

function adminMatching(): components["schemas"]["AdminQuestion"] {
  return {
    id: a,
    type: "matching",
    prompt: "Match.",
    points: 2,
    tags: [],
    level: null,
    skill: null,
    options: [
      { id: c, ordinal: 0, key: "i", text: "Causes", isCorrect: false },
      { id: d, ordinal: 1, key: "ii", text: "Effects", isCorrect: false },
    ],
    matchItems: [
      { id: b, ordinal: 0, key: "A", text: "Paragraph A", correctOptionId: d },
    ],
    matchAllowReuse: true,
    createdAt: at,
    updatedAt: at,
  };
}

describe("R6 question types on the wire", () => {
  it("takes the three new types everywhere a type goes", () => {
    for (const type of ["tfng", "matching", "essay"])
      expect(schema("QuestionType")(type), type).toBe(true);
    for (const type of ["tfng", "matching", "essay"])
      expect(schema("AiQuestionType")(type), `AI ${type}`).toBe(true);
  });

  it("gives a student a matching paper without its key", () => {
    const question = studentMatching();
    expect(schema("StudentQuestion")(question)).toBe(true);
    const keyed = {
      ...question,
      matchItems: [{ ...question.matchItems![0]!, correctOptionId: c }],
    };
    expect(schema("StudentQuestion")(keyed)).toBe(false);
  });

  it("gives a teacher the rubric, the items and their keys", () => {
    expect(schema("AdminQuestion")(adminEssay())).toBe(true);
    expect(schema("AdminQuestion")(adminMatching())).toBe(true);
    const seventh = adminEssay();
    seventh.rubric = Array.from({ length: 7 }, (_, ordinal) => ({
      id: a,
      ordinal,
      label: "Criterion",
    }));
    expect(schema("AdminQuestion")(seventh)).toBe(false);
  });

  it("bounds selectCount, maxWords and a match item's option index in a write", () => {
    const input = schema("QuestionInput");
    const choose = (selectCount: number) => ({
      type: "multiple_choice",
      prompt: "Choose TWO",
      points: 2,
      options: [1, 2, 3].map((n) => ({ text: String(n), isCorrect: n < 3 })),
      selectCount,
    });
    expect(input(choose(2))).toBe(true);
    expect(input(choose(1))).toBe(false);
    expect(input(choose(8))).toBe(false);
    const limit = (maxWords: number) => ({
      type: "short_answer",
      prompt: "Name it",
      points: 1,
      maxWords,
    });
    expect(input(limit(3))).toBe(true);
    expect(input(limit(0))).toBe(false);
    expect(input(limit(101))).toBe(false);
    const matching = (correctOption: number) => ({
      type: "matching",
      prompt: "Match",
      points: 1,
      options: [
        { text: "i", isCorrect: false },
        { text: "ii", isCorrect: false },
      ],
      matchItems: [{ text: "A", correctOption }],
      matchAllowReuse: false,
    });
    expect(input(matching(1))).toBe(true);
    expect(input(matching(12))).toBe(false);
    expect(input(matching(-1))).toBe(false);
  });

  it("answers a matching question with pairs of ids", () => {
    const answer = schema("Answer");
    expect(answer({ type: "matching", pairs: { [a]: c, [b]: d } })).toBe(true);
    expect(answer({ type: "matching", pairs: { [a]: "ii" } })).toBe(false);
  });

  it("shows bands, criteria and correct matches on a result", () => {
    const result: components["schemas"]["ResultQuestion"] = {
      id: a,
      sectionId: b,
      type: "essay",
      prompt: "Write.",
      points: 9,
      answer: { type: "text", value: "My essay." },
      band: 6.5,
      criteria: [{ id: c, label: "Task response", band: 7 }],
      skill: "writing",
      correctMatches: [{ itemId: a, optionId: c }],
    };
    expect(schema("ResultQuestion")(result)).toBe(true);
    expect(schema("ResultQuestion")({ ...result, band: 6.25 })).toBe(false);
    expect(
      schema("AttemptScore")({ earned: 6.5, total: 9, pendingManual: 0, band: 6.5 }),
    ).toBe(true);
    expect(
      schema("AttemptScore")({ earned: 2, total: 9, pendingManual: 0, band: 2.5 }),
    ).toBe(false);
  });

  it("grades with points, a band in half steps from 3 to 9, or criteria", () => {
    const grade = (item: object) => gradeBody({ items: [{ questionId: a, ...item }] });
    expect(grade({ points: 1 })).toBe(true);
    expect(grade({ band: 6.5 })).toBe(true);
    expect(grade({ criteria: [{ criterionId: b, band: 7 }] })).toBe(true);
    expect(grade({ band: 6.25 })).toBe(false);
    expect(grade({ band: 9.5 })).toBe(false);
    expect(grade({ criteria: [] })).toBe(false);
  });

  it("lets the Word import draft hold tfng, Choose N and a word limit", () => {
    const draft = schema("ImportDraftQuestion");
    const question = {
      id: "q1",
      label: "1",
      type: "tfng",
      prompt: { format: "legacy_markdown_v1", markdown: "The city is old." },
      options: [],
      blanks: [],
      answer: { state: "unknown", optionIds: [], evidence: [] },
      points: "1",
      origins: {
        type: "inferred_structure",
        prompt: "source_explicit",
        options: "defaulted",
        answer: "defaulted",
        points: "defaulted",
      },
      source: [],
    };
    expect(draft(question), JSON.stringify(draft.errors)).toBe(true);
    expect(draft({ ...question, type: "multiple_choice", selectCount: 2 })).toBe(true);
    expect(draft({ ...question, type: "short_answer", maxWords: 2 })).toBe(true);
    expect(draft({ ...question, type: "matching" })).toBe(false);
  });
});
