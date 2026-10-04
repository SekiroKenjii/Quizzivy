import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import {
  IDLE_AFTER_MS,
  useIdlePolling,
  useRefetchOnResume,
} from "@/hooks/useIdlePolling";

const TEN_MINUTES = 10 * 60_000;
const INPUTS = [
  "pointerdown",
  "pointermove",
  "keydown",
  "wheel",
  "touchstart",
] as const;

function input(type = "pointermove") {
  act(() => {
    window.dispatchEvent(new Event(type));
  });
}

function pass(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

function setVisibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", {
    value: state,
    configurable: true,
  });
}

function turn(state: "visible" | "hidden") {
  setVisibility(state);
  act(() => {
    document.dispatchEvent(new Event("visibilitychange"));
  });
}

function poller(enabled = true) {
  return renderHook(
    (props: { enabled: boolean }) => useIdlePolling(15_000, props.enabled),
    {
      initialProps: { enabled },
    },
  );
}

function resumer(refetch: () => unknown, enabled = true) {
  return renderHook(
    (props: { refetch: () => unknown; enabled: boolean }) =>
      useRefetchOnResume(props.refetch, props.enabled),
    { initialProps: { refetch, enabled } },
  );
}

type Listening = [type: string, listener: unknown, capture: boolean];

function captureOf(options: unknown) {
  if (typeof options === "boolean") return options;
  return Boolean((options as AddEventListenerOptions | undefined)?.capture);
}

function listenerLedger(target: EventTarget) {
  const add = vi.spyOn(target, "addEventListener");
  const remove = vi.spyOn(target, "removeEventListener");
  const entries = (calls: unknown[][]): Listening[] =>
    calls.map(([type, listener, options]) => [
      String(type),
      listener,
      captureOf(options),
    ]);
  return {
    added: (type: string) => entries(add.mock.calls).filter(([t]) => t === type),
    attached(type: string) {
      const removed = entries(remove.mock.calls);
      return entries(add.mock.calls).filter(([t, listener, capture]) => {
        if (t !== type) return false;
        const at = removed.findIndex(
          ([rt, rl, rc]) => rt === t && rl === listener && rc === capture,
        );
        if (at === -1) return true;
        removed.splice(at, 1);
        return false;
      });
    },
    options: (type: string) =>
      add.mock.calls.find(([t]) => t === type)?.[2] as AddEventListenerOptions,
  };
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(document, "visibilityState");
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("useIdlePolling", () => {
  it("goes idle after ten minutes", () => {
    expect(IDLE_AFTER_MS).toBe(TEN_MINUTES);
  });

  it("returns the interval while enabled", () => {
    expect(poller().result.current).toBe(15_000);
  });

  it("returns false while disabled, and the interval once enabled", () => {
    const hook = poller(false);
    expect(hook.result.current).toBe(false);
    hook.rerender({ enabled: true });
    expect(hook.result.current).toBe(15_000);
    hook.rerender({ enabled: false });
    expect(hook.result.current).toBe(false);
  });

  it("keeps the interval at nine minutes fifty-nine and drops it at ten", () => {
    const hook = poller();
    pass(TEN_MINUTES - 1_000);
    expect(hook.result.current).toBe(15_000);
    pass(999);
    expect(hook.result.current).toBe(15_000);
    pass(1);
    expect(hook.result.current).toBe(false);
  });

  it.each(INPUTS)("starts the ten minutes again on %s", (type) => {
    const hook = poller();
    pass(TEN_MINUTES - 1_000);
    input(type);
    pass(TEN_MINUTES - 1);
    expect(hook.result.current).toBe(15_000);
    pass(1);
    expect(hook.result.current).toBe(false);
  });

  it("counts the ten minutes from an input early in the window", () => {
    const hook = poller();
    pass(1);
    input();
    pass(TEN_MINUTES - 1);
    expect(hook.result.current).toBe(15_000);
    pass(1);
    expect(hook.result.current).toBe(false);
  });

  it("does not count an event that is not input", () => {
    const hook = poller();
    pass(TEN_MINUTES - 1_000);
    input("pointerup");
    input("keyup");
    input("scroll");
    pass(1_000);
    expect(hook.result.current).toBe(false);
  });

  it("sees an input whose propagation a handler stops", () => {
    const hook = poller();
    const button = document.createElement("button");
    button.addEventListener("keydown", (event) => event.stopPropagation());
    document.body.append(button);
    pass(TEN_MINUTES - 1_000);
    act(() => {
      button.dispatchEvent(new Event("keydown", { bubbles: true }));
    });
    pass(1_000);
    expect(hook.result.current).toBe(15_000);
    button.remove();
  });

  it("brings the interval back on the first input after idleness", () => {
    const hook = poller();
    pass(TEN_MINUTES);
    expect(hook.result.current).toBe(false);
    input("keydown");
    expect(hook.result.current).toBe(15_000);
    pass(TEN_MINUTES - 1);
    expect(hook.result.current).toBe(15_000);
    pass(1);
    expect(hook.result.current).toBe(false);
  });

  it("returns false while the document is hidden and the interval when it is visible again", () => {
    const hook = poller();
    turn("hidden");
    expect(hook.result.current).toBe(false);
    turn("visible");
    expect(hook.result.current).toBe(15_000);
  });

  it("starts hidden when it mounts in a hidden document", () => {
    setVisibility("hidden");
    const hook = poller();
    expect(hook.result.current).toBe(false);
    turn("visible");
    expect(hook.result.current).toBe(15_000);
  });

  it("brings the interval back when an idle user's tab becomes visible", () => {
    const hook = poller();
    turn("hidden");
    pass(TEN_MINUTES);
    turn("visible");
    expect(hook.result.current).toBe(15_000);
    pass(TEN_MINUTES - 1);
    expect(hook.result.current).toBe(15_000);
    pass(1);
    expect(hook.result.current).toBe(false);
  });

  it("starts the ten minutes again when the tab becomes visible", () => {
    const hook = poller();
    turn("hidden");
    pass(TEN_MINUTES - 1_000);
    turn("visible");
    expect(vi.getTimerCount()).toBe(1);
    pass(TEN_MINUTES - 1);
    expect(hook.result.current).toBe(15_000);
    pass(1);
    expect(hook.result.current).toBe(false);
  });

  it("does not count a visibility event that changes nothing as a return", () => {
    const hook = poller();
    pass(TEN_MINUTES - 1_000);
    turn("visible");
    pass(1_000);
    expect(hook.result.current).toBe(false);
    turn("visible");
    expect(hook.result.current).toBe(false);
  });

  it("shares one set of passive listeners between two consumers and removes it after the last", () => {
    const onWindow = listenerLedger(window);
    const onDocument = listenerLedger(document);
    const first = poller();
    const second = poller();

    for (const type of INPUTS) {
      expect(onWindow.added(type)).toHaveLength(1);
      expect(onWindow.options(type)).toMatchObject({ passive: true });
    }
    expect(onDocument.added("visibilitychange")).toHaveLength(1);

    first.unmount();
    for (const type of INPUTS) expect(onWindow.attached(type)).toHaveLength(1);
    pass(TEN_MINUTES);
    expect(second.result.current).toBe(false);

    second.unmount();
    for (const type of INPUTS) expect(onWindow.attached(type)).toHaveLength(0);
    expect(onDocument.attached("visibilitychange")).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("attaches nothing for a disabled consumer", () => {
    const onWindow = listenerLedger(window);
    poller(false);
    for (const type of INPUTS) expect(onWindow.added(type)).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("keeps one timer however many inputs arrive", () => {
    poller();
    const timers = vi.getTimerCount();
    for (let i = 0; i < 50; i += 1) {
      pass(1_000);
      input();
    }
    expect(timers).toBe(1);
    expect(vi.getTimerCount()).toBe(1);
  });

  it("does not re-render a consumer on a mouse move, and renders once when idleness flips", () => {
    let renders = 0;
    renderHook(() => {
      renders += 1;
      return useIdlePolling(15_000);
    });
    expect(renders).toBe(1);
    for (let i = 0; i < 20; i += 1) input("pointermove");
    expect(renders).toBe(1);
    pass(TEN_MINUTES);
    expect(renders).toBe(2);
    input("pointermove");
    expect(renders).toBe(3);
    input("pointermove");
    expect(renders).toBe(3);
  });

  it("counts mounting as activity after the last consumer left an idle store", () => {
    const first = poller();
    pass(TEN_MINUTES);
    expect(first.result.current).toBe(false);
    first.unmount();
    pass(60_000);
    const seen: (number | false)[] = [];
    const second = renderHook(() => {
      const interval = useIdlePolling(15_000);
      seen.push(interval);
      return interval;
    });
    expect(seen).toEqual([15_000]);
    pass(TEN_MINUTES - 1);
    expect(second.result.current).toBe(15_000);
    pass(1);
    expect(second.result.current).toBe(false);
  });

  it("gives a consumer that joins an idle store no interval until an input", () => {
    const first = poller();
    pass(TEN_MINUTES);
    const second = poller();
    expect(first.result.current).toBe(false);
    expect(second.result.current).toBe(false);
    input();
    expect(first.result.current).toBe(15_000);
    expect(second.result.current).toBe(15_000);
  });

  it("answers the interval in a server render", () => {
    function Probe() {
      return <output>{String(useIdlePolling(15_000))}</output>;
    }
    expect(renderToString(<Probe />)).toBe("<output>15000</output>");
  });
});

describe("useRefetchOnResume", () => {
  it("does not refetch on mount", () => {
    const refetch = vi.fn();
    resumer(refetch);
    pass(1_000);
    expect(refetch).not.toHaveBeenCalled();
  });

  it("refetches once on the first input after idleness, and not on the inputs after it", () => {
    const refetch = vi.fn();
    poller();
    resumer(refetch);
    pass(TEN_MINUTES);
    expect(refetch).not.toHaveBeenCalled();
    input("keydown");
    expect(refetch).toHaveBeenCalledTimes(1);
    input("keydown");
    input("pointermove");
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("is enabled when no second argument is given", () => {
    const refetch = vi.fn();
    renderHook(() => useRefetchOnResume(refetch));
    pass(TEN_MINUTES);
    input();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("refetches once each time the user comes back", () => {
    const refetch = vi.fn();
    resumer(refetch);
    pass(TEN_MINUTES);
    input();
    pass(TEN_MINUTES);
    input();
    expect(refetch).toHaveBeenCalledTimes(2);
  });

  it("refetches once when the document becomes visible again", () => {
    const refetch = vi.fn();
    resumer(refetch);
    turn("hidden");
    expect(refetch).not.toHaveBeenCalled();
    turn("visible");
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("does not refetch on a visibility event that changes nothing", () => {
    const refetch = vi.fn();
    resumer(refetch);
    turn("visible");
    expect(refetch).not.toHaveBeenCalled();
  });

  it("does not refetch on an input while the user is active", () => {
    const refetch = vi.fn();
    resumer(refetch);
    for (const type of INPUTS) input(type);
    pass(TEN_MINUTES - 1);
    input();
    expect(refetch).not.toHaveBeenCalled();
  });

  it("does not refetch because enabled turns true", () => {
    const refetch = vi.fn();
    poller();
    const hook = resumer(refetch, false);
    hook.rerender({ refetch, enabled: true });
    expect(refetch).not.toHaveBeenCalled();
  });

  it("does not refetch because enabled turns true in an idle store", () => {
    const refetch = vi.fn();
    poller();
    const hook = resumer(refetch, false);
    pass(TEN_MINUTES);
    hook.rerender({ refetch, enabled: true });
    expect(refetch).not.toHaveBeenCalled();
    input();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("never refetches while disabled", () => {
    const disabled = vi.fn();
    const enabled = vi.fn();
    resumer(disabled, false);
    resumer(enabled);
    pass(TEN_MINUTES);
    input();
    turn("hidden");
    turn("visible");
    expect(enabled).toHaveBeenCalledTimes(2);
    expect(disabled).not.toHaveBeenCalled();
  });

  it("stops refetching once disabled", () => {
    const refetch = vi.fn();
    const hook = resumer(refetch);
    hook.rerender({ refetch, enabled: false });
    turn("hidden");
    turn("visible");
    expect(refetch).not.toHaveBeenCalled();
  });

  it("calls the refetch of the latest render", () => {
    const first = vi.fn();
    const second = vi.fn();
    const onWindow = listenerLedger(window);
    const hook = resumer(first);
    hook.rerender({ refetch: second, enabled: true });
    expect(onWindow.added("keydown")).toHaveLength(1);
    pass(TEN_MINUTES);
    input();
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it("refetches once when an idle user's tab comes back, and not on the input after it", () => {
    const refetch = vi.fn();
    resumer(refetch);
    turn("hidden");
    pass(TEN_MINUTES);
    turn("visible");
    expect(refetch).toHaveBeenCalledTimes(1);
    input();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("keeps listening after the last poller has left", () => {
    const refetch = vi.fn();
    const polled = poller();
    resumer(refetch);
    polled.unmount();
    pass(TEN_MINUTES);
    input();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("shares one set of listeners and one timer with a poller and another resume hook", () => {
    const onWindow = listenerLedger(window);
    resumer(vi.fn());
    resumer(vi.fn());
    poller();
    for (const type of INPUTS) expect(onWindow.added(type)).toHaveLength(1);
    expect(vi.getTimerCount()).toBe(1);
  });

  it("removes the listeners when it was the only consumer", () => {
    const onWindow = listenerLedger(window);
    const hook = resumer(vi.fn());
    for (const type of INPUTS) expect(onWindow.attached(type)).toHaveLength(1);
    hook.unmount();
    for (const type of INPUTS) expect(onWindow.attached(type)).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
  });
});
