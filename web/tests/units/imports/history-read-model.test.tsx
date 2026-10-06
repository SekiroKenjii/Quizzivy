import { expect, it } from "vitest";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportsListPage from "@/features/imports/pages/teacher/ImportsListPage";
import { listWordImports } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, wordImport } from "./fixtures";
import "@/lib/i18n";

type History = Awaited<ReturnType<typeof listWordImports>>;
const FACETS = {
  all: 42,
  processing: 19,
  needsReview: 7,
  failed: 0,
  committed: 11,
  cancelled: 5,
};

function response(
  items: History["items"],
  facets = FACETS,
  page = 1,
  total = items.length,
) {
  return contractJson("/teacher/imports", "get", 200, {
    items,
    facets,
    page,
    pageSize: 30,
    total,
  });
}

function mount(entry = "/teacher/imports") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/imports", element: <ImportsListPage /> }],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, user: userEvent.setup() };
}

function filters() {
  return screen.getByRole("group", { name: "Lọc theo trạng thái" });
}

function row(title: string) {
  return screen
    .getByRole("link", { name: title })
    .closest<HTMLElement>('[role="row"]')!;
}

it("shows every response facet including zero without counting only the filtered page", async () => {
  server.use(
    http.get(`${BASE}/teacher/imports`, () =>
      response([{ ...wordImport(), reviewCounts: null }], FACETS, 1, 7),
    ),
  );
  mount("/teacher/imports?status=needs_review&size=30");
  await screen.findByRole("table");
  expect(
    within(filters())
      .getAllByRole("button")
      .map((button) => button.textContent),
  ).toEqual([
    "Tất cả trạng thái 42",
    "Đang xử lý 19",
    "Sẵn sàng rà soát 7",
    "Xử lý không thành công 0",
    "Đã tạo bản nháp 11",
    "Đã huỷ 5",
  ]);
  expect(
    within(filters()).getByRole("button", { name: "Sẵn sàng rà soát 7" }),
  ).toHaveAttribute("aria-pressed", "true");
});

it("distinguishes legacy unknown, computed zero and positive current review findings", async () => {
  const items = [
    {
      ...wordImport({
        id: "018f0000-0000-7000-8000-000000000101",
        title: "Cũ",
        status: "needs_review",
      }),
      reviewCounts: null,
    },
    {
      ...wordImport({
        id: "018f0000-0000-7000-8000-000000000102",
        title: "Đã rà",
        status: "needs_review",
      }),
      reviewCounts: { needsAction: 0, toConfirm: 0 },
    },
    {
      ...wordImport({
        id: "018f0000-0000-7000-8000-000000000103",
        title: "Cần sửa",
        status: "needs_review",
      }),
      reviewCounts: { needsAction: 32768, toConfirm: 2 },
    },
    {
      ...wordImport({
        id: "018f0000-0000-7000-8000-000000000104",
        title: "Cần xác nhận",
        status: "needs_review",
      }),
      reviewCounts: { needsAction: 0, toConfirm: 1 },
    },
  ];
  server.use(http.get(`${BASE}/teacher/imports`, () => response(items)));
  mount();
  await screen.findByRole("table");
  expect(within(row("Cũ")).getByText("Mở bản rà soát")).toBeInTheDocument();
  expect(within(row("Cũ")).queryByText("Không còn mục cần xử lý")).toBeNull();
  expect(within(row("Đã rà")).getByText("Không còn mục cần xử lý")).toBeInTheDocument();
  expect(within(row("Đã rà")).queryByText("0 cần xử lý")).toBeNull();
  expect(within(row("Cần sửa")).getByText("32768 cần xử lý")).toBeInTheDocument();
  expect(within(row("Cần sửa")).getByText("2 cần xác nhận")).toBeInTheDocument();
  expect(within(row("Cần xác nhận")).getByText("1 cần xác nhận")).toBeInTheDocument();
  expect(within(row("Cần xác nhận")).queryByText("0 cần xử lý")).toBeNull();
});

it("uses all four Processing states while resetting page and preserving compact URL search and size", async () => {
  const queries: URLSearchParams[] = [];
  server.use(
    http.get(`${BASE}/teacher/imports`, ({ request }) => {
      queries.push(new URL(request.url).searchParams);
      return response(
        [{ ...wordImport(), reviewCounts: null }],
        FACETS,
        Number(queries.at(-1)?.get("page") ?? 1),
        90,
      );
    }),
  );
  const { router, user } = mount("/teacher/imports?q=hk1&page=3&size=30&keep=a&keep=b");
  await screen.findByRole("table");
  await user.click(within(filters()).getByRole("button", { name: /^Đang xử lý/ }));
  await waitFor(() =>
    expect(queries.at(-1)?.getAll("status")).toEqual([
      "awaiting_sources",
      "queued",
      "processing",
      "committing",
    ]),
  );
  const params = new URLSearchParams(router.state.location.search);
  expect(params.getAll("status")).toEqual(["processing"]);
  expect(params.get("page")).toBeNull();
  expect(params.get("q")).toBe("hk1");
  expect(params.get("size")).toBe("30");
  expect(params.getAll("keep")).toEqual(["a", "b"]);
  expect(queries.at(-1)?.get("page")).toBe("1");
  expect(queries.at(-1)?.get("limit")).toBe("30");
});

it("does not relabel previous-search counts as current and adopts the new search facets", async () => {
  let release = () => {};
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  let searched = false;
  server.use(
    http.get(`${BASE}/teacher/imports`, async ({ request }) => {
      if (new URL(request.url).searchParams.get("q") === "khác") {
        searched = true;
        await gate;
        return response([], {
          all: 0,
          processing: 0,
          needsReview: 0,
          failed: 0,
          committed: 0,
          cancelled: 0,
        });
      }
      return response([{ ...wordImport(), reviewCounts: null }]);
    }),
  );
  const { user } = mount();
  try {
    await screen.findByRole("button", { name: "Tất cả trạng thái 42" });
    await user.type(screen.getByRole("searchbox"), "khác");
    await waitFor(() => expect(searched).toBe(true));
    expect(
      within(filters()).getByRole("button", { name: "Tất cả trạng thái" }),
    ).toBeInTheDocument();
    expect(
      within(filters()).queryByRole("button", { name: "Tất cả trạng thái 42" }),
    ).toBeNull();
    release();
    expect(
      await screen.findByRole("button", { name: "Tất cả trạng thái 0" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Không có lần nhập nào khớp bộ lọc.")).toBeInTheDocument();
  } finally {
    release();
  }
});

it("keeps grouped status search duplicate parameters and size when opening the next page", async () => {
  const queries: URLSearchParams[] = [];
  server.use(
    http.get(`${BASE}/teacher/imports`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      queries.push(query);
      return response(
        [{ ...wordImport(), reviewCounts: null }],
        FACETS,
        Number(query.get("page") ?? 1),
        90,
      );
    }),
  );
  const { router, user } = mount(
    "/teacher/imports?q=hk1&status=processing&size=30&keep=a&keep=b",
  );
  await screen.findByRole("table");
  await user.click(screen.getByRole("link", { name: "Trang 2" }));
  await waitFor(() => expect(queries.at(-1)?.get("page")).toBe("2"));
  expect(queries.at(-1)?.getAll("status")).toEqual([
    "awaiting_sources",
    "queued",
    "processing",
    "committing",
  ]);
  const params = new URLSearchParams(router.state.location.search);
  expect(params.get("q")).toBe("hk1");
  expect(params.get("status")).toBe("processing");
  expect(params.get("size")).toBe("30");
  expect(params.getAll("keep")).toEqual(["a", "b"]);
});
