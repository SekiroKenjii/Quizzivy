import {
  useCallback,
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useState,
  type PointerEvent,
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
 * questionShowing reports whether the question's pane is on screen. Under an
 * open stimulus drawer, or in a hidden pane, it is not, and the keys that
 * answer or flag must not act on a question the student cannot see.
 */
export function questionShowing(questionId: string): boolean {
  if (document.querySelector("[data-drawer-open]") !== null) return false;
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

const SEEN_PREFIX = "quizzivy.stimulus-seen.";

function readSeen(attemptId: string | undefined): ReadonlySet<string> {
  if (attemptId === undefined) return new Set();
  try {
    const raw = sessionStorage.getItem(SEEN_PREFIX + attemptId);
    const parsed: unknown = raw === null ? [] : JSON.parse(raw);
    return new Set(
      Array.isArray(parsed) ? parsed.filter((id) => typeof id === "string") : [],
    );
  } catch {
    return new Set();
  }
}

/**
 * useSeenStimuli remembers which groups' drawers the student has opened in
 * this attempt, in `sessionStorage['quizzivy.stimulus-seen.<attemptId>']`,
 * so the handle stops nudging and its tip goes once a group has been seen.
 * Without an attempt, or where storage is refused, it remembers in memory.
 */
export function useSeenStimuli(attemptId: string | undefined) {
  const [seen, setSeen] = useState(() => readSeen(attemptId));
  useEffect(() => {
    if (attemptId === undefined || seen.size === 0) return;
    try {
      sessionStorage.setItem(SEEN_PREFIX + attemptId, JSON.stringify([...seen]));
    } catch {
      return;
    }
  }, [attemptId, seen]);
  const mark = useCallback((groupId: string) => {
    setSeen((current) =>
      current.has(groupId) ? current : new Set(current).add(groupId),
    );
  }, []);
  return { seen, mark };
}

/** DRAWER_SHARE and DRAWER_MAX_PX size the stimulus drawer: min(88%, 420px). */
export const DRAWER_SHARE = 0.88;
export const DRAWER_MAX_PX = 420;

const START_PX = 10;
const HORIZONTAL = 1.2;
const OPEN_PAST = 0.25;
const CLOSE_PAST = 0.3;
const SWALLOW_MS = 60;

interface Drag {
  x0: number;
  y0: number;
  width: number;
  open0: boolean;
  horizontal: boolean;
  offset: number | null;
}

function capture(element: HTMLElement, pointerId: number) {
  try {
    element.setPointerCapture(pointerId);
  } catch {
    return;
  }
}

/**
 * useDrawerSwipe lets a finger or a pointer pull the stimulus drawer open
 * from the left edge of `root` and push it closed. A swipe starts after 10px
 * of travel that is more than 1.2 times as horizontal as vertical, in the
 * direction that moves the drawer; anything else is left to the page, so a
 * vertical scroll or a tap is never taken. Released, it opens past a quarter
 * of the drawer's width and closes when dragged back past 30%, and calls
 * `onSettle` with the result; a click within 60ms of a swipe is swallowed.
 * `offset` is the drawer's place while dragging, in pixels from fully open
 * (0) to fully closed (minus `width`), and null otherwise.
 */
export function useDrawerSwipe({
  root,
  enabled,
  open,
  onSettle,
}: {
  root: RefObject<HTMLElement | null>;
  enabled: boolean;
  open: boolean;
  onSettle: (open: boolean) => void;
}) {
  const drag = useRef<Drag | null>(null);
  const swallowUntil = useRef(0);
  const [offset, setOffset] = useState<number | null>(null);
  const [width, setWidth] = useState(DRAWER_MAX_PX);
  if (!enabled && offset !== null) setOffset(null);

  const onPointerDown = (event: PointerEvent<HTMLElement>) => {
    if (!enabled || (event.pointerType === "mouse" && event.button !== 0)) return;
    const full = root.current?.getBoundingClientRect().width ?? 0;
    drag.current = {
      x0: event.clientX,
      y0: event.clientY,
      width: Math.min(Math.round(full * DRAWER_SHARE), DRAWER_MAX_PX),
      open0: open,
      horizontal: false,
      offset: null,
    };
  };

  const onPointerMove = (event: PointerEvent<HTMLElement>) => {
    const current = drag.current;
    if (current === null) return;
    const dx = event.clientX - current.x0;
    const dy = event.clientY - current.y0;
    if (!current.horizontal) {
      if (Math.abs(dx) < START_PX && Math.abs(dy) < START_PX) return;
      const toward = current.open0 ? dx < 0 : dx > 0;
      if (Math.abs(dx) <= Math.abs(dy) * HORIZONTAL || !toward || current.width <= 0) {
        drag.current = null;
        return;
      }
      current.horizontal = true;
      setWidth(current.width);
      capture(event.currentTarget, event.pointerId);
    }
    const from = current.open0 ? dx : dx - current.width;
    current.offset = Math.max(-current.width, Math.min(0, from));
    setOffset(current.offset);
  };

  const onPointerUp = () => {
    const current = drag.current;
    drag.current = null;
    if (current === null || !current.horizontal) return;
    swallowUntil.current = Date.now() + SWALLOW_MS;
    const at = current.offset ?? (current.open0 ? 0 : -current.width);
    const share = current.open0 ? CLOSE_PAST : 1 - OPEN_PAST;
    setOffset(null);
    onSettle(at > -current.width * share);
  };

  useEffect(() => {
    const element = root.current;
    if (element === null) return;
    const swallow = (event: Event) => {
      if (Date.now() >= swallowUntil.current) return;
      event.preventDefault();
      event.stopPropagation();
    };
    element.addEventListener("click", swallow, true);
    return () => element.removeEventListener("click", swallow, true);
  }, [root]);

  return {
    offset,
    width,
    handlers: {
      onPointerDown,
      onPointerMove,
      onPointerUp,
      onPointerCancel: onPointerUp,
    },
  };
}

/**
 * useSharedAudioPlaying reports whether a shared recording under `root` is
 * playing, from the media events that reach it: what the drawer's handle
 * marks with its dot.
 */
export function useSharedAudioPlaying(root: RefObject<HTMLElement | null>): boolean {
  const [playing, setPlaying] = useState(false);
  useEffect(() => {
    const element = root.current;
    if (element === null) return;
    const update = () =>
      setPlaying(
        [
          ...element.querySelectorAll<HTMLAudioElement>("[data-shared-audio] audio"),
        ].some((audio) => !audio.paused),
      );
    const events = ["play", "playing", "pause", "ended", "emptied"];
    for (const name of events) element.addEventListener(name, update, true);
    return () => {
      for (const name of events) element.removeEventListener(name, update, true);
    };
  }, [root]);
  return playing;
}

function dialogOpen(): boolean {
  return (
    document.querySelector(
      '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
    ) !== null
  );
}

/**
 * useStimulusDrawer runs the phone's stimulus drawer for the open question
 * (`visit`, which changes on every move, so any move closes it) of the group
 * `groupId`. It opens from the handle or a swipe and remembers the group as
 * seen; it closes from its close button, the backdrop, Esc or a swipe back,
 * and then gives the focus back to the handle, while `dismiss` closes it
 * without, for a move that lands elsewhere. Opening moves the focus to the
 * close button. `progress` is how far it is out, from 0 to 1, for the
 * backdrop; `fresh` says the group has not been opened in this attempt.
 */
export function useStimulusDrawer({
  root,
  enabled,
  visit,
  groupId,
  attemptId,
}: {
  root: RefObject<HTMLElement | null>;
  enabled: boolean;
  visit: string;
  groupId: string | undefined;
  attemptId: string | undefined;
}) {
  const [openFor, setOpenFor] = useState<string | null>(null);
  if (openFor !== null && openFor !== visit) setOpenFor(null);
  const open = enabled && openFor === visit;
  const { seen, mark } = useSeenStimuli(attemptId);
  const handleRef = useRef<HTMLButtonElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const backToHandleRef = useRef(false);
  const wasOpenRef = useRef(open);

  const show = () => {
    setOpenFor(visit);
    if (groupId !== undefined) mark(groupId);
  };
  const close = () => {
    backToHandleRef.current = true;
    setOpenFor(null);
  };
  const dismiss = () => setOpenFor(null);
  const swipe = useDrawerSwipe({
    root,
    enabled,
    open,
    onSettle: (next) => (next ? show() : close()),
  });

  useEffect(() => {
    if (open && !wasOpenRef.current) closeRef.current?.focus();
    if (!open && wasOpenRef.current && backToHandleRef.current)
      handleRef.current?.focus();
    backToHandleRef.current = false;
    wasOpenRef.current = open;
  }, [open]);

  const onEscape = useEffectEvent((event: KeyboardEvent) => {
    if (event.key !== "Escape" || event.defaultPrevented || dialogOpen()) return;
    close();
  });
  useEffect(() => {
    if (!open) return;
    const listener = (event: KeyboardEvent) => onEscape(event);
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, [open]);

  let progress = open ? 1 : 0;
  if (swipe.offset !== null) progress = 1 + swipe.offset / swipe.width;
  return {
    open,
    out: open || swipe.offset !== null,
    offset: swipe.offset,
    progress,
    fresh: groupId !== undefined && !seen.has(groupId),
    show,
    close,
    dismiss,
    handlers: swipe.handlers,
    handleRef,
    closeRef,
  };
}
