import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useIntegrityAutoSubmit } from "@/features/integrity/useIntegrityAutoSubmit";
import { useTakeTestStore } from "@/features/take-test/store";
import { submitAttempt } from "@/features/take-test/api";
import { session } from "../take-test/support";

vi.mock("@/features/take-test/api", () => ({
  submitAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  getAttempt: vi.fn(),
}));

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-09-01T08:00:00Z"));
  useTakeTestStore.getState().reset();
  vi.mocked(submitAttempt).mockReset();
});
afterEach(() => {
  useTakeTestStore.getState().reset();
  vi.useRealTimers();
});

function start(limit: number) {
  const payload = session({
    serverTime: "2026-09-01T08:00:00Z",
    deadlineAt: "2026-09-01T09:00:00Z",
  });
  payload.integrity = {
    ...payload.integrity,
    maxFocusLoss: limit,
    onLimitExceeded: "auto_submit",
  };
  vi.mocked(submitAttempt).mockResolvedValue({
    ...payload.attempt,
    status: "submitted",
    submittedAt: "2026-09-01T08:00:01Z",
  });
  useTakeTestStore.getState().hydrate(payload);
}

it("submits immediately only after the allowed count is exceeded", async () => {
  start(1);
  const view = renderHook(({ count }) => useIntegrityAutoSubmit(count), {
    initialProps: { count: 1 },
  });
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(submitAttempt).not.toHaveBeenCalled();
  view.rerender({ count: 2 });
  expect(view.result.current).toBe(true);
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(submitAttempt).toHaveBeenCalledExactlyOnceWith("att-1", {
    reason: "auto_submit",
  });
  expect(useTakeTestStore.getState().submitState).toBe("done");
});

it("does not submit for an unlimited policy", async () => {
  start(0);
  const view = renderHook(() => useIntegrityAutoSubmit(100));
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(view.result.current).toBe(false);
  expect(submitAttempt).not.toHaveBeenCalled();
});
