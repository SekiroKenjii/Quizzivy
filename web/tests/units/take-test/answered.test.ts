import { describe, expect, it } from "vitest";
import { answered } from "@/features/take-test/answered";
import type { StudentQuestion } from "@/features/take-test/api";

/** Any question; only `blanks` is ever read. */
function question(over: Partial<StudentQuestion> = {}): StudentQuestion {
  return { id: "q1", type: "single_choice", prompt: "…", points: 1, ...over };
}

const twoBlanks = question({
  type: "fill_blank",
  blanks: [
    { id: "b1", ordinal: 1 },
    { id: "b2", ordinal: 2 },
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
  });

  it("needs text, whitespace not counting", () => {
    expect(answered(question(), { type: "text", value: "   " })).toBe(false);
    expect(answered(question(), { type: "text", value: "I wake up at six." })).toBe(
      true,
    );
  });
});

/**
 * Per-blank grading (O-17) is what makes a partly-filled question worth
 * warning about: one blank of four scores a quarter, so calling it done hides
 * three quarters of the marks from the one screen that exists to catch that.
 */
describe("answered, for a fill_blank", () => {
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
  });

  it("is true only once every blank has content", () => {
    expect(
      answered(twoBlanks, { type: "fill_blank", values: { b1: "went", b2: "has" } }),
    ).toBe(true);
  });

  // A value keyed to a blank this question does not have proves nothing about
  // the blanks it does have.
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
