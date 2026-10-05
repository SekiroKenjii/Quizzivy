import { afterEach, beforeEach, expect, it } from "vitest";
import { beginSession, clearSession, pending } from "@/features/integrity/buffer";
import { recordSharedAudioEvent } from "@/features/integrity/useIntegrityMonitor";

beforeEach(() => {
  sessionStorage.clear();
  beginSession("att-1", "ses-1");
});
afterEach(() => clearSession("att-1"));

it.each([0, 12000, 86_400_000])(
  "records a known whole duration %i without a question",
  (durationMs) => {
    recordSharedAudioEvent("att-1", "audio_ended", "r1", durationMs);
    expect(pending()[0]?.meta).toEqual({
      scope: "group",
      recordingId: "r1",
      durationMs,
    });
    expect(pending()[0]).not.toHaveProperty("questionId");
  },
);

it.each([-1, 1.5, NaN, Infinity, -Infinity])(
  "omits an invalid device duration %s",
  (durationMs) => {
    recordSharedAudioEvent("att-1", "audio_ended", "r1", durationMs);
    expect(pending()[0]?.meta).toEqual({ scope: "group", recordingId: "r1" });
  },
);

it("records a block without duration even if one is passed", () => {
  recordSharedAudioEvent("att-1", "audio_blocked", "r1", 12000);
  expect(pending()[0]?.meta).toEqual({ scope: "group", recordingId: "r1" });
});
