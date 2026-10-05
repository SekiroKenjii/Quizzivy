import { act, cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MarqueeText } from "@/components/shared/MarqueeText";

const title = "Mid-term Reading Mock · Urban green space and the history of maps";
const widths = new Map<string, number>();
const observers: ObservedResize[] = [];
const mediaListeners = new Set<() => void>();
let frameWidth = 180;
let reduced = false;

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
  const node = track ? track.firstElementChild : root.firstElementChild;
  expect(node).not.toBeNull();
  return node!;
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

function expectLiveOriginal(root: Element) {
  const active = observers.filter((observer) => !observer.disconnected);
  expect(active).toHaveLength(1);
  const targets = [...active[0]!.targets];
  expect(targets).toHaveLength(2);
  expect(targets[0]).toBe(root);
  expect(targets[1]).toBe(original(root));
  expect(targets.every((node) => node.isConnected)).toBe(true);
}

function expectMoving(root: Element, moving: boolean) {
  expect(Boolean(root.querySelector(".qz-marquee-track"))).toBe(moving);
  expect(root.querySelectorAll('[aria-hidden="true"]')).toHaveLength(moving ? 1 : 0);
  expect(root.classList.contains("qz-marquee-masked")).toBe(moving);
  expectLiveOriginal(root);
}

beforeEach(() => {
  frameWidth = 180;
  reduced = false;
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
    } else if (this.getAttribute("aria-hidden") === "true") {
      width += 32;
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
    emit(original(root));
    expectMoving(root, true);
    widths.set(title, 463);
    emit(original(root));
    expectMoving(root, false);
  });

  it("disconnects replaced targets and all subscriptions on unmount", () => {
    const { container, unmount } = render(<MarqueeText text={title} />);
    const root = container.firstElementChild!;
    const movingOriginal = original(root);
    resize(root, 471);
    expect(movingOriginal.isConnected).toBe(false);
    expectMoving(root, false);
    expect(observers.slice(0, -1).every((observer) => observer.disconnected)).toBe(
      true,
    );
    const count = observers.length;
    emit(movingOriginal);
    expect(observers).toHaveLength(count);
    unmount();
    expect(observers.every((observer) => observer.disconnected)).toBe(true);
    expect(observers.every((observer) => observer.targets.size === 0)).toBe(true);
    expect(mediaListeners.size).toBe(0);
    emit(root);
    expect(observers).toHaveLength(count);
  });
});
