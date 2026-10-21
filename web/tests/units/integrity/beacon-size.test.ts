import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { BEACON_BODY_LIMIT } from "@/features/integrity/beacon";
import { clearSession, pending, record } from "@/features/integrity/buffer";
import type { IntegrityPolicy } from "@/features/take-test/api";

vi.mock("@/features/take-test/api", () => ({ flushEvents: vi.fn() }));

const ATTEMPT = "att-1";
const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

let sent: Blob[] = [];
let taken = true;

beforeEach(() => {
  sessionStorage.clear();
  clearSession(ATTEMPT);
  sent = [];
  taken = true;
  Object.defineProperty(navigator, "sendBeacon", {
    configurable: true,
    value: vi.fn((_url: string, body: Blob) => {
      sent.push(body);
      return taken;
    }),
  });
});
afterEach(() => {
  clearSession(ATTEMPT);
  vi.restoreAllMocks();
});

function mountWithEvents(count: number) {
  const view = renderHook(() =>
    useIntegrityMonitor({
      attemptId: ATTEMPT,
      sessionId: "ses-1",
      beaconToken: "token",
      policy,
      questionId: "018f0000-0000-7000-8000-0000000000a1",
    }),
  );
  for (let at = 0; at < count; at += 1)
    record(ATTEMPT, "context_menu", {
      questionId: "018f0000-0000-7000-8000-0000000000a1",
    });
  return view;
}

const leave = () => window.dispatchEvent(new PageTransitionEvent("pagehide"));

it("sends a beacon the browser can take when 500 events are waiting, and leaves the rest buffered", async () => {
  mountWithEvents(500);

  leave();

  expect(sent).toHaveLength(1);
  expect(sent[0]!.size).toBeLessThanOrEqual(BEACON_BODY_LIMIT);
  const body = JSON.parse(await sent[0]!.text()) as {
    events: { kind: string; clientSeq: number }[];
  };
  expect(body.events.length).toBeGreaterThan(300);
  expect(body.events.length).toBeLessThan(500);
  expect(body.events[0]?.clientSeq).toBe(0);
  expect(pending()).toHaveLength(501 - body.events.length);
  expect(pending()[0]?.clientSeq).toBe(body.events.length);
});

it("keeps every event when the browser refuses the beacon", () => {
  taken = false;
  mountWithEvents(500);

  leave();

  expect(pending()).toHaveLength(501);
});
