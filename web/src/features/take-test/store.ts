import { create } from "zustand";
import { ApiError } from "@/lib/api/errors";
import {
  getAttempt,
  recordAudioPlay,
  saveAnswers,
  submitAttempt,
  type Answer,
  type AttemptSession,
  type IntegrityPolicy,
  type StudentQuestion,
} from "./api";

/**
 * Why the paper is no longer writable. Null while the student is working.
 *
 * These are states, not errors: §9's whole argument is that a test must never
 * throw a stack trace at a fifteen-year-old halfway through. Each one has an
 * explanation the UI renders and an action it offers, so the store records
 * which happened and lets the screen say it.
 */
export type LockReason =
  /** Opened elsewhere. This tab lost and the other one is now the attempt (§10.1, E2E 7). */
  | "superseded"
  /** Time is up. The client's job is to submit, not to keep typing. */
  | "deadline"
  /** Already submitted or voided; nothing further to do. */
  | "closed";

export type SubmitState = "idle" | "inFlight" | "done";

interface TakeTestState {
  attemptId: string | null;
  sessionId: string | null;
  /** Already in presentation order; the server shuffled it and it must not move. */
  questions: StudentQuestion[];
  answers: Record<string, Answer>;
  /** Question ids edited since the server last confirmed them. */
  dirty: Set<string>;
  /** When each answer was last edited locally, in device time. */
  touchedAt: Record<string, number>;
  audioPlays: Record<string, number>;
  integrity: IntegrityPolicy | null;

  deadlineAt: number;
  /** serverTime minus device time at load. See remainingMs. */
  offsetMs: number;

  lock: LockReason | null;
  submitState: SubmitState;
  flushInFlight: boolean;
  retryDelayMs: number;
  /** When the server last confirmed a save, per its clock. Null until one lands. */
  lastSavedAt: string | null;

  hydrate: (session: AttemptSession) => void;
  setAnswer: (questionId: string, answer: Answer) => void;
  notePlay: (questionId: string) => void;
  flush: () => Promise<void>;
  submit: (reason?: "manual" | "timer_expired" | "auto_submit") => Promise<void>;
  lockNow: (reason: LockReason) => void;
  reset: () => void;
}

/** The first retry delay, doubling to RETRY_CEILING_MS. */
const RETRY_BASE_MS = 1_000;
const RETRY_CEILING_MS = 30_000;

/** §3's builder debounce, and the same reasoning: fast enough to feel saved. */
export const FLUSH_DEBOUNCE_MS = 1_500;

const initial = {
  attemptId: null,
  sessionId: null,
  questions: [] as StudentQuestion[],
  answers: {} as Record<string, Answer>,
  dirty: new Set<string>(),
  touchedAt: {} as Record<string, number>,
  audioPlays: {} as Record<string, number>,
  integrity: null,
  deadlineAt: 0,
  offsetMs: 0,
  lock: null,
  submitState: "idle" as SubmitState,
  flushInFlight: false,
  retryDelayMs: RETRY_BASE_MS,
  lastSavedAt: null,
};

export const useTakeTestStore = create<TakeTestState>((set, get) => ({
  ...initial,

  /**
   * Applies a payload from start, resume or refetch.
   *
   * The RESUME MERGE lives here, and it only ever runs one way. Server answers
   * are the base; a locally dirty answer overwrites it. Dirty means "edited
   * since the server last confirmed it", so a dirty answer is by construction
   * newer than anything the server holds -- which is why this needs no
   * per-answer timestamp from the API, and could not use one anyway: the
   * contract does not send updated_at, and a device clock is not evidence.
   *
   * Never the reverse. §1.2's promise is that a refresh does not lose work, and
   * letting the server's older copy win would break exactly that.
   */
  hydrate: (session) =>
    set((state) => {
      const answers = { ...session.answers };
      for (const questionId of state.dirty) {
        const local = state.answers[questionId];
        if (local !== undefined) answers[questionId] = local;
      }
      return {
        attemptId: session.attempt.id,
        sessionId: session.sessionId,
        questions: session.questions,
        answers,
        audioPlays: session.audioPlays,
        integrity: session.integrity,
        deadlineAt: Date.parse(session.attempt.deadlineAt),
        // Measured once per payload, and every save returns serverTime so it
        // keeps being re-measured over a long test rather than drifting from
        // whatever it was at the start.
        offsetMs: Date.parse(session.serverTime) - Date.now(),
        lock: lockFor(session),
      };
    }),

  setAnswer: (questionId, answer) => {
    if (get().lock !== null) return;
    set((state) => {
      const dirty = new Set(state.dirty);
      dirty.add(questionId);
      return {
        answers: { ...state.answers, [questionId]: answer },
        dirty,
        touchedAt: { ...state.touchedAt, [questionId]: Date.now() },
      };
    });
    scheduleFlush();
  },

  /**
   * Counts a play the moment it starts, then lets the server correct it.
   *
   * Optimistic in both directions §11.4 asks for: the number on screen moves
   * immediately, because a student watching "còn 2 lượt nghe" not change has no
   * way to tell a slow network from a lost play; and a failed POST is NOT
   * rolled back, because the play really did happen and the next fetch is what
   * settles the count. Nothing here can block playback -- it is called after
   * .play(), returns nothing, and swallows its own failure.
   */
  notePlay: (questionId) => {
    const { attemptId, audioPlays } = get();
    if (attemptId === null) return;

    set({
      audioPlays: { ...audioPlays, [questionId]: (audioPlays[questionId] ?? 0) + 1 },
    });
    recordAudioPlay(attemptId, questionId)
      .then((counted) =>
        set((state) => ({
          audioPlays: { ...state.audioPlays, [questionId]: counted.plays },
        })),
      )
      .catch(() => {
        // The count is the server's, and it will be right on the next fetch.
        // There is nothing to tell the student here that is not noise.
      });
  },

  /**
   * Sends everything dirty, and clears only what is still unchanged when the
   * reply arrives.
   *
   * The subtle part is what happens to an answer edited WHILE the request is in
   * flight. Marking the whole batch clean on success would discard that edit
   * silently -- the value stays on screen, the server never receives it, and
   * the student finds out at grading. So the batch is compared against
   * touchedAt afterwards and anything newer stays dirty for the next flush.
   *
   * A failure clears nothing, ever. The local answer is the only copy.
   */
  flush: async () => {
    const state = get();
    const { attemptId, sessionId } = state;
    if (attemptId === null || sessionId === null) return;
    if (state.lock !== null || state.flushInFlight || state.dirty.size === 0) return;

    const sending = [...state.dirty];
    const sentAt = Date.now();
    const answers: Record<string, Answer> = {};
    for (const questionId of sending) {
      const answer = state.answers[questionId];
      if (answer !== undefined) answers[questionId] = answer;
    }

    set({ flushInFlight: true });
    try {
      const saved = await saveAnswers(attemptId, { sessionId, answers });
      set((current) => {
        const dirty = new Set(current.dirty);
        for (const questionId of sending) {
          if ((current.touchedAt[questionId] ?? 0) <= sentAt) dirty.delete(questionId);
        }
        return {
          dirty,
          flushInFlight: false,
          retryDelayMs: RETRY_BASE_MS,
          offsetMs: Date.parse(saved.serverTime) - Date.now(),
          lastSavedAt: saved.savedAt,
        };
      });
    } catch (error) {
      set((current) => ({
        flushInFlight: false,
        lock: lockForError(error) ?? current.lock,
        retryDelayMs: Math.min(current.retryDelayMs * 2, RETRY_CEILING_MS),
      }));
      // Retry on the backoff, not on the debounce. A student who has stopped
      // typing because the wifi dropped is exactly the student whose work is
      // sitting only in this tab, and nothing else will come along to send it.
      if (get().lock === null) scheduleFlush(get().retryDelayMs);
    }
  },

  /**
   * Idempotent while a submit is in flight, which is the whole point: the timer
   * firing auto-submit at the same moment the student taps must produce one
   * request, not two. The server refuses the second anyway -- this is so it is
   * never sent.
   */
  submit: async (reason = "manual") => {
    const state = get();
    const { attemptId } = state;
    if (attemptId === null || state.submitState !== "idle") return;

    set({ submitState: "inFlight" });
    try {
      // Everything typed goes with it. A submit that raced the debounce would
      // otherwise leave the last answer unsent.
      await get().flush();
      await submitAttempt(attemptId, { reason });
      set({ submitState: "done", lock: "closed" });
    } catch (error) {
      const lock = lockForError(error);
      if (lock === "closed") {
        // Already submitted -- by the other tab, or by an earlier request whose
        // reply was lost. The attempt is in, which is what was wanted.
        set({ submitState: "done", lock });
        return;
      }
      set({ submitState: "idle", lock: lock ?? state.lock });
    }
  },

  lockNow: (reason) => {
    cancelScheduledFlush();
    set({ lock: reason });
  },

  reset: () => {
    cancelScheduledFlush();
    set({ ...initial, dirty: new Set<string>() });
  },
}));

/**
 * One pending flush at a time, rescheduled by whichever came last.
 *
 * Module-level rather than in the store because it is a handle to a timer, not
 * state anything renders -- and because two components mounting the same
 * attempt must share it, or a second copy would double every request.
 */
let pendingFlush: ReturnType<typeof setTimeout> | undefined;

export function scheduleFlush(delayMs: number = FLUSH_DEBOUNCE_MS) {
  cancelScheduledFlush();
  pendingFlush = setTimeout(() => {
    pendingFlush = undefined;
    void useTakeTestStore.getState().flush();
  }, delayMs);
}

export function cancelScheduledFlush() {
  if (pendingFlush !== undefined) {
    clearTimeout(pendingFlush);
    pendingFlush = undefined;
  }
}

/** Remaining time, from the SERVER's clock. */
export function remainingMs(
  state: Pick<TakeTestState, "deadlineAt" | "offsetMs">,
): number {
  // Never Date.now() alone. A device five minutes fast would show a student
  // five minutes less than they have, and one five minutes slow would keep
  // typing into a paper the server has already closed. offsetMs is the
  // correction, re-measured on every save.
  return Math.max(0, state.deadlineAt - (Date.now() + state.offsetMs));
}

/** An attempt that arrives already finished is read-only from the first render. */
function lockFor(session: AttemptSession): LockReason | null {
  return session.attempt.status === "in_progress" ? null : "closed";
}

function lockForError(error: unknown): LockReason | null {
  if (!(error instanceof ApiError)) return null;
  switch (error.code) {
    case "SESSION_SUPERSEDED":
      return "superseded";
    case "DEADLINE_PASSED":
      return "deadline";
    case "ATTEMPT_CLOSED":
      return "closed";
    default:
      return null;
  }
}

/** Non-reactive reads, for the timer and the pagehide flush. */
export const takeTestStore = {
  getState: () => useTakeTestStore.getState(),
  flush: () => useTakeTestStore.getState().flush(),
};

export { getAttempt };
