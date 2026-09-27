import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGroupPlaybackStore } from "@/features/take-test/groupPlayback";
import { useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import { saveAnswers, submitAttempt } from "@/features/take-test/api";
import { session, text } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const submitted = vi.mocked(submitAttempt);
const saved = vi.mocked(saveAnswers);

const serverNow = new Date("2026-09-01T08:00:00.000Z");
const deadline = new Date("2026-09-01T08:30:00.000Z");
const store = () => useTakeTestStore.getState();

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(serverNow);
  submitted.mockReset().mockResolvedValue(
    session({
      serverTime: serverNow.toISOString(),
      deadlineAt: deadline.toISOString(),
      status: "submitted",
    }).attempt,
  );
  saved.mockReset();
  store().reset();
});
afterEach(() => {
  store().reset();
  vi.useRealTimers();
});

function start(serverTime = serverNow) {
  store().hydrate(
    session({
      serverTime: serverTime.toISOString(),
      deadlineAt: deadline.toISOString(),
    }),
  );
}

/**
 * E2E 5: time running out submits the paper. Armed from the server's clock,
 * not the device's, and told to the server as `timer_expired` so the attempt
 * ends at the deadline rather than at the request's arrival.
 */
describe("the deadline", () => {
  it("submits the paper when it arrives, as a timeout", async () => {
    start();
    await vi.advanceTimersByTimeAsync(29 * 60_000);
    expect(submitted).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(submitted).toHaveBeenCalledWith("att-1", { reason: "timer_expired" });
  });

  it("fires on the server's minute, not the device's", async () => {
    // The device is five minutes fast: it thinks it is 08:05 when the server says 08:00.
    vi.setSystemTime(new Date(serverNow.getTime() + 5 * 60_000));
    start();
    await vi.advanceTimersByTimeAsync(25 * 60_000 + 500);
    expect(submitted).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(5 * 60_000);
    expect(submitted).toHaveBeenCalledTimes(1);
  });

  it("is re-armed when a save re-measures the clock", async () => {
    start();
    // The server now says it is later than the device thought: 08:20, not 08:10.
    saved.mockResolvedValue({
      serverTime: "2026-09-01T08:20:00.000Z",
      savedAt: "2026-09-01T08:20:00.000Z",
      deadlineAt: deadline.toISOString(),
    });
    await vi.advanceTimersByTimeAsync(10 * 60_000);
    store().setAnswer("q1", text("late"));
    await vi.advanceTimersByTimeAsync(2_000); // the debounce, and the reply
    expect(saved).toHaveBeenCalled();
    // 08:20 on the server means ten minutes left, not twenty.
    await vi.advanceTimersByTimeAsync(10 * 60_000 + 100);
    expect(submitted).toHaveBeenCalledTimes(1);
  });

  it("is disarmed by leaving the paper", async () => {
    start();
    store().reset();
    await vi.advanceTimersByTimeAsync(31 * 60_000);
    expect(submitted).not.toHaveBeenCalled();
  });

  it("is disarmed by a lock -- the other tab's clock will do it", async () => {
    start();
    store().lockNow("superseded");
    await vi.advanceTimersByTimeAsync(31 * 60_000);
    expect(submitted).not.toHaveBeenCalled();
  });
});

describe("a deadline that moves while the paper is open", () => {
  const later = new Date(deadline.getTime() + 10 * 60_000);

  function saveReports(deadlineAt: string) {
    saved.mockResolvedValue({
      serverTime: new Date().toISOString(),
      savedAt: new Date().toISOString(),
      deadlineAt,
    });
  }

  async function answerAndSave() {
    store().setAnswer("q1", text("an answer"));
    await vi.advanceTimersByTimeAsync(2_000);
  }

  it("re-arms the timer at the later deadline an autosave reports", async () => {
    start();
    saveReports(later.toISOString());
    await answerAndSave();
    expect(store().deadlineAt).toBe(later.getTime());

    await vi.advanceTimersByTimeAsync(deadline.getTime() - Date.now() + 1_000);
    expect(submitted).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(10 * 60_000 + 2_000);
    expect(submitted).toHaveBeenCalledTimes(1);
    expect(submitted).toHaveBeenCalledWith("att-1", { reason: "timer_expired" });
  });

  it("carries the later deadline into the local draft and the shared-audio store", async () => {
    start();
    let reply: () => void = () => undefined;
    saved.mockImplementation(
      () =>
        new Promise((resolve) => {
          reply = () =>
            resolve({
              serverTime: new Date().toISOString(),
              savedAt: new Date().toISOString(),
              deadlineAt: later.toISOString(),
            });
        }),
    );
    store().setAnswer("q1", text("typed"));
    await vi.advanceTimersByTimeAsync(1_600);
    store().setAnswer("q2", text("typed while the save was out"));
    const draft = () =>
      JSON.parse(localStorage.getItem("quizzivy.answer-draft.att-1") ?? "{}") as {
        deadlineAt?: number;
      };
    expect(draft().deadlineAt).toBe(deadline.getTime());

    reply();
    await vi.advanceTimersByTimeAsync(0);

    expect(store().dirty.has("q2")).toBe(true);
    expect(draft().deadlineAt).toBe(later.getTime());
    expect(useGroupPlaybackStore.getState().deadlineAt).toBe(later.getTime());
  });

  it("ignores a deadline it cannot read, rather than firing at once", async () => {
    start();
    saveReports("not a time");
    await answerAndSave();
    expect(store().deadlineAt).toBe(deadline.getTime());
    await vi.advanceTimersByTimeAsync(60_000);
    expect(submitted).not.toHaveBeenCalled();
  });

  it("waits for the deadline a refused early timeout names, without retrying in between", async () => {
    start();
    submitted.mockRejectedValueOnce(
      new ApiError({
        status: 409,
        code: "DEADLINE_NOT_REACHED",
        message: "Chưa hết giờ: thời gian làm bài đã được gia hạn.",
        details: { deadlineAt: later.toISOString() },
      }),
    );

    await vi.advanceTimersByTimeAsync(deadline.getTime() - Date.now() + 100);
    expect(submitted).toHaveBeenCalledTimes(1);
    expect(store().submitState).toBe("idle");
    expect(store().deadlineAt).toBe(later.getTime());

    await vi.advanceTimersByTimeAsync(9 * 60_000);
    expect(submitted).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(submitted).toHaveBeenCalledTimes(2);
    expect(submitted).toHaveBeenLastCalledWith("att-1", { reason: "timer_expired" });
    expect(store().submitState).toBe("done");
  });
});
