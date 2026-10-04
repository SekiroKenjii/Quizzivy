import { act } from "@testing-library/react";
import { onTestFinished } from "vitest";
import { registerContentElement } from "@/layouts/shell/contentWidth";

/**
 * contentWidth puts the teacher console's content area at `px`, the width
 * `useContentWidthAtLeast` and `useContentBand` answer from; with no call
 * they answer "wide". Call it inside a test or a `beforeEach`: it registers a
 * stand-in for the shell's `<main>` and a ResizeObserver that fires only when
 * told, and takes both away when the test finishes. The returned `resize`
 * moves to another width and tells the store, for a test of what a collapsed
 * sidebar or a narrower window changes.
 */
export function contentWidth(px: number) {
  let width = px;
  const element = document.createElement("main");
  Object.defineProperty(element, "offsetWidth", {
    configurable: true,
    get: () => width,
  });

  const observers = new Set<Observer>();
  class Observer implements ResizeObserver {
    private readonly callback: ResizeObserverCallback;
    private readonly targets = new Set<Element>();
    constructor(callback: ResizeObserverCallback) {
      this.callback = callback;
      observers.add(this);
    }
    observe(target: Element) {
      this.targets.add(target);
    }
    unobserve(target: Element) {
      this.targets.delete(target);
    }
    disconnect() {
      this.targets.clear();
    }
    tell(target: Element) {
      if (!this.targets.has(target)) return;
      this.callback([{ target } as ResizeObserverEntry], this);
    }
  }

  const previous = Object.getOwnPropertyDescriptor(globalThis, "ResizeObserver");
  Object.defineProperty(globalThis, "ResizeObserver", {
    configurable: true,
    writable: true,
    value: Observer,
  });
  act(() => registerContentElement(element));

  onTestFinished(() => {
    act(() => registerContentElement(null));
    if (previous) Object.defineProperty(globalThis, "ResizeObserver", previous);
    else Reflect.deleteProperty(globalThis, "ResizeObserver");
  });

  return {
    resize(next: number) {
      width = next;
      act(() => {
        for (const observer of observers) observer.tell(element);
      });
    },
  };
}
