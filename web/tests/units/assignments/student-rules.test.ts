import { describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { studentRules, type RulesDraft } from "@/features/assignments/studentRules";

/**
 * The deck calls this panel "the feature": integrity policy is the setting
 * teachers get wrong most often, because the words that reach the student are
 * three screens away from the switch. These tests hold the switch and the
 * sentence together.
 */
function draft(overrides: Partial<RulesDraft> = {}): RulesDraft {
  return {
    durationMinutes: 45,
    maxAttempts: 1,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: true },
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
    ...overrides,
  };
}

function rules(over: Partial<RulesDraft> = {}) {
  return studentRules(draft(over), i18n.t, "vi");
}

describe("what the student will read", () => {
  it("always states the clock, because it is the one rule with no switch", () => {
    expect(rules()[0]).toContain("45 phút");
  });

  it("names what is hidden as well as what is shown", () => {
    const sentence = rules().at(-1)!;
    expect(sentence).toContain("điểm");
    expect(sentence).toContain("giải thích");
    // The half that stops a student hunting for a missing answer key.
    expect(sentence).toContain("không xem");
    expect(sentence).toContain("đáp án đúng");
  });

  it("says nothing reopens when every review switch is off", () => {
    const sentence = rules({
      review: { showScore: false, showCorrectAnswers: false, showExplanations: false },
    }).at(-1)!;
    expect(sentence).toContain("không mở lại");
    expect(sentence).not.toContain("không xem");
  });

  it("drops the 'but not' clause when everything is shown", () => {
    const sentence = rules({
      review: { showScore: true, showCorrectAnswers: true, showExplanations: true },
    }).at(-1)!;
    expect(sentence).not.toContain("không xem");
  });

  it("stays silent about switches that are off", () => {
    const off = rules({
      integrity: {
        requireFullscreen: false,
        blockCopyPaste: false,
        maxFocusLoss: 0,
        onLimitExceeded: "flag",
        minAwayMs: 3000,
      },
    });
    expect(off.join(" ")).not.toContain("Sao chép");
    expect(off.join(" ")).not.toContain("toàn màn hình");
    expect(off.join(" ")).not.toContain("Rời trang");
  });

  it("promises the consequence the teacher actually chose", () => {
    const flagged = rules().join(" ");
    expect(flagged).toContain("đánh dấu");

    const warned = rules({
      integrity: { ...draft().integrity, onLimitExceeded: "warn" },
    }).join(" ");
    expect(warned).toContain("nhắc");
    expect(warned).not.toContain("đánh dấu");
  });

  it("mentions extra attempts only when there are extra attempts", () => {
    expect(rules().join(" ")).not.toContain("lượt");
    expect(rules({ maxAttempts: 2 }).join(" ")).toContain("2 lượt");
  });
});
