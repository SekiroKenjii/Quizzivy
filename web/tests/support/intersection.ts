/**
 * jsdom has no IntersectionObserver. This one records every observed element
 * and lets a test say "it scrolled into view", which is the whole contract a
 * lazy list relies on.
 */
type Callback = (entries: IntersectionObserverEntry[]) => void;

const observed = new Map<Element, Callback>();

export function installIntersectionObserver() {
  observed.clear();
  class Fake {
    private cb: Callback;
    constructor(cb: Callback) {
      this.cb = cb;
    }
    observe(el: Element) {
      observed.set(el, this.cb);
    }
    unobserve(el: Element) {
      observed.delete(el);
    }
    disconnect() {
      for (const [el, cb] of observed) if (cb === this.cb) observed.delete(el);
    }
    takeRecords() {
      return [];
    }
    root = null;
    rootMargin = "";
    thresholds = [0];
  }
  Object.defineProperty(globalThis, "IntersectionObserver", {
    configurable: true,
    writable: true,
    value: Fake,
  });
}

/** Every observed element scrolls into view. */
export function scrollAllIntoView() {
  for (const [el, cb] of observed) {
    cb([{ isIntersecting: true, target: el } as IntersectionObserverEntry]);
  }
}

export function observedCount() {
  return observed.size;
}
