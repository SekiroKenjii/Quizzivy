import { sendBeaconFlush } from "./beacon";
import { clearSession, drain, restore, suspendSession } from "./buffer";

/**
 * releaseSession is what the engine does with the integrity buffer when it
 * unmounts. While the attempt goes on, what is buffered is handed to the
 * beacon and the sequence stays stored, so a return in the same session
 * numbers on from it; events the browser does not take stay stored for that
 * session's next flush. Once the attempt has ended the buffer is forgotten.
 */
export function releaseSession(input: {
  attemptId: string;
  sessionId: string | null;
  beaconToken: string;
  ended: boolean;
}): void {
  const { attemptId, sessionId, beaconToken, ended } = input;
  if (ended) {
    clearSession(attemptId);
    return;
  }
  const events = drain(attemptId);
  const sent =
    sessionId !== null &&
    beaconToken !== "" &&
    sendBeaconFlush({ attemptId, sessionId, beaconToken, events });
  if (!sent) restore(attemptId, events);
  suspendSession();
}
