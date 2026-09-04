import { create } from "zustand";
import {
  drain as drainEvents,
  pending as pendingEvents,
  restore as restoreEvents,
} from "@/features/integrity/buffer";
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

/** Why the paper is no longer writable. Null while the student is working. */
export type LockReason =
  /** Opened elsewhere. This tab lost and the other one is now the attempt (§10.1, E2E 7). */
  | "superseded"
  /** Time is up. The client's job is to submit, not to keep typing. */
  | "deadline"
  /** Already submitted or voided; nothing further to do. */
  | "closed";

export type SubmitState = "idle" | "inFlight" | "done";
export type SubmitReason = "manual" | "timer_expired" | "auto_submit";

interface TakeTestState {
  attemptId: string | null;
  sessionId: string | null;
  // Append-only event access for the pagehide beacon (D-03).
  beaconToken: string;
  /** Already in presentation order; the server shuffled it and it must not move. */
  questions: StudentQuestion[];
  answers: Record<string, Answer>;
  /** Question ids edited since the server last confirmed them. */
  dirty: Set<string>;
  /** When each answer was last edited locally, in device time. */
  touchedAt: Record<string, number>;
  audioPlays: Record<string, number>;
  integrity: IntegrityPolicy | null;
  // Questions the student marked to come back to (S-06's "đánh dấu").
  flags: ReadonlySet<string>;
  // Counted away episodes before this sitting, per the server.
  focusLossCount: number;

  deadlineAt: number;
  /** serverTime minus device time at load. See remainingMs. */
  offsetMs: number;

  lock: LockReason | null;
  submitState: SubmitState;
  submitReason: SubmitReason | null;
  /** Per the server's clock, for the screen that follows the submission. */
  submittedAt: string | null;
  flushInFlight: boolean;
  retryDelayMs: number;
  /** When the server last confirmed a save, per its clock. Null until one lands. */
  lastSavedAt: string | null;

  hydrate: (session: AttemptSession) => void;
  setAnswer: (questionId: string, answer: Answer) => void;
  toggleFlag: (questionId: string) => void;
  notePlay: (questionId: string) => void;
  flush: () => Promise<void>;
  submit: (reason?: SubmitReason) => Promise<void>;
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
  beaconToken: "",
  questions: [] as StudentQuestion[],
  answers: {} as Record<string, Answer>,
  dirty: new Set<string>(),
  touchedAt: {} as Record<string, number>,
  audioPlays: {} as Record<string, number>,
  integrity: null,
  flags: new Set<string>() as ReadonlySet<string>,
  focusLossCount: 0,
  deadlineAt: 0,
  offsetMs: 0,
  lock: null,
  submitState: "idle" as SubmitState,
  submitReason: null as SubmitReason | null,
  submittedAt: null as string | null,
  flushInFlight: false,
  retryDelayMs: RETRY_BASE_MS,
  lastSavedAt: null,
};

export const useTakeTestStore = create<TakeTestState>((set, get) => ({
  ...initial,

  // Applies a payload from start, resume or refetch.
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
        beaconToken: session.beaconToken,
        questions: session.questions,
        answers,
        audioPlays: session.audioPlays,
        integrity: session.integrity,
        flags:
          state.attemptId === session.attempt.id
            ? state.flags
            : readFlags(session.attempt.id),
        focusLossCount:
          state.attemptId === session.attempt.id
            ? state.focusLossCount
            : (session.attempt.integrity?.focusLossCount ?? 0),
        deadlineAt: Date.parse(session.attempt.deadlineAt),
        offsetMs: Date.parse(session.serverTime) - Date.now(),
        lock: lockFor(session),
      };
    }),

  toggleFlag: (questionId) => {
    const { attemptId, flags } = get();
    if (attemptId === null) return;
    const next = new Set(flags);
    if (!next.delete(questionId)) next.add(questionId);
    writeFlags(attemptId, next);
    set({ flags: next });
  },

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

  // Counts a play the moment it starts, then lets the server correct it.
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
      });
  },

  // Sends everything dirty, and clears only what is still unchanged when the reply arrives.
  flush: async () => {
    const state = get();
    const { attemptId, sessionId } = state;
    if (attemptId === null || sessionId === null) return;
    if (state.lock !== null || state.flushInFlight) return;
    if (state.dirty.size === 0 && pendingEvents().length === 0) return;

    const events = drainEvents(attemptId);
    const sending = [...state.dirty];
    const sentAt = Date.now();
    const answers: Record<string, Answer> = {};
    for (const questionId of sending) {
      const answer = state.answers[questionId];
      if (answer !== undefined) answers[questionId] = answer;
    }

    set({ flushInFlight: true });
    try {
      const saved = await saveAnswers(attemptId, {
        sessionId,
        answers,
        ...(events.length === 0 ? {} : { events }),
      });
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
      // Back in the buffer for the next attempt.
      restoreEvents(attemptId, events);

      set((current) => ({
        flushInFlight: false,
        lock: lockForError(error) ?? current.lock,
        retryDelayMs: Math.min(current.retryDelayMs * 2, RETRY_CEILING_MS),
      }));
      // Retry on the backoff, not on the debounce.
      if (get().lock === null) scheduleFlush(get().retryDelayMs);
    }
  },

  submit: async (reason = "manual") => {
    const state = get();
    const { attemptId } = state;
    if (attemptId === null || state.submitState !== "idle") return;

    set({ submitState: "inFlight" });
    try {
      // Everything typed goes with it.
      await get().flush();
      const attempt = await submitAttempt(attemptId, { reason });
      set({
        submitState: "done",
        lock: "closed",
        submitReason: reason,
        submittedAt: attempt.submittedAt ?? serverNow(state),
      });
    } catch (error) {
      const lock = lockForError(error);
      if (lock === "closed") {
        set({
          submitState: "done",
          lock,
          submitReason: reason,
          submittedAt: serverNow(state),
        });
        return;
      }
      set({ submitState: "idle", lock: lock ?? state.lock });
    }
  },

  lockNow: (reason) => {
    cancelScheduledFlush();
    cancelDeadline();
    set({ lock: reason });
  },

  reset: () => {
    cancelScheduledFlush();
    cancelDeadline();
    set({ ...initial, dirty: new Set<string>(), flags: new Set<string>() });
  },
}));

function serverNow(state: Pick<TakeTestState, "offsetMs">): string {
  return new Date(Date.now() + state.offsetMs).toISOString();
}

/** The deadline, armed as a single timeout rather than polled. */
let deadlineTimer: ReturnType<typeof setTimeout> | undefined;

export function armDeadline() {
  cancelDeadline();
  const state = useTakeTestStore.getState();
  if (state.attemptId === null || state.lock !== null || state.submitState !== "idle")
    return;
  deadlineTimer = setTimeout(() => {
    deadlineTimer = undefined;
    void useTakeTestStore.getState().submit("timer_expired");
  }, remainingMs(state));
}

export function cancelDeadline() {
  if (deadlineTimer !== undefined) {
    clearTimeout(deadlineTimer);
    deadlineTimer = undefined;
  }
}

useTakeTestStore.subscribe((state, previous) => {
  if (
    state.deadlineAt !== previous.deadlineAt ||
    state.offsetMs !== previous.offsetMs
  ) {
    armDeadline();
  }
});

const FLAGS_PREFIX = "quizzivy.flags.";

function readFlags(attemptId: string): ReadonlySet<string> {
  try {
    const raw = sessionStorage.getItem(FLAGS_PREFIX + attemptId);
    const parsed: unknown = raw === null ? [] : JSON.parse(raw);
    return new Set(
      Array.isArray(parsed) ? parsed.filter((v) => typeof v === "string") : [],
    );
  } catch {
    return new Set();
  }
}

function writeFlags(attemptId: string, flags: ReadonlySet<string>): void {
  try {
    sessionStorage.setItem(FLAGS_PREFIX + attemptId, JSON.stringify([...flags]));
  } catch {
    // In memory only, like the event buffer: worse on a reload, no reason to stop.
  }
}

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
  // Never Date.now() alone.
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
