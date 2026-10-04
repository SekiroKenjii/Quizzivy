import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, renderHook, screen } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import {
  registerContentElement,
  useContentBand,
  useContentWidthAtLeast,
} from "@/layouts/shell/contentWidth";
import { contentWidth } from "@tests/support/contentWidth";

const COLUMNS = [560, 700, 760, 860] as const;

function box(width: number, padding = 0) {
  let current = width;
  const element = document.createElement("main");
  Object.defineProperty(element, "offsetWidth", {
    configurable: true,
    get: () => current,
  });
  element.style.paddingLeft = `${padding}px`;
  element.style.paddingRight = `${padding}px`;
  return {
    element,
    set(next: number, nextPadding = padding) {
      current = next;
      element.style.paddingLeft = `${nextPadding}px`;
      element.style.paddingRight = `${nextPadding}px`;
    },
  };
}

function stubResizeObserver() {
  const observers: Stub[] = [];
  class Stub {
    readonly callback: ResizeObserverCallback;
    readonly targets = new Map<Element, ResizeObserverOptions | undefined>();
    constructor(callback: ResizeObserverCallback) {
      this.callback = callback;
      observers.push(this);
    }
    observe(target: Element, options?: ResizeObserverOptions) {
      this.targets.set(target, options);
    }
    unobserve(target: Element) {
      this.targets.delete(target);
    }
    disconnect() {
      this.targets.clear();
    }
  }
  vi.stubGlobal("ResizeObserver", Stub);
  return {
    created: () => observers.length,
    observing: (element: Element) =>
      observers.filter((observer) => observer.targets.has(element)),
    fire(element: Element) {
      act(() => {
        for (const observer of observers) {
          if (!observer.targets.has(element)) continue;
          observer.callback([], observer as unknown as ResizeObserver);
        }
      });
    },
  };
}

function register(element: HTMLElement | null) {
  act(() => registerContentElement(element));
}

function atLeast(px: number) {
  return renderHook(() => useContentWidthAtLeast(px)).result;
}

function band(thresholds: readonly number[] = COLUMNS) {
  return renderHook(() => useContentBand(thresholds)).result;
}

describe("the content width with no element registered", () => {
  it("answers wide to every threshold", () => {
    expect(atLeast(860).current).toBe(true);
    expect(atLeast(100_000).current).toBe(true);
  });

  it("answers the last band", () => {
    expect(band().current).toBe(4);
    expect(band([]).current).toBe(0);
  });
});

describe("the content width in a server render", () => {
  it("answers wide, whatever is registered", () => {
    contentWidth(300);
    function Probe() {
      const wide = useContentWidthAtLeast(860);
      const columns = useContentBand(COLUMNS);
      return <output>{`${wide} ${columns}`}</output>;
    }
    expect(renderToString(<Probe />)).toBe("<output>true 4</output>");
  });
});

describe("useContentWidthAtLeast", () => {
  it.each([
    [859, false],
    [860, true],
    [861, true],
  ])("at %i answers %s to the deck's 860", (width, expected) => {
    contentWidth(width);
    expect(atLeast(860).current).toBe(expected);
  });

  it("follows the width across the threshold in both directions", () => {
    const area = contentWidth(859);
    const wide = atLeast(860);
    expect(wide.current).toBe(false);
    area.resize(860);
    expect(wide.current).toBe(true);
    area.resize(859);
    expect(wide.current).toBe(false);
  });

  it("does not re-render its consumer from 900 to 1000, and does at 800", () => {
    const area = contentWidth(900);
    let renders = 0;
    const hook = renderHook(() => {
      renders += 1;
      return useContentWidthAtLeast(860);
    });
    expect(renders).toBe(1);
    area.resize(1000);
    expect(renders).toBe(1);
    expect(hook.result.current).toBe(true);
    area.resize(800);
    expect(renders).toBe(2);
    expect(hook.result.current).toBe(false);
  });

  it("answers for a new threshold when the consumer asks for another", () => {
    contentWidth(800);
    const hook = renderHook(
      (props: { px: number }) => useContentWidthAtLeast(props.px),
      {
        initialProps: { px: 860 },
      },
    );
    expect(hook.result.current).toBe(false);
    hook.rerender({ px: 760 });
    expect(hook.result.current).toBe(true);
  });
});

describe("useContentBand", () => {
  it.each([
    [500, 0],
    [559, 0],
    [560, 1],
    [699, 1],
    [700, 2],
    [759, 2],
    [760, 3],
    [859, 3],
    [860, 4],
    [1200, 4],
  ])("at %i meets %i of 560, 700, 760 and 860", (width, expected) => {
    contentWidth(width);
    expect(band().current).toBe(expected);
  });

  it("takes a new array at every render", () => {
    const area = contentWidth(710);
    let renders = 0;
    const hook = renderHook(() => {
      renders += 1;
      return useContentBand([560, 700, 760, 860]);
    });
    expect(hook.result.current).toBe(2);
    area.resize(750);
    expect(renders).toBe(1);
    area.resize(900);
    expect(hook.result.current).toBe(4);
    expect(renders).toBe(2);
  });

  it("does not re-render its consumer inside a band, and does when the band changes", () => {
    const area = contentWidth(710);
    let renders = 0;
    const hook = renderHook(() => {
      renders += 1;
      return useContentBand(COLUMNS);
    });
    expect(hook.result.current).toBe(2);
    area.resize(750);
    area.resize(700);
    expect(renders).toBe(1);
    area.resize(760);
    expect(renders).toBe(2);
    expect(hook.result.current).toBe(3);
    area.resize(400);
    expect(renders).toBe(3);
    expect(hook.result.current).toBe(0);
  });
});

describe("a consumer that has unmounted", () => {
  it("is not asked again", () => {
    const area = contentWidth(710);
    let reads = 0;
    const thresholds = new Proxy([...COLUMNS], {
      get(target, key, receiver) {
        reads += 1;
        return Reflect.get(target, key, receiver);
      },
    });
    const hook = renderHook(() => useContentBand(thresholds));
    expect(hook.result.current).toBe(2);
    area.resize(600);
    expect(hook.result.current).toBe(1);
    expect(reads).toBeGreaterThan(0);
    hook.unmount();
    reads = 0;
    area.resize(900);
    expect(reads).toBe(0);
  });
});

describe("registerContentElement", () => {
  afterEach(() => {
    register(null);
    vi.unstubAllGlobals();
  });

  it("subtracts the horizontal padding from the border-box width", () => {
    stubResizeObserver();
    register(box(1000, 28).element);
    expect(atLeast(944).current).toBe(true);
    expect(atLeast(945).current).toBe(false);
  });

  it("reads the padding again at every measure", () => {
    const observer = stubResizeObserver();
    const main = box(1000, 28);
    register(main.element);
    const fits = atLeast(972);
    expect(fits.current).toBe(false);
    main.set(1000, 14);
    observer.fire(main.element);
    expect(fits.current).toBe(true);
  });

  it("observes the border box of the element", () => {
    const observer = stubResizeObserver();
    const main = box(900);
    register(main.element);
    const watching = observer.observing(main.element);
    expect(watching).toHaveLength(1);
    expect(watching[0]?.targets.get(main.element)).toEqual({ box: "border-box" });
  });

  it("tells a consumer that mounted before the element was registered", () => {
    stubResizeObserver();
    const wide = atLeast(860);
    const columns = band();
    expect(wide.current).toBe(true);
    register(box(600).element);
    expect(wide.current).toBe(false);
    expect(columns.current).toBe(1);
  });

  it("answers a page that mounts with its shell wide once, then by the measure, and wide again when the shell is gone", () => {
    stubResizeObserver();
    function Shell({ children }: Readonly<{ children: ReactNode }>) {
      return (
        <main
          ref={(element) => {
            if (element) Object.defineProperty(element, "offsetWidth", { value: 600 });
            registerContentElement(element);
          }}
        >
          {children}
        </main>
      );
    }
    const seen: boolean[] = [];
    const page = renderHook(
      () => {
        const wide = useContentWidthAtLeast(860);
        seen.push(wide);
        return wide;
      },
      { wrapper: Shell },
    );
    expect(seen).toEqual([true, false]);
    page.unmount();
    expect(atLeast(860).current).toBe(true);
  });

  it("returns to wide when null is registered, and stops observing", () => {
    const observer = stubResizeObserver();
    const main = box(600);
    register(main.element);
    const wide = atLeast(860);
    const columns = band();
    expect(wide.current).toBe(false);
    register(null);
    expect(wide.current).toBe(true);
    expect(columns.current).toBe(4);
    expect(observer.observing(main.element)).toHaveLength(0);
  });

  it("replaces the first element with a second and stops observing the first", () => {
    const observer = stubResizeObserver();
    const first = box(600);
    const second = box(900);
    register(first.element);
    const wide = atLeast(860);
    expect(wide.current).toBe(false);

    register(second.element);
    expect(wide.current).toBe(true);
    expect(observer.observing(first.element)).toHaveLength(0);
    expect(observer.observing(second.element)).toHaveLength(1);

    first.set(2000);
    second.set(700);
    observer.fire(second.element);
    expect(wide.current).toBe(false);
  });

  it("does not observe the same element twice when it is registered again", () => {
    const observer = stubResizeObserver();
    const main = box(900);
    register(main.element);
    register(main.element);
    expect(observer.created()).toBe(1);
    expect(observer.observing(main.element)).toHaveLength(1);
  });

  it("measures once and stays without a ResizeObserver", () => {
    vi.stubGlobal("ResizeObserver", undefined);
    const main = box(700);
    expect(() => register(main.element)).not.toThrow();
    const fits = atLeast(700);
    expect(fits.current).toBe(true);
    expect(atLeast(701).current).toBe(false);
    main.set(400);
    expect(fits.current).toBe(true);
    expect(() => register(null)).not.toThrow();
  });
});

describe("the contentWidth helper", () => {
  const original = globalThis.ResizeObserver;

  it("registers a width, resizes it and tells the store", () => {
    const area = contentWidth(500);
    const wide = atLeast(860);
    expect(wide.current).toBe(false);
    area.resize(1200);
    expect(wide.current).toBe(true);
    expect(globalThis.ResizeObserver).not.toBe(original);
  });

  it("has unregistered and put ResizeObserver back after the test that used it", () => {
    expect(atLeast(100_000).current).toBe(true);
    expect(globalThis.ResizeObserver).toBe(original);
  });

  it("leaves another observer of the page alone", () => {
    const area = contentWidth(500);
    const other = vi.fn();
    const observer = new ResizeObserver(other);
    observer.observe(document.body);
    area.resize(900);
    expect(other).not.toHaveBeenCalled();
  });

  it("holds its width for a shell that registers its own main, and resizes that one", () => {
    function Shell() {
      const columns = useContentBand(COLUMNS);
      return (
        <main ref={registerContentElement}>
          <output>{columns}</output>
        </main>
      );
    }
    const area = contentWidth(859);
    render(<Shell />);
    expect(screen.getByRole("status")).toHaveTextContent("3");
    area.resize(1200);
    expect(screen.getByRole("status")).toHaveTextContent("4");
    area.resize(600);
    expect(screen.getByRole("status")).toHaveTextContent("1");
  });

  describe("called from beforeEach", () => {
    let area: ReturnType<typeof contentWidth>;
    beforeEach(() => {
      area = contentWidth(700);
    });

    it("holds for the test", () => {
      const columns = band();
      expect(columns.current).toBe(2);
      area.resize(860);
      expect(columns.current).toBe(4);
    });
  });

  it("has unregistered after a test that registered in beforeEach", () => {
    expect(band().current).toBe(4);
  });
});
