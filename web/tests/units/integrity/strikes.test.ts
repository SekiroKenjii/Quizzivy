import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { clearSession, pending } from "@/features/integrity/buffer";
import type { IntegrityPolicy } from "@/features/take-test/api";

const ATTEMPT = "att-1";

const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 3,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

function monitor(over: Partial<IntegrityPolicy> = {}) {
  return renderHook(() =>
    useIntegrityMonitor({
      attemptId: ATTEMPT,
      sessionId: "ses-1",
      beaconToken: "beacon",
      policy: { ...policy, ...over },
    }),
  );
}

/**
 * jsdom reports `document.hidden` as false and never changes it, so a test that
 * dispatches visibilitychange without this is dispatching "the tab came back".
 */
function setHidden(hidden: boolean) {
  Object.defineProperty(document, "hidden", { configurable: true, get: () => hidden });
}

/** A tab switch and back, `awayMs` apart on the wall clock. */
function awayFor(awayMs: number) {
  act(() => {
    window.dispatchEvent(new Event("blur"));
  });
  vi.setSystemTime(Date.now() + awayMs);
  act(() => {
    window.dispatchEvent(new Event("focus"));
  });
}

beforeEach(() => {
  sessionStorage.clear();
  clearSession(ATTEMPT);
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-09-01T08:00:00.000Z"));
});
afterEach(() => {
  vi.useRealTimers();
  setHidden(false);
});

/**
 * §10.1's away-duration model. "A 2-second blur is a notification; a 90-second
 * blur is a search." Counting bare events would punish the student whose phone
 * buzzed and reward nobody.
 */
describe("away episodes", () => {
  it.each([
    ["does not count a two-second blur", 2000, 0],
    ["counts a ninety-second blur", 90_000, 1],
    ["counts one exactly at the threshold", 3000, 1],
  ])("%s", (_name, ms, strikes) => {
    const { result } = monitor();
    awayFor(ms);
    expect(result.current.strikes).toBe(strikes);
  });

  it("honours a policy that sets its own threshold", () => {
    const { result } = monitor({ minAwayMs: 10_000 });
    awayFor(5000);
    expect(result.current.strikes).toBe(0);
    awayFor(11_000);
    expect(result.current.strikes).toBe(1);
  });

  // A tab switch fires window blur AND visibilitychange.
  it("treats a blur and a hide as one episode", () => {
    const { result } = monitor();

    // What a real tab switch does: blur, then hidden.
    act(() => {
      window.dispatchEvent(new Event("blur"));
      setHidden(true);
      document.dispatchEvent(new Event("visibilitychange"));
    });
    vi.setSystemTime(Date.now() + 90_000);
    act(() => {
      setHidden(false);
      document.dispatchEvent(new Event("visibilitychange"));
      window.dispatchEvent(new Event("focus"));
    });

    expect(result.current.strikes).toBe(1);
  });

  it("stores both endpoints, so duration is visible to the teacher", () => {
    monitor();
    awayFor(4200);

    const kinds = pending().map((e) => e.kind);
    expect(kinds).toContain("window_blur");
    expect(kinds).toContain("window_focus");

    const back = pending().find((e) => e.kind === "window_focus");
    expect(back?.meta).toEqual({ awayMs: 4200 });
  });

  it("adds up across separate episodes", () => {
    const { result } = monitor();
    awayFor(5000);
    awayFor(1000);
    awayFor(8000);
    expect(result.current.strikes).toBe(2);
  });

  // "trong 24 giây" in the dialog is this number.
  it("reports how long the counted episode lasted", () => {
    const { result } = monitor();
    awayFor(2000);
    expect(result.current.lastAwayMs).toBeNull();
    awayFor(24_000);
    expect(result.current.lastAwayMs).toBe(24_000);
  });
});
