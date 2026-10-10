import { act } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import type { AdminQuestion } from "@/features/question-bank/api";
import { DraftPreviewDialog } from "@/features/tests/components/DraftPreviewDialog";
import { QuestionPickerDialog } from "@/features/tests/components/QuestionPickerDialog";
import {
  installIntersectionObserver,
  scrollAllIntoView,
} from "@tests/support/intersection";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

function question(n: number, overrides: Partial<AdminQuestion> = {}): AdminQuestion {
  return {
    level: null,
    skill: null,
    id: `018f0000-0000-7000-8000-${String(n).padStart(12, "0")}`,
    type: "short_answer",
    prompt: `Câu số ${n}`,
    media: null,
    audio: null,
    transcript: null,
    options: [],
    blanks: [],
    points: 1,
    explanation: null,
    sampleAnswer: null,
    tags: [],
    usedInTests: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

/** A bank larger than one page and larger than the contract's largest page. */
const BANK = Array.from({ length: 130 }, (_, index) =>
  index === 0
    ? question(1, { type: "single_choice", level: "b1", usedInTests: 2, options: [] })
    : index === 1
      ? question(2, { usedInTests: 1 })
      : question(index + 1),
);
let requests: URLSearchParams[] = [];

beforeEach(() => {
  installIntersectionObserver();
  requests = [];
  server.use(
    http.get(`${BASE}/teacher/questions`, ({ request }) => {
      const params = new URL(request.url).searchParams;
      requests.push(params);
      const q = params.get("q") ?? "";
      const page = Number(params.get("page") ?? "1");
      const limit = Number(params.get("limit") ?? "20");
      const matching = BANK.filter((item) =>
        q === "" ? true : item.prompt === `Câu số ${q.replace(/^\D+/, "")}`,
      );
      return contractJson("/teacher/questions", "get", 200, {
        facets: {
          levels: { pre_a1: 0, a1: 0, a2: 0, b1: 1, b2: 0, c1: 0, c2: 0 },
          skills: {
            grammar: 0,
            vocabulary: 0,
            reading: 0,
            listening: 0,
            writing: 0,
            speaking: 0,
          },
          all: matching.length,
          single_choice: 1,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: Math.max(0, matching.length - 1),
        },
        tags: [],
        bankTotal: BANK.length,
        items: matching.slice((page - 1) * limit, page * limit),
        page,
        pageSize: limit,
        total: matching.length,
      });
    }),
  );
});

describe("Add from question bank", () => {
  function renderPicker(excluded: string[] = []) {
    const onPick = vi.fn();
    const onOpenChange = vi.fn();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <QuestionPickerDialog
          open
          excluded={new Set(excluded)}
          destination="Phần 1"
          onOpenChange={onOpenChange}
          onPick={onPick}
        />
      </QueryClientProvider>,
    );
    return { user: userEvent.setup(), onPick, onOpenChange };
  }

  async function dialog() {
    return screen.findByRole("dialog", { name: "Thêm từ ngân hàng câu hỏi" });
  }

  it("lists the bank with type, level and use, and marks what the test holds", async () => {
    renderPicker([BANK[2]!.id]);
    const picker = await dialog();
    expect(within(picker).getByText("Chọn câu hỏi để thêm vào Phần 1.")).toBeVisible();
    const first = await within(picker).findByRole("checkbox", {
      name: /Câu số 1(?!\d)/,
    });
    expect(first).toHaveTextContent("Một đáp án · B1 · dùng trong 2 đề");
    expect(
      within(picker).getByRole("checkbox", { name: /Câu số 2(?!\d)/ }),
    ).toHaveTextContent("Tự luận · dùng trong 1 đề");
    const held = within(picker).getByRole("checkbox", { name: /Câu số 3(?!\d)/ });
    expect(held).toBeDisabled();
    expect(held).toHaveTextContent("Đã có trong đề");
  });

  it("pages past the first page as the list reaches its end", async () => {
    renderPicker();
    const picker = await dialog();
    await within(picker).findByRole("checkbox", { name: /Câu số 50(?!\d)/ });
    expect(
      within(picker).queryByRole("checkbox", { name: /Câu số 51(?!\d)/ }),
    ).toBeNull();

    act(() => scrollAllIntoView());

    expect(
      await within(picker).findByRole("checkbox", { name: /Câu số 100(?!\d)/ }),
    ).toBeInTheDocument();
    act(() => scrollAllIntoView());
    expect(
      await within(picker).findByRole("checkbox", { name: /Câu số 130(?!\d)/ }),
    ).toBeInTheDocument();
    expect(requests.map((params) => params.get("page"))).toEqual([null, "2", "3"]);
  });

  it("finds question 101 on the server and keeps every tick across searches", async () => {
    const { user, onPick, onOpenChange } = renderPicker();
    const picker = await dialog();
    await user.click(
      await within(picker).findByRole("checkbox", { name: /Câu số 2(?!\d)/ }),
    );

    await user.type(within(picker).getByRole("searchbox"), "101");
    const found = await within(picker).findByRole("checkbox", {
      name: /Câu số 101(?!\d)/,
    });
    expect(requests.at(-1)?.get("q")).toBe("101");
    await user.click(found);

    await user.clear(within(picker).getByRole("searchbox"));
    await user.type(within(picker).getByRole("searchbox"), "120");
    await user.click(
      await within(picker).findByRole("checkbox", { name: /Câu số 120(?!\d)/ }),
    );

    await user.clear(within(picker).getByRole("searchbox"));
    expect(
      await within(picker).findByRole("checkbox", { name: /Câu số 2(?!\d)/ }),
    ).toHaveAttribute("aria-checked", "true");

    await user.click(within(picker).getByRole("button", { name: "Thêm 3 câu hỏi" }));

    expect(onPick).toHaveBeenCalledWith([BANK[1]!.id, BANK[100]!.id, BANK[119]!.id]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("keeps Add disabled until a question is ticked", async () => {
    renderPicker();
    const picker = await dialog();
    await within(picker).findByRole("checkbox", { name: /Câu số 1(?!\d)/ });
    expect(within(picker).getByRole("button", { name: "Thêm câu hỏi" })).toBeDisabled();
  });

  it("says when nothing matches", async () => {
    const { user } = renderPicker();
    const picker = await dialog();
    await user.type(within(picker).getByRole("searchbox"), "999");
    expect(await within(picker).findByText("Không có câu hỏi nào khớp.")).toBeVisible();
  });
});

const PREVIEW = BANK.slice(0, 3);

describe("Student preview", () => {
  it("steps through the questions with Previous and Next, from the one being edited", async () => {
    const scrolled: Element[] = [];
    Element.prototype.scrollIntoView = function (this: Element) {
      scrolled.push(this);
    };
    render(
      <QueryClientProvider client={new QueryClient()}>
        <DraftPreviewDialog
          open
          questions={PREVIEW.map((item) => ({ sectionId: "s1", question: item }))}
          startAt={PREVIEW[1]!.id}
          onOpenChange={vi.fn()}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    const dialog = await screen.findByRole("dialog", { name: "Xem như học viên" });
    expect(
      within(dialog).getByText(
        "Những gì học viên thấy. Câu trả lời ở đây không được lưu.",
      ),
    ).toBeVisible();
    expect(within(dialog).getByRole("status")).toHaveTextContent("Câu 2 / 3");
    await waitFor(() => expect(scrolled.at(-1)).toHaveTextContent("Câu số 2"));

    await user.click(within(dialog).getByRole("button", { name: "Câu tiếp" }));
    expect(within(dialog).getByRole("status")).toHaveTextContent("Câu 3 / 3");
    expect(scrolled.at(-1)).toHaveTextContent("Câu số 3");
    expect(within(dialog).getByRole("button", { name: "Câu tiếp" })).toBeDisabled();

    await user.click(within(dialog).getByRole("button", { name: "Câu trước" }));
    await user.click(within(dialog).getByRole("button", { name: "Câu trước" }));
    expect(within(dialog).getByRole("status")).toHaveTextContent("Câu 1 / 3");
    expect(within(dialog).getByRole("button", { name: "Câu trước" })).toBeDisabled();
  });
});
