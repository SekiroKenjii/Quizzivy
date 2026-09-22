import { beforeEach, expect, it } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
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

const SECOND = "018f0000-0000-7000-8000-0000000000b2";
let savedPrompt = question.prompt;

beforeEach(() => {
  savedPrompt = question.prompt;
  server.use(
    http.get(`${BASE}/admin/tests/:id`, () =>
      contractJson("/admin/tests/{id}", "get", 200, {
        ...test,
        questionCount: 2,
        totalPoints: 2,
        sections: [{ ...test.sections[0], questionIds: [QUESTION_ID, SECOND] }],
      }),
    ),
    http.get(`${BASE}/admin/questions/:id`, ({ params }) =>
      contractJson("/admin/questions/{id}", "get", 200, {
        ...question,
        id: String(params.id),
        prompt: params.id === SECOND ? "Second question" : savedPrompt,
      }),
    ),
  );
});

async function mount() {
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
  await screen.findByDisplayValue(question.prompt);
  await act(async () => {
    await import("@/features/tests/components/OutlineTree");
  });
  await screen.findByRole("button", { name: "Second question" });
  return userEvent.setup();
}

it("waits for the latest save before switching and shows that edit on the first return", async () => {
  let finish: (() => void) | undefined;
  server.use(
    http.patch(`${BASE}/admin/questions/:id`, async ({ request }) => {
      const body = (await request.json()) as { prompt: string };
      await new Promise<void>((resolve) => {
        finish = resolve;
      });
      savedPrompt = body.prompt;
      return contractJson("/admin/questions/{id}", "patch", 200, {
        ...question,
        prompt: savedPrompt,
      });
    }),
  );
  const user = await mount();
  await user.type(screen.getByDisplayValue(question.prompt), " Updated");
  await user.click(screen.getByRole("button", { name: "Second question" }));
  await waitFor(() => expect(finish).toBeDefined());
  expect(screen.getByLabelText("Nội dung câu hỏi")).toHaveValue(
    `${question.prompt} Updated`,
  );
  await act(async () => {
    finish?.();
  });
  await screen.findByDisplayValue("Second question");
  await user.click(
    await screen.findByRole("button", { name: `${question.prompt} Updated` }),
  );
  await waitFor(() =>
    expect(screen.getByLabelText("Nội dung câu hỏi")).toHaveValue(
      `${question.prompt} Updated`,
    ),
  );
});

it("keeps the edited question open when its final save fails", async () => {
  server.use(
    http.patch(`${BASE}/admin/questions/:id`, () =>
      HttpResponse.json(
        { error: { code: "INTERNAL_ERROR", message: "Save failed" } },
        { status: 500 },
      ),
    ),
  );
  const user = await mount();
  await user.type(screen.getByDisplayValue(question.prompt), " Unsaved");
  await user.click(screen.getByRole("button", { name: "Second question" }));
  expect(
    await screen.findByText("Save failed", { selector: '[role="alert"]' }),
  ).toBeVisible();
  expect(screen.getByLabelText("Nội dung câu hỏi")).toHaveValue(
    `${question.prompt} Unsaved`,
  );
});
