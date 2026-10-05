import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Buffered } from "@/features/integrity/buffer";

const ATTEMPT = "att-1";
const KEY = "quizzivy.integrity." + ATTEMPT;
const T0 = Date.parse("2026-09-01T08:00:00.000Z");
const COLUMN_LIMIT = 2_147_483_647;

const load = () => import("@/features/integrity/buffer");
let buffer: Awaited<ReturnType<typeof load>>;

const iso = (ms: number) => new Date(ms).toISOString();

function clockAt(elapsedMs: number) {
  vi.setSystemTime(T0 + elapsedMs);
}

function anchorAt(elapsedMs: number) {
  buffer.anchorSequence(ATTEMPT, iso(T0), iso(T0 + elapsedMs));
}

function readAt(elapsedMs: number) {
  clockAt(elapsedMs);
  anchorAt(elapsedMs);
}

function begin() {
  buffer.beginSession(ATTEMPT, "ses-1");
}

function recorded(): number | undefined {
  buffer.record(ATTEMPT, "paste");
  return buffer.pending().at(-1)?.clientSeq;
}

const numbers = () => buffer.pending().map((event) => event.clientSeq);

beforeEach(async () => {
  vi.resetModules();
  buffer = await load();
  vi.useFakeTimers({ now: T0 });
  sessionStorage.clear();
  buffer.clearSession(ATTEMPT);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("numbering an event", () => {
  it("counts from zero when nobody said when the attempt started", () => {
    clockAt(5000);
    begin();

    expect([recorded(), recorded(), recorded()]).toEqual([0, 1, 2]);
  });

  it("numbers an event by its offset from the attempt's start", () => {
    readAt(5000);
    begin();

    expect(recorded()).toBe(5000);
  });

  it("bumps by one inside the same millisecond", () => {
    readAt(5000);
    begin();

    expect([recorded(), recorded(), recorded()]).toEqual([5000, 5001, 5002]);
  });

  it("goes back to the offset once the clock has passed the bump", () => {
    readAt(5000);
    begin();
    recorded();
    recorded();
    recorded();

    clockAt(9000);

    expect(recorded()).toBe(9000);
  });

  it("stores the same shape as before", () => {
    readAt(5000);
    begin();
    recorded();

    expect(
      JSON.parse(sessionStorage.getItem(KEY) ?? "null") as Buffered | null,
    ).toEqual({
      sessionId: "ses-1",
      nextSeq: 5001,
      events: [{ kind: "paste", occurredAt: iso(T0 + 5000), clientSeq: 5000 }],
    });
  });
});

describe("two tabs under one session", () => {
  it("gives two tabs of one session different numbers a millisecond apart", () => {
    readAt(5000);
    begin();
    expect(recorded()).toBe(5000);

    buffer.suspendSession();
    sessionStorage.clear();
    begin();
    clockAt(5001);

    expect(recorded()).toBe(5001);
  });

  it("gives a duplicated tab different numbers too", () => {
    readAt(5000);
    begin();
    recorded();
    clockAt(6000);
    recorded();
    const copy = sessionStorage.getItem(KEY)!;
    clockAt(6500);
    expect(recorded()).toBe(6500);

    buffer.suspendSession();
    sessionStorage.setItem(KEY, copy);
    begin();
    clockAt(7000);

    expect(recorded()).toBe(7000);
    expect(numbers()).toEqual([5000, 6000, 7000]);
  });
});

describe("a device clock that is wrong", () => {
  it("never goes below the number after the last one, when the clock jumps back", () => {
    readAt(5000);
    begin();
    expect(recorded()).toBe(5000);

    clockAt(1000);

    expect(recorded()).toBe(5001);
  });

  it.each([
    ["an hour behind", -3_600_000],
    ["a day ahead", 86_400_000],
  ])(
    "numbers by the server's clock on a device whose clock is %s",
    (_name, wrongByMs) => {
      clockAt(5000 + wrongByMs);
      anchorAt(5000);
      begin();
      expect(recorded()).toBe(5000);

      vi.advanceTimersByTime(2000);

      expect(recorded()).toBe(7000);
    },
  );

  it("counts from the stored number when the device's clock falls back past the attempt's start after the read", () => {
    readAt(5000);
    begin();

    clockAt(-60_000);

    expect([recorded(), recorded()]).toEqual([0, 1]);
  });

  it("stops at the bound when the device's clock jumps far ahead, and counts on from it", () => {
    readAt(5000);
    begin();

    clockAt(3_000_000_000);

    expect([recorded(), recorded()]).toEqual([2_000_000_000, 2_000_000_001]);
    for (const issued of numbers()) expect(issued).toBeLessThan(COLUMN_LIMIT);
  });

  it("measures again at the next read", () => {
    readAt(5000);
    begin();

    clockAt(20_000);
    anchorAt(10_000);

    expect(recorded()).toBe(10_000);
  });
});

describe("an anchor that does not apply", () => {
  it("ignores an anchor for another attempt", () => {
    clockAt(5000);
    buffer.anchorSequence("att-2", iso(T0), iso(T0 + 5000));
    begin();

    expect(recorded()).toBe(0);
  });

  it.each([
    ["start", "not a time", iso(T0 + 5000)],
    ["server time", iso(T0), "not a time"],
  ])("ignores a %s it cannot read", (_name, startedAt, serverTime) => {
    clockAt(5000);
    buffer.anchorSequence(ATTEMPT, startedAt, serverTime);
    begin();

    expect(recorded()).toBe(0);
  });
});
