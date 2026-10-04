import { useSyncExternalStore } from "react";

const WIDE = Number.POSITIVE_INFINITY;

const listeners = new Set<() => void>();
let element: HTMLElement | null = null;
let observer: ResizeObserver | null = null;
let width = WIDE;

function pixels(value: string) {
  return Number.parseFloat(value) || 0;
}

function contentWidthOf(area: HTMLElement) {
  const style = getComputedStyle(area);
  return area.offsetWidth - pixels(style.paddingLeft) - pixels(style.paddingRight);
}

function measure() {
  width = element ? contentWidthOf(element) : WIDE;
  for (const listener of [...listeners]) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

const wide = () => true;

/**
 * registerContentElement names the element whose content width the console's
 * pages lay themselves out by: the shell passes it as the ref of `<main>`.
 * One element at a time: a second call replaces the first and stops observing
 * it, and `null` unregisters. The width is the element's border-box width
 * minus its computed horizontal padding, the deck's `tw`, so it changes when
 * the sidebar collapses and not when a scrollbar appears. It is measured here
 * and on every resize; without a ResizeObserver it is measured once. While no
 * element is registered every answer is "wide".
 */
export function registerContentElement(next: HTMLElement | null): void {
  if (next === element) return;
  observer?.disconnect();
  observer = null;
  element = next;
  if (next && typeof ResizeObserver === "function") {
    observer = new ResizeObserver(measure);
    observer.observe(next, { box: "border-box" });
  }
  measure();
}

/**
 * useContentWidthAtLeast reports whether the content area is at least `px`
 * wide, and true while no element is registered. The consumer re-renders
 * only when the answer changes.
 */
export function useContentWidthAtLeast(px: number): boolean {
  return useSyncExternalStore(subscribe, () => width >= px, wide);
}

/**
 * useContentBand counts how many of `thresholds`, given in ascending order,
 * the content area's width meets: 0 below the first, `thresholds.length` at
 * the last and while no element is registered. It is one subscription for a
 * whole set of thresholds, and the consumer re-renders only when the count
 * changes.
 */
export function useContentBand(thresholds: readonly number[]): number {
  return useSyncExternalStore(
    subscribe,
    () => thresholds.filter((threshold) => width >= threshold).length,
    () => thresholds.length,
  );
}
