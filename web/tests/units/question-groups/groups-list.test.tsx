import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";
import { http } from "msw";
import { DeckScale } from "@/components/ui/deck-scale";
import GroupsListPage from "@/features/question-groups/pages/teacher/GroupsListPage";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const API = "http://localhost:8080";

function summary(n: number, archived: boolean) {
  return {
    id: `018f0000-0000-7000-8000-0000000003${archived ? "a" : "0"}${n}`,
    title: `${archived ? "Archived" : "Active"} passage ${n}`,
    revision: 1,
    questionCount: 2,
    recordingCount: 0,
    totalPoints: 2,
    tags: [],
    archivedAt: archived ? "2026-09-20T00:00:00Z" : null,
    updatedAt: "2026-09-20T00:00:00Z",
  };
}

function serve(rows: { active: number; archived: number }) {
  server.use(
    http.get(`${API}/teacher/question-groups`, ({ request }) => {
      const url = new URL(request.url);
      const archived = url.searchParams.get("status") === "archived";
      const q = url.searchParams.get("q") ?? "";
      const stored = archived ? rows.archived : rows.active;
      const count = q === "" ? stored : 0;
      const items = Array.from({ length: count }, (_, i) => summary(i + 1, archived));
      return contractJson("/teacher/question-groups", "get", 200, {
        items,
        page: 1,
        pageSize: 20,
        total: items.length,
      });
    }),
  );
}

function renderList(path = "/teacher/question-bank/groups") {
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/question-bank/groups",
        element: (
          <DeckScale>
            <GroupsListPage />
          </DeckScale>
        ),
      },
      { path: "/teacher/question-bank/groups/:id", element: <p>editor page</p> },
      { path: "/teacher/question-bank", element: <p>bank page</p> },
    ],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, user: userEvent.setup() };
}

describe("the groups list's selection", () => {
  it("clears when the status switches, since Active and Archived offer different actions", async () => {
    serve({ active: 2, archived: 1 });
    const { user } = renderList();
    await user.click(
      await screen.findByRole("checkbox", { name: "Chọn Active passage 1" }),
    );
    await user.click(screen.getByRole("checkbox", { name: "Chọn Active passage 2" }));
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Đã lưu trữ" }));
    expect(await screen.findByText("Archived passage 1")).toBeInTheDocument();
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
    expect(screen.queryByRole("button", { name: /Lưu trữ/ })).toBeNull();
  });

  it("holds across a search", async () => {
    serve({ active: 2, archived: 0 });
    const { user } = renderList();
    await user.click(
      await screen.findByRole("checkbox", { name: "Chọn Active passage 1" }),
    );
    await user.type(screen.getByRole("searchbox"), "zzz");
    expect(await screen.findByText("Chưa có nhóm phù hợp")).toBeInTheDocument();
    expect(screen.getByText("Đã chọn 1")).toBeInTheDocument();
  });
});
