import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FLUSH_DEBOUNCE_MS, useTakeTestStore } from "@/features/take-test/store";
import { saveAnswers } from "@/features/take-test/api";
import { ApiError } from "@/lib/api/errors";
import { session, text } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  getAttempt: vi.fn(),
}));

const saved = vi.mocked(saveAnswers);

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";

beforeEach(() => {
  saved.mockReset();
  saved.mockResolvedValue({ serverTime: now, savedAt: now });
  useTakeTestStore.getState().reset();
});

afterEach(() => useTakeTestStore.getState().reset());

/**
 * §1.2's promise, in one direction only: a refresh does not lose work. The
 * merge runs one way because losing the student's typing is the failure this
 * whole feature exists to prevent, and showing them a stale server copy is
 * exactly that failure wearing a different hat.
 */
describe("the resume merge", () => {
  it("keeps a local answer the server has not seen", async () => {
    const store = useTakeTestStore.getState();
    store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
    store.setAnswer("q1", text("what I just typed"));

    // A reload: the same attempt comes back, and the server has nothing for q1.
    useTakeTestStore
      .getState()
      .hydrate(session({ serverTime: now, deadlineAt: deadline }));

    expect(useTakeTestStore.getState().answers["q1"]).toEqual(
      text("what I just typed"),
    );
    expect(useTakeTestStore.getState().dirty.has("q1")).toBe(true);
  });

  it("prefers the local answer over an older one the server holds", () => {
    const store = useTakeTestStore.getState();
    store.hydrate(
      session({
        serverTime: now,
        deadlineAt: deadline,
        answers: { q1: text("first") },
      }),
    );
    store.setAnswer("q1", text("second"));

    useTakeTestStore.getState().hydrate(
      session({
        serverTime: now,
        deadlineAt: deadline,
        answers: { q1: text("first") },
      }),
    );

    expect(useTakeTestStore.getState().answers["q1"]).toEqual(text("second"));
  });

  it("takes the server's answer once the flush has confirmed it", async () => {
    const store = useTakeTestStore.getState();
    store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
    store.setAnswer("q1", text("typed"));
    await useTakeTestStore.getState().flush();
    expect(useTakeTestStore.getState().dirty.size).toBe(0);

    // The server is now the authority for q1, and a later reload takes its word
    // -- including a value the teacher's reset put there.
    useTakeTestStore.getState().hydrate(
      session({
        serverTime: now,
        deadlineAt: deadline,
        answers: { q1: text("from the server") },
      }),
    );

    expect(useTakeTestStore.getState().answers["q1"]).toEqual(text("from the server"));
  });

  it("drops nothing in either direction", () => {
    const store = useTakeTestStore.getState();
    store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
    store.setAnswer("q2", text("local only"));

    useTakeTestStore.getState().hydrate(
      session({
        serverTime: now,
        deadlineAt: deadline,
        answers: { q1: text("server only"), q3: text("server only") },
      }),
    );

    const { answers } = useTakeTestStore.getState();
    expect(Object.keys(answers).sort()).toEqual(["q1", "q2", "q3"]);
    expect(answers["q2"]).toEqual(text("local only"));
  });
});

/**
 * The case that makes the dirty set worth tracking per question rather than as
 * a single flag.
 */
describe("an answer edited while its flush is in the air", () => {
  it("stays dirty rather than being marked saved", async () => {
    let release: (() => void) | undefined;
    saved.mockImplementation(
      () =>
        new Promise((resolve) => {
          release = () => resolve({ serverTime: now, savedAt: now });
        }),
    );

    const store = useTakeTestStore.getState();
    store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
    store.setAnswer("q1", text("first"));

    const inFlight = useTakeTestStore.getState().flush();
    // The student keeps typing while the request is out.
    await new Promise((r) => setTimeout(r, 2));
    useTakeTestStore.getState().setAnswer("q1", text("second"));
    release?.();
    await inFlight;

    const state = useTakeTestStore.getState();
    expect(state.answers["q1"]).toEqual(text("second"));
    expect(state.dirty.has("q1")).toBe(true);
  });

  it("does not hold back the answers that were genuinely saved", async () => {
    let release: (() => void) | undefined;
    saved.mockImplementation(
      () =>
        new Promise((resolve) => {
          release = () => resolve({ serverTime: now, savedAt: now });
        }),
    );

    const store = useTakeTestStore.getState();
    store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
    store.setAnswer("q1", text("one"));
    store.setAnswer("q2", text("two"));

    const inFlight = useTakeTestStore.getState().flush();
    await new Promise((r) => setTimeout(r, 2));
    useTakeTestStore.getState().setAnswer("q1", text("one, edited"));
    release?.();
    await inFlight;

    const state = useTakeTestStore.getState();
    expect(state.dirty.has("q1")).toBe(true);
    expect(state.dirty.has("q2")).toBe(false);
  });
});

describe("a failed flush", () => {
  it("keeps every local answer and backs off", async () => {
    saved.mockRejectedValue(new Error("offline"));

    const store = useTakeTestStore.getState();
    store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
    store.setAnswer("q1", text("typed"));

    const before = useTakeTestStore.getState().retryDelayMs;
    await useTakeTestStore.getState().flush();

    const state = useTakeTestStore.getState();
    expect(state.answers["q1"]).toEqual(text("typed"));
    expect(state.dirty.has("q1")).toBe(true);
    expect(state.retryDelayMs).toBeGreaterThan(before);
    expect(state.lock).toBeNull();
  });
});

/**
 * The debounce and the backoff, which exist so nothing outside the store has to
 * remember to send anything.
 */
describe("scheduling", () => {
  it("flushes on the debounce after the student stops typing", async () => {
    vi.useFakeTimers();
    try {
      const store = useTakeTestStore.getState();
      store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
      store.setAnswer("q1", text("typed"));

      expect(saved).not.toHaveBeenCalled();
      await vi.advanceTimersByTimeAsync(FLUSH_DEBOUNCE_MS);
      expect(saved).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("collapses a burst of typing into one request", async () => {
    vi.useFakeTimers();
    try {
      const store = useTakeTestStore.getState();
      store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
      for (const value of ["a", "ab", "abc"]) {
        useTakeTestStore.getState().setAnswer("q1", text(value));
        await vi.advanceTimersByTimeAsync(FLUSH_DEBOUNCE_MS / 3);
      }
      expect(saved).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(FLUSH_DEBOUNCE_MS);
      expect(saved).toHaveBeenCalledTimes(1);
      expect(saved.mock.calls[0]?.[1].answers).toEqual({ q1: text("abc") });
    } finally {
      vi.useRealTimers();
    }
  });

  // The student who stopped typing because the wifi dropped is exactly the one
  // whose work exists only in this tab; nothing else will come along to send it.
  it("retries a failed flush on its own, without further typing", async () => {
    vi.useFakeTimers();
    try {
      saved.mockRejectedValueOnce(new Error("offline"));
      const store = useTakeTestStore.getState();
      store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
      store.setAnswer("q1", text("typed"));

      await vi.advanceTimersByTimeAsync(FLUSH_DEBOUNCE_MS);
      expect(saved).toHaveBeenCalledTimes(1);
      expect(useTakeTestStore.getState().dirty.has("q1")).toBe(true);

      await vi.advanceTimersByTimeAsync(useTakeTestStore.getState().retryDelayMs);
      expect(saved).toHaveBeenCalledTimes(2);
      expect(useTakeTestStore.getState().dirty.size).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it("stops retrying once the session is lost, rather than asking forever", async () => {
    vi.useFakeTimers();
    try {
      saved.mockRejectedValue(
        new ApiError({
          status: 409,
          code: "SESSION_SUPERSEDED",
          message: "Bài làm này đã được mở ở nơi khác.",
        }),
      );
      const store = useTakeTestStore.getState();
      store.hydrate(session({ serverTime: now, deadlineAt: deadline }));
      store.setAnswer("q1", text("typed"));

      await vi.advanceTimersByTimeAsync(FLUSH_DEBOUNCE_MS);
      expect(saved).toHaveBeenCalledTimes(1);

      await vi.advanceTimersByTimeAsync(5 * 60_000);
      expect(saved).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });
});
