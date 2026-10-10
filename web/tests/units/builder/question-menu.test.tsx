import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { Toaster, toast } from "@/components/ui/sonner";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const FIRST = "018f0000-0000-7000-8000-0000000000b1";
const SECOND = "018f0000-0000-7000-8000-0000000000b2";
const COPY = "018f0000-0000-7000-8000-0000000000b3";
const PART_ONE = "018f0000-0000-7000-8000-0000000000c1";
const PART_TWO = "018f0000-0000-7000-8000-0000000000c2";

type Section = { id: string | null; title: string; questionIds: string[] };
let saved: Section[][] = [];
let duplicated: string[] = [];

function question(id: string, prompt: string): components["schemas"]["AdminQuestion"] {
  return {
    level: null,
    skill: null,
    id,
    type: "short_answer",
    prompt,
    media: null,
    audio: null,
    transcript: null,
    options: [],
    blanks: [],
    points: 1,
    explanation: null,
    sampleAnswer: null,
    tags: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

const PROMPTS: Record<string, string> = {
  [FIRST]: "Câu đầu tiên",
  [SECOND]: "Câu thứ hai",
  [COPY]: "Câu đầu tiên",
};

function testBody(sections: Section[]): components["schemas"]["Test"] {
  return {
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: null,
    id: TEST_ID,
    title: "Unit 5",
    description: null,
    status: "draft",
    currentVersion: 0,
    totalPoints: 2,
    questionCount: 2,
    audioCount: 0,
    sections: sections.map((section, ordinal) => ({
      id: section.id ?? PART_TWO,
      ordinal,
      title: section.title,
      instructions: null,
      questionIds: section.questionIds,
    })),
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: `2026-01-02T00:00:0${saved.length}.000000Z`,
  };
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  saved = [];
  duplicated = [];
  server.use(
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson(
        "/teacher/tests/{id}",
        "get",
        200,
        testBody([
          { id: PART_ONE, title: "Phần 1", questionIds: [FIRST, SECOND] },
          { id: PART_TWO, title: "Phần 2", questionIds: [] },
        ]),
      ),
    ),
    http.get(`${BASE}/teacher/questions/:id`, ({ params }) =>
      contractJson(
        "/teacher/questions/{id}",
        "get",
        200,
        question(String(params.id), PROMPTS[String(params.id)] ?? "?"),
      ),
    ),
    http.post(`${BASE}/teacher/questions/:id/duplicate`, ({ params }) => {
      duplicated.push(String(params.id));
      return contractJson(
        "/teacher/questions/{id}/duplicate",
        "post",
        201,
        question(COPY, PROMPTS[COPY]!),
      );
    }),
    http.patch(`${BASE}/teacher/tests/:id`, async ({ request }) => {
      const body = (await request.json()) as { sections: Section[] };
      saved.push(body.sections);
      return contractJson("/teacher/tests/{id}", "patch", 200, testBody(body.sections));
    }),
  );
});

afterEach(() => {
  toast.dismiss();
  vi.useRealTimers();
});

async function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  expect(await screen.findByRole("heading", { name: "Câu 1" })).toBeVisible();
  return user;
}

async function choose(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(screen.getByRole("button", { name: "Thao tác với câu hỏi" }));
  await user.click(await screen.findByRole("menuitem", { name }));
}

async function lastSave() {
  await vi.advanceTimersByTimeAsync(1500);
  await waitFor(() => expect(saved.length).toBeGreaterThan(0));
  return saved.at(-1)!.map((section) => section.questionIds);
}

describe("the editor pane's question menu", () => {
  it("duplicates the question right after itself and opens the copy", async () => {
    const user = await renderBuilder();

    await choose(user, "Nhân bản");

    await waitFor(() => expect(duplicated).toEqual([FIRST]));
    expect(await lastSave()).toEqual([[FIRST, COPY, SECOND], []]);
    expect(await screen.findByRole("heading", { name: "Câu 2" })).toBeVisible();
  });

  it("moves the question to the end of the chosen section", async () => {
    const user = await renderBuilder();

    await choose(user, "Chuyển sang phần khác");
    const dialog = await screen.findByRole("dialog", { name: "Chuyển câu 1" });
    await user.click(within(dialog).getByRole("combobox", { name: "Phần" }));
    await user.click(await screen.findByRole("option", { name: "Phần 2" }));
    await user.click(within(dialog).getByRole("button", { name: "Chuyển" }));

    expect(await lastSave()).toEqual([[SECOND], [FIRST]]);
  });

  it("asks before deleting, then removes it from the test and opens the next", async () => {
    const user = await renderBuilder();

    await choose(user, "Xoá câu hỏi");
    const dialog = await screen.findByRole("dialog", {
      name: "Xoá câu hỏi này khỏi đề?",
    });
    expect(
      within(dialog).getByText(
        "Câu hỏi vẫn còn trong ngân hàng câu hỏi nếu được lấy từ đó.",
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));

    expect(await lastSave()).toEqual([[SECOND], []]);
    expect(await screen.findByRole("heading", { name: "Câu 1" })).toBeVisible();
    expect(screen.getByRole("textbox", { name: /Nội dung câu hỏi/ })).toHaveTextContent(
      "Câu thứ hai",
    );
  });
});
