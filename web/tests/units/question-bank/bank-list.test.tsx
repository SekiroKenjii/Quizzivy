import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import {
  BANK_API,
  bankFacets,
  bankPage,
  bankQuestion,
  renderBank,
} from "@tests/support/questionBank";
import { viewport } from "@tests/support/viewport";
import {
  BANK_ASIDE_WIDTH,
  bankThresholds,
} from "@/features/question-bank/pages/teacher/bankColumns";
import "@/lib/i18n";

let requests: URL[] = [];

function answer(body: ReturnType<typeof bankPage>) {
  server.use(
    http.get(`${BANK_API}/teacher/questions`, ({ request }) => {
      requests.push(new URL(request.url));
      return contractJson("/teacher/questions", "get", 200, body);
    }),
  );
}

beforeEach(() => {
  requests = [];
  answer(
    bankPage(
      [
        bankQuestion(),
        bankQuestion({
          id: "018f0000-0000-7000-8000-0000000000b2",
          type: "fill_blank",
          prompt: "She {{1}} in Hanoi since 2019.",
          level: "a2",
          tags: [],
          usedInTests: 1,
        }),
      ],
      { bankTotal: 1284, total: 2 },
    ),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the question bank", () => {
  it("names itself, counts the bank and links to a new question", async () => {
    renderBank();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Ngân hàng câu hỏi" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("1.284 câu hỏi dùng lại được cho mọi đề thi."),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Câu hỏi mới" })).toHaveAttribute(
      "href",
      "/teacher/question-bank/new",
    );
    expect(screen.queryByRole("button", { name: /Nhập/ })).toBeNull();
  });

  it("draws the deck's columns, a blank as a gap, and a row that opens the editor", async () => {
    const { user, router } = renderBank();
    const table = await screen.findByRole("table", { name: "Ngân hàng câu hỏi" });
    expect(
      within(table)
        .getAllByRole("columnheader")
        .map((cell) => cell.textContent),
    ).toEqual(["", "Câu hỏi", "Loại", "Trình độ", "Đã dùng", "Cập nhật"]);
    expect(within(table).getByText("She ___ in Hanoi since 2019.")).toBeInTheDocument();
    expect(within(table).getByText("A2")).toBeInTheDocument();
    expect(within(table).getByText("3 đề")).toBeInTheDocument();
    expect(within(table).getByText("unit-5")).toBeInTheDocument();
    expect(within(table).queryByRole("button", { name: /Bỏ thẻ/ })).toBeNull();

    await user.click(within(table).getByRole("link", { name: /Người phụ nữ/ }));
    expect(router.state.location.pathname).toBe(
      "/teacher/question-bank/018f0000-0000-7000-8000-0000000000b1",
    );
  });

  it("keeps the deck's columns where they fit beside the aside, and drops those that would overflow", async () => {
    const width = contentWidth(976);
    renderBank();
    const table = await screen.findByRole("table");
    const headers = () =>
      within(table)
        .getAllByRole("columnheader")
        .map((cell) => cell.textContent);
    expect(headers()).toEqual([
      "",
      "Câu hỏi",
      "Loại",
      "Trình độ",
      "Đã dùng",
      "Cập nhật",
    ]);

    width.resize(908);
    expect(headers()).toEqual(["", "Câu hỏi", "Loại", "Trình độ", "Đã dùng"]);

    width.resize(720);
    expect(headers()).toEqual(["", "Câu hỏi", "Trình độ"]);
    expect(within(table).getByText("Điền từ · A2")).toBeInTheDocument();

    width.resize(500);
    expect(headers()).toEqual(["", "Câu hỏi"]);
  });

  it("takes the deck's thresholds where there is no aside", () => {
    expect(bankThresholds(0)).toEqual({
      level: 540,
      type: 640,
      used: 780,
      updated: 900,
    });
    expect(bankThresholds(BANK_ASIDE_WIDTH)).toEqual({
      level: 598,
      type: 750,
      used: 842,
      updated: 944,
    });
  });

  it("composes every filter and the search into one query", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { user } = renderBank("/teacher/question-bank", true);
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    const filters = screen.getByRole("complementary", { name: "Bộ lọc" });

    await user.click(within(filters).getByLabelText(/^Điền từ/));
    await user.click(within(filters).getByLabelText(/^Có audio/));
    await user.click(within(filters).getByLabelText(/^B1/));
    await user.click(within(filters).getByLabelText(/^Nghe/));
    await user.type(screen.getByLabelText(/Tìm trong nội dung/), "nghe");
    await vi.advanceTimersByTimeAsync(300);

    await waitFor(() => {
      const last = requests.at(-1)!;
      expect(last.searchParams.getAll("type")).toEqual(["fill_blank"]);
      expect(last.searchParams.get("hasAudio")).toBe("true");
      expect(last.searchParams.getAll("level")).toEqual(["b1"]);
      expect(last.searchParams.getAll("skill")).toEqual(["listening"]);
      expect(last.searchParams.get("q")).toBe("nghe");
    });
  });

  it("debounces the search rather than asking per keystroke", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { user } = renderBank("/teacher/question-bank", true);
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    const before = requests.length;

    await user.type(screen.getByLabelText(/Tìm trong nội dung/), "nghe");
    expect(requests).toHaveLength(before);

    await vi.advanceTimersByTimeAsync(300);
    await waitFor(() => expect(requests).toHaveLength(before + 1));
  });

  it("reads its filters, page and size from the URL", async () => {
    renderBank(
      "/teacher/question-bank?type=short_answer&level=c1&tag=a&tag=b&tagMatch=all&page=2&size=30",
    );
    await waitFor(() => expect(requests.length).toBeGreaterThan(0));
    const first = requests[0]!;
    expect(first.searchParams.getAll("type")).toEqual(["short_answer"]);
    expect(first.searchParams.getAll("level")).toEqual(["c1"]);
    expect(first.searchParams.getAll("tag")).toEqual(["a", "b"]);
    expect(first.searchParams.get("tagMatch")).toBe("all");
    expect(first.searchParams.get("page")).toBe("2");
    expect(first.searchParams.get("limit")).toBe("30");
  });

  it("counts each filter option from the server, and Audio not at all", async () => {
    answer(
      bankPage([bankQuestion()], {
        facets: bankFacets({
          fill_blank: 305,
          levels: { pre_a1: 0, a1: 12, a2: 0, b1: 488, b2: 0, c1: 0, c2: 0 },
          skills: {
            grammar: 402,
            vocabulary: 0,
            reading: 0,
            listening: 0,
            writing: 0,
            speaking: 0,
          },
        }),
      }),
    );
    renderBank();
    const filters = await screen.findByRole("complementary", { name: "Bộ lọc" });
    await within(filters).findByText("305");
    expect(
      within(filters)
        .getByLabelText(/^Điền từ/)
        .closest("label"),
    ).toHaveTextContent("Điền từ305");
    expect(within(filters).getByLabelText(/^B1/).closest("label")).toHaveTextContent(
      "B1488",
    );
    expect(
      within(filters)
        .getByLabelText(/^Ngữ pháp/)
        .closest("label"),
    ).toHaveTextContent("Ngữ pháp402");
    expect(
      within(filters)
        .getByLabelText(/^Có audio/)
        .closest("label"),
    ).toHaveTextContent(/^Có audio$/);
    expect(
      within(filters)
        .getAllByRole("group")
        .map((group) => group.querySelector("legend")?.textContent),
    ).toEqual(["Loại câu", "Trình độ", "Kỹ năng"]);
  });
});

describe("the question bank's states", () => {
  it("says the bank is empty, and the header still offers a new question", async () => {
    answer(bankPage([], { bankTotal: 0 }));
    renderBank();
    expect(
      await screen.findByText("Ngân hàng chưa có câu hỏi nào."),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Câu hỏi mới" })).toBeInTheDocument();
  });

  it("offers Clear tags when the tags match nothing, and clears only them", async () => {
    answer(bankPage([], { bankTotal: 40, total: 0 }));
    const { user, router } = renderBank(
      "/teacher/question-bank?tag=ielts&type=fill_blank",
    );
    expect(
      await screen.findByText("Không có câu hỏi nào mang các thẻ này."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Bỏ chọn thẻ" }));
    expect(router.state.location.search).toBe("?type=fill_blank");
  });

  it("offers Clear filters when other filters match nothing", async () => {
    answer(bankPage([], { bankTotal: 40, total: 0 }));
    const { user, router } = renderBank("/teacher/question-bank?type=fill_blank&q=x");
    expect(
      await screen.findByText("Không có câu hỏi nào khớp với bộ lọc này."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Xoá bộ lọc" }));
    expect(router.state.location.search).toBe("");
  });

  it("shows a skeleton while it loads", async () => {
    server.use(http.get(`${BANK_API}/teacher/questions`, () => new Promise(() => {})));
    renderBank();
    expect(
      await screen.findByRole("status", { name: "Đang tải…" }),
    ).toBeInTheDocument();
  });

  it("says when it could not load, and retries", async () => {
    let calls = 0;
    server.use(
      http.get(`${BANK_API}/teacher/questions`, () => {
        calls += 1;
        return HttpResponse.json(
          { error: { code: "INTERNAL", message: "boom", requestId: "r1" } },
          { status: 500 },
        );
      }),
    );
    const { user } = renderBank();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không tải được ngân hàng câu hỏi.");
    await user.click(within(alert).getByRole("button", { name: /Thử lại/ }));
    await waitFor(() => expect(calls).toBe(2));
  });

  it("pages through the URL with the deck's pager", async () => {
    answer(bankPage([bankQuestion()], { bankTotal: 45, total: 45 }));
    renderBank();
    const pager = await screen.findByRole("navigation", { name: "Phân trang" });
    expect(pager).toHaveTextContent("1–10 trên 45 câu hỏi");
    expect(
      within(pager).getByRole("link", { name: "Trang sau" }).getAttribute("href"),
    ).toMatch(/\?page=2$/);
  });

  it("opens its filters in a sheet below 1024px", async () => {
    viewport(900);
    const { user } = renderBank("/teacher/question-bank?type=fill_blank&level=b1");
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    expect(screen.queryByRole("complementary", { name: "Bộ lọc" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Bộ lọc, đang dùng 2" }));
    const sheet = await screen.findByRole("dialog", { name: "Bộ lọc" });
    expect(within(sheet).getByLabelText(/^Điền từ/)).toBeChecked();
  });
});
