import { describe, expect, it } from "vitest";
import { timelineRows } from "@/features/integrity/timeline";
import type { IntegrityEvent } from "@/features/attempts/api";

function event(id: number, over: Partial<IntegrityEvent> = {}): IntegrityEvent {
  return {
    id,
    kind: "audio_play",
    occurredAt: "2026-09-04T02:15:41Z",
    offsetMs: 1000,
    clientSeq: id,
    sessionId: "s1",
    ...over,
  };
}

describe("shared recording timeline rows", () => {
  it("numbers the plays of each shared recording by themselves", () => {
    const rows = timelineRows(
      [
        event(1, { meta: { recordingId: "A" } }),
        event(2, { meta: { recordingId: "B" } }),
        event(3, { meta: { recordingId: "A" } }),
      ],
      "audio",
    );
    expect(rows.map((row) => row.playNo)).toEqual([1, 1, 2]);
  });

  it("still numbers a question's plays by the question", () => {
    const rows = timelineRows(
      [
        event(1, { questionId: "q1", meta: { recordingId: "A" } }),
        event(2, { meta: { recordingId: "A" } }),
        event(3, { questionId: "q1", meta: { recordingId: "B" } }),
      ],
      "audio",
    );
    expect(rows.map((row) => row.playNo)).toEqual([1, 1, 2]);
  });

  it("keeps the fallback numbering when recording metadata is missing or the wrong type", () => {
    const rows = timelineRows(
      [
        event(1),
        event(2, { meta: { recordingId: 123 } }),
        event(3, { meta: { recordingId: null } }),
        event(4, { meta: { recordingId: "A" } }),
      ],
      "audio",
    );
    expect(rows.map((row) => row.playNo)).toEqual([1, 2, 3, 1]);
  });

  it("suppresses shared ends while keeping blocked rows and unpaired plays", () => {
    const rows = timelineRows(
      [
        event(1, { meta: { recordingId: "A" } }),
        event(2, {
          kind: "audio_ended",
          meta: { recordingId: "A", durationMs: 12000 },
        }),
        event(3, { kind: "audio_blocked", meta: { recordingId: "B" } }),
      ],
      "audio",
    );
    expect(rows.map((row) => [row.event.kind, row.playNo, row.ongoing])).toEqual([
      ["audio_play", 1, false],
      ["audio_blocked", null, false],
    ]);
  });
});
