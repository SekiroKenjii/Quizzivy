import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTakeTestStore } from "@/features/take-test/store";
import { session } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";
const store = () => useTakeTestStore.getState();
const start = () => store().hydrate(session({ serverTime: now, deadlineAt: deadline }));

beforeEach(() => {
  sessionStorage.clear();
  store().reset();
});
afterEach(() => store().reset());

/**
 * S-06's "đánh dấu" is the student's own, and the contract has no field for
 * it. It survives what the event buffer survives -- a reload of this tab --
 * and no more.
 */
describe("flags", () => {
  it("toggle on and off", () => {
    start();
    store().toggleFlag("q1");
    expect(store().flags.has("q1")).toBe(true);
    store().toggleFlag("q1");
    expect(store().flags.has("q1")).toBe(false);
  });

  it("survive a reload of the same attempt", () => {
    start();
    store().toggleFlag("q2");
    store().reset();
    expect(store().flags.size).toBe(0);
    start();
    expect(store().flags.has("q2")).toBe(true);
  });

  it("are kept across a refetch of the same attempt", () => {
    start();
    store().toggleFlag("q2");
    sessionStorage.clear();
    start();
    expect(store().flags.has("q2")).toBe(true);
  });

  it("do nothing before there is an attempt", () => {
    store().toggleFlag("q1");
    expect(store().flags.size).toBe(0);
  });
});
