import { describe, expect, it, vi } from "vitest";
import {
  toFormValues,
  listQuestions,
  updateQuestion,
} from "@/features/question-bank/api";
import { emptyQuestion, questionSchema } from "@/features/question-bank/questionSchema";
import type { AdminQuestion } from "@/features/question-bank/api";
import { api } from "@/lib/api/client";

vi.mock("@/lib/api/client", () => ({ api: vi.fn() }));

const legacy: AdminQuestion = {
  id: "019535d9-3df7-79fb-b466-fa907fa17f9e",
  type: "single_choice",
  prompt: "Pick",
  points: 1,
  tags: [],
  level: "pre_a1",
  skill: "reading",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  options: Array.from({ length: 9 }, (_, i) => ({
    id: "019535d9-3df7-79fb-b466-fa907fa17f9e",
    ordinal: i,
    text: String(i),
    isCorrect: i === 0,
  })),
};
const question: AdminQuestion = { ...legacy, options: legacy.options!.slice(0, 8) };

describe("bank metadata preservation", () => {
  it("keeps a stored ninth option in the form and refuses to save it, as the server does", () => {
    const values = toFormValues(legacy);
    expect(values.options).toHaveLength(9);
    const result = questionSchema.safeParse(values);
    expect(result.success).toBe(false);
    expect(result.error?.issues.map((issue) => issue.message)).toEqual([
      "questionEditor.errors.maxOptions",
    ]);
  });
  it("keeps metadata and eight options through the real form schema and submit", () => {
    const parsed = questionSchema.parse(toFormValues(question));
    expect(parsed.level).toBe("pre_a1");
    expect(parsed.skill).toBe("reading");
    expect(parsed.options).toHaveLength(8);
    updateQuestion(question.id, parsed);
    expect(api).toHaveBeenLastCalledWith("patch", "/teacher/questions/{id}", {
      path: { id: question.id },
      body: parsed,
    });
  });
  it("normalizes unset defaults and refuses unknown enums", () => {
    expect(emptyQuestion()).toMatchObject({ level: null, skill: null });
    const unset = questionSchema.parse({
      ...toFormValues(question),
      level: undefined,
      skill: undefined,
    });
    updateQuestion(question.id, unset);
    expect(api).toHaveBeenLastCalledWith("patch", "/teacher/questions/{id}", {
      path: { id: question.id },
      body: { ...unset, level: null, skill: null },
    });
    for (const field of ["level", "skill"])
      expect(
        questionSchema.safeParse({ ...toFormValues(question), [field]: "unknown" })
          .success,
      ).toBe(false);
  });
  it("forwards repeatable dimensions and any/all without changing defaults", () => {
    const signal = new AbortController().signal;
    listQuestions(
      {
        level: ["a1", "c2"],
        skill: ["reading", "writing"],
        tag: ["one", "two"],
        tagMatch: "all",
      },
      signal,
    );
    expect(api).toHaveBeenLastCalledWith("get", "/teacher/questions", {
      query: {
        level: ["a1", "c2"],
        skill: ["reading", "writing"],
        tag: ["one", "two"],
        tagMatch: "all",
      },
      signal,
    });
    listQuestions();
    expect(api).toHaveBeenLastCalledWith("get", "/teacher/questions", { query: {} });
  });
});
