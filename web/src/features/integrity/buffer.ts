import type { IntegrityEventInput } from "@/features/take-test/api";

/** The event buffer, and the sequence number that makes a retry safe. */
export interface Buffered {
  sessionId: string;
  nextSeq: number;
  events: IntegrityEventInput[];
}

const KEY_PREFIX = "quizzivy.integrity.";
const SEQUENCE_BOUND = 2_000_000_000;

let state: Buffered | null = null;
let anchor: { attemptId: string; startedAt: number } | null = null;

function storageKey(attemptId: string): string {
  return KEY_PREFIX + attemptId;
}

/** anchorSequence calibrates the attempt's offset from server time without guaranteeing unique numbers. */
export function anchorSequence(
  attemptId: string,
  startedAt: string,
  serverTime: string,
): void {
  const elapsed = Date.parse(serverTime) - Date.parse(startedAt);
  anchor = Number.isNaN(elapsed)
    ? null
    : { attemptId, startedAt: Date.now() - elapsed };
}

function sequenceFor(attemptId: string, nextSeq: number): number {
  if (anchor === null || anchor.attemptId !== attemptId) return nextSeq;
  return Math.max(nextSeq, Math.min(SEQUENCE_BOUND, Date.now() - anchor.startedAt));
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

/** Starts or resumes a session's buffer. */
export function beginSession(attemptId: string, sessionId: string): void {
  const stored = read(attemptId);
  if (stored !== null && stored.sessionId === sessionId) {
    state = stored;
    return;
  }
  write(attemptId, { sessionId, nextSeq: 0, events: [] });
}

/**
 * record buffers one event of the attempt under the number anchorSequence
 * describes, and nothing before a session has begun.
 */
export function record(
  attemptId: string,
  kind: string,
  extra: { questionId?: string; meta?: Record<string, unknown> } = {},
): void {
  if (state === null) return;
  const clientSeq = sequenceFor(attemptId, state.nextSeq);
  const event: IntegrityEventInput = {
    kind,
    occurredAt: new Date().toISOString(),
    clientSeq,
    ...(extra.questionId === undefined ? {} : { questionId: extra.questionId }),
    ...(extra.meta === undefined ? {} : { meta: extra.meta }),
  };
  write(attemptId, {
    ...state,
    nextSeq: clientSeq + 1,
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

/**
 * suspendSession stops recording without forgetting what is stored, so the
 * same session numbers on from it when the engine opens again.
 */
export function suspendSession(): void {
  state = null;
}

/**
 * clearSession forgets this attempt entirely: the buffered events and the
 * sequence. Used when the attempt has ended.
 */
export function clearSession(attemptId: string): void {
  state = null;
  try {
    sessionStorage.removeItem(storageKey(attemptId));
  } catch {
    // Nothing to do, and nothing that depends on it.
  }
}
