import { create } from "zustand";
import { useGroupPlaybackStore } from "./groupPlayback";
import { clearDraft, readDraft, writeDraft } from "./draft";
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
  type StudentSection,
  type StudentGroup,
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
  studentId: string | null;
  sessionId: string | null;
  // Append-only event access for the pagehide beacon (D-03).
  beaconToken: string;
  /** Already in presentation order; the server shuffled it and it must not move. */
  questions: StudentQuestion[];
  /** In test order; questions never interleave two of them (S-08's rail). */
  sections: StudentSection[];
  groups: StudentGroup[];
  testTitle: string;
  remainingAttempts: number;
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
  flush: () => Promise<boolean>;
  submit: (reason?: SubmitReason) => Promise<void>;
  lockNow: (reason: LockReason) => void;
  reset: (options?: { keepDraft?: boolean }) => void;
}

/** The first retry delay, doubling to RETRY_CEILING_MS. */
const RETRY_BASE_MS = 1_000;
const RETRY_CEILING_MS = 30_000;
let activeFlush: Promise<boolean> | null = null;
let generation = 0;

/** §3's builder debounce, and the same reasoning: fast enough to feel saved. */
export const FLUSH_DEBOUNCE_MS = 1_500;

const initial = {
  attemptId: null,
  studentId: null,
  sessionId: null,
  beaconToken: "",
  questions: [] as StudentQuestion[],
  sections: [] as StudentSection[],
  groups: [] as StudentGroup[],
  testTitle: "",
  remainingAttempts: 0,
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
  hydrate: (session) => {
    const current = get();
    if (
      current.attemptId !== null &&
      (current.attemptId !== session.attempt.id ||
        current.sessionId !== session.sessionId)
    ) {
      current.reset({ keepDraft: true });
    }
    set((state) => {
      const sameAttempt = state.attemptId === session.attempt.id;
      const recovered =
        session.attempt.status === "in_progress"
          ? readDraft(
              session.attempt.id,
              session.attempt.studentId,
              session.sessionId,
              Date.parse(session.serverTime),
            )
          : {};
      const dirty = sameAttempt
        ? new Set(state.dirty)
        : new Set(Object.keys(recovered));
      const answers = { ...session.answers, ...recovered };
      for (const questionId of sameAttempt ? state.dirty : []) {
        const local = state.answers[questionId];
        if (local !== undefined) answers[questionId] = local;
      }
      return {
        attemptId: session.attempt.id,
        studentId: session.attempt.studentId,
        dirty,
        sessionId: session.sessionId,
        beaconToken: session.beaconToken,
        questions: session.questions,
        sections: session.sections,
        groups: session.groups ?? [],
        testTitle: session.testTitle,
        remainingAttempts: session.remainingAttempts ?? 0,
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
    });
    useGroupPlaybackStore
      .getState()
      .hydrate(session, (reason) => get().lockNow(reason));
  },

  toggleFlag: (questionId) => {
    const { attemptId, flags } = get();
    if (attemptId === null) return;
    const next = new Set(flags);
    if (!next.delete(questionId)) next.add(questionId);
    writeFlags(attemptId, next);
    set({ flags: next });
  },

  setAnswer: (questionId, answer) => {
    if (
      get().lock !== null ||
      get().submitState !== "idle" ||
      get().submitReason === "auto_submit"
    )
      return;
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
    const epoch = generation;
    recordAudioPlay(attemptId, questionId)
      .then((counted) => {
        if (generation !== epoch) return;
        set((state) => ({
          audioPlays: { ...state.audioPlays, [questionId]: counted.plays },
        }));
      })
      .catch(() => {
        // The count is the server's, and it will be right on the next fetch.
      });
  },

  flush: async () => {
    if (activeFlush !== null) return activeFlush;
    const state = get();
    const { attemptId, sessionId } = state;
    if (attemptId === null || sessionId === null || state.lock !== null) return false;
    if (state.dirty.size === 0 && pendingEvents().length === 0) return true;
    cancelScheduledFlush();
    const epoch = generation;
    const events = drainEvents(attemptId);
    const answers: Record<string, Answer> = {};
    for (const questionId of state.dirty) {
      const answer = state.answers[questionId];
      if (answer !== undefined) answers[questionId] = answer;
    }
    set({ flushInFlight: true });
    const request = (async () => {
      try {
        const saved = await saveAnswers(attemptId, {
          sessionId,
          answers,
          ...(events.length === 0 ? {} : { events }),
        });
        if (generation !== epoch) return false;
        set((current) => {
          const dirty = new Set(current.dirty);
          for (const [id, answer] of Object.entries(answers)) {
            if (current.answers[id] === answer) dirty.delete(id);
          }
          return {
            dirty,
            flushInFlight: false,
            retryDelayMs: RETRY_BASE_MS,
            offsetMs: Date.parse(saved.serverTime) - Date.now(),
            lastSavedAt: saved.savedAt,
          };
        });
        if (get().dirty.size > 0) scheduleFlush();
        return true;
      } catch (error) {
        if (generation !== epoch) return false;
        restoreEvents(attemptId, events);
        set((current) => ({
          flushInFlight: false,
          lock: lockForError(error) ?? current.lock,
          retryDelayMs: Math.min(current.retryDelayMs * 2, RETRY_CEILING_MS),
        }));
        if (get().lock === null) scheduleFlush(get().retryDelayMs);
        return false;
      }
    })();
    activeFlush = request;
    try {
      return await request;
    } finally {
      if (activeFlush === request) activeFlush = null;
    }
  },

  submit: async (reason = "manual") => {
    const state = get();
    const { attemptId } = state;
    if (
      attemptId === null ||
      state.submitState !== "idle" ||
      state.lock === "superseded"
    )
      return;
    const epoch = generation;

    set({ submitState: "inFlight", submitReason: reason });
    try {
      let saved = await get().flush();
      if (generation !== epoch) return;
      if (saved && get().dirty.size > 0) saved = await get().flush();
      if (generation !== epoch) return;
      if (
        get().lock === "superseded" ||
        (!saved &&
          (get().dirty.size > 0 ||
            (reason === "auto_submit" && pendingEvents().length > 0)) &&
          get().lock === null)
      ) {
        set({ submitState: "idle" });
        return;
      }
      if (!(await flushSharedBeforeSubmit(epoch))) return;
      const attempt = await submitAttempt(attemptId, { reason });
      if (generation !== epoch) return;
      set({
        submitState: "done",
        lock: "closed",
        submitReason: reason,
        submittedAt: attempt.submittedAt ?? serverNow(state),
      });
    } catch (error) {
      if (generation !== epoch) return;
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

  reset: (options) => {
    useGroupPlaybackStore.getState().reset(options?.keepDraft);
    if (options?.keepDraft !== true && get().attemptId !== null)
      clearDraft(get().attemptId!);
    generation += 1;
    activeFlush = null;
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
  if (
    state.attemptId === null ||
    (state.lock !== null && state.lock !== "deadline") ||
    state.submitState !== "idle"
  )
    return;
  deadlineTimer = setTimeout(
    () => {
      deadlineTimer = undefined;
      void useTakeTestStore
        .getState()
        .submit(state.submitReason === "auto_submit" ? "auto_submit" : "timer_expired");
    },
    Math.max(
      remainingMs(state),
      state.submitReason === "timer_expired" ? state.retryDelayMs : 0,
    ),
  );
}

export function cancelDeadline() {
  if (deadlineTimer !== undefined) {
    clearTimeout(deadlineTimer);
    deadlineTimer = undefined;
  }
}

useTakeTestStore.subscribe((state, previous) => {
  if (state.lock !== null && state.lock !== previous.lock)
    useGroupPlaybackStore.getState().lockNow(state.lock);
  if (
    state.attemptId !== null &&
    state.studentId !== null &&
    (state.answers !== previous.answers ||
      state.dirty !== previous.dirty ||
      state.lock !== previous.lock)
  ) {
    persistPending(state);
    if (state.attemptId !== previous.attemptId && state.dirty.size > 0) scheduleFlush();
  }
  if (
    state.deadlineAt !== previous.deadlineAt ||
    state.offsetMs !== previous.offsetMs ||
    (state.submitState === "idle" && previous.submitState === "inFlight")
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

function persistPending(state: TakeTestState) {
  if (state.attemptId === null || state.studentId === null || state.sessionId === null)
    return;
  const pending: Record<string, Answer> = {};
  if (state.lock !== "closed" && state.lock !== "superseded") {
    for (const id of state.dirty) {
      const answer = state.answers[id];
      if (answer !== undefined) pending[id] = answer;
    }
  }
  writeDraft(
    state.attemptId,
    state.studentId,
    state.sessionId,
    state.deadlineAt,
    pending,
  );
}

async function flushSharedBeforeSubmit(epoch: number): Promise<boolean> {
  await useGroupPlaybackStore.getState().flush();
  if (generation !== epoch) return false;
  if (useTakeTestStore.getState().lock === "superseded") {
    useTakeTestStore.setState({ submitState: "idle" });
    return false;
  }
  return true;
}
