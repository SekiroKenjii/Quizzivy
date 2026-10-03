import {
  useCallback,
  useLayoutEffect,
  useRef,
  type RefObject,
  type UIEvent,
} from "react";
import type { StudentGroup } from "./api";

/**
 * hasPassage reports whether a group brings something to read beside its
 * questions. Such a group gets the passage pane; a group with recordings
 * only is drawn inside the question pane.
 */
export function hasPassage(group: StudentGroup | undefined): group is StudentGroup {
  return group !== undefined && group.stimuli.length > 0;
}

/**
 * useKeptScroll keeps a pane's scroll position while the pane is hidden. A
 * hidden element has no scroll box, so the browser forgets where it was; the
 * hook remembers the last position scrolled to and puts it back when the pane
 * shows again. Pass the scrolling element's ref and give that element the
 * returned `onScroll`.
 */
export function useKeptScroll<T extends HTMLElement>(
  paneRef: RefObject<T | null>,
  hidden: boolean,
) {
  const top = useRef(0);
  useLayoutEffect(() => {
    if (!hidden && paneRef.current) paneRef.current.scrollTop = top.current;
  }, [hidden, paneRef]);
  return useCallback((event: UIEvent<T>) => {
    top.current = event.currentTarget.scrollTop;
  }, []);
}
