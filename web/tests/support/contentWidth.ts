import { act } from "@testing-library/react";
import { onTestFinished } from "vitest";
import { registerContentElement } from "@/layouts/shell/contentWidth";

/**
 * contentWidth puts the teacher console's content area at `px`, the width
 * `useContentWidthAtLeast` and `useContentBand` answer from. With no call
 * they answer "wide" in a tree without the shell and narrow, every threshold
 * unmet, in a tree that renders it: jsdom lays nothing out, so the shell's
 * `<main>` measures 0. Call it inside a test or a `beforeEach`: it registers
 * a stand-in for the shell's `<main>` and a ResizeObserver that fires only
 * when told, and takes both away when the test finishes. A `<main>` the tree
 * under test registers in the stand-in's place is measured at the same
 * width. The returned `resize` moves to another width and tells the store,
 * for a test of what a collapsed sidebar or a narrower window changes.
 */
export function contentWidth(px: number) {
  let width = px;

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
      if (target.localName !== "main") return;
      Object.defineProperty(target, "offsetWidth", {
        configurable: true,
        get: () => width,
      });
    }
    unobserve(target: Element) {
      this.targets.delete(target);
    }
    disconnect() {
      this.targets.clear();
    }
    tell() {
      for (const target of this.targets) {
        if (target.localName !== "main") continue;
        this.callback([{ target } as ResizeObserverEntry], this);
      }
    }
  }

  const previous = Object.getOwnPropertyDescriptor(globalThis, "ResizeObserver");
  Object.defineProperty(globalThis, "ResizeObserver", {
    configurable: true,
    writable: true,
    value: Observer,
  });
  act(() => registerContentElement(document.createElement("main")));

  onTestFinished(() => {
    act(() => registerContentElement(null));
    if (previous) Object.defineProperty(globalThis, "ResizeObserver", previous);
    else Reflect.deleteProperty(globalThis, "ResizeObserver");
  });

  return {
    resize(next: number) {
      width = next;
      act(() => {
        for (const observer of observers) observer.tell();
      });
    },
  };
}
