import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/TestBuilderPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";

const test = {
  id: TEST_ID,
  title: "Unit 5",
  description: null,
  status: "draft" as const,
  currentVersion: 0,
  totalPoints: 1,
  questionCount: 1,
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
  id: QUESTION_ID,
  type: "single_choice" as const,
  prompt: "They ___ to the museum.",
  media: null,
  audio: null,
  transcript: null,
  options: [
    { id: "018f0000-0000-7000-8000-0000000000d1", text: "went", isCorrect: true },
    { id: "018f0000-0000-7000-8000-0000000000d2", text: "have gone", isCorrect: false },
  ],
  blanks: [],
  points: 1,
  explanation: null,
  sampleAnswer: null,
  tags: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

let patches: { title?: string }[] = [];

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  patches = [];
  server.use(
    http.get(`${BASE}/admin/tests/:id`, () =>
      contractJson("/admin/tests/{id}", "get", 200, test),
    ),
    http.get(`${BASE}/admin/questions/:id`, () =>
      contractJson("/admin/questions/{id}", "get", 200, question),
    ),
    http.patch(`${BASE}/admin/tests/:id`, async ({ request }) => {
      patches.push((await request.json()) as { title?: string });
      return contractJson("/admin/tests/{id}", "patch", 200, test);
    }),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

async function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/admin/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return {
    user: userEvent.setup({ advanceTimers: vi.advanceTimersByTime }),
    title: await screen.findByLabelText("Tên đề thi"),
  };
}

describe("the builder's autosave", () => {
  it("coalesces edits inside the 1.5s window into one request", async () => {
    const { user, title } = await renderBuilder();

    await user.type(title, "abc");
    expect(patches, "typing must not save on every keystroke").toHaveLength(0);

    await vi.advanceTimersByTimeAsync(1500);

    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]?.title).toBe("Unit 5abc");
  });

  it("saves again for an edit made after the window closed", async () => {
    const { user, title } = await renderBuilder();

    await user.type(title, "a");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(patches).toHaveLength(1));

    await user.type(title, "b");
    await vi.advanceTimersByTimeAsync(1500);

    await waitFor(() => expect(patches).toHaveLength(2));
    expect(patches[1]?.title).toBe("Unit 5ab");
  });

  it("says when it saved, in words rather than a spinner", async () => {
    const { user, title } = await renderBuilder();

    await user.type(title, "a");
    await vi.advanceTimersByTimeAsync(1500);

    expect(await screen.findByText(/Đã lưu \d\d:\d\d/)).toBeInTheDocument();
  });

  it("surfaces a stale write as 'open somewhere else', and stops saving", async () => {
    server.use(
      http.patch(`${BASE}/admin/tests/:id`, () =>
        Response.json(
          { error: { code: "STALE_WRITE", message: "Đã có thay đổi ở nơi khác." } },
          { status: 409 },
        ),
      ),
    );
    const { user, title } = await renderBuilder();

    await user.type(title, "a");
    await vi.advanceTimersByTimeAsync(1500);

    expect(await screen.findByText(/mở ở nơi khác/)).toBeInTheDocument();
    // Retrying would overwrite whatever the other tab saved, which is the loss
    // the version guard exists to prevent.
    expect(screen.getByRole("button", { name: "Phát hành" })).toBeDisabled();
  });
});
