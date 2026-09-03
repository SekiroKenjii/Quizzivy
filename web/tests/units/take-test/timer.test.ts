import { afterEach, describe, expect, it, vi } from "vitest";
import { remainingMs, useTakeTestStore } from "@/features/take-test/store";
import { session } from "./support";

vi.mock("@/features/take-test/api", async () => {
  const actual = await vi.importActual<object>("@/features/take-test/api");
  return {
    ...actual,
    saveAnswers: vi.fn(),
    submitAttempt: vi.fn(),
    getAttempt: vi.fn(),
  };
});

afterEach(() => {
  vi.useRealTimers();
  useTakeTestStore.getState().reset();
});

/**
 * §9's timer is server-authoritative, and this is the half of that which lives
 * in the browser. A device clock is not evidence about how long is left.
 */
describe("the timer reads the server's clock, not the device's", () => {
  it("is unaffected by a device running five minutes fast", () => {
    // The server says it is 08:00 and the paper is due at 08:30.
    const serverNow = new Date("2026-09-01T08:00:00.000Z");
    const deadline = new Date("2026-09-01T08:30:00.000Z");

    vi.useFakeTimers();
    vi.setSystemTime(new Date(serverNow.getTime() + 5 * 60_000));

    useTakeTestStore.getState().hydrate(
      session({
        serverTime: serverNow.toISOString(),
        deadlineAt: deadline.toISOString(),
      }),
    );

    const { deadlineAt, offsetMs } = useTakeTestStore.getState();
    // Thirty minutes.
    expect(remainingMs({ deadlineAt, offsetMs })).toBe(30 * 60_000);
    expect(offsetMs).toBe(-5 * 60_000);
  });

  it("is unaffected by a device running five minutes slow", () => {
    const serverNow = new Date("2026-09-01T08:00:00.000Z");
    const deadline = new Date("2026-09-01T08:30:00.000Z");

    vi.useFakeTimers();
    vi.setSystemTime(new Date(serverNow.getTime() - 5 * 60_000));

    useTakeTestStore.getState().hydrate(
      session({
        serverTime: serverNow.toISOString(),
        deadlineAt: deadline.toISOString(),
      }),
    );

    const { deadlineAt, offsetMs } = useTakeTestStore.getState();
    // Not thirty-five.
    expect(remainingMs({ deadlineAt, offsetMs })).toBe(30 * 60_000);
  });

  it("counts down as time passes, and stops at zero rather than going negative", () => {
    const serverNow = new Date("2026-09-01T08:00:00.000Z");
    vi.useFakeTimers();
    vi.setSystemTime(serverNow);

    useTakeTestStore.getState().hydrate(
      session({
        serverTime: serverNow.toISOString(),
        deadlineAt: new Date(serverNow.getTime() + 60_000).toISOString(),
      }),
    );
    const read = () => {
      const { deadlineAt, offsetMs } = useTakeTestStore.getState();
      return remainingMs({ deadlineAt, offsetMs });
    };

    expect(read()).toBe(60_000);
    vi.advanceTimersByTime(45_000);
    expect(read()).toBe(15_000);
    vi.advanceTimersByTime(60_000);
    expect(read()).toBe(0);
  });
});
