import { BASE_URL } from "@/lib/api/client";
import type { IntegrityEventInput } from "@/features/take-test/api";
import { drain, MAX_EVENTS_PER_REQUEST, pending } from "./buffer";

/**
 * BEACON_BODY_LIMIT is the most bytes of one beacon body the client sends.
 * `sendBeacon` refuses a body above 64 KiB, and the browser applies that
 * quota to everything a page has queued, so a page sends one beacon.
 */
export const BEACON_BODY_LIMIT = 60_000;

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

/**
 * flushByBeacon hands the browser the oldest buffered events that fit one
 * beacon body: at most MAX_EVENTS_PER_REQUEST of them and BEACON_BODY_LIMIT
 * bytes, and always the first. They leave the buffer only when the browser
 * took them; the rest stays for the session's next request. It reports
 * whether nothing was left unsent by a refusal.
 */
export function flushByBeacon(input: {
  attemptId: string;
  sessionId: string;
  beaconToken: string;
}): boolean {
  const { attemptId, sessionId, beaconToken } = input;
  const events = fitting(pending().slice(0, MAX_EVENTS_PER_REQUEST), {
    beaconToken,
    sessionId,
  });
  if (!sendBeaconFlush({ attemptId, sessionId, beaconToken, events })) return false;
  drain(attemptId, events.length);
  return true;
}

function fitting(
  events: IntegrityEventInput[],
  envelope: { beaconToken: string; sessionId: string },
): IntegrityEventInput[] {
  const encoder = new TextEncoder();
  let size = encoder.encode(JSON.stringify({ ...envelope, events: [] })).length;
  const batch: IntegrityEventInput[] = [];
  for (const event of events) {
    size += encoder.encode(JSON.stringify(event)).length + 1;
    if (batch.length > 0 && size > BEACON_BODY_LIMIT) break;
    batch.push(event);
  }
  return batch;
}
