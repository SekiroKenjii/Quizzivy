import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
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

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const question = {
  id: QUESTION_ID,
  type: "short_answer" as const,
  prompt: "Viết 2–3 câu tả thói quen buổi sáng.",
  media: null,
  audio: null,
  transcript: null,
  options: [],
  blanks: [],
  points: 5,
  explanation: null,
  sampleAnswer: null,
  tags: ["writing"],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

beforeEach(() => {
  vi.stubGlobal(
    "matchMedia",
    vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }),
  );
  server.use(
    http.get(`${BASE}/admin/tests/${TEST_ID}`, () =>
      contractJson("/admin/tests/{id}", "get", 200, {
        id: TEST_ID,
        title: "Unit 5",
        description: null,
        status: "draft",
        currentVersion: 0,
        totalPoints: 5,
        questionCount: 1,
        audioCount: 0,
        sections: [
          {
            id: "018f0000-0000-7000-8000-0000000000c1",
            ordinal: 0,
            title: "Phần 1",
            instructions: null,
            questionIds: [QUESTION_ID],
          },
        ],
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      }),
    ),
    http.get(`${BASE}/admin/questions/${QUESTION_ID}`, () =>
      contractJson("/admin/questions/{id}", "get", 200, question),
    ),
  );
});

function renderBuilder() {
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
  return userEvent.setup();
}

describe("the builder where a third column does not fit", () => {
  it("keeps the question settings reachable from the bar", async () => {
    const user = renderBuilder();
    await user.click(
      await screen.findByRole("button", { name: /tả thói quen/ }, { timeout: 5000 }),
    );

    const trigger = await screen.findByRole(
      "button",
      { name: "Cài đặt câu hỏi" },
      { timeout: 5000 },
    );
    expect(screen.queryByRole("dialog")).toBeNull();

    await user.click(trigger);
    const sheet = await screen.findByRole("dialog", {}, { timeout: 5000 });
    expect(within(sheet).getByLabelText("Điểm")).toHaveValue(5);
  });

  it("keeps the settings out of the layout while they live in the sheet", async () => {
    const user = renderBuilder();
    await user.click(
      await screen.findByRole("button", { name: /tả thói quen/ }, { timeout: 5000 }),
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Cài đặt câu hỏi" }),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByRole("complementary", { name: "Cài đặt câu hỏi" })).toBeNull();
  });
});
