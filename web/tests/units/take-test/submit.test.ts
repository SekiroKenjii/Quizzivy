import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTakeTestStore } from "@/features/take-test/store";
import { saveAnswers, submitAttempt } from "@/features/take-test/api";
import { ApiError } from "@/lib/api/errors";
import { session, text } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  getAttempt: vi.fn(),
}));

const saved = vi.mocked(saveAnswers);
const submitted = vi.mocked(submitAttempt);

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";

function attemptRow(status: "submitted" | "in_progress" = "submitted") {
  return session({ serverTime: now, deadlineAt: deadline, status }).attempt;
}

beforeEach(() => {
  saved.mockReset().mockResolvedValue({ serverTime: now, savedAt: now });
  submitted.mockReset().mockResolvedValue(attemptRow());
  useTakeTestStore.getState().reset();
});

afterEach(() => useTakeTestStore.getState().reset());

function start() {
  useTakeTestStore
    .getState()
    .hydrate(session({ serverTime: now, deadlineAt: deadline }));
}

describe("submitting", () => {
  /**
   * The timer firing auto-submit at the same moment the student taps must
   * produce ONE request. The server refuses the second anyway -- this is so it
   * is never sent, because the student would see the refusal.
   */
  it("issues one request when tapped twice", async () => {
    start();
    const store = useTakeTestStore.getState();
    await Promise.all([store.submit(), store.submit()]);

    expect(submitted).toHaveBeenCalledTimes(1);
    expect(useTakeTestStore.getState().submitState).toBe("done");
  });

  it("ignores a later tap once the attempt is in", async () => {
    start();
    await useTakeTestStore.getState().submit();
    await useTakeTestStore.getState().submit();

    expect(submitted).toHaveBeenCalledTimes(1);
  });

  // A submit racing the 1.5s debounce would otherwise leave the last thing
  // typed unsent, which is the one answer the student most remembers writing.
  it("sends what is still unflushed before it submits", async () => {
    start();
    useTakeTestStore.getState().setAnswer("q1", text("the last thing I typed"));
    await useTakeTestStore.getState().submit();

    expect(saved).toHaveBeenCalledTimes(1);
    expect(saved.mock.calls[0]?.[1].answers).toEqual({
      q1: text("the last thing I typed"),
    });
    expect(submitted).toHaveBeenCalledTimes(1);
  });

  it("carries the reason, so a timeout is not recorded as a student's choice", async () => {
    start();
    await useTakeTestStore.getState().submit("timer_expired");
    expect(submitted.mock.calls[0]?.[1]).toEqual({ reason: "timer_expired" });
  });

  /**
   * Already submitted -- by the other tab, or by a request whose reply was lost
   * on a flaky connection. The attempt is in, which is what was wanted, so this
   * is a success the student should not be shown an error for.
   */
  it("treats ATTEMPT_CLOSED as done rather than as a failure", async () => {
    start();
    submitted.mockRejectedValue(
      new ApiError({
        status: 409,
        code: "ATTEMPT_CLOSED",
        message: "Bài làm này đã được nộp.",
      }),
    );

    await useTakeTestStore.getState().submit();

    const state = useTakeTestStore.getState();
    expect(state.submitState).toBe("done");
    expect(state.lock).toBe("closed");
  });

  it("lets the student try again after a network failure", async () => {
    start();
    submitted.mockRejectedValueOnce(new Error("offline"));

    await useTakeTestStore.getState().submit();
    expect(useTakeTestStore.getState().submitState).toBe("idle");

    submitted.mockResolvedValue(attemptRow());
    await useTakeTestStore.getState().submit();
    expect(useTakeTestStore.getState().submitState).toBe("done");
  });
});

/**
 * E2E 7. The tab that lost goes read-only with something to read, rather than
 * throwing a stack trace at a fifteen-year-old halfway through a test.
 */
describe("losing the session to another device", () => {
  it("locks the paper instead of failing", async () => {
    start();
    saved.mockRejectedValue(
      new ApiError({
        status: 409,
        code: "SESSION_SUPERSEDED",
        message: "Bài làm này đã được mở ở nơi khác.",
      }),
    );

    useTakeTestStore.getState().setAnswer("q1", text("typed here"));
    await useTakeTestStore.getState().flush();

    expect(useTakeTestStore.getState().lock).toBe("superseded");
  });

  it("stops accepting edits once locked, and keeps what was already typed", async () => {
    start();
    useTakeTestStore.getState().setAnswer("q1", text("before"));
    useTakeTestStore.getState().lockNow("superseded");
    useTakeTestStore.getState().setAnswer("q1", text("after"));

    expect(useTakeTestStore.getState().answers["q1"]).toEqual(text("before"));
  });

  it("stops flushing, so the losing tab does not keep asking", async () => {
    start();
    useTakeTestStore.getState().setAnswer("q1", text("typed"));
    useTakeTestStore.getState().lockNow("superseded");
    await useTakeTestStore.getState().flush();

    expect(saved).not.toHaveBeenCalled();
  });

  it("distinguishes the deadline from a takeover, because the answers differ", async () => {
    start();
    saved.mockRejectedValue(
      new ApiError({
        status: 409,
        code: "DEADLINE_PASSED",
        message: "Đã hết giờ làm bài.",
      }),
    );
    useTakeTestStore.getState().setAnswer("q1", text("typed"));
    await useTakeTestStore.getState().flush();

    // "Submit" is advice this tab can act on; "you lost the session" is not.
    expect(useTakeTestStore.getState().lock).toBe("deadline");
  });
});
