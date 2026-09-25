import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { focusManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportDetailPage from "@/features/imports/pages/ImportDetailPage";
import type { WordImport } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, IMPORT_ID, run, wordImport } from "./fixtures";
import "@/lib/i18n";

let reads = 0;
let states: WordImport[] = [];
let cancels: { expectedRevision: number }[] = [];

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  reads = 0;
  cancels = [];
  states = [wordImport({ status: "processing", run: run({ stage: "extraction" }) })];
  server.use(
    http.get(`${BASE}/admin/imports/:id`, () => {
      const next = states[Math.min(reads, states.length - 1)]!;
      reads += 1;
      return contractJson("/admin/imports/{id}", "get", 200, next);
    }),
    http.post(`${BASE}/admin/imports/:id/cancel`, async ({ request }) => {
      cancels.push((await request.json()) as { expectedRevision: number });
      return contractJson(
        "/admin/imports/{id}/cancel",
        "post",
        200,
        wordImport({ status: "cancelled", revision: 6 }),
      );
    }),
  );
});

afterEach(() => {
  focusManager.setFocused(true);
  vi.useRealTimers();
});

async function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/imports/:id", element: <ImportDetailPage /> },
      { path: "/admin/imports/:id/review", element: <p>review page</p> },
      { path: "/admin/imports", element: <p>history</p> },
    ],
    { initialEntries: [`/admin/imports/${IMPORT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByRole("list", { name: "Các bước xử lý" });
  return userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
}

describe("the processing screen", () => {
  it("shows the real stage and polls every two seconds while the tab is visible", async () => {
    await renderDetail();
    const steps = screen.getByRole("list", { name: "Các bước xử lý" });
    expect(within(steps).getByText("Đọc nội dung").closest("li")).toHaveAttribute(
      "aria-current",
      "step",
    );
    expect(screen.queryByText(/%/), "no invented percentage").not.toBeInTheDocument();
    expect(reads).toBe(1);

    await act(() => vi.advanceTimersByTimeAsync(2000));
    expect(reads).toBe(2);
    await act(() => vi.advanceTimersByTimeAsync(2000));
    expect(reads).toBe(3);
  });

  it("stops polling while the tab is hidden and resumes when it is shown", async () => {
    await renderDetail();
    act(() => focusManager.setFocused(false));
    await act(() => vi.advanceTimersByTimeAsync(8000));
    expect(reads, "a hidden tab does not poll").toBe(1);

    act(() => focusManager.setFocused(true));
    await act(() => vi.advanceTimersByTimeAsync(2000));
    expect(reads).toBeGreaterThan(1);
  });

  it("stops polling once the import reaches a terminal state", async () => {
    states = [
      wordImport({ status: "processing", run: run({ stage: "recognition" }) }),
      wordImport({
        status: "needs_review",
        run: run({ status: "succeeded", stage: "ready" }),
      }),
    ];
    await renderDetail();
    await act(() => vi.advanceTimersByTimeAsync(2000));
    expect(
      await screen.findByText("Sẵn sàng rà soát", { selector: "div" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Tiếp tục rà soát" })).toHaveAttribute(
      "href",
      `/admin/imports/${IMPORT_ID}/review`,
    );
    const settled = reads;
    await act(() => vi.advanceTimersByTimeAsync(10000));
    expect(reads, "no polling after needs_review").toBe(settled);
  });

  it("says plainly that cancelling a first run closes the import", async () => {
    const user = await renderDetail();
    await user.click(screen.getByRole("button", { name: "Huỷ xử lý" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/lần nhập sẽ được đóng/)).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("button", { name: "Huỷ và đóng lần nhập" }),
    );

    expect(await screen.findByText("Lần nhập đã được đóng")).toBeInTheDocument();
    expect(cancels).toEqual([{ expectedRevision: 4 }]);
  });

  it("offers to stop a reprocess while keeping the review, and says it did not finish", async () => {
    states = [
      wordImport({ status: "processing", draftRevision: 3, run: run({ keyPaper: 2 }) }),
    ];
    server.use(
      http.post(`${BASE}/admin/imports/:id/cancel`, async ({ request }) => {
        cancels.push((await request.json()) as { expectedRevision: number });
        return contractJson(
          "/admin/imports/{id}/cancel",
          "post",
          200,
          wordImport({
            status: "needs_review",
            revision: 6,
            draftRevision: 3,
            run: run({ status: "cancelled", keyPaper: 2 }),
          }),
        );
      }),
    );
    const user = await renderDetail();
    expect(
      screen.getByRole("link", { name: "Xem bản rà soát hiện tại" }),
    ).toHaveAttribute("href", `/admin/imports/${IMPORT_ID}/review`);
    expect(screen.queryByRole("button", { name: "Huỷ xử lý" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Dừng xử lý lại" }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/mọi chỉnh sửa của bạn được giữ nguyên/),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Dừng xử lý lại" }));

    expect(
      await screen.findByText(/Lần xử lý lại gần nhất đã được dừng/, {
        selector: "p:not([aria-live])",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Tiếp tục rà soát" })).toBeInTheDocument();
  });

  it("shows a run waiting to retry without a current stage", async () => {
    states = [
      wordImport({
        status: "queued",
        run: run({
          status: "queued",
          stage: "extraction",
          errorCode: "CONVERSION_BUSY",
        }),
      }),
    ];
    await renderDetail();
    expect(
      screen.getByText("Đang chờ thử lại tự động. Bạn không cần làm gì."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/thử lại sau/)).toBeNull();
    expect(screen.getByText("Lần thử 1/3")).toBeInTheDocument();
    const steps = screen.getByRole("list", { name: "Các bước xử lý" });
    expect(
      within(steps)
        .queryAllByRole("listitem")
        .some((item) => item.getAttribute("aria-current") === "step"),
    ).toBe(false);
    expect(screen.queryByText(/Đã chạy/)).toBeNull();
  });

  it("hides the attempt count while a released first attempt waits", async () => {
    states = [
      wordImport({
        status: "queued",
        run: run({
          status: "queued",
          stage: "queued",
          attempt: 0,
          errorCode: "WORKER_INTERRUPTED",
        }),
      }),
    ];
    await renderDetail();
    expect(
      screen.getByText("Đang chờ thử lại tự động. Bạn không cần làm gì."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Lần thử/)).toBeNull();
  });

  it("keeps the last known state on screen when a background poll fails", async () => {
    let fail = false;
    server.use(
      http.get(`${BASE}/admin/imports/:id`, () => {
        reads += 1;
        if (fail) return new Response(null, { status: 503 });
        return contractJson(
          "/admin/imports/{id}",
          "get",
          200,
          wordImport({ status: "processing", run: run() }),
        );
      }),
    );
    await renderDetail();
    fail = true;
    await act(() => vi.advanceTimersByTimeAsync(10000));
    expect(
      await screen.findByText(/Không cập nhật được thông tin mới nhất/),
    ).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Các bước xử lý" })).toBeInTheDocument();
    expect(screen.queryByText("Không tải được lần nhập này.")).toBeNull();
  });
});
