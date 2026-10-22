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
      "Đúng / Sai / Không có thông tin",
    );
    for (const text of ["Đúng", "Sai", "Không có thông tin"])
      expect(screen.getByText(text)).toBeInTheDocument();
    expect(screen.queryByText("Not given")).toBeNull();
    expect(screen.getByRole("radio", { name: /C/ })).toBeChecked();
    expect(
      screen.getAllByText("Đúng / Sai / Không có thông tin").length,
    ).toBeGreaterThan(0);
    for (const key of ["A", "B", "C"])
      expect(screen.getByText(key)).toBeInTheDocument();
    expect(screen.queryByText("NG")).toBeNull();
    expect(screen.queryByRole("button", { name: /Thêm lựa chọn/ })).toBeNull();
    expect(
      screen.queryByRole("button", { name: /Xoá lựa chọn|Xóa lựa chọn/ }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: /Sửa lựa chọn/ })).toBeNull();
  });

  it("is offered in the type select", async () => {
    const { user } = await renderReview();
    await user.click(screen.getByRole("combobox", { name: "Dạng câu" }));
    const types = await screen.findByRole("listbox");
    expect(
      within(types).getByRole("option", { name: "Đúng / Sai / Không có thông tin" }),
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
    expect(screen.getByText("Đánh dấu đúng 2 đáp án")).toBeInTheDocument();

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

describe("a Choose N count the options cannot hold", () => {
  it("says so at the field, and a removal that leaves too few options resets it", async () => {
    serve(
      question({
        id: "q1",
        label: "1",
        type: "multiple_choice",
        selectCount: 3,
        answer: { state: "known", optionIds: ["q1-a", "q1-b", "q1-c"], evidence: [] },
      }),
    );
    const { user } = await renderReview();
    const count = screen.getByRole("combobox", { name: "Số đáp án cần chọn" });
    expect(count).toHaveTextContent("Chọn 3");
    expect(screen.queryByRole("alert")).toBeNull();

    await user.click(
      screen.getByRole("button", { name: /Xoá lựa chọn D|Xóa lựa chọn D/ }),
    );
    await waitFor(() => expect(count).toHaveTextContent("Không cố định"));
    expect(
      screen.getByText(/^Đã đặt lại Chọn 3/, { selector: '[role="status"]' }),
    ).toHaveTextContent(
      'Đã đặt lại Chọn 3 thành "Không cố định" vì chỉ còn 3 phương án.',
    );
    await settle();
    await waitFor(() => expect(saved()).not.toHaveProperty("selectCount"));
  });

  it("explains an imported count that is too large instead of asking to mark every option", async () => {
    serve(
      question({
        id: "q1",
        label: "1",
        type: "multiple_choice",
        selectCount: 2,
        options: [option("q1-a", "A", "Hanoi"), option("q1-b", "B", "Hue")],
        answer: { state: "known", optionIds: ["q1-a"], evidence: [] },
      }),
    );
    await renderReview();
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Chọn 2 cần ít nhất 3 phương án");
    expect(alert).not.toHaveTextContent("Cần đánh dấu đúng");
  });
});

describe("a type change that needs confirming, made from the keyboard", () => {
  for (const [name, key] of [
    ["Enter", "{Enter}"],
    ["Space", " "],
  ] as const)
    it(`keeps the dialog open when the type is picked with ${name}`, async () => {
      serve(
        question({
          id: "q1",
          label: "1",
          type: "multiple_choice",
          selectCount: 2,
          answer: { state: "known", optionIds: ["q1-a", "q1-b"], evidence: [] },
        }),
      );
      const { user } = await renderReview();
      screen.getByRole("combobox", { name: "Dạng câu" }).focus();
      await user.keyboard("{ArrowDown}");
      const types = await screen.findByRole("listbox");
      within(types).getByRole("option", { name: "Một đáp án" }).focus();
      await user.keyboard(key);
      await vi.advanceTimersByTimeAsync(500);
      expect(screen.getByRole("dialog")).toBeInTheDocument();
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
