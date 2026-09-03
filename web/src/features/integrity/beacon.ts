import { BASE_URL } from "@/lib/api/client";
import type { IntegrityEventInput } from "@/features/take-test/api";

/** The pagehide flush (D-03, §10.6). */
export function sendBeaconFlush(input: {
  attemptId: string;
  sessionId: string;
  beaconToken: string;
  events: IntegrityEventInput[];
}): boolean {
  if (input.events.length === 0) return true;
  if (typeof navigator === "undefined" || typeof navigator.sendBeacon !== "function") {
    return false;
  }

  const body = new Blob(
    [
      JSON.stringify({
        beaconToken: input.beaconToken,
        sessionId: input.sessionId,
        events: input.events,
      }),
    ],
    { type: "text/plain" },
  );
  return navigator.sendBeacon(
    `${BASE_URL}/app/attempts/${input.attemptId}/events`,
    body,
  );
}
