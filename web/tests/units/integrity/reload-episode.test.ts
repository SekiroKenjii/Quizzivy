import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { clearSession, pending } from "@/features/integrity/buffer";
import type { IntegrityPolicy } from "@/features/take-test/api";

vi.mock("@/features/take-test/api", () => ({ flushEvents: vi.fn() }));

const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

beforeEach(() => {
  sessionStorage.clear();
  clearSession("att-1");
  renderHook(() =>
    useIntegrityMonitor({
      attemptId: "att-1",
      sessionId: "ses-1",
      beaconToken: "",
      policy,
      questionId: null,
    }),
  );
});
afterEach(() => {
  Object.defineProperty(document, "hidden", { configurable: true, value: false });
  clearSession("att-1");
  vi.restoreAllMocks();
});

function hide() {
  Object.defineProperty(document, "hidden", { configurable: true, value: true });
  document.dispatchEvent(new Event("visibilitychange"));
}

const kinds = () => pending().map((event) => event.kind);

it("records no leave after a page that is unloading, so a reload opens no away episode", () => {
  window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: false }));
  hide();
  window.dispatchEvent(new Event("blur"));

  expect(kinds()).toEqual(["page_hide"]);
});

it("still records the leave of a page that may return from the back/forward cache", () => {
  window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true }));
  hide();

  expect(kinds()).toEqual(["page_hide", "tab_hidden"]);
});
