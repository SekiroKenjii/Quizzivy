import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { getAttempt } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import { session } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
      { path: "/app", element: <p>home</p> },
    ],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  vi.mocked(getAttempt).mockReset();
  useTakeTestStore.getState().reset();
});
afterEach(() => useTakeTestStore.getState().reset());

describe("when the paper does not load", () => {
  it("offers a retry that refetches, with the request id to quote", async () => {
    vi.mocked(getAttempt)
      .mockRejectedValueOnce(
        new ApiError({
          status: 500,
          code: "UNKNOWN",
          message: "boom",
          requestId: "req_9",
        }),
      )
      .mockResolvedValue({
        ...session({
          serverTime: "2026-09-01T08:00:00.000Z",
          deadlineAt: "2026-09-01T09:00:00.000Z",
        }),
        questions: [{ id: "q1", type: "true_false", prompt: "True?", points: 1 }],
      });
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không mở được bài làm.",
    );
    expect(screen.getByText("req_9")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(getAttempt).toHaveBeenCalledTimes(2));
    expect(await screen.findByRole("button", { name: "Thoát" })).toBeInTheDocument();
  });

  it("always leaves a way home", async () => {
    vi.mocked(getAttempt).mockRejectedValue(new Error("offline"));
    const user = userEvent.setup();
    const router = renderPage();

    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Về trang chủ" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/app"));
  });
});
