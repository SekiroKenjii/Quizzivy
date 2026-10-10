import { beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import type { components } from "@/lib/api/schema";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const SECOND_ID = "018f0000-0000-7000-8000-0000000000b2";
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
const question: components["schemas"]["AdminQuestion"] = {
  level: null,
  skill: null,
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
let display: ReturnType<typeof viewport>;
let patches: { id: string; body: { points: number; tags: string[] } }[];
beforeEach(() => {
  display = viewport(1024);
  patches = [];
  server.use(
    http.get(`${BASE}/teacher/tests/${TEST_ID}`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, {
        skills: [],
        assignments: { live: 0, scheduled: 0, closed: 0 },
        unpublishedChanges: null,
        id: TEST_ID,
        title: "Unit 5",
        description: null,
        status: "draft",
        currentVersion: 0,
        totalPoints: 14,
        questionCount: 2,
        audioCount: 0,
        sections: [
          {
            id: "018f0000-0000-7000-8000-0000000000c1",
            ordinal: 0,
            title: "Phần 1",
            instructions: null,
            questionIds: [QUESTION_ID, SECOND_ID],
          },
        ],
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      } satisfies components["schemas"]["Test"]),
    ),
    http.get(`${BASE}/teacher/questions/:id`, ({ params }) =>
      contractJson(
        "/teacher/questions/{id}",
        "get",
        200,
        params.id === SECOND_ID
          ? {
              ...question,
              id: SECOND_ID,
              prompt: "Câu hỏi thứ hai độc lập",
              points: 9,
              tags: ["second"],
            }
          : question,
      ),
    ),
    http.patch(`${BASE}/teacher/questions/:id`, async ({ params, request }) => {
      const body = (await request.json()) as { points: number; tags: string[] };
      patches.push({ id: String(params.id), body });
      return contractJson("/teacher/questions/{id}", "patch", 200, {
        ...question,
        id: String(params.id),
        points: body.points,
        tags: body.tags,
      });
    }),
  );
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
  return userEvent.setup();
}

describe("builder settings without a second inline settings column", () => {
  it("keeps standalone settings out of the wide editor and opens exactly one accessible dialog", async () => {
    const user = renderBuilder();
    await user.click(
      await screen.findByRole("button", { name: /tả thói quen/ }, { timeout: 5000 }),
    );
    expect(screen.queryByRole("complementary", { name: "Cài đặt câu hỏi" })).toBeNull();
    const trigger = screen.getByRole("button", { name: "Cài đặt câu hỏi" });
    expect(trigger).toHaveAttribute("aria-label", "Cài đặt câu hỏi");
    await user.click(trigger);
    const dialog = await screen.findByRole("dialog", { name: "Cài đặt câu hỏi" });
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(within(dialog).getByLabelText("Điểm")).toHaveValue(5);
    expect(within(dialog).getByLabelText("Thẻ")).toBeInTheDocument();
    expect(within(dialog).queryByText("Media của câu hỏi")).toBeNull();
    expect(screen.getByText("Media của câu hỏi")).toBeInTheDocument();
  });
  it("returns focus to the actual Settings action after Escape", async () => {
    display.resize(768);
    const user = renderBuilder();
    await user.click(
      await screen.findByRole("button", { name: /tả thói quen/ }, { timeout: 5000 }),
    );
    const trigger = screen.getByRole("button", { name: "Cài đặt câu hỏi" });
    await user.click(trigger);
    await screen.findByRole("dialog", { name: "Cài đặt câu hỏi" });
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(trigger).toHaveFocus();
  });
  it("retains the open tags draft, node, focus and controlled points across viewport changes", async () => {
    const user = renderBuilder();
    await user.click(
      await screen.findByRole("button", { name: /tả thói quen/ }, { timeout: 5000 }),
    );
    await user.click(screen.getByRole("button", { name: "Cài đặt câu hỏi" }));
    const dialog = await screen.findByRole("dialog");
    const points = within(dialog).getByLabelText("Điểm");
    await user.clear(points);
    await user.type(points, "4.5");
    const tags = within(dialog).getByLabelText("Thẻ");
    await user.type(tags, "partial-tag");
    for (const width of [360, 768, 1440, 1024]) {
      act(() => display.resize(width));
      expect(screen.getByRole("dialog")).toBe(dialog);
      expect(within(dialog).getByLabelText("Thẻ")).toBe(tags);
      expect(tags).toHaveValue("partial-tag");
      expect(tags).toHaveFocus();
      expect(points).toHaveValue(4.5);
    }
    await user.keyboard("{Enter}");
    await waitFor(
      () =>
        expect(patches.at(-1)?.body).toMatchObject({
          points: 4.5,
          tags: ["writing", "partial-tag"],
        }),
      { timeout: 4000 },
    );
    expect(patches.at(-1)?.id).toBe(QUESTION_ID);
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Cài đặt câu hỏi" }));
    expect(
      within(await screen.findByRole("dialog")).getByLabelText("Điểm"),
    ).toHaveValue(4.5);
  });
  it("flushes the prior question and never applies its settings to the next selection", async () => {
    const user = renderBuilder();
    await user.click(
      await screen.findByRole("button", { name: /tả thói quen/ }, { timeout: 5000 }),
    );
    await user.click(screen.getByRole("button", { name: "Cài đặt câu hỏi" }));
    const points = within(await screen.findByRole("dialog")).getByLabelText("Điểm");
    await user.clear(points);
    await user.type(points, "7");
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: /Câu hỏi thứ hai độc lập/ }));
    await user.click(screen.getByRole("button", { name: "Cài đặt câu hỏi" }));
    expect(
      within(await screen.findByRole("dialog")).getByLabelText("Điểm"),
    ).toHaveValue(9);
    expect(patches).toEqual([
      expect.objectContaining({
        id: QUESTION_ID,
        body: expect.objectContaining({ points: 7 }),
      }),
    ]);
  });
});
