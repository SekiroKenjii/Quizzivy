import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { http } from "msw";
import { server } from "@tests/support/server";
import {
  SUMMARY_POLL_MS,
  useNotificationSummary,
} from "@/features/notifications/useNotificationSummary";
import { notificationKeys } from "@/features/notifications/api";
import { BASE, harness, summaryResponse, errorResponse, deferred } from "./support";

async function tick(ms = 0) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}
async function settled(check: () => boolean) {
  for (let n = 0; n < 50 && !check(); n += 1) await tick(10);
  expect(check()).toBe(true);
}
function visibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", {
    value: state,
    configurable: true,
  });
  act(() => document.dispatchEvent(new Event("visibilitychange")));
}
beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  cleanup();
  Reflect.deleteProperty(document, "visibilityState");
  vi.useRealTimers();
});

describe("idle notification summary", () => {
  it("returns zero while pending, polls at60s, stops after ten idle minutes and refetches once at the first key", async () => {
    let requests = 0;
    server.use(
      http.get(`${BASE}/me/summary`, () => {
        requests += 1;
        return summaryResponse(7);
      }),
    );
    const hook = renderHook(() => useNotificationSummary(), harness());
    expect(hook.result.current.unread).toBe(0);
    expect(SUMMARY_POLL_MS).toBe(60_000);
    await settled(() => hook.result.current.unread === 7);
    expect(requests).toBe(1);
    await tick(59_000);
    expect(requests).toBe(1);
    await tick(1_000);
    await settled(() => requests === 2 && hook.result.current.unread === 7);
    await tick(600_000);
    const paused = requests;
    await tick(120_000);
    expect(requests).toBe(paused);
    act(() => window.dispatchEvent(new Event("keydown")));
    await settled(() => requests === paused + 1 && hook.result.current.unread === 7);
    act(() => window.dispatchEvent(new Event("keydown")));
    await tick(1_000);
    expect(requests).toBe(paused + 1);
  });
  it("returns zero on initial error and failed refetch even though prior success stays cached", async () => {
    let fail = true;
    server.use(
      http.get(`${BASE}/me/summary`, () =>
        fail ? errorResponse() : summaryResponse(5),
      ),
    );
    const setup = harness();
    const hook = renderHook(() => useNotificationSummary(), setup);
    await settled(
      () =>
        setup.client.getQueryState(notificationKeys.summary)?.status === "error" &&
        hook.result.current.unread === 0,
    );
    expect(hook.result.current.unread).toBe(0);
    fail = false;
    await act(async () => {
      await setup.client.invalidateQueries({ queryKey: notificationKeys.summary });
    });
    await settled(() => hook.result.current.unread === 5);
    fail = true;
    await act(async () => {
      await setup.client.invalidateQueries({ queryKey: notificationKeys.summary });
    });
    await settled(
      () =>
        setup.client.getQueryState(notificationKeys.summary)?.status === "error" &&
        hook.result.current.unread === 0,
    );
    expect(setup.client.getQueryData(notificationKeys.summary)).toEqual({
      unreadNotifications: 5,
    });
    expect(hook.result.current.unread).toBe(0);
  });
  it("keeps the last count during a refresh, so the dot does not blink, and takes the new one when it answers", async () => {
    const gate = deferred<void>();
    let requests = 0;
    server.use(
      http.get(`${BASE}/me/summary`, async () => {
        requests += 1;
        if (requests > 1) await gate.promise;
        return summaryResponse(requests > 1 ? 3 : 2);
      }),
    );
    const setup = harness();
    const hook = renderHook(() => useNotificationSummary(), setup);
    await settled(() => hook.result.current.unread === 2);
    let refresh!: Promise<void>;
    act(() => {
      refresh = setup.client.invalidateQueries({ queryKey: notificationKeys.summary });
    });
    await settled(() => requests === 2);
    expect(hook.result.current.unread).toBe(2);
    await tick(50);
    expect(hook.result.current.unread, "no blink while the refresh is out").toBe(2);
    gate.resolve();
    await act(async () => {
      await refresh;
    });
    await settled(() => hook.result.current.unread === 3);
  });
  it("sends no request or resume poll while disabled and hides even a cached count", async () => {
    let requests = 0;
    server.use(
      http.get(`${BASE}/me/summary`, () => {
        requests += 1;
        return summaryResponse(3);
      }),
    );
    const setup = harness();
    setup.client.setQueryData(notificationKeys.summary, { unreadNotifications: 9 });
    const hook = renderHook(({ enabled }) => useNotificationSummary(enabled), {
      ...setup,
      initialProps: { enabled: false },
    });
    expect(hook.result.current.unread).toBe(0);
    await tick(720_000);
    act(() => window.dispatchEvent(new Event("keydown")));
    visibility("hidden");
    visibility("visible");
    await tick(60_000);
    expect(requests).toBe(0);
    expect(hook.result.current.unread).toBe(0);
    hook.rerender({ enabled: true });
    await settled(() => requests === 1 && hook.result.current.unread === 3);
    hook.rerender({ enabled: false });
    expect(hook.result.current.unread).toBe(0);
    await tick(120_000);
    expect(requests).toBe(1);
  });
  it("does not poll a hidden tab and refetches once when it returns", async () => {
    let requests = 0;
    server.use(
      http.get(`${BASE}/me/summary`, () => {
        requests += 1;
        return summaryResponse(4);
      }),
    );
    const hook = renderHook(() => useNotificationSummary(), harness());
    await settled(() => hook.result.current.unread === 4);
    visibility("hidden");
    await tick(120_000);
    expect(requests).toBe(1);
    visibility("visible");
    await settled(() => requests === 2 && hook.result.current.unread === 4);
    act(() => window.dispatchEvent(new Event("keydown")));
    await tick(1_000);
    expect(requests).toBe(2);
  });
});
