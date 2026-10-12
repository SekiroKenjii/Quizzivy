/**
 * revealWithin scrolls `frame` sideways, and nothing else, until `element`
 * lies inside it with `padding` to spare on the side where it was cut. An
 * element already inside stays where it is. A browser does this by itself
 * only for a focused element that is wholly out of view, and it may scroll
 * the page as well.
 */
export function revealWithin(frame: HTMLElement, element: Element, padding = 0) {
  const outer = frame.getBoundingClientRect();
  const box = element.getBoundingClientRect();
  if (box.left < outer.left) {
    frame.scrollLeft -= outer.left - box.left + padding;
  } else if (box.right > outer.right) {
    frame.scrollLeft += box.right - outer.right + padding;
  }
}
