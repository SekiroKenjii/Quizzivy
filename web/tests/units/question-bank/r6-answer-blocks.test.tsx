import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Toaster } from "@/components/ui/sonner";
import { AnswerArea } from "@/features/question-bank/components/AnswerArea";
import {
  blockingIssue,
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";

type Option = QuestionValues["options"][number];

const TRUE_ID = "0190a000-0000-7000-8000-000000000001";
const FALSE_ID = "0190a000-0000-7000-8000-000000000002";

function option(text: string, isCorrect = false, id: string | null = null): Option {
  return { id, text, isCorrect };
}

function renderArea(start: Partial<QuestionValues>) {
  const seen: QuestionValues[] = [];
  function Harness() {
    const [value, setValue] = useState<QuestionValues>({
      ...emptyQuestion(),
      prompt: "Đề bài",
      ...start,
    });
    return (
      <>
        <AnswerArea
          value={value}
          onChange={(next) => {
            seen.push(next);
            setValue(next);
          }}
        />
        <Toaster />
      </>
    );
  }
  render(<Harness />);
  return { user: userEvent.setup(), seen, last: () => seen.at(-1)! };
}

describe("True / False with “Include Not given” (DG-113)", () => {
  it("adds a third card and stores tfng, then takes False back when Not given was the answer", async () => {
    const { user, last } = renderArea({
      type: "true_false",
      options: [option("True", true, TRUE_ID), option("False", false, FALSE_ID)],
    });
    expect(screen.getAllByRole("radio")).toHaveLength(2);
    const toggle = screen.getByRole("switch", { name: /Thêm “Không có thông tin”/ });
    expect(toggle).toHaveAttribute("aria-checked", "false");

    await user.click(toggle);
    expect(last().type).toBe("tfng");
    expect(last().options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: true },
      { id: FALSE_ID, text: "False", isCorrect: false },
      { id: null, text: "Not given", isCorrect: false },
    ]);
    expect(screen.getAllByRole("radio")).toHaveLength(3);

    await user.click(screen.getByRole("radio", { name: "Không có thông tin" }));
    expect(last().options.map((entry) => entry.isCorrect)).toEqual([
      false,
      false,
      true,
    ]);
    expect(blockingIssue(last(), "x")).toBeNull();

    await user.click(toggle);
    expect(last().type).toBe("true_false");
    expect(last().options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: false },
      { id: FALSE_ID, text: "False", isCorrect: true },
    ]);
  });

  it("keeps a True answer through the switch", async () => {
    const { user, last } = renderArea({
      type: "tfng",
      options: [option("True", true), option("False"), option("Not given")],
    });
    expect(screen.getByRole("radio", { name: "Đúng" })).toBeChecked();
    await user.click(screen.getByRole("switch", { name: /Không có thông tin/ }));
    expect(last().type).toBe("true_false");
    expect(last().options[0]).toMatchObject({ text: "True", isCorrect: true });
  });
});

describe("“Students choose exactly N” on a multiple choice", () => {
  const four = [option("a", true), option("b", true), option("c"), option("d")];

  it("is off by default, starts at two and asks for exactly N ticks", async () => {
    const { user, last } = renderArea({ type: "multiple_choice", options: four });
    expect(screen.queryByRole("spinbutton")).toBeNull();
    await user.click(screen.getByRole("switch", { name: "Học viên chọn đúng" }));
    expect(last().selectCount).toBe(2);
    expect(
      screen.getByRole("group", { name: /đánh dấu 2 đáp án đúng/ }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Mỗi lựa chọn đúng được phần điểm của mình/)).toBeVisible();

    const count = screen.getByRole("spinbutton", { name: "Số đáp án học viên chọn" });
    await user.clear(count);
    await user.type(count, "3");
    expect(last().selectCount).toBe(3);
    expect(
      screen.getByText("Đánh dấu đúng 3 đáp án đúng, bằng số đáp án học viên chọn."),
    ).toBeVisible();
    expect(blockingIssue(last(), "x")).toBe("questionEditor.errors.selectCountTicks");

    await user.clear(count);
    await user.type(count, "4");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Học viên chọn từ 2 đến 7 đáp án, và ít hơn số phương án.",
    );
    expect(blockingIssue(last(), "x")).not.toBeNull();

    await user.click(screen.getByRole("switch", { name: "Học viên chọn đúng" }));
    expect(last().selectCount).toBeNull();
    expect(blockingIssue(last(), "x")).toBeNull();
  });
});

describe("a short answer's word limit", () => {
  it("is optional, 1 to 100, and empty clears it", async () => {
    const { user, last } = renderArea({ type: "short_answer", options: [] });
    const limit = screen.getByRole("spinbutton", { name: "Giới hạn số từ" });
    expect(limit).toHaveValue(null);
    await user.type(limit, "120");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Giới hạn số từ là số nguyên từ 1 đến 100.",
    );
    expect(blockingIssue(last(), "x")).toBe("questionEditor.errors.maxWordsRange");
    await user.clear(limit);
    await user.type(limit, "50");
    expect(last().maxWords).toBe(50);
    expect(blockingIssue(last(), "x")).toBeNull();
    await user.clear(limit);
    expect(last().maxWords).toBeNull();
  });
});
