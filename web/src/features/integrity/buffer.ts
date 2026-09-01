import type { IntegrityEventInput } from "@/features/take-test/api";

/**
 * The event buffer, and the sequence number that makes a retry safe.
 *
 * Module-level rather than React state for two reasons. The listeners that fill
 * it fire outside React -- `pagehide` most of all, where there is no render
 * left to schedule -- and the autosave flush that drains it lives in the store,
 * which is not a component either.
 *
 * Backed by sessionStorage, so a same-tab reload continues the sequence rather
 * than restarting it and colliding with what was already sent. Not
 * localStorage: a buffer that outlives the tab would flush a dead session's
 * events into a live one.
 */
export interface Buffered {
  sessionId: string;
  nextSeq: number;
  events: IntegrityEventInput[];
}

const KEY_PREFIX = "quizzivy.integrity.";

let state: Buffered | null = null;

function storageKey(attemptId: string): string {
  return KEY_PREFIX + attemptId;
}

/**
 * Every read and write is guarded.
 *
 * Private mode and "block site data" make sessionStorage throw on ACCESS, not
 * merely return null, and an integrity monitor that can crash the page it is
 * watching has failed at the only thing that matters (§10.6).
 */
function read(attemptId: string): Buffered | null {
  try {
    const raw = sessionStorage.getItem(storageKey(attemptId));
    return raw === null ? null : (JSON.parse(raw) as Buffered);
  } catch {
    return null;
  }
}

function write(attemptId: string, next: Buffered): void {
  state = next;
  try {
    sessionStorage.setItem(storageKey(attemptId), JSON.stringify(next));
  } catch {
    // In memory only. Worse on a reload, and no reason to stop.
  }
}

/**
 * Starts or resumes a session's buffer.
 *
 * A NEW session id resets the sequence to zero, which is safe precisely because
 * the server's uniqueness key includes session_id (D-01): the same clientSeq
 * from two sessions is two rows, not a collision. The same session id keeps
 * counting, so a reload does not re-send seq 1 as something new.
 */
export function beginSession(attemptId: string, sessionId: string): void {
  const stored = read(attemptId);
  if (stored !== null && stored.sessionId === sessionId) {
    state = stored;
    return;
  }
  write(attemptId, { sessionId, nextSeq: 0, events: [] });
}

export function record(
  attemptId: string,
  kind: string,
  extra: { questionId?: string; meta?: Record<string, unknown> } = {},
): void {
  if (state === null) return;
  const event: IntegrityEventInput = {
    kind,
    occurredAt: new Date().toISOString(),
    clientSeq: state.nextSeq,
    ...(extra.questionId === undefined ? {} : { questionId: extra.questionId }),
    ...(extra.meta === undefined ? {} : { meta: extra.meta }),
  };
  write(attemptId, {
    ...state,
    nextSeq: state.nextSeq + 1,
    events: [...state.events, event],
  });
}

/** Everything waiting, without removing it. */
export function pending(): IntegrityEventInput[] {
  return state === null ? [] : state.events;
}

/**
 * Takes the buffer for a flush.
 *
 * The sequence is NOT rewound -- these numbers are spent whether or not the
 * request arrives, and reusing one would make a genuinely new event look like
 * a retry of an old one and be silently dropped by ON CONFLICT DO NOTHING.
 */
export function drain(attemptId: string): IntegrityEventInput[] {
  if (state === null) return [];
  const taken = state.events;
  write(attemptId, { ...state, events: [] });
  return taken;
}

/**
 * Puts a failed batch back, in front of anything recorded since.
 *
 * Fire-and-forget means the flush must not block anything, not that the events
 * are worth throwing away: the next flush carries them, and the server
 * deduplicates on (attempt, session, clientSeq) if some of them did land.
 */
export function restore(attemptId: string, events: IntegrityEventInput[]): void {
  if (state === null || events.length === 0) return;
  write(attemptId, { ...state, events: [...events, ...state.events] });
}

/** Forgets this attempt entirely. Used when the take-test screen unmounts. */
export function clearSession(attemptId: string): void {
  state = null;
  try {
    sessionStorage.removeItem(storageKey(attemptId));
  } catch {
    // Nothing to do, and nothing that depends on it.
  }
}
