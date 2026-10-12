import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useGroupPlaybackStore } from "@/features/take-test/groupPlayback";
import { useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import {
  recordGroupAudioPlay,
  saveAnswers,
  submitAttempt,
} from "@/features/take-test/api";
import { previewGroup } from "@tests/support/groupPreview";
import { session, text } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const submitted = vi.mocked(submitAttempt);
const saved = vi.mocked(saveAnswers);
const counted = vi.mocked(recordGroupAudioPlay);

const serverNow = new Date("2026-09-01T08:00:00.000Z");
const deadline = new Date("2026-09-01T08:30:00.000Z");
const untilDeadline = deadline.getTime() - serverNow.getTime();
const recordingId = previewGroup.recordings[0]!.id;
const handedIn = session({
  serverTime: serverNow.toISOString(),
  deadlineAt: deadline.toISOString(),
  status: "submitted",
}).attempt;
const store = () => useTakeTestStore.getState();
const refusal = () =>
  new ApiError({ status: 409, code: "DEADLINE_PASSED", message: "late" });

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(serverNow);
  submitted.mockReset().mockResolvedValue(handedIn);
  saved.mockReset();
  counted.mockReset();
  store().reset();
  localStorage.clear();
});
afterEach(() => {
  store().reset();
  vi.useRealTimers();
});

function start() {
  store().hydrate(
    session({
      serverTime: serverNow.toISOString(),
      deadlineAt: deadline.toISOString(),
    }),
  );
}

function startListening() {
  store().hydrate({
    ...session({
      serverTime: serverNow.toISOString(),
      deadlineAt: deadline.toISOString(),
    }),
    groups: [previewGroup],
    groupAudioPlays: { [recordingId]: 0 },
  });
}

it("hands in a paper that a refused shared-recording play locked, when the device's deadline arrives", async () => {
  startListening();
  counted.mockRejectedValue(refusal());

  useGroupPlaybackStore.getState().notePlay(recordingId);
  await vi.advanceTimersByTimeAsync(0);
  expect(counted).toHaveBeenCalledTimes(1);
  expect(submitted).not.toHaveBeenCalled();
  expect(store().lock).toBe("deadline");

  await vi.advanceTimersByTimeAsync(untilDeadline - 1_000);
  expect(submitted).not.toHaveBeenCalled();

  await vi.advanceTimersByTimeAsync(2_000);
  expect(submitted).toHaveBeenCalledTimes(1);
  expect(submitted).toHaveBeenCalledWith("att-1", { reason: "timer_expired" });
  expect(store().submitState).toBe("done");
});

it("hands in a paper locked by lockNow alone", async () => {
  start();
  store().lockNow("deadline");

  await vi.advanceTimersByTimeAsync(untilDeadline - 1_000);
  expect(submitted).not.toHaveBeenCalled();

  await vi.advanceTimersByTimeAsync(2_000);
  expect(submitted).toHaveBeenCalledTimes(1);
  expect(submitted).toHaveBeenCalledWith("att-1", { reason: "timer_expired" });
});

it("hands in a paper that a refused save locked, as before", async () => {
  start();
  saved.mockRejectedValue(refusal());

  store().setAnswer("q1", text("typed"));
  await vi.advanceTimersByTimeAsync(2_000);
  expect(saved).toHaveBeenCalledTimes(1);
  expect(submitted).not.toHaveBeenCalled();
  expect(store().lock).toBe("deadline");

  await vi.advanceTimersByTimeAsync(untilDeadline);
  expect(submitted).toHaveBeenCalledTimes(1);
  expect(submitted).toHaveBeenCalledWith("att-1", { reason: "timer_expired" });
});

it("hands the paper in once when a play that was in flight is refused after a save was", async () => {
  startListening();
  let refusePlay: () => void = () => undefined;
  counted.mockImplementation(
    () =>
      new Promise((_resolve, reject) => {
        refusePlay = () => reject(refusal());
      }),
  );

  useGroupPlaybackStore.getState().notePlay(recordingId);
  expect(counted).toHaveBeenCalledTimes(1);
  expect(store().lock).toBeNull();

  saved.mockRejectedValue(refusal());
  store().setAnswer("q1", text("typed"));
  await vi.advanceTimersByTimeAsync(2_000);
  expect(submitted).not.toHaveBeenCalled();
  expect(store().lock).toBe("deadline");

  refusePlay();
  await vi.advanceTimersByTimeAsync(0);
  expect(submitted).not.toHaveBeenCalled();
  expect(store().lock).toBe("deadline");

  await vi.advanceTimersByTimeAsync(untilDeadline + 10 * 60_000);
  expect(submitted).toHaveBeenCalledTimes(1);
  expect(submitted).toHaveBeenCalledWith("att-1", { reason: "timer_expired" });
});

it("tries again when that submit fails", async () => {
  start();
  submitted.mockRejectedValueOnce(new Error("offline"));
  store().lockNow("deadline");

  await vi.advanceTimersByTimeAsync(untilDeadline + 100);
  expect(submitted).toHaveBeenCalledTimes(1);
  expect(store().submitState).toBe("idle");
  expect(store().lock).toBe("deadline");

  await vi.advanceTimersByTimeAsync(2_000);
  expect(submitted).toHaveBeenCalledTimes(2);
  expect(store().submitState).toBe("done");
});

it("does not arm a timer while a submit is in flight", async () => {
  start();
  let finish: () => void = () => undefined;
  submitted.mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = () => resolve(handedIn);
      }),
  );

  void store().submit("manual");
  store().lockNow("deadline");

  await vi.advanceTimersByTimeAsync(untilDeadline + 1_000);
  expect(submitted).toHaveBeenCalledTimes(1);
  expect(submitted).toHaveBeenCalledWith("att-1", { reason: "manual" });

  finish();
  await vi.advanceTimersByTimeAsync(0);
  expect(store().submitState).toBe("done");
  expect(submitted).toHaveBeenCalledTimes(1);
});

it.each(["superseded", "closed"] as const)(
  "leaves a paper locked as %s alone",
  async (lock) => {
    start();
    store().lockNow(lock);

    await vi.advanceTimersByTimeAsync(untilDeadline + 10 * 60_000);
    expect(submitted).not.toHaveBeenCalled();
  },
);

it("does not submit before the device's deadline, however the lock arrived", async () => {
  start();
  store().lockNow("deadline");

  await vi.advanceTimersByTimeAsync(29 * 60_000);
  expect(submitted).not.toHaveBeenCalled();
});
