import { describe, expect, it } from "vitest";
import { answered } from "@/features/take-test/answered";

/** One meaning of "đã trả lời", shared by the dots and the review's counts. */
describe("answered", () => {
  it("is false for nothing at all", () => {
    expect(answered(undefined)).toBe(false);
  });

  it("needs a selection for a choice", () => {
    expect(answered({ type: "choice", optionIds: [] })).toBe(false);
    expect(answered({ type: "choice", optionIds: ["o1"] })).toBe(true);
  });

  it("needs a real pick for true/false", () => {
    expect(answered({ type: "true_false", value: false })).toBe(true);
  });

  it("needs any blank filled, whitespace not counting", () => {
    expect(answered({ type: "fill_blank", values: { b1: " ", b2: "" } })).toBe(false);
    expect(answered({ type: "fill_blank", values: { b1: "", b2: "went" } })).toBe(true);
  });

  it("needs text, whitespace not counting", () => {
    expect(answered({ type: "text", value: "   " })).toBe(false);
    expect(answered({ type: "text", value: "I wake up at six." })).toBe(true);
  });
});
