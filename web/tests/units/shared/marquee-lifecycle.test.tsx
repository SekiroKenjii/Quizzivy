import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MarqueeText } from "@/components/shared/MarqueeText";

const title = "Mid-term Reading Mock · Urban green space and the history of maps";
const widths = new Map<string, number>();
const observers: ObservedResize[] = [];
const mediaListeners = new Set<() => void>();
let frameWidth = 180;
let reduced = false;
let inlineHasZeroObservedBox = false;

class ObservedResize implements ResizeObserver {
  readonly targets = new Set<Element>();
  disconnected = false;

  readonly callback: ResizeObserverCallback;

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
    observers.push(this);
  }

  observe(target: Element) {
    this.targets.add(target);
  }

  unobserve(target: Element) {
    this.targets.delete(target);
  }

  disconnect() {
    this.disconnected = true;
    this.targets.clear();
  }
}

function original(root: Element) {
  const track = root.querySelector(".qz-marquee-track");
  const node = track
    ? track.firstElementChild
    : root.querySelector(':scope > span:not([data-slot="marquee-measure"])');
  expect(node).not.toBeNull();
  return node!;
}

function measureCopy(root: Element) {
  return root.querySelector('[data-slot="marquee-measure"]');
}

function deliverIntrinsicLayout(root: Element) {
  const target = measureCopy(root) ?? original(root);
  if (
    inlineHasZeroObservedBox &&
    !measureCopy(root) &&
    !target.parentElement?.classList.contains("qz-marquee-track")
  )
    return;
  emit(target);
}

function emit(target: Element) {
  act(() => {
    for (const observer of [...observers]) {
      if (!observer.disconnected && observer.targets.has(target)) {
        observer.callback([], observer);
      }
    }
  });
}

function resize(root: Element, width: number) {
  frameWidth = width;
  emit(root);
}

function changeReduced(value: boolean) {
  act(() => {
    reduced = value;
    for (const listener of mediaListeners) listener();
  });
}

function expectLiveMeasure(root: Element) {
  const active = observers.filter((observer) => !observer.disconnected);
  expect(active).toHaveLength(1);
  const targets = [...active[0]!.targets];
  expect(targets).toHaveLength(2);
  expect(targets[0]).toBe(root);
  expect(targets[1]).toBe(measureCopy(root));
  expect(targets.every((node) => node.isConnected)).toBe(true);
}

function expectMoving(root: Element, moving: boolean) {
  expect(Boolean(root.querySelector(".qz-marquee-track"))).toBe(moving);
  expect(root.querySelectorAll('.qz-marquee-track [aria-hidden="true"]')).toHaveLength(
    moving ? 1 : 0,
  );
  expect(measureCopy(root)).toHaveAttribute("aria-hidden", "true");
  expect(root.classList.contains("qz-marquee-masked")).toBe(moving);
  expectLiveMeasure(root);
}

beforeEach(() => {
  frameWidth = 180;
  reduced = false;
  inlineHasZeroObservedBox = false;
  widths.clear();
  widths.set(title, 463);
  widths.set("Short", 37);
  observers.length = 0;
  mediaListeners.clear();
  vi.stubGlobal("ResizeObserver", ObservedResize);
  vi.spyOn(window, "matchMedia").mockImplementation(
    (query) =>
      ({
        media: query,
        get matches() {
          return query.includes("prefers-reduced-motion") && reduced;
        },
        addEventListener: (_event: string, listener: () => void) => {
          mediaListeners.add(listener);
        },
        removeEventListener: (_event: string, listener: () => void) => {
          mediaListeners.delete(listener);
        },
      }) as unknown as MediaQueryList,
  );
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(function (
    this: HTMLElement,
  ) {
    return this.classList.contains("qz-marquee") ? frameWidth : 0;
  });
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (
    this: HTMLElement,
  ) {
    let width = this.isConnected ? (widths.get(this.textContent ?? "") ?? 0) : 0;
    if (this.classList.contains("qz-marquee-track")) {
      width = 2 * (widths.get(this.firstElementChild?.textContent ?? "") ?? 0) + 32;
    } else if (this.classList.contains("qz-marquee")) {
      width = frameWidth;
    } else if (
      this.parentElement?.classList.contains("qz-marquee-track") &&
      this.getAttribute("aria-hidden") === "true"
    ) {
      width += 32;
    }
    if (this.dataset.slot === "marquee-measure") {
      if (this.hidden || this.classList.contains("hidden")) width = 0;
      else if (
        this.classList.contains("w-full") ||
        this.classList.contains("max-w-full")
      )
        width = Math.min(width, frameWidth);
    }
    return { width } as DOMRect;
  });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("MarqueeText live measurement", () => {
  it("observes one live original through overflow, fit, and overflow", () => {
    const { container } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expectMoving(root, true);
    resize(root, 471);
    expectMoving(root, false);
    expect(root).not.toHaveAttribute("title");
    resize(root, 180);
    expectMoving(root, true);
    expect(root).toHaveAttribute("title", title);
  });

  it("rebinds after a fitting title starts moving and later fits again", () => {
    frameWidth = 900;
    const { container } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expectMoving(root, false);
    resize(root, 180);
    expectMoving(root, true);
    resize(root, 472);
    expectMoving(root, false);
    resize(root, 180);
    expectMoving(root, true);
  });

  it("restarts a shortened title when its new frame becomes narrower", () => {
    const { container, rerender } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    rerender(<MarqueeText text="Short" />);
    expectMoving(root, false);
    expect(root).not.toHaveAttribute("title");
    resize(root, 12);
    expectMoving(root, true);
    expect(root).toHaveAttribute("title", "Short");
    resize(root, 40);
    expectMoving(root, false);
  });

  it("rebinds across repeated reduced-motion changes, text changes, and resizes", () => {
    reduced = true;
    const { container, rerender } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expectMoving(root, false);
    expect(root).toHaveClass("text-ellipsis");
    expect(root).toHaveAttribute("title", title);
    changeReduced(false);
    expectMoving(root, true);
    changeReduced(true);
    expectMoving(root, false);
    rerender(<MarqueeText text="Short" />);
    resize(root, 12);
    expectMoving(root, false);
    expect(root).toHaveAttribute("title", "Short");
    changeReduced(false);
    expectMoving(root, true);
    changeReduced(true);
    expectMoving(root, false);
    resize(root, 80);
    changeReduced(false);
    expectMoving(root, false);
    expect(root).not.toHaveAttribute("title");
    rerender(<MarqueeText text={title} />);
    expectMoving(root, true);
  });

  it("uses the strict frame-plus-one threshold after live resizes", () => {
    frameWidth = 462;
    const { container } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expectMoving(root, false);
    resize(root, 461);
    expectMoving(root, true);
    resize(root, 462);
    expectMoving(root, false);
    resize(root, 463);
    expectMoving(root, false);
  });

  it("responds to the observed original changing width without a text update", () => {
    frameWidth = 500;
    const { container } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expectMoving(root, false);
    widths.set(title, 600);
    emit(measureCopy(root)!);
    expectMoving(root, true);
    widths.set(title, 463);
    emit(measureCopy(root)!);
    expectMoving(root, false);
  });

  it("disconnects replaced targets and all subscriptions on unmount", () => {
    const { container, unmount } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    const movingOriginal = original(root);
    const stableMeasure = measureCopy(root);
    resize(root, 471);
    expect(movingOriginal.isConnected).toBe(false);
    expect(measureCopy(root)).toBe(stableMeasure);
    expect(stableMeasure?.isConnected).toBe(true);
    expectMoving(root, false);
    expect(observers.slice(0, -1).every((observer) => observer.disconnected)).toBe(
      true,
    );
    const count = observers.length;
    emit(movingOriginal);
    expect(observers).toHaveLength(count);
    unmount();
    expect(stableMeasure?.isConnected).toBe(false);
    expect(observers.every((observer) => observer.disconnected)).toBe(true);
    expect(observers.every((observer) => observer.targets.size === 0)).toBe(true);
    expect(mediaListeners.size).toBe(0);
    emit(root);
    expect(observers).toHaveLength(count);
  });
});

describe("MarqueeText intrinsic layout", () => {
  it("starts and stops from intrinsic layout delivery while the static visible inline box has no size entry", () => {
    inlineHasZeroObservedBox = true;
    frameWidth = 100;
    widths.set(title, 85);
    const { container } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expect(root.querySelector(".qz-marquee-track")).toBeNull();
    expect(root).not.toHaveAttribute("title");
    const stableMeasure = measureCopy(root);
    widths.set(title, 172);
    deliverIntrinsicLayout(root);
    expect(root.querySelector(".qz-marquee-track")).not.toBeNull();
    expect(root).toHaveAttribute("title", title);
    expect(measureCopy(root)).toBe(stableMeasure);
    widths.set(title, 85);
    deliverIntrinsicLayout(root);
    expect(root.querySelector(".qz-marquee-track")).toBeNull();
    expect(root).not.toHaveAttribute("title");
    expect(measureCopy(root)).toBe(stableMeasure);
  });

  it("keeps reduced ellipsis current through intrinsic growth, shrink, and normal-motion restoration", () => {
    inlineHasZeroObservedBox = true;
    reduced = true;
    frameWidth = 100;
    widths.set(title, 85);
    const { container } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    const stableMeasure = measureCopy(root);
    widths.set(title, 172);
    deliverIntrinsicLayout(root);
    expect(root).toHaveAttribute("title", title);
    expect(root).toHaveClass("text-ellipsis");
    expect(root.querySelector(".qz-marquee-track")).toBeNull();
    widths.set(title, 85);
    deliverIntrinsicLayout(root);
    expect(root).not.toHaveAttribute("title");
    widths.set(title, 172);
    deliverIntrinsicLayout(root);
    changeReduced(false);
    expect(root.querySelector(".qz-marquee-track")).not.toBeNull();
    expect(measureCopy(root)).toBe(stableMeasure);
    changeReduced(true);
    expect(root.querySelector(".qz-marquee-track")).toBeNull();
    expect(root).toHaveClass("text-ellipsis");
    expect(root).toHaveAttribute("title", title);
    expect(measureCopy(root)).toBe(stableMeasure);
  });

  it("compares fractional intrinsic width strictly below, equal to, and above frame plus one", () => {
    inlineHasZeroObservedBox = true;
    frameWidth = 100;
    widths.set(title, 100.999);
    const { container } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expect(root).not.toHaveAttribute("title");
    widths.set(title, 101);
    deliverIntrinsicLayout(root);
    expect(root.querySelector(".qz-marquee-track")).toBeNull();
    expect(root).not.toHaveAttribute("title");
    widths.set(title, 101.001);
    deliverIntrinsicLayout(root);
    expect(root.querySelector(".qz-marquee-track")).not.toBeNull();
    expect(root).toHaveAttribute("title", title);
    widths.set(title, 101);
    deliverIntrinsicLayout(root);
    expect(root.querySelector(".qz-marquee-track")).toBeNull();
    expect(root).not.toHaveAttribute("title");
  });

  it.each([false, true])(
    "exposes one accessible title and a separate noninteractive intrinsic measure while reduced=%s",
    (reduce) => {
      reduced = reduce;
      frameWidth = 500;
      const { container, rerender } = render(
        <button>
          <MarqueeText text={title} />
        </button>,
      );
      const root = container.querySelector(".qz-marquee")!;
      expect(screen.getByRole("button", { name: title })).toBe(
        container.firstElementChild,
      );
      const copy = measureCopy(root);
      expect(copy).not.toBeNull();
      expect(copy).toHaveAttribute("aria-hidden", "true");
      expect(copy).not.toHaveAttribute("role");
      expect(copy).not.toHaveAttribute("tabindex");
      expect(copy).not.toHaveAttribute("hidden");
      expect(copy).toHaveClass(
        "absolute",
        "invisible",
        "pointer-events-none",
        "w-max",
        "max-w-none",
        "whitespace-nowrap",
      );
      for (const forbidden of [
        "hidden",
        "sr-only",
        "w-full",
        "max-w-full",
        "inline-block",
      ]) {
        expect(copy).not.toHaveClass(forbidden);
      }
      expect(copy?.parentElement).toBe(root);
      expect(copy?.closest(".qz-marquee-track")).toBeNull();
      expect(copy?.childElementCount).toBe(0);
      expect(root).toHaveClass("relative");
      expect(original(root).className).toBe("");
      resize(root, 180);
      expect(screen.getByRole("button", { name: title })).toBe(
        container.firstElementChild,
      );
      expect(measureCopy(root)).toBe(copy);
      expect(
        root.querySelectorAll('.qz-marquee-track [aria-hidden="true"]'),
      ).toHaveLength(reduce ? 0 : 1);
      expect(copy?.getBoundingClientRect().width).toBe(463);
      expect(root.getBoundingClientRect().width).toBe(180);
      rerender(
        <button>
          <MarqueeText text="Short" />
        </button>,
      );
      expect(screen.getByRole("button", { name: "Short" })).toBe(
        container.firstElementChild,
      );
      expect(measureCopy(root)).toBe(copy);
      expect(copy?.textContent).toBe("Short");
    },
  );

  it("measures immediately and on text changes without ResizeObserver", () => {
    vi.stubGlobal("ResizeObserver", undefined);
    frameWidth = 100;
    widths.set(title, 85);
    const { container, rerender } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    expect(root).not.toHaveAttribute("title");
    rerender(<MarqueeText text="Short" />);
    expect(root).not.toHaveAttribute("title");
    widths.set(title, 172);
    rerender(<MarqueeText text={title} />);
    expect(root).toHaveAttribute("title", title);
    expect(root.querySelector(".qz-marquee-track")).not.toBeNull();
    expect(observers).toHaveLength(0);
  });

  it("disconnects the stable layout copy and frame completely on unmount", () => {
    inlineHasZeroObservedBox = true;
    frameWidth = 100;
    widths.set(title, 85);
    const { container, unmount } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    const copy = measureCopy(root);
    expect(copy).not.toBeNull();
    const active = observers.filter((observer) => !observer.disconnected);
    expect(active).toHaveLength(1);
    expect(active[0]!.targets.has(copy!)).toBe(true);
    unmount();
    expect(copy?.isConnected).toBe(false);
    expect(
      observers.every(
        (observer) => observer.disconnected && observer.targets.size === 0,
      ),
    ).toBe(true);
    expect(mediaListeners.size).toBe(0);
    const count = observers.length;
    widths.set(title, 172);
    emit(copy!);
    emit(root);
    expect(observers).toHaveLength(count);
  });
});
