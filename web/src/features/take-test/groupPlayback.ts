import { create } from "zustand";
import { ApiError } from "@/lib/api/errors";
import { recordGroupAudioPlay, type AttemptSession } from "./api";
import {
  forgetGroupPlay,
  persistGroupPlay,
  readGroupPlays,
  type PendingGroupPlay,
} from "./groupPlaybackDraft";

type PlaybackLock = "closed" | "superseded" | "deadline";
type PlaybackState = {
  attemptId: string | null;
  studentId: string | null;
  sessionId: string | null;
  deadlineAt: number;
  offsetMs: number;
  recordings: ReadonlySet<string>;
  confirmed: Record<string, number>;
  pending: PendingGroupPlay[];
  lock: PlaybackLock | null;
  syncing: boolean;
  retryDelay: number;
  onLock: ((reason: PlaybackLock) => void) | null;
  hydrate: (session: AttemptSession, onLock: (reason: PlaybackLock) => void) => void;
  notePlay: (recordingId: string) => void;
  flush: () => Promise<boolean>;
  lockNow: (reason: PlaybackLock) => void;
  reset: (keepDraft?: boolean) => void;
};

const initial = {
  attemptId: null,
  studentId: null,
  sessionId: null,
  deadlineAt: 0,
  offsetMs: 0,
  recordings: new Set<string>(),
  confirmed: {} as Record<string, number>,
  pending: [] as PendingGroupPlay[],
  lock: null,
  syncing: false,
  retryDelay: 1000,
  onLock: null,
};
let generation = 0;
let active: Promise<boolean> | null = null;
let retryTimer: ReturnType<typeof setTimeout> | undefined;

/** groupPlayCount never adds pending gestures to a server total that may already include them. */
export function groupPlayCount(
  state: Pick<PlaybackState, "confirmed" | "pending">,
  recordingId: string,
): number {
  return state.pending.reduce(
    (count, play) =>
      play.recordingId === recordingId ? Math.max(count, play.minimumPlays) : count,
    state.confirmed[recordingId] ?? 0,
  );
}

/** useGroupPlaybackStore shares one retry queue across all children and players of the current attempt. */
export const useGroupPlaybackStore = create<PlaybackState>((set, get) => ({
  ...initial,
  hydrate: (session, onLock) => {
    const current = get();
    if (
      current.attemptId !== session.attempt.id ||
      current.sessionId !== session.sessionId ||
      current.studentId !== session.attempt.studentId
    )
      current.reset(true);
    const recordings = new Set(
      session.groups?.flatMap((group) =>
        group.recordings.map((recording) => recording.id),
      ),
    );
    const recovered = readGroupPlays(
      session.attempt.studentId,
      session.attempt.id,
      Date.parse(session.serverTime),
      Date.parse(session.attempt.deadlineAt),
    );
    const pending = [
      ...new Map(
        [...recovered, ...get().pending].map((play) => [play.playId, play]),
      ).values(),
    ].filter((play) => recordings.has(play.recordingId));
    const confirmed = { ...session.groupAudioPlays };
    for (const [id, count] of Object.entries(get().confirmed))
      confirmed[id] = Math.max(confirmed[id] ?? 0, count);
    set({
      attemptId: session.attempt.id,
      studentId: session.attempt.studentId,
      sessionId: session.sessionId,
      deadlineAt: Date.parse(session.attempt.deadlineAt),
      offsetMs: Date.parse(session.serverTime) - Date.now(),
      recordings,
      confirmed,
      pending,
      onLock,
      lock: session.attempt.status === "in_progress" ? null : "closed",
    });
    if (get().lock === "closed") get().lockNow("closed");
    else if (pending.length > 0) schedule(0);
  },
  notePlay: (recordingId) => {
    const state = get();
    if (
      !state.attemptId ||
      !state.studentId ||
      state.lock ||
      !state.recordings.has(recordingId)
    )
      return;
    const play = {
      studentId: state.studentId,
      attemptId: state.attemptId,
      deadlineAt: state.deadlineAt,
      recordingId,
      playId: crypto.randomUUID(),
      minimumPlays: groupPlayCount(state, recordingId) + 1,
    };
    persistGroupPlay(play);
    set({ pending: [...state.pending, play] });
    void get().flush();
  },
  flush: async () => {
    if (active) return active;
    const { attemptId, sessionId, lock } = get();
    if (!attemptId || !sessionId || lock !== null) return false;
    if (get().pending.length === 0) return true;
    cancelRetry();
    const epoch = generation;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 3000);
    const signal = controller.signal;
    set({ syncing: true });
    const request = (async () => {
      try {
        while (get().pending.length > 0) {
          if (generation !== epoch || signal.aborted || get().lock !== null)
            return false;
          const play = get().pending[0]!;
          const counted = await recordGroupAudioPlay(
            attemptId,
            { recordingId: play.recordingId, playId: play.playId, sessionId },
            signal,
          );
          if (generation !== epoch) return false;
          forgetGroupPlay(play.playId);
          set((state) => ({
            confirmed: {
              ...state.confirmed,
              [play.recordingId]: Math.max(
                state.confirmed[play.recordingId] ?? 0,
                counted.plays,
              ),
            },
            pending: withoutPlay(state.pending, play.playId),
            retryDelay: 1000,
          }));
        }
        return true;
      } catch (error) {
        if (generation !== epoch) return false;
        const reason = playbackLock(error);
        if (reason) {
          get().lockNow(reason);
          get().onLock?.(reason);
        } else set((state) => ({ retryDelay: Math.min(30000, state.retryDelay * 2) }));
        return false;
      } finally {
        clearTimeout(timeout);
        if (generation === epoch) {
          set({ syncing: false });
          if (get().pending.length > 0 && get().lock === null)
            schedule(get().retryDelay);
        }
      }
    })();
    active = request;
    try {
      return await request;
    } finally {
      if (active === request) active = null;
    }
  },
  lockNow: (reason) => {
    cancelRetry();
    if (reason === "closed" || reason === "deadline") {
      for (const play of get().pending) forgetGroupPlay(play.playId);
      set({ pending: [] });
    }
    set({ lock: reason });
  },
  reset: (keepDraft = false) => {
    if (!keepDraft) for (const play of get().pending) forgetGroupPlay(play.playId);
    generation += 1;
    active = null;
    cancelRetry();
    set({ ...initial });
  },
}));

function schedule(delay: number) {
  cancelRetry();
  retryTimer = setTimeout(() => {
    retryTimer = undefined;
    void useGroupPlaybackStore.getState().flush();
  }, delay);
}

function cancelRetry() {
  if (retryTimer !== undefined) clearTimeout(retryTimer);
  retryTimer = undefined;
}

function playbackLock(error: unknown): PlaybackLock | null {
  if (!(error instanceof ApiError)) return null;
  if (error.code === "SESSION_SUPERSEDED") return "superseded";
  if (error.code === "ATTEMPT_CLOSED") return "closed";
  if (error.code === "DEADLINE_PASSED") return "deadline";
  return null;
}

function withoutPlay(pending: PendingGroupPlay[], playId: string) {
  return pending.filter((play) => play.playId !== playId);
}
