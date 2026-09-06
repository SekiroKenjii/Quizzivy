import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import {
  beginSession,
  clearSession,
  pending,
  record,
} from "@/features/integrity/buffer";
import type { IntegrityPolicy } from "@/features/take-test/api";

const ATTEMPT = "att-1";
const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

let sent: { url: string; body: Blob }[] = [];

beforeEach(() => {
  sessionStorage.clear();
  clearSession(ATTEMPT);
  sent = [];
  Object.defineProperty(navigator, "sendBeacon", {
    configurable: true,
    value: vi.fn((url: string, body: Blob) => {
      sent.push({ url, body });
      return true;
    }),
  });
});
afterEach(() => vi.restoreAllMocks());

function mount() {
  return renderHook(() =>
    useIntegrityMonitor({
      attemptId: ATTEMPT,
      sessionId: "ses-1",
      beaconToken: "the-beacon-token",
      policy,
    }),
  );
}

async function bodyOf(blob: Blob): Promise<Record<string, unknown>> {
  return JSON.parse(await blob.text()) as Record<string, unknown>;
}

/**
 * [D-03] navigator.sendBeacon, because a fetch fired from pagehide is routinely
 * cancelled as the page goes away -- and the tab closing is itself the event
 * the timeline most wants.
 */
describe("the pagehide flush", () => {
  it("sends a text/plain blob carrying the beacon token", async () => {
    mount();
    act(() => {
      record(ATTEMPT, "paste");
      window.dispatchEvent(new Event("pagehide"));
    });

    expect(sent).toHaveLength(1);
    // text/plain is CORS-safelisted, so the request skips preflight.
    expect(sent[0]?.body.type).toBe("text/plain");
    expect(sent[0]?.url).toMatch(/\/app\/attempts\/att-1\/events$/);

    const body = await bodyOf(sent[0]!.body);
    // The credential is in the BODY because sendBeacon cannot set headers.
    expect(body["beaconToken"]).toBe("the-beacon-token");
    expect(body["sessionId"]).toBe("ses-1");
  });

  it("includes the page_hide event itself", async () => {
    mount();
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });

    const body = await bodyOf(sent[0]!.body);
    const kinds = (body["events"] as { kind: string }[]).map((e) => e.kind);
    expect(kinds).toContain("page_hide");
  });

  it("carries everything still buffered", async () => {
    mount();
    act(() => {
      record(ATTEMPT, "tab_hidden");
      record(ATTEMPT, "tab_visible");
      window.dispatchEvent(new Event("pagehide"));
    });

    const body = await bodyOf(sent[0]!.body);
    expect(body["events"] as unknown[]).toHaveLength(3);
  });

  it("empties the buffer", () => {
    mount();
    act(() => {
      record(ATTEMPT, "copy");
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(pending()).toEqual([]);
  });

  it("sends nothing when there is nothing to send", () => {
    beginSession(ATTEMPT, "ses-1");
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(sent).toHaveLength(0);
  });
});
