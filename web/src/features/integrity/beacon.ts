import { BASE_URL } from "@/lib/api/client";
import type { IntegrityEventInput } from "@/features/take-test/api";

/**
 * The pagehide flush (D-03, §10.6).
 *
 * navigator.sendBeacon rather than fetch, because a fetch fired from `pagehide`
 * is routinely cancelled as the page goes away and this is the one moment the
 * timeline most wants recorded -- the tab closing is itself the event.
 *
 * text/plain rather than application/json, and the credential in the BODY
 * rather than a header, for the same underlying reason: sendBeacon cannot set
 * headers at all, and text/plain is CORS-safelisted so the request skips
 * preflight. A preflight fired on unload is not reliably delivered, which would
 * lose exactly the flush this exists for.
 */
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
