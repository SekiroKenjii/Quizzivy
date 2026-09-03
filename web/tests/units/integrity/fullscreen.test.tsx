import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FullscreenBar } from "@/features/integrity/components/FullscreenBar";
import { enterFullscreen } from "@/features/integrity/fullscreen";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { clearSession, pending } from "@/features/integrity/buffer";
import type { IntegrityPolicy } from "@/features/take-test/api";
import "@/lib/i18n";

const ATTEMPT = "att-1";

const policy: IntegrityPolicy = {
  requireFullscreen: true,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

/** jsdom has no Fullscreen API; these two are what the code reads. */
function fullscreen(over: { enabled: boolean; element?: Element | null }) {
  Object.defineProperty(document, "fullscreenEnabled", {
    configurable: true,
    get: () => over.enabled,
  });
  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => over.element ?? null,
  });
}

const request = vi.fn<() => Promise<void>>();

beforeEach(() => {
  sessionStorage.clear();
  clearSession(ATTEMPT);
  request.mockReset().mockResolvedValue(undefined);
  document.documentElement.requestFullscreen = request;
  fullscreen({ enabled: true });
});
afterEach(() => {
  fullscreen({ enabled: false });
});

/**
 * The exit is an inline bar, not a dialog (S-07): a modal over an exited
 * fullscreen is a trap, and Esc has to keep working.
 */
describe("the bar", () => {
  it("offers the way back, from a click", async () => {
    const user = userEvent.setup();
    render(<FullscreenBar />);
    expect(screen.getByText("Bạn đã thoát chế độ toàn màn hình.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Quay lại toàn màn hình" }));
    expect(request).toHaveBeenCalledWith({ navigationUI: "hide" });
  });

  // Every iPhone. A button that cannot work is worse than a sentence.
  it("says so when the browser has no fullscreen", () => {
    fullscreen({ enabled: false });
    render(<FullscreenBar />);
    expect(screen.getByText(/không hỗ trợ chế độ toàn màn hình/)).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
  });
});

describe("entering", () => {
  it("swallows a refusal, because fullscreen never blocks the paper", async () => {
    request.mockRejectedValue(new TypeError("Permissions check failed"));
    await expect(enterFullscreen()).resolves.toBeUndefined();
  });

  it("does nothing when already fullscreen", async () => {
    fullscreen({ enabled: true, element: document.body });
    await enterFullscreen();
    expect(request).not.toHaveBeenCalled();
  });
});

describe("the monitor", () => {
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

  it("follows the document in and out", () => {
    const { result } = monitor();
    expect(result.current.fullscreen).toBe(false);

    fullscreen({ enabled: true, element: document.body });
    act(() => document.dispatchEvent(new Event("fullscreenchange")));
    expect(result.current.fullscreen).toBe(true);

    fullscreen({ enabled: true, element: null });
    act(() => document.dispatchEvent(new Event("fullscreenchange")));
    expect(result.current.fullscreen).toBe(false);
  });

  // §10.1: `fullscreen_enter` / `fullscreen_exit` -- only when
  // `requireFullscreen` is on.
  it("records the events only when the policy requires fullscreen", () => {
    monitor({ requireFullscreen: false });
    act(() => document.dispatchEvent(new Event("fullscreenchange")));
    expect(pending().map((e) => e.kind)).not.toContain("fullscreen_exit");
  });

  it("records an exit and an entry when it does", () => {
    monitor();
    act(() => document.dispatchEvent(new Event("fullscreenchange")));
    fullscreen({ enabled: true, element: document.body });
    act(() => document.dispatchEvent(new Event("fullscreenchange")));
    expect(pending().map((e) => e.kind)).toEqual([
      "fullscreen_exit",
      "fullscreen_enter",
    ]);
  });
});
