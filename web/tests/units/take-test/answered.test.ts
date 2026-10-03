import { describe, expect, it } from "vitest";
import { answered } from "@/features/take-test/answered";
import type { StudentQuestion } from "@/features/take-test/api";

/** Any question; only `blanks` is ever read. */
function question(over: Partial<StudentQuestion> = {}): StudentQuestion {
  return {
    id: "q1",
    sectionId: "s1",
    type: "single_choice",
    prompt: "…",
    points: 1,
    ...over,
  };
}

const twoBlanks = question({
  type: "fill_blank",
  blanks: [
    { id: "b1", ordinal: 1, caseSensitive: false },
    { id: "b2", ordinal: 2, caseSensitive: false },
  ],
});

/** One meaning of "đã trả lời", shared by the dots and the review's counts. */
describe("answered", () => {
  it("is false for nothing at all", () => {
    expect(answered(question(), undefined)).toBe(false);
  });

  it("needs a selection for a choice", () => {
    expect(answered(question(), { type: "choice", optionIds: [] })).toBe(false);
    expect(answered(question(), { type: "choice", optionIds: ["o1"] })).toBe(true);
  });

  it("needs a real pick for true/false", () => {
    expect(answered(question(), { type: "true_false", value: false })).toBe(true);
    expect(answered(question(), { type: "true_false", value: null } as never)).toBe(
      false,
    );
  });

  it("is not answered by a type it does not know", () => {
    expect(
      answered(question(), { type: "essay", value: "words" } as never),
    ).toBeFalsy();
    expect(answered(question(), { value: "words" } as never)).toBeFalsy();
  });

  it("needs text, whitespace not counting", () => {
    expect(answered(question(), { type: "text", value: "   " })).toBe(false);
    expect(
      answered(question(), {
        type: "text",
        value:
          "\u0009\u000a\u000b\u000c\u000d\u0020\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff",
      }),
    ).toBe(false);
    expect(answered(question(), { type: "text", value: "\u200b" })).toBe(true);
    expect(answered(question(), { type: "text", value: "I wake up at six." })).toBe(
      true,
    );
  });
});

/** Per-blank grading (O-17): one blank of four scores a quarter, not nothing. */
describe("answered, for a fill_blank", () => {
  it("is false for a question that has no blanks", () => {
    const none = question({ type: "fill_blank" });
    expect(answered(none, { type: "fill_blank", values: {} })).toBe(false);
  });

  it("is false when no blank has been typed into", () => {
    expect(answered(twoBlanks, { type: "fill_blank", values: {} })).toBe(false);
    expect(
      answered(twoBlanks, { type: "fill_blank", values: { b1: " ", b2: "" } }),
    ).toBe(false);
  });

  it("is false when some blanks are filled and others are not", () => {
    expect(answered(twoBlanks, { type: "fill_blank", values: { b1: "went" } })).toBe(
      false,
    );
    expect(
      answered(twoBlanks, { type: "fill_blank", values: { b1: "went", b2: "  " } }),
    ).toBe(false);
    expect(
      answered(twoBlanks, { type: "fill_blank", values: { b1: "went", b2: " \t" } }),
    ).toBe(false);
  });

  it("is true only once every blank has content", () => {
    expect(
      answered(twoBlanks, { type: "fill_blank", values: { b1: "went", b2: "has" } }),
    ).toBe(true);
  });

  it("ignores values that belong to no blank on this question", () => {
    expect(
      answered(twoBlanks, {
        type: "fill_blank",
        values: { b1: "went", b2: "has", stale: "x" },
      }),
    ).toBe(true);
    expect(answered(twoBlanks, { type: "fill_blank", values: { stale: "x" } })).toBe(
      false,
    );
  });
});
