import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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
  nextVersion: 1,
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
  sampleAnswer: null,
  tags: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

let patches = 0;

beforeEach(() => {
  patches = 0;
  vi.useFakeTimers({ shouldAdvanceTime: true });
  server.use(
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, test),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, question),
    ),
    http.patch(`${BASE}/teacher/questions/:id`, () => {
      patches++;
      return contractJson("/teacher/questions/{id}", "patch", 400, {
        error: {
          code: "VALIDATION_FAILED",
          message: "Dữ liệu câu hỏi không hợp lệ.",
          details: { "options[2].text": "Phương án không được để trống." },
          requestId: "018f0000-0000-7000-8000-0000000000f1",
        },
      });
    }),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
}

describe("a question save the server refuses", () => {
  it("says it was not saved, with the server's reason, and keeps the editor up", async () => {
    const user = renderBuilder();
    const option = await screen.findByRole("textbox", { name: "Lựa chọn B" });
    await user.type(option, " to");
    await vi.advanceTimersByTimeAsync(2_000);

    await waitFor(() => expect(patches).toBe(1));
    expect(await screen.findByText("Chưa lưu được")).toBeInTheDocument();
    expect(screen.getByText("Dữ liệu câu hỏi không hợp lệ.")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Lựa chọn B" })).toHaveValue(
      "have gone to",
    );
    await vi.advanceTimersByTimeAsync(5_000);
    expect(patches).toBe(1);
  });
});

describe("a question the form schema refuses", () => {
  it("is not sent, and the label says what to fix", async () => {
    const user = renderBuilder();
    await user.click(await screen.findByRole("button", { name: "Thêm lựa chọn" }));
    await vi.advanceTimersByTimeAsync(2_000);

    expect(await screen.findByText("Hãy điền mọi lựa chọn.")).toBeInTheDocument();
    expect(screen.getByText("Chưa lưu được")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Lựa chọn C" })).toBeInTheDocument();
    expect(patches).toBe(0);
  });
});
