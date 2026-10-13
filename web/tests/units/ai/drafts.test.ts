import { describe, expect, it } from "vitest";
import type { AiFinding, AiGeneratedQuestion } from "@/features/ai/api";
import { draftQuestion, withFix } from "@/features/question-bank/aiDrafts";
import {
  questionSchema,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";

const OPTION = "018f0000-0000-7000-8000-0000000000d1";
const BLANK = "018f0000-0000-7000-8000-0000000000f1";

function draft(question: AiGeneratedQuestion) {
  return draftQuestion({ question, level: "b1" }, "Bản nháp AI");
}

describe("a drafted question", () => {
  it.each<AiGeneratedQuestion>([
    {
      type: "single_choice",
      prompt: "Pick one.",
      options: ["Yes", "No"],
      answerKey: ["No"],
      explanation: "Because.",
    },
    {
      type: "multiple_choice",
      prompt: "Pick two.",
      options: ["A", "B", "C"],
      answerKey: ["A", "C"],
      explanation: "",
    },
    {
      type: "true_false",
      prompt: "Parks are decoration.",
      options: ["True", "False"],
      answerKey: ["False"],
      explanation: "",
    },
    {
      type: "fill_blank",
      prompt: "They are called ___.",
      answerKey: ["play streets", "play street"],
      explanation: "",
    },
    {
      type: "short_answer",
      prompt: "What comes first?",
      answerKey: ["green space"],
      explanation: "",
    },
  ])("is a $type the bank write accepts, tagged and levelled", (question) => {
    const values = draft(question);
    expect(questionSchema.safeParse(values).success).toBe(true);
    expect(values.tags).toEqual(["Bản nháp AI"]);
    expect(values.level).toBe("b1");
    expect(values.points).toBe(1);
  });

  it("ticks the key's options and keeps an explanation only when there is one", () => {
    const values = draft({
      type: "multiple_choice",
      prompt: "Pick two.",
      options: ["A", "B", "C"],
      answerKey: ["A", "C"],
      explanation: " ",
    });
    expect(values.options.map((option) => option.isCorrect)).toEqual([
      true,
      false,
      true,
    ]);
    expect(values.explanation).toBeNull();
  });

  it("writes the canonical true/false options with the key ticked", () => {
    const values = draft({
      type: "true_false",
      prompt: "Parks are decoration.",
      answerKey: ["False"],
      explanation: "",
    });
    expect(values.options).toEqual([
      { id: null, text: "True", isCorrect: false },
      { id: null, text: "False", isCorrect: true },
    ]);
  });

  it("makes a blank's first gap its one blank, or adds one at the end", () => {
    const gap = draft({
      type: "fill_blank",
      prompt: "They are ___ and ____.",
      answerKey: ["parks"],
      explanation: "",
    });
    expect(gap.prompt).toBe("They are {{1}} and ____.");
    expect(gap.blanks).toEqual([
      { id: null, ordinal: 1, acceptedAnswers: ["parks"], caseSensitive: false },
    ]);
    expect(
      draft({
        type: "fill_blank",
        prompt: "Name it:",
        answerKey: ["x"],
        explanation: "",
      }).prompt,
    ).toBe("Name it: {{1}}");
  });

  it("carries no tag when tagging is off", () => {
    expect(
      draftQuestion(
        {
          question: {
            type: "short_answer",
            prompt: "Q?",
            answerKey: ["a"],
            explanation: "",
          },
          level: "c1",
        },
        null,
      ).tags,
    ).toEqual([]);
  });
});

describe("a check's fix", () => {
  const values: QuestionValues = {
    ...draft({
      type: "single_choice",
      prompt: "Pick one.",
      options: ["Yes", "No"],
      answerKey: ["No"],
      explanation: "",
    }),
    options: [
      { id: OPTION, text: "Yes", isCorrect: false },
      { id: "018f0000-0000-7000-8000-0000000000d2", text: "No", isCorrect: true },
    ],
    blanks: [],
  };

  function fixed(fix: AiFinding["fix"], base: QuestionValues = values) {
    return withFix(base, {
      severity: "minor",
      questionId: "018f0000-0000-7000-8000-0000000000b1",
      number: 1,
      type: base.type,
      title: "Pick one.",
      issue: "An issue.",
      fix,
    });
  }

  it("rewrites the option it names", () => {
    expect(
      fixed({
        kind: "option_text",
        description: "Reword.",
        payload: { optionId: OPTION, text: "Yes, always" },
      }).options[0],
    ).toEqual({ id: OPTION, text: "Yes, always", isCorrect: false });
  });

  it("adds an accepted answer once", () => {
    const blank: QuestionValues = {
      ...values,
      type: "fill_blank",
      options: [],
      prompt: "Name {{1}}.",
      blanks: [
        { id: BLANK, ordinal: 1, acceptedAnswers: ["park"], caseSensitive: false },
      ],
    };
    const add = {
      kind: "accepted_answer" as const,
      description: "Add.",
      payload: { blankId: BLANK, text: "parks" },
    };
    expect(fixed(add, blank).blanks[0]!.acceptedAnswers).toEqual(["park", "parks"]);
    expect(fixed(add, fixed(add, blank)).blanks[0]!.acceptedAnswers).toEqual([
      "park",
      "parks",
    ]);
  });

  it("replaces the explanation with plain text, and leaves advice alone", () => {
    expect(
      fixed({
        kind: "explanation",
        description: "Explain.",
        payload: { text: "Because." },
      }).explanation,
    ).toBe("Because.");
    expect(fixed({ kind: "none", description: "Think about it." })).toBe(values);
  });
});
