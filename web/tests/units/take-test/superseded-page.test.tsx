import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import FocusLayout from "@/layouts/FocusLayout";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { clearSession, pending } from "@/features/integrity/buffer";
import { flushEvents, getAttempt } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { DECK_ATTEMPT, deckSession } from "./deckSession";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  flushEvents: vi.fn(),
}));

const NOW = new Date("2026-09-22T05:00:00.000Z");
const pass = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
  sessionStorage.clear();
  localStorage.clear();
  clearSession(DECK_ATTEMPT);
  vi.mocked(flushEvents)
    .mockReset()
    .mockResolvedValue(undefined as never);
});
afterEach(() => {
  useTakeTestStore.getState().reset();
  clearSession(DECK_ATTEMPT);
  vi.useRealTimers();
});

it("records nothing on a paper another tab took over", async () => {
  vi.mocked(getAttempt).mockResolvedValue({
    ...deckSession(NOW),
    beaconToken: "",
    superseded: true,
  });
  const router = createMemoryRouter(
    [
      {
        path: "/app/attempts/:attemptId",
        element: <FocusLayout />,
        children: [{ index: true, element: <TakeTestPage /> }],
      },
    ],
    { initialEntries: [`/app/attempts/${DECK_ATTEMPT}`] },
  );
  render(<RouterProvider router={router} />);
  await pass(0);

  act(() => {
    document.dispatchEvent(new Event("contextmenu"));
    window.dispatchEvent(new Event("blur"));
  });
  await pass(0);

  expect(useTakeTestStore.getState().lock).toBe("superseded");
  expect(pending()).toEqual([]);
});
