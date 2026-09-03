import { describe, expect, it } from "vitest";
import { strikeState } from "@/features/integrity/strikes";
import type { IntegrityPolicy } from "@/features/take-test/api";

const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 2,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

/**
 * The boundary the dialog and the indicator both stand on. "Quá 2 lần" on the
 * intro means more than two: the second episode spends the last allowance and
 * the third carries the consequence.
 */
describe("strikeState", () => {
  it("reads maxFocusLoss 0 as no limit", () => {
    const state = strikeState({ ...policy, maxFocusLoss: 0 }, 7);
    expect(state.limit).toBeNull();
    expect(state.remaining).toBeNull();
    expect(state.exceeded).toBe(false);
  });

  it("counts down", () => {
    expect(strikeState(policy, 1)).toMatchObject({ remaining: 1, exceeded: false });
  });

  it("spends the last allowance at the limit without exceeding it", () => {
    expect(strikeState(policy, 2)).toMatchObject({ remaining: 0, exceeded: false });
  });

  it("exceeds only past the limit", () => {
    expect(strikeState(policy, 3)).toMatchObject({ remaining: 0, exceeded: true });
  });

  it("keeps warn as warn", () => {
    expect(strikeState({ ...policy, onLimitExceeded: "warn" }, 0).consequence).toBe(
      "warn",
    );
  });

  it("reads auto_submit as flag until T-5.1 builds the countdown", () => {
    expect(
      strikeState({ ...policy, onLimitExceeded: "auto_submit" }, 0).consequence,
    ).toBe("flag");
  });
});
