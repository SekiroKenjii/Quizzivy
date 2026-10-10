import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import type { AdminQuestion } from "@/features/question-bank/api";
import { DraftPreviewDialog } from "@/features/tests/components/DraftPreviewDialog";
import { QuestionPickerDialog } from "@/features/tests/components/QuestionPickerDialog";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

function question(n: number, overrides: Partial<AdminQuestion> = {}): AdminQuestion {
  return {
    level: null,
    skill: null,
    id: `018f0000-0000-7000-8000-00000000000${n}`,
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

const BANK = [
  question(1, { type: "single_choice", level: "b1", usedInTests: 2, options: [] }),
  question(2, { usedInTests: 1 }),
  question(3),
];

beforeEach(() => {
  server.use(
    http.get(`${BASE}/teacher/questions`, () =>
      contractJson("/teacher/questions", "get", 200, {
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
          all: 3,
          single_choice: 1,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 2,
        },
        tags: [],
        bankTotal: 3,
        items: BANK,
        page: 1,
        pageSize: 100,
        total: 3,
      }),
    ),
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

  it("lists the bank less the test's questions, with type, level and use", async () => {
    renderPicker([BANK[2]!.id]);
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm từ ngân hàng câu hỏi",
    });
    expect(within(dialog).getByText("Chọn câu hỏi để thêm vào Phần 1.")).toBeVisible();
    const first = await within(dialog).findByRole("checkbox", { name: /Câu số 1/ });
    expect(first).toHaveTextContent("Một đáp án · B1 · dùng trong 2 đề");
    expect(
      within(dialog).getByRole("checkbox", { name: /Câu số 2/ }),
    ).toHaveTextContent("Tự luận · dùng trong 1 đề");
    expect(within(dialog).queryByRole("checkbox", { name: /Câu số 3/ })).toBeNull();
  });

  it("adds every ticked question, in the order shown", async () => {
    const { user, onPick, onOpenChange } = renderPicker();
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm từ ngân hàng câu hỏi",
    });
    await user.click(await within(dialog).findByRole("checkbox", { name: /Câu số 3/ }));
    await user.click(within(dialog).getByRole("checkbox", { name: /Câu số 1/ }));

    await user.click(within(dialog).getByRole("button", { name: "Thêm 2 câu hỏi" }));

    expect(onPick).toHaveBeenCalledWith([BANK[0]!.id, BANK[2]!.id]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("asks for at least one question", async () => {
    const { user, onPick } = renderPicker();
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm từ ngân hàng câu hỏi",
    });
    await within(dialog).findByRole("checkbox", { name: /Câu số 1/ });

    await user.click(within(dialog).getByRole("button", { name: "Thêm câu hỏi" }));

    expect(await within(dialog).findByText("Chọn ít nhất một câu")).toBeVisible();
    expect(onPick).not.toHaveBeenCalled();
  });
});

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
          questions={BANK.map((item) => ({ sectionId: "s1", question: item }))}
          startAt={BANK[1]!.id}
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
