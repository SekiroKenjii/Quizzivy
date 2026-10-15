import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import {
  beginSession,
  clearSession,
  pending,
  record,
} from "@/features/integrity/buffer";
import { flushEvents, type IntegrityPolicy } from "@/features/take-test/api";

vi.mock("@/features/take-test/api", () => ({
  flushEvents: vi.fn(),
}));

const sent = vi.mocked(flushEvents);
const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

function monitor(sessionId: string) {
  return renderHook(() =>
    useIntegrityMonitor({
      attemptId: "att-1",
      sessionId,
      beaconToken: "token",
      policy,
      questionId: null,
    }),
  );
}

beforeEach(() => {
  sessionStorage.clear();
  clearSession("att-1");
  sent.mockReset().mockResolvedValue(undefined as never);
});
afterEach(() => clearSession("att-1"));

it("sends the events a taken-over tab left, under that tab's session, when it starts the new one", () => {
  beginSession("att-1", "ses-1");
  record("att-1", "tab_hidden");
  record("att-1", "window_blur");
  const left = pending();

  monitor("ses-2");

  expect(sent).toHaveBeenCalledExactlyOnceWith("att-1", {
    sessionId: "ses-1",
    events: left,
  });
  expect(pending()).toEqual([]);
});

it("sends nothing when the tab starts the session it already holds", () => {
  beginSession("att-1", "ses-1");
  record("att-1", "tab_hidden");

  monitor("ses-1");

  expect(sent).not.toHaveBeenCalled();
  expect(pending()).toHaveLength(1);
});

it("lets a refused send go, because the events are best effort", () => {
  sent.mockRejectedValue(new Error("403"));
  beginSession("att-1", "ses-1");
  record("att-1", "tab_hidden");

  expect(() => monitor("ses-2")).not.toThrow();
});

it("sends a long stranded buffer in slices of at most 500, one after another", async () => {
  beginSession("att-1", "ses-1");
  for (let at = 0; at < 1100; at += 1) record("att-1", "window_blur");

  monitor("ses-2");
  await vi.waitFor(() => expect(sent).toHaveBeenCalledTimes(3));

  expect(sent.mock.calls.map(([, body]) => body.events.length)).toEqual([
    500, 500, 100,
  ]);
  expect(sent.mock.calls.every(([, body]) => body.sessionId === "ses-1")).toBe(true);
});

it("drops the rest of a stranded buffer when a slice is refused", async () => {
  sent.mockRejectedValue(new Error("403"));
  beginSession("att-1", "ses-1");
  for (let at = 0; at < 1100; at += 1) record("att-1", "window_blur");

  monitor("ses-2");
  await vi.waitFor(() => expect(sent).toHaveBeenCalledTimes(1));
  await Promise.resolve();

  expect(sent).toHaveBeenCalledTimes(1);
});
