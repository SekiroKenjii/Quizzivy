import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";

const test = {
  skills: [],
  assignments: { live: 0, scheduled: 0, closed: 0 },
  unpublishedChanges: null,
  id: TEST_ID,
  title: "Unit 5",
  description: null,
  status: "draft" as const,
  currentVersion: 0,
  totalPoints: 1,
  questionCount: 1,
  audioCount: 0,
  sections: [
    {
      id: "018f0000-0000-7000-8000-0000000000c1",
      ordinal: 0,
      title: "Ngữ pháp",
      instructions: null,
      questionIds: [QUESTION_ID],
    },
  ],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

const question = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "single_choice" as const,
  prompt: "They ___ to the museum.",
  media: null,
  audio: null,
  transcript: null,
  options: [
    {
      id: "018f0000-0000-7000-8000-0000000000d1",
      ordinal: 0,
      text: "went",
      isCorrect: true,
    },
    {
      id: "018f0000-0000-7000-8000-0000000000d2",
      ordinal: 1,
      text: "have gone",
      isCorrect: false,
    },
  ],
  blanks: [],
  points: 1,
  explanation: null,
  sampleAnswer: "went",
  tags: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  server.use(
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, test),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, question),
    ),
    http.patch(`${BASE}/teacher/tests/:id`, async () => {
      await new Promise((resolve) => setTimeout(resolve, 60_000));
      return contractJson("/teacher/tests/{id}", "patch", 200, test);
    }),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

async function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> },
      { path: "/teacher/tests/:id", element: <p>detail page</p> },
      { path: "/teacher/tests", element: <p>tests list</p> },
    ],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return {
    user: userEvent.setup({ advanceTimers: vi.advanceTimersByTime }),
    title: await screen.findByLabelText("Tên đề thi"),
    router,
  };
}

describe("the builder's bar", () => {
  it("previews the draft as a student, without the key", async () => {
    const { user } = await renderBuilder();
    await screen.findByText("They ___ to the museum.");

    await user.click(screen.getByRole("button", { name: "Xem như học viên" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("They ___ to the museum.")).toBeInTheDocument();
    expect(within(dialog).getByText("went")).toBeInTheDocument();
    expect(within(dialog).getByText("have gone")).toBeInTheDocument();
    expect(within(dialog).queryByText(/đáp án đúng/i)).toBeNull();
    expect(dialog.innerHTML).not.toContain("isCorrect");
  });

  it("sends Phiên bản to the detail's history card, not the same page as the preview", async () => {
    const { user, router } = await renderBuilder();

    await user.click(screen.getByRole("button", { name: "Phiên bản" }));

    await waitFor(() => expect(router.state.location.hash).toBe("#versions"));
    expect(router.state.location.pathname).toBe(`/teacher/tests/${TEST_ID}`);
  });

  it("asks before leaving while a save is still in flight, and stays on Ở lại", async () => {
    const { user, title, router } = await renderBuilder();
    await user.type(title, " (sửa)");
    vi.advanceTimersByTime(2_000);

    await user.click(screen.getByRole("button", { name: "Quay lại" }));

    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText("Rời trang khi chưa lưu xong?"),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(router.state.location.pathname).toBe(`/teacher/tests/${TEST_ID}/edit`);

    await user.click(screen.getByRole("button", { name: "Quay lại" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: "Rời đi" }),
    );
    await waitFor(() => expect(router.state.location.pathname).toBe("/teacher/tests"));
  });
});
