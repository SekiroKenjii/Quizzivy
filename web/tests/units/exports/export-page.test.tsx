import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import DataExportPage from "@/features/exports/pages/admin/DataExportPage";
import type { components } from "@/lib/api/schema";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { server } from "@tests/support/server";
import "@/lib/i18n";

type Export = components["schemas"]["DataExport"];

const BASE = "http://localhost:8080";
const ID = "019535d9-3df7-79fb-b466-fa907fa17f90";

const QUEUED: Export = {
  id: ID,
  status: "queued",
  includeAnswers: false,
  bytes: null,
  createdAt: "2026-10-19T03:00:00Z",
  completedAt: null,
  expiresAt: null,
  downloadedAt: null,
  downloads: 0,
};

const READY: Export = {
  ...QUEUED,
  status: "ready",
  includeAnswers: true,
  bytes: 3 * 1024 * 1024,
  completedAt: "2026-10-19T03:04:00Z",
  expiresAt: "2026-10-20T03:04:00Z",
};

const failure = (code: string, message: string) => ({
  error: { code, message, requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e" },
});

function serves(answer: (read: number) => Response) {
  const seen = { reads: 0 };
  server.use(
    http.get(`${BASE}/admin/data-exports/:id`, () => {
      seen.reads += 1;
      return answer(seen.reads);
    }),
  );
  return seen;
}

const body = (value: Export) =>
  contractJson("/admin/data-exports/{id}", "get", 200, value);

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/exports/:id", element: <DataExportPage /> }],
    { initialEntries: [`/admin/exports/${ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  contentWidth(1080);
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("the export page", () => {
  it("reads a queued export again every five seconds until it is ready", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const answers = [QUEUED, { ...QUEUED, status: "running" as const }, READY];
    const seen = serves((read) => body(answers[Math.min(read, 3) - 1] ?? READY));
    renderPage();

    expect(
      await screen.findByText("Yêu cầu đang chờ tới lượt. Trang này tự cập nhật."),
    ).toBeVisible();
    expect(seen.reads).toBe(1);

    await act(() => vi.advanceTimersByTimeAsync(4_000));
    expect(seen.reads).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(1_000));
    expect(
      await screen.findByText("Đang tạo tệp. Trang này tự cập nhật."),
    ).toBeVisible();
    expect(seen.reads).toBe(2);

    await act(() => vi.advanceTimersByTimeAsync(5_000));
    expect(await screen.findByRole("button", { name: "Tải xuống" })).toBeVisible();
    expect(seen.reads).toBe(3);

    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(seen.reads).toBe(3);
  });

  it("shows a ready export's size and expiry and downloads it once", async () => {
    const clicked: string[] = [];
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      clicked.push(this.href);
    });
    let current: Export = READY;
    serves(() => body(current));
    server.use(
      http.post(`${BASE}/admin/data-exports/:id/download`, () => {
        current = { ...READY, downloads: 1, downloadedAt: "2026-10-19T04:00:00Z" };
        return contractJson("/admin/data-exports/{id}/download", "post", 200, {
          url: "https://files.example.test/export.zip?sig=1",
          expiresInSeconds: 60,
        });
      }),
    );
    renderPage();
    const user = userEvent.setup({ delay: null });

    expect(await screen.findByText("3 MB")).toBeVisible();
    expect(screen.getByText("Có câu trả lời")).toBeVisible();
    expect(screen.getByText("Hết hạn lúc")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Tải xuống" }));

    expect(
      await screen.findByText(/^Đã tải lúc \d{2}:\d{2}, \d{2}\/\d{2}\/\d{4}$/),
    ).toBeVisible();
    expect(clicked).toEqual(["https://files.example.test/export.zip?sig=1"]);
    expect(screen.queryByRole("button", { name: "Tải xuống" })).toBeNull();
  });

  it("refreshes and says so when the file is no longer available", async () => {
    let current: Export = READY;
    serves(() => body(current));
    server.use(
      http.post(`${BASE}/admin/data-exports/:id/download`, () => {
        current = { ...READY, status: "expired" };
        return contractJson(
          "/admin/data-exports/{id}/download",
          "post",
          410,
          failure("EXPORT_UNAVAILABLE", "Bản xuất không còn nữa."),
        );
      }),
    );
    renderPage();
    const user = userEvent.setup({ delay: null });

    await user.click(await screen.findByRole("button", { name: "Tải xuống" }));

    expect(
      await screen.findByText(
        "Liên kết đã hết hạn. Hãy yêu cầu xuất lại từ Cài đặt hệ thống.",
      ),
    ).toBeVisible();
  });

  it("says a failed export failed, and does not poll it", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const seen = serves(() => body({ ...QUEUED, status: "failed" }));
    renderPage();

    expect(
      await screen.findByText(
        "Không tạo được tệp xuất. Hãy yêu cầu xuất lại từ Cài đặt hệ thống.",
      ),
    ).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(20_000));
    expect(seen.reads).toBe(1);
  });

  it("says an expired export has expired and offers no download", async () => {
    serves(() => body({ ...READY, status: "expired" }));
    renderPage();

    expect(
      await screen.findByText(
        "Liên kết đã hết hạn. Hãy yêu cầu xuất lại từ Cài đặt hệ thống.",
      ),
    ).toBeVisible();
    expect(screen.queryByRole("button", { name: "Tải xuống" })).toBeNull();
  });

  it("says another user's export was not found", async () => {
    serves(() =>
      contractJson(
        "/admin/data-exports/{id}",
        "get",
        404,
        failure("NOT_FOUND", "Không tìm thấy."),
      ),
    );
    renderPage();

    expect(await screen.findByText("Không tìm thấy bản xuất này")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Tải xuống" })).toBeNull();
  });

  it("offers a retry when the export cannot be read", async () => {
    const seen = serves((read) =>
      read === 1
        ? contractJson(
            "/admin/data-exports/{id}",
            "get",
            500,
            failure("INTERNAL", "Lỗi máy chủ."),
          )
        : body(READY),
    );
    renderPage();
    const user = userEvent.setup({ delay: null });

    await user.click(await screen.findByRole("button", { name: "Thử lại" }));

    await waitFor(() => expect(seen.reads).toBe(2));
    expect(await screen.findByRole("button", { name: "Tải xuống" })).toBeVisible();
  });
});
