import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTakeTestStore } from "@/features/take-test/store";
import { session } from "../take-test/support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";

function payload(focusLossCount: number | null) {
  const s = session({ serverTime: now, deadlineAt: deadline });
  s.attempt.integrity =
    focusLossCount === null ? null : { focusLossCount, flagged: false };
  return s;
}

const store = () => useTakeTestStore.getState();

beforeEach(() => store().reset());
afterEach(() => store().reset());

/**
 * The count a sitting starts from. Taken from the first payload and then kept:
 * a refetch mid-test (an expired audio URL, say) returns a count that already
 * includes the episodes this tab flushed, and the monitor has counted those
 * itself. Adding them again would show each strike twice.
 */
describe("focusLossCount baseline", () => {
  it("comes from the server on the first payload", () => {
    store().hydrate(payload(2));
    expect(store().focusLossCount).toBe(2);
  });

  it("is kept across a refetch of the same attempt", () => {
    store().hydrate(payload(2));
    store().hydrate(payload(5));
    expect(store().focusLossCount).toBe(2);
  });

  it("is read fresh after a reset, which is a new sitting", () => {
    store().hydrate(payload(2));
    store().reset();
    store().hydrate(payload(5));
    expect(store().focusLossCount).toBe(5);
  });

  it("is zero when the payload carries no integrity block", () => {
    store().hydrate(payload(null));
    expect(store().focusLossCount).toBe(0);
  });
});
