import { describe, expect, it } from "vitest";
import {
  FOREVER,
  retentionMonths,
  retentionShortened,
  retentionValue,
} from "@/features/organization/retention";
import { sectionOfField } from "@/features/organization/systemSections";

const SAVED = {
  keepAnswersMonths: 24,
  keepFocusEventsMonths: 13,
  anonymiseDisabledAfterMonths: null,
} as const;

describe("retentionShortened", () => {
  it("is false for the same policy and for every lengthening", () => {
    expect(retentionShortened(SAVED, SAVED)).toBe(false);
    expect(retentionShortened(SAVED, { ...SAVED, keepAnswersMonths: null })).toBe(
      false,
    );
    expect(retentionShortened(SAVED, { ...SAVED, keepFocusEventsMonths: 24 })).toBe(
      false,
    );
  });

  it.each([
    ["answers", { ...SAVED, keepAnswersMonths: 12 }],
    ["focus events", { ...SAVED, keepFocusEventsMonths: 6 }],
    ["anonymising", { ...SAVED, anonymiseDisabledAfterMonths: 24 }],
  ] as const)("is true when only %s are kept for less time", (_, next) => {
    expect(retentionShortened(SAVED, next)).toBe(true);
  });

  it("treats forever as the longest answer to keep", () => {
    expect(
      retentionShortened(
        { ...SAVED, keepAnswersMonths: null },
        { ...SAVED, keepAnswersMonths: 24 },
      ),
    ).toBe(true);
  });
});

describe("retention select values", () => {
  it("round-trips months and never", () => {
    expect(retentionValue(null)).toBe(FOREVER);
    expect(retentionValue(12)).toBe("12");
    expect(retentionMonths(FOREVER)).toBeNull();
    expect(retentionMonths("24")).toBe(24);
  });
});

describe("the sections own their form paths", () => {
  it("opens Data & privacy for a retention field", () => {
    expect(sectionOfField("retention.keepAnswersMonths")?.id).toBe("data");
    expect(sectionOfField("signIn.google")?.id).toBe("sign-in");
  });
});
