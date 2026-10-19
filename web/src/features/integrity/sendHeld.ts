import { flushEvents } from "@/features/take-test/api";
import { inBatches, surrender } from "./buffer";
import type { IntegrityEventInput } from "@/features/take-test/api";

/**
 * sendHeld sends events under the session they were recorded in, in requests
 * of at most MAX_EVENTS_PER_REQUEST, one after another. The caller has
 * already dropped them from the buffer: a refusal or a failure ends the send
 * and the rest is lost, as the spec says.
 */
export async function sendHeld(
  attemptId: string,
  sessionId: string,
  events: IntegrityEventInput[],
): Promise<void> {
  try {
    for (const batch of inBatches(events))
      await flushEvents(attemptId, { sessionId, events: batch });
  } catch {
    return;
  }
}

/**
 * abandonSession is what a tab does when it learns its session was taken over:
 * the events it holds under that session go out once, whatever the answer, and
 * nothing is recorded until the tab holds a session again.
 */
export function abandonSession(attemptId: string, sessionId: string): Promise<void> {
  return sendHeld(attemptId, sessionId, surrender(attemptId, sessionId));
}
