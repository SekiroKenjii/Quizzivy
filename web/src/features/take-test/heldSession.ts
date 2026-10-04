const PREFIX = "quizzivy.session.";
const SESSION_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const held = new Map<string, string>();

/**
 * heldSession returns the session this tab was given for the attempt, or null
 * when it holds none: a tab holds a session from a start or a resume it asked
 * for, or from a read that did not say it was superseded.
 */
export function heldSession(attemptId: string): string | null {
  const sessionId = held.get(attemptId) ?? stored(attemptId);
  return sessionId !== null && SESSION_ID.test(sessionId) ? sessionId : null;
}

/**
 * holdSession remembers the session for this tab's lifetime, so a reload names
 * it. When the storage refuses the write the stored value is removed: the next
 * read then names no session and is answered as a new tab.
 */
export function holdSession(attemptId: string, sessionId: string): void {
  held.set(attemptId, sessionId);
  try {
    sessionStorage.setItem(PREFIX + attemptId, sessionId);
  } catch {
    forget(attemptId);
  }
}

function stored(attemptId: string): string | null {
  try {
    return sessionStorage.getItem(PREFIX + attemptId);
  } catch {
    return null;
  }
}

function forget(attemptId: string): void {
  try {
    sessionStorage.removeItem(PREFIX + attemptId);
  } catch {
    return;
  }
}
