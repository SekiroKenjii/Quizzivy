import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useGroupPlaybackStore } from "@/features/take-test/groupPlayback";
import { FLUSH_DEBOUNCE_MS, useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import {
  recordGroupAudioPlay,
  saveAnswers,
  submitAttempt,
  type AttemptSession,
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
const store = () => useTakeTestStore.getState();
const refusal = (code: "DEADLINE_PASSED" | "SESSION_SUPERSEDED") =>
  new ApiError({ status: 409, code, message: code });

function paper(status: AttemptSession["attempt"]["status"] = "in_progress") {
  return session({
    serverTime: serverNow.toISOString(),
    deadlineAt: deadline.toISOString(),
    status,
  });
}

function takenOver(): AttemptSession {
  return { ...paper(), beaconToken: "", superseded: true };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(serverNow);
  submitted.mockReset().mockResolvedValue(paper("submitted").attempt);
  saved.mockReset();
  counted.mockReset();
  store().reset();
  localStorage.clear();
});
afterEach(() => {
  store().reset();
  vi.useRealTimers();
});

it.each([
  ["superseded", takenOver],
  ["closed", () => paper("submitted")],
] as const)(
  "keeps a paper locked as %s when a deadline refusal reaches it, and hands nothing in at the device's deadline",
  async (lock, payload) => {
    store().hydrate(payload());
    expect(store().lock).toBe(lock);

    store().lockNow("deadline");
    expect(store().lock).toBe(lock);

    await vi.advanceTimersByTimeAsync(untilDeadline + 10 * 60_000);
    expect(submitted).not.toHaveBeenCalled();
    expect(store().lock).toBe(lock);
  },
);

it("keeps a paper another device took over as it is when a play that was in flight is refused for time", async () => {
  store().hydrate({
    ...paper(),
    groups: [previewGroup],
    groupAudioPlays: { [recordingId]: 0 },
  });
  let refusePlay: () => void = () => undefined;
  counted.mockImplementation(
    () =>
      new Promise((_resolve, reject) => {
        refusePlay = () => reject(refusal("DEADLINE_PASSED"));
      }),
  );
  useGroupPlaybackStore.getState().notePlay(recordingId);
  expect(counted).toHaveBeenCalledTimes(1);

  saved.mockRejectedValue(refusal("SESSION_SUPERSEDED"));
  store().setAnswer("q1", text("typed"));
  await vi.advanceTimersByTimeAsync(FLUSH_DEBOUNCE_MS);
  expect(store().lock).toBe("superseded");

  refusePlay();
  await vi.advanceTimersByTimeAsync(0);
  expect(store().lock).toBe("superseded");

  await vi.advanceTimersByTimeAsync(untilDeadline + 10 * 60_000);
  expect(submitted).not.toHaveBeenCalled();
  expect(store().lock).toBe("superseded");
});
