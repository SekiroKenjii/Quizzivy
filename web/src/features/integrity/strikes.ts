import type { IntegrityPolicy } from "@/features/take-test/api";

/**
 * Where a student stands against §10.2's focus-loss limit, in one shape that
 * the dialog and the indicator both read -- so "còn 1 lần" in the strip and
 * "còn 1 lần" in the dialog can never disagree.
 *
 * The boundary is the contract's word: `onLimitExceeded` fires when the count
 * is OVER the limit, not at it. The intro says "quá 2 lần" and means it: with
 * `maxFocusLoss: 2`, the second episode spends the last allowance and the
 * third is the one that carries a consequence.
 */
export interface StrikeState {
  /** Counted away episodes, this sitting and earlier ones together. */
  count: number;
  /** `maxFocusLoss`, or null for §10.3's 0, which means no limit. */
  limit: number | null;
  /** Episodes left before the next one exceeds the limit. Null when unlimited. */
  remaining: number | null;
  exceeded: boolean;
  /**
   * What exceeding does. `auto_submit` reads as `flag` until T-5.1 builds its
   * countdown: the attempt is still marked, and telling the student that is
   * truer than promising a submission this build cannot make.
   */
  consequence: "warn" | "flag";
}

export function strikeState(policy: IntegrityPolicy, count: number): StrikeState {
  const limit = policy.maxFocusLoss > 0 ? policy.maxFocusLoss : null;
  return {
    count,
    limit,
    remaining: limit === null ? null : Math.max(0, limit - count),
    exceeded: limit !== null && count > limit,
    consequence: policy.onLimitExceeded === "warn" ? "warn" : "flag",
  };
}
