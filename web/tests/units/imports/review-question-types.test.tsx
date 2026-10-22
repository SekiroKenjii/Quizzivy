import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import type { ImportDraftQuestion } from "@/features/imports/api";
import { option, question, review, section } from "./fixtures";
import { renderReview, serveReview, type ReviewServer } from "./reviewHarness";
import "@/lib/i18n";

let state: ReviewServer;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
});

afterEach(() => {
  vi.useRealTimers();
});

function serve(q: ImportDraftQuestion) {
  serveReview(review([section([q])], []), state);
}

function saved(): ImportDraftQuestion {
  return state.puts.at(-1)!.sections[0]!.items[0]!.question!;
}

async function settle() {
  await vi.advanceTimersByTimeAsync(1500);
  await waitFor(() => expect(state.puts.length).toBeGreaterThan(0));
}

describe("a tfng question in the review", () => {
  beforeEach(() => {
    serve(
      question({
        id: "q1",
        label: "1",
        type: "tfng",
        options: [
          option("o-t", "T", "True"),
          option("o-f", "F", "False"),
          option("o-ng", "NG", "Not given"),
        ],
        answer: { state: "known", optionIds: ["o-ng"], evidence: [] },
      }),
    );
  });

  it("names its type and reads its three fixed options in the teacher's language", async () => {
    await renderReview();
    expect(screen.getByRole("combobox", { name: "Dạng câu" })).toHaveTextContent(
      "Đúng/Sai/Không có thông tin",
    );
    for (const text of ["Đúng", "Sai", "Không có thông tin"])
      expect(screen.getByText(text)).toBeInTheDocument();
    expect(screen.queryByText("Not given")).toBeNull();
    expect(screen.getByRole("radio", { name: /NG/ })).toBeChecked();
    expect(screen.queryByRole("button", { name: /Thêm lựa chọn/ })).toBeNull();
    expect(
      screen.queryByRole("button", { name: /Xoá lựa chọn|Xóa lựa chọn/ }),
    ).toBeNull();
    for (const button of screen.getAllByRole("button", { name: /Sửa lựa chọn/ }))
      expect(button).toBeDisabled();
  });

  it("is offered in the type select", async () => {
    const { user } = await renderReview();
    await user.click(screen.getByRole("combobox", { name: "Dạng câu" }));
    const types = await screen.findByRole("listbox");
    expect(
      within(types).getByRole("option", { name: "Đúng/Sai/Không có thông tin" }),
    ).toBeInTheDocument();
  });
});

describe("a Choose N multiple choice in the review", () => {
  beforeEach(() => {
    serve(
      question({
        id: "q1",
        label: "1",
        type: "multiple_choice",
        selectCount: 2,
        answer: { state: "known", optionIds: ["q1-a"], evidence: [] },
      }),
    );
  });

  it("shows the count, says when the key marks another number, and saves a change", async () => {
    const { user } = await renderReview();
    const count = screen.getByRole("combobox", { name: "Số đáp án cần chọn" });
    expect(count).toHaveTextContent("Chọn 2");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Cần đánh dấu đúng 2 phương án",
    );
    expect(screen.getByText("Chọn đúng 2 đáp án đúng")).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: /B/ }));
    await waitFor(() =>
      expect(screen.queryByText(/Cần đánh dấu đúng 2 phương án/)).toBeNull(),
    );
    await settle();
    expect(saved().selectCount).toBe(2);
    expect(saved().answer.optionIds).toEqual(["q1-a", "q1-b"]);

    await user.click(count);
    const choices = await screen.findByRole("listbox");
    expect(
      within(choices)
        .getAllByRole("option")
        .map((item) => item.textContent),
    ).toEqual(["Không cố định, đúng hết mới có điểm", "Chọn 2", "Chọn 3"]);
    await user.click(
      within(choices).getByRole("option", {
        name: "Không cố định, đúng hết mới có điểm",
      }),
    );
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(saved()).not.toHaveProperty("selectCount"));
  });
});

describe("a short answer's word limit in the review", () => {
  beforeEach(() => {
    serve(
      question({
        id: "q1",
        label: "1",
        type: "short_answer",
        options: [],
        maxWords: 2,
        answer: { state: "known", optionIds: [], evidence: [], text: "in Hue" },
      }),
    );
  });

  it("shows the limit, refuses a value out of bounds, and saves or clears it", async () => {
    const { user } = await renderReview();
    const field = screen.getByLabelText("Giới hạn số từ");
    expect(field).toHaveValue("2");

    await user.clear(field);
    await user.type(field, "101");
    expect(screen.getByRole("alert")).toHaveTextContent("Nhập số nguyên từ 1 đến 100");

    await user.clear(field);
    await user.type(field, "5");
    await settle();
    await waitFor(() => expect(saved().maxWords).toBe(5));

    await user.clear(field);
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(saved()).not.toHaveProperty("maxWords"));
  });
});
