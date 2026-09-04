import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { CommandPalette } from "@/features/search/CommandPalette";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";

beforeEach(() => {
  server.use(
    http.get(`${BASE}/admin/tests`, () =>
      contractJson("/admin/tests", "get", 200, {
        facets: { all: 1, draft: 0, published: 1, archived: 0 },
        tags: [],
        items: [
          {
            id: TEST_ID,
            title: "Unit 5 — Present perfect",
            description: null,
            status: "published" as const,
            currentVersion: 1,
            totalPoints: 2,
            questionCount: 1,
            audioCount: 0,
            sections: [],
            createdAt: "2026-01-01T00:00:00Z",
            updatedAt: "2026-01-02T00:00:00Z",
          },
        ],
        page: 1,
        pageSize: 5,
        total: 1,
      }),
    ),
    http.get(`${BASE}/admin/questions`, () =>
      contractJson("/admin/questions", "get", 200, {
        facets: {
          all: 1,
          single_choice: 1,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 0,
        },
        tags: [],
        bankTotal: 1,
        items: [
          {
            id: QUESTION_ID,
            type: "single_choice" as const,
            prompt: "Unit 5 · Người phụ nữ đề nghị làm gì?",
            media: null,
            audio: null,
            transcript: null,
            options: [],
            blanks: [],
            points: 2,
            explanation: null,
            sampleAnswer: null,
            tags: ["unit-5"],
            createdAt: "2026-01-01T00:00:00Z",
            updatedAt: "2026-01-01T00:00:00Z",
          },
        ],
        page: 1,
        pageSize: 5,
        total: 1,
      }),
    ),
  );
});

function renderPalette() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin", element: <CommandPalette open onOpenChange={() => {}} /> },
      { path: "/admin/tests/:id", element: <p>trang đề thi</p> },
      { path: "/admin/question-bank/:id", element: <p>trang câu hỏi</p> },
    ],
    { initialEntries: ["/admin"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the command palette, from the keyboard alone", () => {
  it("offers its results as options inside a named listbox", async () => {
    const user = renderPalette();
    await user.type(screen.getByRole("combobox"), "unit");

    expect(
      await screen.findByRole("option", { name: /Present perfect/ }),
    ).toBeInTheDocument();
    const list = screen.getByRole("listbox", { name: "Tìm kiếm và lệnh" });
    expect(within(list).getAllByRole("option")).toHaveLength(2);
    expect(within(list).getByRole("group", { name: "Đề thi" })).toBeInTheDocument();
    expect(within(list).getByRole("group", { name: "Câu hỏi" })).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toHaveAttribute("aria-expanded", "true");
  });

  it("moves aria-activedescendant with the arrows and opens what it names", async () => {
    const user = renderPalette();
    const input = screen.getByRole("combobox");
    await user.type(input, "unit");
    await screen.findByRole("option", { name: /Present perfect/ });

    expect(input).toHaveAttribute(
      "aria-activedescendant",
      screen.getAllByRole("option")[0]?.id,
    );

    await user.keyboard("{ArrowDown}");
    const question = screen.getByRole("option", { name: /Người phụ nữ đề nghị/ });
    expect(input).toHaveAttribute("aria-activedescendant", question.id);
    expect(question).toHaveAttribute("aria-selected", "true");
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "false");

    await user.keyboard("{ArrowDown}{ArrowUp}");
    expect(input).toHaveAttribute("aria-activedescendant", question.id);

    await user.keyboard("{Enter}");
    expect(await screen.findByText("trang câu hỏi")).toBeInTheDocument();
  });
});
