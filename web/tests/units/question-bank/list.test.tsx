import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import QuestionBankPage from "@/features/question-bank/pages/QuestionBankPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

function question(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    id: "018f0000-0000-7000-8000-0000000000b1",
    type: "single_choice" as const,
    prompt: "Người phụ nữ đề nghị làm gì?",
    media: null,
    audio: null,
    transcript: null,
    options: [],
    blanks: [],
    points: 2,
    explanation: null,
    sampleAnswer: null,
    tags: ["unit-5", "listening"],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

let requests: URL[] = [];

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  requests = [];
  server.use(
    http.get(`${BASE}/admin/questions`, ({ request }) => {
      requests.push(new URL(request.url));
      return contractJson("/admin/questions", "get", 200, {
        items: [question()],
        nextCursor: null,
      });
    }),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

function renderBank() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/question-bank", element: <QuestionBankPage /> },
      { path: "/admin/question-bank/new", element: <p>new question page</p> },
    ],
    { initialEntries: ["/admin/question-bank"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
}

describe("the question bank list", () => {
  it("composes a filter and a search into one query", async () => {
    const user = renderBank();
    await screen.findByText("Người phụ nữ đề nghị làm gì?");

    await user.click(screen.getByLabelText("Điền từ"));
    await user.type(screen.getByLabelText(/Tìm trong nội dung/), "nghe");
    await vi.advanceTimersByTimeAsync(300);

    await waitFor(() => {
      const last = requests.at(-1)!;
      expect(last.searchParams.get("type")).toBe("fill_blank");
      expect(last.searchParams.get("q")).toBe("nghe");
    });
  });

  it("debounces the search rather than asking per keystroke", async () => {
    const user = renderBank();
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    const before = requests.length;

    await user.type(screen.getByLabelText(/Tìm trong nội dung/), "nghe");
    expect(requests).toHaveLength(before);

    await vi.advanceTimersByTimeAsync(300);
    await waitFor(() => expect(requests.length).toBe(before + 1));
  });

  it("pages by cursor, never by offset", async () => {
    server.use(
      http.get(`${BASE}/admin/questions`, ({ request }) => {
        const url = new URL(request.url);
        requests.push(url);
        return contractJson("/admin/questions", "get", 200, {
          items: [
            question({
              id: `018f0000-0000-7000-8000-00000000${url.searchParams.get("cursor") ? "0002" : "0001"}`,
            }),
          ],
          nextCursor: url.searchParams.get("cursor") ? null : "cursor-2",
        });
      }),
    );
    const user = renderBank();

    await user.click(await screen.findByRole("button", { name: "Xem thêm" }));

    await waitFor(() => expect(requests).toHaveLength(2));
    expect(requests[1]?.searchParams.get("cursor")).toBe("cursor-2");
    expect(requests.some((url) => url.searchParams.has("offset"))).toBe(false);
    expect(requests.some((url) => url.searchParams.has("page"))).toBe(false);
  });

  it("offers one sentence and one action when nothing matches", async () => {
    server.use(
      http.get(`${BASE}/admin/questions`, () =>
        contractJson("/admin/questions", "get", 200, { items: [], nextCursor: null }),
      ),
    );
    renderBank();

    expect(
      await screen.findByText("Ngân hàng chưa có câu hỏi nào."),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Câu hỏi mới" })).toHaveLength(2);
  });

  it("distinguishes an empty bank from an empty filter", async () => {
    server.use(
      http.get(`${BASE}/admin/questions`, () =>
        contractJson("/admin/questions", "get", 200, { items: [], nextCursor: null }),
      ),
    );
    const user = renderBank();
    await screen.findByText("Ngân hàng chưa có câu hỏi nào.");

    await user.click(screen.getByLabelText("Tự luận"));

    expect(
      await screen.findByText("Không có câu hỏi nào khớp với bộ lọc này."),
    ).toBeInTheDocument();
  });

  it("previews audio in the row rather than in a dialog", async () => {
    server.use(
      http.get(`${BASE}/admin/questions`, () =>
        contractJson("/admin/questions", "get", 200, {
          items: [
            question({
              media: {
                id: "018f0000-0000-7000-8000-0000000000e1",
                kind: "audio",
                url: "https://example.test/a.mp3",
                mimeType: "audio/mpeg",
                bytes: 1024,
                durationMs: 110_000,
                originalFilename: "unit5-listening-2.mp3",
                createdAt: "2026-01-01T00:00:00Z",
              },
            }),
          ],
          nextCursor: null,
        }),
      ),
    );
    const user = renderBank();

    await user.click(await screen.findByRole("button", { name: "Nghe thử" }));

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByLabelText("unit5-listening-2.mp3")).toBeInTheDocument();
    expect(screen.getByText(/1:50 · unit5-listening-2\.mp3/)).toBeInTheDocument();
  });

  it("says so when a preview URL has expired, rather than a player that will not start", async () => {
    let listings = 0;
    server.use(
      http.get(`${BASE}/admin/questions`, () => {
        listings += 1;
        return contractJson("/admin/questions", "get", 200, {
          items: [
            question({
              media: {
                id: "018f0000-0000-7000-8000-0000000000e1",
                kind: "audio",
                url: `https://example.test/a.mp3?sig=${listings}`,
                mimeType: "audio/mpeg",
                bytes: 1024,
                durationMs: 110_000,
                originalFilename: "unit5-listening-2.mp3",
                createdAt: "2026-01-01T00:00:00Z",
              },
            }),
          ],
          nextCursor: null,
        });
      }),
    );
    const user = renderBank();

    await user.click(await screen.findByRole("button", { name: "Nghe thử" }));
    const player = screen.getByLabelText("unit5-listening-2.mp3");

    // The signed URL was minted when the list loaded and lives ten minutes
    // (§11.2); pressing play after that gets a 403 and, without this, silence.
    fireEvent.error(player);

    expect(await screen.findByRole("alert")).toHaveTextContent(/hết hạn/);

    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    await waitFor(() => expect(listings).toBe(2));
    // A fresh URL means a fresh row: the expired state must not stick.
    expect(await screen.findByLabelText("unit5-listening-2.mp3")).toBeInTheDocument();
  });
});
