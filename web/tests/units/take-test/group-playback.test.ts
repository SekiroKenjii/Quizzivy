import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { recordGroupAudioPlay, submitAttempt } from "@/features/take-test/api";
import {
  groupPlayCount,
  useGroupPlaybackStore,
} from "@/features/take-test/groupPlayback";
import {
  clearGroupPlayDrafts,
  readGroupPlays,
} from "@/features/take-test/groupPlaybackDraft";
import { useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import { previewGroup } from "@tests/support/groupPreview";
import { session } from "./support";

vi.mock("@/features/take-test/api", () => ({
  recordGroupAudioPlay: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
}));
const counted = vi.mocked(recordGroupAudioPlay);
const recordingId = previewGroup.recordings[0]!.id;
const secondId = "01935000-0000-7000-8000-000000000055";
const state = () => useGroupPlaybackStore.getState();
const displayed = () => groupPlayCount(state(), recordingId);

function payload(plays = 0) {
  return {
    ...session({
      serverTime: new Date().toISOString(),
      deadlineAt: new Date(Date.now() + 3600000).toISOString(),
    }),
    groups: [
      {
        ...previewGroup,
        recordings: [
          ...previewGroup.recordings,
          { ...previewGroup.recordings[0]!, id: secondId },
        ],
      },
    ],
    groupAudioPlays: { [recordingId]: plays },
  };
}

beforeEach(() => {
  useTakeTestStore.getState().reset();
  localStorage.clear();
  counted.mockReset();
});
afterEach(() => {
  useTakeTestStore.getState().reset();
  vi.useRealTimers();
});

it("counts synchronously and serializes gestures without duplicating an in-flight retry", async () => {
  const replies: ((value: {
    playId: string;
    plays: number;
    maxPlays: number;
  }) => void)[] = [];
  counted.mockImplementation(() => new Promise((resolve) => replies.push(resolve)));
  useTakeTestStore.getState().hydrate(payload());
  state().notePlay(recordingId);
  state().notePlay(recordingId);
  const flushed = state().flush();
  expect(displayed()).toBe(2);
  expect(counted).toHaveBeenCalledTimes(1);
  const first = counted.mock.calls[0]![1].playId;
  replies[0]!({ playId: first, plays: 1, maxPlays: 2 });
  await vi.waitFor(() => expect(counted).toHaveBeenCalledTimes(2));
  expect(displayed()).toBe(2);
  const second = counted.mock.calls[1]![1].playId;
  expect(first).not.toBe(second);
  replies[1]!({ playId: second, plays: 2, maxPlays: 2 });
  await flushed;
  expect(state().pending).toHaveLength(0);
  expect(displayed()).toBe(2);
  expect(
    Object.keys(localStorage).filter((key) => key.startsWith("quizzivy.group-play.")),
  ).toHaveLength(0);
});

it("retries the same gesture after a lost response, reload and takeover without double counting", async () => {
  counted.mockRejectedValue(new Error("response lost after server commit"));
  useTakeTestStore.getState().hydrate(payload(1));
  state().notePlay(recordingId);
  await state().flush();
  const original = counted.mock.calls[0]![1].playId;
  expect(displayed()).toBe(2);
  useTakeTestStore.getState().reset({ keepDraft: true });
  useTakeTestStore.getState().hydrate({ ...payload(2), sessionId: "new-session" });
  expect(displayed()).toBe(2);
  counted.mockImplementation(async (_id, body) => ({
    playId: body.playId,
    plays: 2,
    maxPlays: 2,
  }));
  await state().flush();
  expect(counted.mock.lastCall?.[1]).toEqual({
    recordingId,
    playId: original,
    sessionId: "new-session",
  });
  expect(displayed()).toBe(2);
  expect(state().pending).toHaveLength(0);
});

it("keeps same-file recording allowances independent and permits over-limit plays", async () => {
  counted.mockImplementation(async (_id, body) => ({
    playId: body.playId,
    plays: body.recordingId === recordingId ? 5 : 1,
    maxPlays: 2,
  }));
  useTakeTestStore.getState().hydrate(payload(4));
  state().notePlay(recordingId);
  state().notePlay(secondId);
  await state().flush();
  expect(displayed()).toBe(5);
  expect(groupPlayCount(state(), secondId)).toBe(1);
});

it("keeps confirmed counts monotonic when a stale refetch arrives", async () => {
  counted.mockImplementation(async (_id, body) => ({
    playId: body.playId,
    plays: 6,
    maxPlays: 2,
  }));
  useTakeTestStore.getState().hydrate(payload(4));
  state().notePlay(recordingId);
  await state().flush();
  useTakeTestStore.getState().hydrate(payload(4));
  expect(displayed()).toBe(6);
});

it("ignores a previous session's late response and stops writing when superseded", async () => {
  let finish:
    ((value: { playId: string; plays: number; maxPlays: number }) => void) | undefined;
  counted.mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  useTakeTestStore.getState().hydrate(payload());
  state().notePlay(recordingId);
  const old = state().flush();
  const original = counted.mock.calls[0]![1].playId;
  useTakeTestStore.getState().hydrate({ ...payload(2), sessionId: "new-session" });
  finish?.({ playId: original, plays: 50, maxPlays: 2 });
  await old;
  expect(displayed()).toBe(2);
  counted.mockRejectedValue(
    new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "superseded" }),
  );
  await state().flush();
  expect(useTakeTestStore.getState().lock).toBe("superseded");
  const count = counted.mock.calls.length;
  state().notePlay(recordingId);
  await state().flush();
  expect(counted).toHaveBeenCalledTimes(count);
});

it("isolates recovery by learner and clears pending telemetry on sign-out", async () => {
  counted.mockRejectedValue(new Error("offline"));
  useTakeTestStore.getState().hydrate(payload());
  state().notePlay(recordingId);
  await state().flush();
  useTakeTestStore.getState().reset({ keepDraft: true });
  const other = payload();
  useTakeTestStore
    .getState()
    .hydrate({ ...other, attempt: { ...other.attempt, studentId: "other-student" } });
  expect(state().pending).toHaveLength(0);
  clearGroupPlayDrafts();
  useTakeTestStore.getState().hydrate(payload());
  expect(state().pending).toHaveLength(0);
});

it("does not block answer submission on unavailable listening telemetry", async () => {
  counted.mockRejectedValue(new Error("offline"));
  const current = payload();
  vi.mocked(submitAttempt).mockResolvedValue({
    ...current.attempt,
    status: "submitted",
    submittedAt: new Date().toISOString(),
  });
  useTakeTestStore.getState().hydrate(current);
  state().notePlay(recordingId);
  await state().flush();
  await useTakeTestStore.getState().submit();
  expect(submitAttempt).toHaveBeenCalled();
  expect(useTakeTestStore.getState().submitState).toBe("done");
});

it("ignores malformed browser entries and expires unconfirmed gestures at the attempt deadline", async () => {
  localStorage.setItem("quizzivy.group-play.corrupt", "{");
  counted.mockRejectedValue(new Error("offline"));
  const current = payload();
  useTakeTestStore.getState().hydrate(current);
  state().notePlay(recordingId);
  await state().flush();
  expect(
    readGroupPlays(current.attempt.studentId, current.attempt.id, Date.now()),
  ).toHaveLength(1);
  const extended = Date.now() + 10800000;
  expect(
    readGroupPlays(
      current.attempt.studentId,
      current.attempt.id,
      Date.now() + 7200000,
      extended,
    )[0]?.deadlineAt,
  ).toBe(extended);
  expect(
    readGroupPlays(current.attempt.studentId, current.attempt.id, Date.now() + 7200000),
  ).toHaveLength(0);
  expect(
    Object.keys(localStorage).filter((key) => key.startsWith("quizzivy.group-play.")),
  ).toHaveLength(0);
});

it("bounds a hung telemetry request before submitting the answers", async () => {
  vi.useFakeTimers();
  counted.mockImplementation(
    (_id, _body, signal) =>
      new Promise((_resolve, reject) =>
        signal?.addEventListener("abort", () => reject(new Error("timeout"))),
      ),
  );
  const current = payload();
  vi.mocked(submitAttempt).mockResolvedValue({
    ...current.attempt,
    status: "submitted",
    submittedAt: new Date().toISOString(),
  });
  useTakeTestStore.getState().hydrate(current);
  state().notePlay(recordingId);
  const submission = useTakeTestStore.getState().submit();
  await vi.advanceTimersByTimeAsync(3000);
  await submission;
  expect(useTakeTestStore.getState().submitState).toBe("done");
});
