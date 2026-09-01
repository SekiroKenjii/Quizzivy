import { beforeEach, describe, expect, it } from "vitest";
import {
  beginSession,
  clearSession,
  drain,
  pending,
  record,
  restore,
} from "@/features/integrity/buffer";

const ATTEMPT = "att-1";

beforeEach(() => {
  sessionStorage.clear();
  clearSession(ATTEMPT);
});

describe("buffering", () => {
  it("numbers events from zero and keeps the order they happened in", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "tab_hidden");
    record(ATTEMPT, "tab_visible");
    record(ATTEMPT, "paste");

    expect(pending().map((e) => [e.kind, e.clientSeq])).toEqual([
      ["tab_hidden", 0],
      ["tab_visible", 1],
      ["paste", 2],
    ]);
  });

  it("carries the question and the meta an event was given", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "audio_play", { questionId: "q1" });
    record(ATTEMPT, "tab_visible", { meta: { awayMs: 4200 } });

    expect(pending()[0]?.questionId).toBe("q1");
    expect(pending()[1]?.meta).toEqual({ awayMs: 4200 });
  });

  it("records nothing before a session has begun", () => {
    record(ATTEMPT, "paste");
    expect(pending()).toEqual([]);
  });
});

describe("draining for a flush", () => {
  it("hands over everything and leaves the buffer empty", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");
    record(ATTEMPT, "cut");

    expect(drain(ATTEMPT)).toHaveLength(2);
    expect(pending()).toEqual([]);
  });

  // The numbers are spent whether or not the request arrived. Reusing one would
  // make a genuinely new event look like a retry and be dropped by the server's
  // ON CONFLICT DO NOTHING.
  it("does not rewind the sequence", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");
    drain(ATTEMPT);
    record(ATTEMPT, "paste");

    expect(pending()[0]?.clientSeq).toBe(1);
  });

  it("puts a failed batch back in front of what arrived since", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");
    const failed = drain(ATTEMPT);
    record(ATTEMPT, "paste");

    restore(ATTEMPT, failed);
    expect(pending().map((e) => e.kind)).toEqual(["copy", "paste"]);
  });
});

/**
 * [D-01] The sequence lives beside the session id, so a reload continues it and
 * a NEW session starts fresh -- which is safe only because the server's
 * uniqueness key includes session_id.
 */
describe("across a reload", () => {
  it("continues the same session's numbering", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");
    record(ATTEMPT, "cut");

    // A reload: the module forgets, sessionStorage does not.
    clearSession2();
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "paste");

    expect(pending().map((e) => e.clientSeq)).toEqual([0, 1, 2]);
  });

  it("restarts numbering when the session changed", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");
    record(ATTEMPT, "cut");

    beginSession(ATTEMPT, "ses-2");
    record(ATTEMPT, "paste");

    // Zero again, and not a collision: two sessions may both hold clientSeq 0.
    expect(pending().map((e) => [e.kind, e.clientSeq])).toEqual([["paste", 0]]);
  });

  it("keeps events a reload interrupted", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");

    clearSession2();
    beginSession(ATTEMPT, "ses-1");

    expect(pending().map((e) => e.kind)).toEqual(["copy"]);
  });
});

/** Drops the in-memory copy without touching storage, as a reload would. */
function clearSession2() {
  // clearSession would also wipe sessionStorage, which is the thing under test.
  const saved = sessionStorage.getItem("quizzivy.integrity." + ATTEMPT);
  clearSession(ATTEMPT);
  if (saved !== null) sessionStorage.setItem("quizzivy.integrity." + ATTEMPT, saved);
}
