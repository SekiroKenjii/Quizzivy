import { beforeEach, describe, expect, it } from "vitest";
import {
  beginSession,
  clearSession,
  MAX_EVENTS_PER_REQUEST,
  drain,
  inBatches,
  pending,
  record,
  restore,
  suspendSession,
  type Buffered,
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

  // The numbers are spent whether or not the request arrived.
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

describe("suspending", () => {
  it("stops recording and keeps what is stored", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");
    record(ATTEMPT, "cut");

    suspendSession();
    expect(pending()).toEqual([]);
    record(ATTEMPT, "paste");
    expect(pending()).toEqual([]);

    const stored = JSON.parse(
      sessionStorage.getItem("quizzivy.integrity." + ATTEMPT) ?? "null",
    ) as Buffered | null;
    expect(stored?.nextSeq).toBe(2);
    expect(stored?.events.map((e) => [e.kind, e.clientSeq])).toEqual([
      ["copy", 0],
      ["cut", 1],
    ]);
  });

  it("numbers on from the stored sequence when the same session begins again", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "copy");
    record(ATTEMPT, "cut");
    drain(ATTEMPT);

    suspendSession();
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "paste");

    expect(pending().map((e) => [e.kind, e.clientSeq])).toEqual([["paste", 2]]);
  });
});

/** Drops the in-memory copy without touching storage, as a reload would. */
function clearSession2() {
  // clearSession would also wipe sessionStorage, which is the thing under test.
  const saved = sessionStorage.getItem("quizzivy.integrity." + ATTEMPT);
  clearSession(ATTEMPT);
  if (saved !== null) sessionStorage.setItem("quizzivy.integrity." + ATTEMPT, saved);
}

describe("starting a session over another session's buffer", () => {
  it("hands back the events another session left, still under that session", () => {
    expect(beginSession(ATTEMPT, "ses-1")).toBeNull();
    record(ATTEMPT, "tab_hidden");
    record(ATTEMPT, "window_blur");

    const stranded = beginSession(ATTEMPT, "ses-2");

    expect(stranded?.sessionId).toBe("ses-1");
    expect(stranded?.events.map((e) => e.kind)).toEqual(["tab_hidden", "window_blur"]);
    expect(pending()).toEqual([]);
  });

  it("hands back nothing for the session it already holds, or for one that left no events", () => {
    beginSession(ATTEMPT, "ses-1");
    record(ATTEMPT, "paste");
    expect(beginSession(ATTEMPT, "ses-1")).toBeNull();
    expect(pending()).toHaveLength(1);

    drain(ATTEMPT);
    expect(beginSession(ATTEMPT, "ses-2")).toBeNull();
  });
});

describe("the contract's cap on the events of one request", () => {
  function recordMany(count: number) {
    beginSession(ATTEMPT, "ses-1");
    for (let at = 0; at < count; at += 1) record(ATTEMPT, "window_blur");
  }

  it("takes the oldest 500 and leaves the rest buffered, in order", () => {
    recordMany(600);

    const taken = drain(ATTEMPT);

    expect(MAX_EVENTS_PER_REQUEST).toBe(500);
    expect(taken).toHaveLength(500);
    expect(taken.map((event) => event.clientSeq)).toEqual(
      Array.from({ length: 500 }, (_, at) => at),
    );
    expect(pending().map((event) => event.clientSeq)).toEqual(
      Array.from({ length: 100 }, (_, at) => 500 + at),
    );
    expect(drain(ATTEMPT)).toHaveLength(100);
    expect(pending()).toEqual([]);
  });

  it("puts a failed slice back in front of the events that waited", () => {
    recordMany(600);

    restore(ATTEMPT, drain(ATTEMPT));

    expect(pending()).toHaveLength(600);
    expect(pending()[0]?.clientSeq).toBe(0);
  });

  it("splits events into slices of at most 500, keeping the order", () => {
    recordMany(1100);

    const slices = inBatches(pending());

    expect(slices.map((slice) => slice.length)).toEqual([500, 500, 100]);
    expect(slices.flat()).toEqual(pending());
  });
});
