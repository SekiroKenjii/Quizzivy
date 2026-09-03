import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import {
  beginSession,
  clearSession,
  pending,
  record,
} from "@/features/integrity/buffer";
import { useTakeTestStore } from "@/features/take-test/store";
import { saveAnswers, submitAttempt } from "@/features/take-test/api";
import { session, text } from "../take-test/support";
import type { IntegrityPolicy } from "@/features/take-test/api";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const saved = vi.mocked(saveAnswers);
const submitted = vi.mocked(submitAttempt);

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";
const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

const flushed = () => new Promise((resolve) => setTimeout(resolve, 0));

beforeEach(() => {
  sessionStorage.clear();
  saved.mockReset().mockResolvedValue({ serverTime: now, savedAt: now });
  submitted
    .mockReset()
    .mockResolvedValue(session({ serverTime: now, deadlineAt: deadline }).attempt);
  useTakeTestStore.getState().reset();
  useTakeTestStore
    .getState()
    .hydrate(session({ serverTime: now, deadlineAt: deadline }));
  clearSession("att-1");
  beginSession("att-1", "ses-1");
});
afterEach(() => useTakeTestStore.getState().reset());

/**
 * §10.6: a failed event flush never blocks answering or submitting, and
 * integrity failure never blocks input. The monitor watches the test; it does
 * not get to end it.
 */
describe("when the integrity flush fails", () => {
  it("still lets the student answer", async () => {
    saved.mockRejectedValue(new Error("offline"));
    record("att-1", "paste");

    useTakeTestStore.getState().setAnswer("q1", text("typed anyway"));
    await useTakeTestStore.getState().flush();

    expect(useTakeTestStore.getState().answers["q1"]).toEqual(text("typed anyway"));
    useTakeTestStore.getState().setAnswer("q1", text("and again"));
    expect(useTakeTestStore.getState().answers["q1"]).toEqual(text("and again"));
  });

  it("still lets the student submit", async () => {
    saved.mockRejectedValue(new Error("offline"));
    record("att-1", "paste");

    await useTakeTestStore.getState().submit();

    expect(submitted).toHaveBeenCalledTimes(1);
    expect(useTakeTestStore.getState().submitState).toBe("done");
  });

  it("keeps the events for the next flush rather than dropping them", async () => {
    saved.mockRejectedValue(new Error("offline"));
    record("att-1", "paste");
    record("att-1", "copy");

    useTakeTestStore.getState().setAnswer("q1", text("typed"));
    await useTakeTestStore.getState().flush();
    await flushed();

    expect(pending().map((e) => e.kind)).toEqual(["paste", "copy"]);
  });

  it("sends them with the batch that succeeds", async () => {
    record("att-1", "paste");
    useTakeTestStore.getState().setAnswer("q1", text("typed"));
    await useTakeTestStore.getState().flush();

    expect(saved.mock.calls[0]?.[1].events?.map((e) => e.kind)).toEqual(["paste"]);
  });
});

/**
 * The listeners themselves must never be able to stop a student working. This
 * is the hook mounted with a policy that blocks pasting -- the one thing it IS
 * allowed to prevent -- and everything else still going through.
 */
describe("the monitor itself", () => {
  function mount(over: Partial<IntegrityPolicy> = {}) {
    return renderHook(() =>
      useIntegrityMonitor({
        attemptId: "att-1",
        sessionId: "ses-1",
        beaconToken: "beacon",
        policy: { ...policy, ...over },
      }),
    );
  }

  it("records a paste and blocks it only when the teacher asked", () => {
    mount({ blockCopyPaste: true });
    const blocked = new Event("paste", { bubbles: true, cancelable: true });
    act(() => {
      document.dispatchEvent(blocked);
    });

    expect(blocked.defaultPrevented).toBe(true);
    expect(pending().map((e) => e.kind)).toContain("paste");
  });

  it("records a paste and allows it when the teacher did not", () => {
    mount({ blockCopyPaste: false });
    const allowed = new Event("paste", { bubbles: true, cancelable: true });
    act(() => {
      document.dispatchEvent(allowed);
    });

    expect(allowed.defaultPrevented).toBe(false);
    expect(pending().map((e) => e.kind)).toContain("paste");
  });

  // §10.1: recorded, never blocked.
  it("never blocks the context menu", () => {
    mount({ blockCopyPaste: true });
    const menu = new Event("contextmenu", { bubbles: true, cancelable: true });
    act(() => {
      document.dispatchEvent(menu);
    });

    expect(menu.defaultPrevented).toBe(false);
    expect(pending().map((e) => e.kind)).toContain("context_menu");
  });

  it("removes every listener when the test screen goes away", () => {
    const { unmount } = mount();
    unmount();

    const before = pending().length;
    act(() => {
      document.dispatchEvent(new Event("paste", { bubbles: true, cancelable: true }));
      window.dispatchEvent(new Event("blur"));
      window.dispatchEvent(new Event("offline"));
    });
    expect(pending().length).toBe(before);
  });
});
