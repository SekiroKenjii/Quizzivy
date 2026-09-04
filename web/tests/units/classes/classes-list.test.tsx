import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ClassesListPage from "@/features/classes/pages/ClassesListPage";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

// Radix's Switch measures itself; jsdom has no ResizeObserver.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const LIVE: components["schemas"]["Class"] = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "Grammar booster",
  description: "Ngữ pháp nền cho học viên mới.",
  studentCount: 18,
  openAssignmentCount: 1,
  selfJoinEnabled: true,
  archivedAt: null,
  joinCode: {
    hint: "P9QR",
    expiresAt: "2099-01-01T00:00:00Z",
    maxUses: null,
    usesCount: 3,
  },
  createdAt: "2026-08-03T03:00:00Z",
};
const ARCHIVED: components["schemas"]["Class"] = {
  id: "018f0000-0000-7000-8000-0000000000c2",
  name: "Summer 2025 — Grammar",
  description: "Khoá hè, kết thúc tháng 8.",
  studentCount: 15,
  openAssignmentCount: 0,
  selfJoinEnabled: true,
  archivedAt: "2026-09-01T00:00:00Z",
  joinCode: null,
  createdAt: "2026-06-09T03:00:00Z",
};

let statuses: (string | null)[] = [];
let posts: unknown[] = [];
let patches: { id: string; body: unknown }[] = [];

function listOf(items: components["schemas"]["Class"][]) {
  return http.get(`${BASE}/admin/classes`, ({ request }) => {
    statuses.push(new URL(request.url).searchParams.get("status"));
    return contractJson("/admin/classes", "get", 200, {
      items,
      page: 1,
      pageSize: 20,
      total: items.length,
      facets: {
        all: items.length,
        joinable: items.filter((c) => c.archivedAt === null && c.joinCode !== null)
          .length,
        archived: items.filter((c) => c.archivedAt !== null).length,
        students: items.reduce((sum, c) => sum + c.studentCount, 0),
      },
    });
  });
}

beforeEach(() => {
  statuses = [];
  posts = [];
  patches = [];
  server.use(
    listOf([LIVE, ARCHIVED]),
    http.post(`${BASE}/admin/classes`, async ({ request }) => {
      posts.push(await request.json());
      return contractJson("/admin/classes", "post", 201, {
        ...LIVE,
        id: "018f0000-0000-7000-8000-0000000000c9",
        name: "Lớp mới",
        studentCount: 0,
        openAssignmentCount: 0,
        joinCode: null,
      });
    }),
    http.patch(`${BASE}/admin/classes/:id`, async ({ request, params }) => {
      const body = (await request.json()) as { archived?: boolean };
      patches.push({ id: String(params["id"]), body });
      return contractJson("/admin/classes/{id}", "patch", 200, {
        ...LIVE,
        archivedAt: body.archived ? "2026-09-04T00:00:00Z" : null,
      });
    }),
  );
});

function renderList() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/classes", element: <ClassesListPage /> },
      { path: "/admin/classes/:id", element: <p>class page</p> },
    ],
    { initialEntries: ["/admin/classes"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

async function openMenu(user: ReturnType<typeof userEvent.setup>, rowName: string) {
  const row = (await screen.findByRole("link", { name: rowName })).closest("tr")!;
  await user.click(within(row).getByRole("button", { name: "Thao tác" }));
  return screen.findByRole("menu");
}

describe("the classes list", () => {
  it("draws G-08's rows: the counts, the join badge and the archived row", async () => {
    renderList();

    expect(await screen.findByText("2 lớp · 33 học viên")).toBeInTheDocument();
    const live = screen.getByRole("link", { name: "Grammar booster" }).closest("tr")!;
    expect(within(live).getByText("Đang mở")).toBeInTheDocument();
    expect(within(live).getByText("••••P9QR")).toBeInTheDocument();
    expect(within(live).getByText("18")).toBeInTheDocument();

    const archived = screen
      .getByRole("link", { name: "Summer 2025 — Grammar" })
      .closest("tr")!;
    expect(within(archived).getByText("Đã lưu trữ")).toBeInTheDocument();
    expect(within(archived).getByText("—")).toBeInTheDocument();
    expect(statuses[0]).toBe("all");
  });

  it("asks the server for the tab it is on", async () => {
    const { user } = renderList();
    await screen.findByText("2 lớp · 33 học viên");

    await user.click(screen.getByRole("tab", { name: /Đã lưu trữ/ }));
    await waitFor(() => expect(statuses).toContain("archived"));
  });

  it("creates a class from two fields and a switch, then opens it", async () => {
    const { user, router } = renderList();
    await user.click(await screen.findByRole("button", { name: "Lớp mới" }));
    const dialog = await screen.findByRole("dialog");

    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));
    expect(await within(dialog).findByText("Hãy đặt tên cho lớp.")).toBeInTheDocument();
    expect(posts).toHaveLength(0);

    await user.type(within(dialog).getByLabelText("Tên lớp"), "  IELTS Foundation  ");
    await user.click(within(dialog).getByRole("switch"));
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));

    await waitFor(() =>
      expect(posts).toEqual([
        { name: "IELTS Foundation", description: null, selfJoinEnabled: true },
      ]),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(
        "/admin/classes/018f0000-0000-7000-8000-0000000000c9",
      ),
    );
  });

  it("archives only after a confirm that repeats what archiving leaves alone", async () => {
    const { user } = renderList();
    const menu = await openMenu(user, "Grammar booster");
    await user.click(within(menu).getByRole("menuitem", { name: "Lưu trữ" }));

    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText("Lưu trữ lớp Grammar booster?"),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("18 học viên vẫn giữ nguyên bài làm và điểm."),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("1 bài đang mở vẫn hiện với học viên cho đến khi đóng."),
    ).toBeInTheDocument();
    expect(patches).toHaveLength(0);

    await user.click(within(dialog).getByRole("button", { name: "Lưu trữ" }));
    await waitFor(() =>
      expect(patches).toEqual([{ id: LIVE.id, body: { archived: true } }]),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("restores from the row without asking", async () => {
    const { user } = renderList();
    const menu = await openMenu(user, "Summer 2025 — Grammar");
    expect(within(menu).queryByRole("menuitem", { name: "Giao bài" })).toBeNull();
    await user.click(within(menu).getByRole("menuitem", { name: "Khôi phục" }));

    await waitFor(() =>
      expect(patches).toEqual([{ id: ARCHIVED.id, body: { archived: false } }]),
    );
  });

  it("shows one sentence and one action when there is nothing yet", async () => {
    server.use(listOf([]));
    renderList();

    expect(await screen.findByText("Chưa có lớp học nào.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Tạo lớp đầu tiên" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Lớp mới" })).toBeNull();
    expect(screen.queryByRole("table")).toBeNull();
  });
});
