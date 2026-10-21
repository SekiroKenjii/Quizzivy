import { flushByBeacon } from "./beacon";
import { clearSession, suspendSession } from "./buffer";

/**
 * releaseSession is what the engine does with the integrity buffer when it
 * unmounts. While the attempt goes on, what is buffered and fits one beacon
 * is handed to it and the sequence stays stored, so a return in the same
 * session numbers on from it; events the browser does not take stay stored
 * for that session's next flush. Once the attempt has ended the buffer is forgotten.
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
  if (sessionId !== null && beaconToken !== "")
    flushByBeacon({ attemptId, sessionId, beaconToken });
  suspendSession();
}
