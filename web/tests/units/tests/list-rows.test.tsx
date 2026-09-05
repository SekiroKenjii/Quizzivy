import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestsListPage from "@/features/tests/pages/TestsListPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const DRAFT_ID = "018f0000-0000-7000-8000-0000000000a1";
const PUBLISHED_ID = "018f0000-0000-7000-8000-0000000000a2";

function test(id: string, title: string, status: "draft" | "published") {
  return {
    id,
    title,
    description: null,
    status,
    currentVersion: status === "published" ? 3 : 0,
    totalPoints: 30,
    questionCount: 24,
    audioCount: 0,
    sections: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

beforeEach(() => {
  server.use(
    http.get(`${BASE}/admin/tests`, () =>
      contractJson("/admin/tests", "get", 200, {
        items: [
          test(DRAFT_ID, "Listening practice 03", "draft"),
          test(PUBLISHED_ID, "Unit 5", "published"),
        ],
        page: 1,
        pageSize: 50,
        total: 2,
        facets: { all: 2, draft: 1, published: 1, archived: 0 },
        tags: [],
      }),
    ),
  );
});

function renderList() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/tests", element: <TestsListPage /> }],
    {
      initialEntries: ["/admin/tests"],
    },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("a row on the tests list", () => {
  it("opens the right thing on click: a draft the builder, a published test the detail", async () => {
    renderList();

    expect(
      await screen.findByRole("link", { name: "Listening practice 03" }),
    ).toHaveAttribute("href", `/admin/tests/${DRAFT_ID}/edit`);
    expect(screen.getByRole("link", { name: "Unit 5" })).toHaveAttribute(
      "href",
      `/admin/tests/${PUBLISHED_ID}`,
    );
  });

  it("carries A-03's menu on a published row, in the deck's order", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    await user.click(screen.getAllByRole("button", { name: "Thao tác" })[1]!);
    const menu = await screen.findByRole("menu");

    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent?.trim()),
    ).toEqual([
      "Xem như học viên",
      "Giao cho lớp",
      "Nhân bản",
      "Lịch sử phiên bản",
      "Lưu trữ",
    ]);
    expect(
      within(menu).getByRole("menuitem", { name: "Giao cho lớp" }),
    ).toHaveAttribute("href", `/admin/assignments/new?testId=${PUBLISHED_ID}`);
  });

  it("labels the status tabs with the one vocabulary every screen uses", async () => {
    renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    expect(screen.getByRole("tab", { name: /^Bản nháp/ })).toHaveTextContent(/1$/);
    expect(screen.getByRole("tab", { name: /^Đã phát hành/ })).toHaveTextContent(/1$/);
  });
});
