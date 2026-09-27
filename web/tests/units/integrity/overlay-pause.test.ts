import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { clearSession, pending } from "@/features/integrity/buffer";
import { useAppState, type Overlay } from "@/stores/appState";

const ATTEMPT = "overlay-attempt";
const WINDOW = { startsAt: "2026-10-01T15:00:00Z", endsAt: "2026-10-01T16:00:00Z" };

beforeEach(() => useAppState.setState({ overlay: { kind: "none" } }));

afterEach(() => {
  clearSession(ATTEMPT);
  useAppState.setState({ overlay: { kind: "none" } });
  vi.useRealTimers();
});

function monitor() {
  return renderHook(() =>
    useIntegrityMonitor({
      attemptId: ATTEMPT,
      sessionId: "session-1",
      beaconToken: "",
      policy: null,
    }),
  );
}

function leaveAndReturn() {
  act(() => {
    window.dispatchEvent(new Event("blur"));
    window.dispatchEvent(new Event("focus"));
    document.dispatchEvent(new Event("visibilitychange"));
  });
}

it.each<[string, Overlay]>([
  ["maintenance", { kind: "maintenance", window: WINDOW }],
  ["sign in again", { kind: "expired" }],
])("records no focus change under the %s overlay", (_name, overlay) => {
  monitor();
  act(() => useAppState.getState().showOverlay(overlay));
  leaveAndReturn();
  expect(pending()).toEqual([]);
});

it("keeps recording under the update card, which is not the platform's doing", () => {
  monitor();
  act(() => useAppState.getState().showOverlay({ kind: "update" }));
  leaveAndReturn();
  expect(pending().map((event) => event.kind)).toEqual([
    "window_blur",
    "window_focus",
    "tab_visible",
  ]);
});

it("drops an away episode that is open when the overlay appears", () => {
  vi.useFakeTimers();
  const { result } = monitor();
  act(() => window.dispatchEvent(new Event("blur")));
  act(() =>
    useAppState.getState().showOverlay({ kind: "maintenance", window: WINDOW }),
  );
  act(() => vi.advanceTimersByTime(60_000));
  act(() => useAppState.getState().closeOverlay());
  act(() => window.dispatchEvent(new Event("focus")));

  expect(result.current.strikes).toBe(0);
  expect(pending().map((event) => event.kind)).toEqual(["window_blur", "window_focus"]);
  expect(pending()[1]).not.toHaveProperty("meta");
});

it("counts an episode again once the overlay is gone", () => {
  vi.useFakeTimers();
  const { result } = monitor();
  act(() => useAppState.getState().showOverlay({ kind: "expired" }));
  act(() => useAppState.getState().closeOverlay());
  act(() => window.dispatchEvent(new Event("blur")));
  act(() => vi.advanceTimersByTime(5_000));
  act(() => window.dispatchEvent(new Event("focus")));

  expect(result.current.strikes).toBe(1);
});
