import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { getAttempt, saveAnswers } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { session } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

beforeEach(() => {
  useTakeTestStore.getState().reset();
  const now = new Date().toISOString();
  vi.mocked(saveAnswers).mockResolvedValue({ serverTime: now, savedAt: now });
  vi.mocked(getAttempt).mockResolvedValue({
    ...session({
      serverTime: now,
      deadlineAt: new Date(Date.now() + 3_600_000).toISOString(),
    }),
    questions: [
      {
        id: "q1",
        sectionId: "s1",
        type: "single_choice",
        prompt: "Choose one",
        points: 1,
        options: [
          { id: "a", text: "First answer" },
          { id: "b", text: "Second answer" },
        ],
      },
      {
        id: "q2",
        sectionId: "s1",
        type: "short_answer",
        prompt: "Explain your answer",
        points: 1,
      },
    ],
  });
});
afterEach(() => useTakeTestStore.getState().reset());

it("keeps navigation and answer shortcuts working after clicking a radio", async () => {
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    {
      initialEntries: ["/app/attempts/att-1"],
    },
  );
  render(<RouterProvider router={router} />);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("radio", { name: "First answer" }));
  await user.keyboard("b");
  expect(screen.getByRole("radio", { name: "Second answer" })).toBeChecked();
  await user.keyboard("f");
  expect(screen.getByRole("button", { name: "Bỏ đánh dấu câu này" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await user.keyboard("{ArrowRight}");
  expect(await screen.findByText("Explain your answer")).toBeVisible();
  const input = screen.getByRole("textbox");
  await user.type(input, "first");
  await user.keyboard("{ArrowLeft}f");
  expect(input).toHaveValue("firsft");
  expect(useTakeTestStore.getState().flags.has("q2")).toBe(false);
});
