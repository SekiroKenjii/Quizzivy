import { describe, expect, it } from "vitest";
import { aiGuardOf, aiReadsOf } from "@/features/question-bank/questionAi";
import type { QuestionValues } from "@/features/question-bank/questionSchema";

function values(overrides: Partial<QuestionValues>): QuestionValues {
  return {
    type: "single_choice",
    level: null,
    skill: null,
    prompt: "Write about your city.",
    promptContent: null,
    mediaAssetId: null,
    mediaAlt: null,
    audio: null,
    transcript: null,
    options: [],
    blanks: [],
    points: 1,
    explanation: null,
    sampleAnswer: null,
    tags: [],
    ...overrides,
  };
}

const choices = [
  { id: null, text: "teacher", isCorrect: false },
  { id: null, text: "nurse", isCorrect: false },
];

describe("asking the assistant about a new type", () => {
  it("asks an essay for its sample answer, which is its key", () => {
    expect(aiGuardOf(values({ type: "essay", rubric: [] }))).toBe("ai.guard.sample");
    expect(aiGuardOf(values({ type: "essay", sampleAnswer: "  ", rubric: [] }))).toBe(
      "ai.guard.sample",
    );
    expect(
      aiGuardOf(values({ type: "essay", sampleAnswer: "Two places.", rubric: [] })),
    ).toBeNull();
  });

  it("asks a matching question for every item's choice", () => {
    const matching = (correct: number[]) =>
      values({
        type: "matching",
        options: choices,
        matchAllowReuse: false,
        matchItems: correct.map((correctOption, i) => ({
          id: null,
          text: `item ${i}`,
          correctOption,
        })),
      });
    expect(aiGuardOf(matching([1, 0]))).toBeNull();
    expect(aiGuardOf(matching([1, -1]))).toBe("ai.guard.key");
    expect(aiGuardOf(matching([]))).toBe("ai.guard.key");
  });

  it("reads a tfng key like any choice", () => {
    const tfng = values({
      type: "tfng",
      options: [
        { id: null, text: "True", isCorrect: false },
        { id: null, text: "False", isCorrect: false },
        { id: null, text: "Not given", isCorrect: true },
      ],
    });
    expect(aiGuardOf(tfng)).toBeNull();
  });

  it("counts a matching item's key as something the assistant reads", () => {
    const item = (correctOption: number) =>
      values({
        type: "matching",
        options: choices,
        matchAllowReuse: false,
        matchItems: [{ id: null, text: "Lan", correctOption }],
      });
    expect(aiReadsOf(item(0))).not.toBe(aiReadsOf(item(1)));
  });
});
