import { describe, expect, it } from "vitest";
import { aiPreferencesOf } from "@/features/auth/preferences";

describe("aiPreferencesOf", () => {
  it("fills every key the account has not stored with the contract's default", () => {
    expect(aiPreferencesOf(undefined, "vi")).toEqual({
      language: "vi",
      tagDrafts: true,
      readStudentAnswers: true,
      readImports: false,
    });
    expect(aiPreferencesOf({}, "en").language).toBe("en");
  });

  it("keeps what the account stored, a false included", () => {
    expect(
      aiPreferencesOf(
        { language: "en", tagDrafts: false, readStudentAnswers: false },
        "vi",
      ),
    ).toEqual({
      language: "en",
      tagDrafts: false,
      readStudentAnswers: false,
      readImports: false,
    });
  });
});
