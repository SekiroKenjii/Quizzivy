import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import TestsListPage from "@/features/tests/pages/teacher/TestsListPage";
import { Toaster, toast } from "@/components/ui/sonner";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const DRAFT_ID = "018f0000-0000-7000-8000-0000000000a1";
const PUBLISHED_ID = "018f0000-0000-7000-8000-0000000000a2";
const ARCHIVED_ID = "018f0000-0000-7000-8000-0000000000a3";
const COPY_ID = "018f0000-0000-7000-8000-0000000000a4";

type Status = "draft" | "published" | "archived";

function test(id: string, title: string, status: Status, over: object = {}) {
  return {
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: null,
    id,
    title,
    description: null,
    status,
    currentVersion: status === "draft" ? 0 : 3,
    totalPoints: 30,
    questionCount: 24,
    audioCount: 0,
    sections: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...over,
  };
}

const DRAFT = test(DRAFT_ID, "Listening practice 03", "draft", {
  questionCount: 1,
  totalPoints: 12.5,
});
const PUBLISHED = test(PUBLISHED_ID, "Unit 5", "published", {
  skills: ["listening", "reading"],
  assignments: { live: 2, scheduled: 1, closed: 4 },
});
const ARCHIVED = test(ARCHIVED_ID, "Old mock", "archived", {
  assignments: { live: 0, scheduled: 0, closed: 1 },
});

let lists: URLSearchParams[] = [];
let deleted: string[] = [];
let duplicated = false;
let failList = false;

beforeEach(() => {
  lists = [];
  deleted = [];
  duplicated = false;
  failList = false;
  server.use(
    http.get(`${BASE}/teacher/tests`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      lists.push(query);
      if (failList) return new Response(null, { status: 500 });
      const all = [
        DRAFT,
        PUBLISHED,
        ...(deleted.includes(ARCHIVED_ID) ? [] : [ARCHIVED]),
        ...(duplicated ? [test(COPY_ID, "Unit 5 (bản sao)", "draft")] : []),
      ];
      const status = query.get("status");
      const items = all.filter((item) => status === null || item.status === status);
      return contractJson("/teacher/tests", "get", 200, {
        items,
        page: 1,
        pageSize: 24,
        total: items.length,
        facets: {
          all: all.length,
          draft: all.filter((item) => item.status === "draft").length,
          published: all.filter((item) => item.status === "published").length,
          archived: all.filter((item) => item.status === "archived").length,
        },
        tags: [],
      });
    }),
    http.delete(`${BASE}/teacher/tests/:id`, ({ params }) => {
      deleted.push(String(params["id"]));
      return new HttpResponse(null, { status: 204 });
    }),
    http.post(`${BASE}/teacher/tests/:id/duplicate`, () => {
      duplicated = true;
      return contractJson(
        "/teacher/tests/{id}/duplicate",
        "post",
        201,
        test(COPY_ID, "Unit 5 (bản sao)", "draft"),
      );
    }),
  );
});

afterEach(() => {
  toast.dismiss();
});

function renderList(initial = "/teacher/tests") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests", element: <TestsListPage /> }],
    { initialEntries: [initial] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

async function card(title: string) {
  const link = await screen.findByRole("link", { name: title });
  return link.closest("article")!;
}

describe("a card on the tests list", () => {
  it("opens the test's detail whatever its status, as the deck's card does", async () => {
    renderList();

    expect(
      await screen.findByRole("link", { name: "Listening practice 03" }),
    ).toHaveAttribute("href", `/teacher/tests/${DRAFT_ID}`);
    expect(screen.getByRole("link", { name: "Unit 5" })).toHaveAttribute(
      "href",
      `/teacher/tests/${PUBLISHED_ID}`,
    );
    expect(screen.getByRole("link", { name: "Old mock" })).toHaveAttribute(
      "href",
      `/teacher/tests/${ARCHIVED_ID}`,
    );
  });

  it("shows the status, the skills its questions practise and its footer", async () => {
    renderList();
    const published = await card("Unit 5");

    expect(within(published).getByText("Đã phát hành")).toBeInTheDocument();
    expect(
      within(within(published).getByRole("list", { name: "Kỹ năng" }))
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual(["Nghe", "Đọc"]);
    expect(within(published).getByText("24 câu hỏi")).toBeInTheDocument();
    expect(within(published).getByText("30 điểm")).toBeInTheDocument();
    expect(within(published).getByText("2 đang mở")).toBeInTheDocument();
    expect(within(published).getByText(/^Sửa /)).toBeInTheDocument();
  });

  it("draws no skill list for a test whose questions name none, and formats points in the locale", async () => {
    renderList();
    const draft = await card("Listening practice 03");

    expect(within(draft).queryByRole("list")).toBeNull();
    expect(within(draft).getByText("12,5 điểm")).toBeInTheDocument();
    expect(within(draft).getByText("Chưa giao")).toBeInTheDocument();
  });

  it("puts an archived test under All with its pill and its closed assignments", async () => {
    renderList();
    const archived = await card("Old mock");

    expect(within(archived).getByText("Đã lưu trữ")).toBeInTheDocument();
    expect(within(archived).getByText("1 đã đóng")).toBeInTheDocument();
  });

  it("keeps Duplicate and Archive in a draft's menu", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Listening practice 03" });

    await user.click(
      screen.getByRole("button", { name: "Thao tác với Listening practice 03" }),
    );
    const menu = await screen.findByRole("menu");

    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent?.trim()),
    ).toEqual(["Nhân bản", "Lưu trữ"]);
  });

  it("leaves assigning a published test to its detail", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    await user.click(screen.getByRole("button", { name: "Thao tác với Unit 5" }));
    const menu = await screen.findByRole("menu");

    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent?.trim()),
    ).toEqual(["Nhân bản", "Lưu trữ"]);
  });

  it("offers Restore, Duplicate and Delete for an archived test, and deletes after confirming", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Old mock" });

    await user.click(screen.getByRole("button", { name: "Thao tác với Old mock" }));
    const menu = await screen.findByRole("menu");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent?.trim()),
    ).toEqual(["Khôi phục", "Nhân bản", "Xoá vĩnh viễn"]);

    await user.click(within(menu).getByRole("menuitem", { name: "Xoá vĩnh viễn" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Old mock")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Xoá vĩnh viễn" }));

    await waitFor(() => expect(deleted).toEqual([ARCHIVED_ID]));
    await waitFor(() =>
      expect(screen.queryByRole("link", { name: "Old mock" })).toBeNull(),
    );
  });

  it("duplicates beside the menu and marks the source and the copy", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    await user.click(screen.getByRole("button", { name: "Nhân bản Unit 5" }));

    const copy = await card("Unit 5 (bản sao)");
    expect(within(copy).getByText("Vừa nhân bản")).toBeInTheDocument();
    expect(within(await card("Unit 5")).getByText("Vừa nhân bản")).toBeInTheDocument();
    expect(
      within(await card("Listening practice 03")).queryByText("Vừa nhân bản"),
    ).toBeNull();
  });

  it("selects a card for the bulk bar without opening it", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    await user.click(screen.getByRole("checkbox", { name: "Chọn Unit 5" }));

    expect(screen.getByRole("checkbox", { name: "Chọn Unit 5" })).toBeChecked();
    expect(
      screen.getByRole("button", { name: "Lưu trữ các mục đã chọn" }),
    ).toBeInTheDocument();
  });
});

describe("the tests list's tabs and search", () => {
  it("has All, Published and Drafts with their counts, and no Archived tab", async () => {
    renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    const tabs = screen.getByRole("group", { name: "Lọc theo trạng thái" });
    expect(
      within(tabs)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Tất cả 3", "Đã phát hành 1", "Bản nháp 1"]);
    expect(within(tabs).getByRole("button", { name: /^Tất cả/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("asks the server for one status when a tab is chosen", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    await user.click(screen.getByRole("button", { name: /^Bản nháp/ }));

    await waitFor(() =>
      expect(screen.queryByRole("link", { name: "Unit 5" })).toBeNull(),
    );
    expect(lists.at(-1)?.get("status")).toBe("draft");
    expect(screen.getByRole("link", { name: "Listening practice 03" })).toBeVisible();
  });

  it("sends the search to the server", async () => {
    const user = renderList();
    await screen.findByRole("link", { name: "Unit 5" });

    await user.type(screen.getByRole("searchbox", { name: "Tìm theo tên đề" }), "Unit");

    await waitFor(() => expect(lists.at(-1)?.get("q")).toBe("Unit"));
  });

  it("reads an old Archived bookmark as All", async () => {
    renderList("/teacher/tests?status=archived");
    await screen.findByRole("link", { name: "Old mock" });

    expect(lists[0]?.get("status")).toBeNull();
  });
});

describe("the tests list's loading and error states", () => {
  it("shows a skeleton while the first page is out", async () => {
    renderList();

    expect(screen.getByRole("status", { name: "Đang tải…" })).toBeInTheDocument();
    await screen.findByRole("link", { name: "Unit 5" });
    expect(screen.queryByRole("status", { name: "Đang tải…" })).toBeNull();
  });

  it("says the tests could not load, and loads them on retry", async () => {
    failList = true;
    const user = renderList();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không tải được danh sách đề thi.",
    );
    failList = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    expect(await screen.findByRole("link", { name: "Unit 5" })).toBeVisible();
  });
});
