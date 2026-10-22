import { describe, expect, it } from "vitest";
import vi from "@/lib/i18n/locales/vi.json";
import en from "@/lib/i18n/locales/en.json";

/**
 * The engine names a question's type as the builder's type menu does
 * (T-D4.5): its own keys, takeTest.type.*, hold the values of
 * questionEditor.type.*, so the two cannot drift.
 */
describe("the engine's type names", () => {
  it.each([
    ["vi", vi],
    ["en", en],
  ] as const)("are the builder's in %s", (_, locale) => {
    expect(locale.takeTest.type).toEqual(locale.questionEditor.type);
  });

  it("are the deck's in English", () => {
    expect(en.takeTest.type).toEqual({
      single_choice: "Single choice",
      multiple_choice: "Multiple choice",
      true_false: "True / False",
      fill_blank: "Fill in the blank",
      short_answer: "Short answer",
      tfng: "True / False / Not given",
      matching: "Matching",
      essay: "Essay",
    });
  });
});
