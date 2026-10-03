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
 * questionShowing reports whether the question's pane is on screen. On a
 * phone showing the passage it is not, and the keys that answer or flag must
 * not act on a question the student cannot see.
 */
export function questionShowing(questionId: string): boolean {
  const sheet = document.getElementById(`answer-question-${questionId}`);
  return sheet === null || sheet.closest("[hidden]") === null;
}

/**
 * useKeptScroll keeps a pane's scroll position while the pane is hidden. A
 * hidden element has no scroll box, so the browser forgets where it was; the
 * hook remembers the last position scrolled to and puts it back when the pane
 * shows again. The position belongs to `owner`: when the owner changes, the
 * pane starts at the top. Pass the scrolling element's ref and give that
 * element the returned `onScroll`.
 */
export function useKeptScroll<T extends HTMLElement>(
  paneRef: RefObject<T | null>,
  hidden: boolean,
  owner?: string,
) {
  const top = useRef(0);
  const kept = useRef(owner);
  useLayoutEffect(() => {
    if (kept.current !== owner) {
      kept.current = owner;
      top.current = 0;
    }
    if (!hidden && paneRef.current) paneRef.current.scrollTop = top.current;
  }, [hidden, owner, paneRef]);
  return useCallback((event: UIEvent<T>) => {
    top.current = event.currentTarget.scrollTop;
  }, []);
}
