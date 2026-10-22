import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Toaster } from "@/components/ui/sonner";
import { AnswerArea } from "@/features/question-bank/components/AnswerArea";
import {
  blockingIssue,
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import { retype } from "@/features/question-bank/questionType";

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

describe("an essay", () => {
  it("defaults to nine points from the starter's one and back, keeps points the teacher set and says it is graded on a band", () => {
    const essay = retype({ ...emptyQuestion(), prompt: "Viết" }, "essay");
    expect(essay).toMatchObject({ type: "essay", points: 9, rubric: [], options: [] });
    expect(retype({ ...emptyQuestion(), points: 4 }, "essay").points).toBe(4);
    expect(retype(essay, "matching").points).toBe(1);
    expect(retype({ ...essay, points: 18 }, "short_answer").points).toBe(18);
    renderArea(essay);
    expect(screen.getByText("Bạn chấm theo band từ 3 đến 9.")).toBeVisible();
    expect(screen.getByRole("textbox", { name: "Đáp án mẫu" })).toBeInTheDocument();
  });

  it("fills the IELTS Writing preset, reorders, renames and removes criteria, and stops at six", async () => {
    const { user, last } = renderArea({
      ...retype(emptyQuestion(), "essay"),
      prompt: "Viết",
    });
    await user.click(
      screen.getByRole("button", { name: "Dùng tiêu chí IELTS Writing" }),
    );
    expect(last().rubric!.map((criterion) => criterion.label)).toEqual([
      "Hoàn thành yêu cầu đề bài",
      "Mạch lạc và liên kết",
      "Vốn từ vựng",
      "Độ đa dạng và chính xác ngữ pháp",
    ]);
    expect(
      screen.queryByRole("button", { name: "Dùng tiêu chí IELTS Writing" }),
    ).toBeNull();
    expect(screen.getByRole("button", { name: "Đưa tiêu chí 1 lên" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Đưa tiêu chí 1 xuống" }));
    expect(last().rubric![0]!.label).toBe("Mạch lạc và liên kết");
    await user.click(screen.getByRole("button", { name: "Xoá tiêu chí 4" }));
    expect(last().rubric).toHaveLength(3);

    for (let i = 0; i < 3; i++)
      await user.click(screen.getByRole("button", { name: "Thêm tiêu chí" }));
    expect(last().rubric).toHaveLength(6);
    await user.click(screen.getByRole("button", { name: "Thêm tiêu chí" }));
    expect(await screen.findByText("Tối đa 6 tiêu chí")).toBeInTheDocument();
    expect(last().rubric).toHaveLength(6);
    expect(blockingIssue(last(), "x")).toBe("questionEditor.errors.criterionRequired");

    await user.type(
      screen.getByRole("textbox", { name: "Tiêu chí 4" }),
      "vốn TỪ vựng ",
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Hai tiêu chí bị trùng tên.");
  });

  it("marks two criteria the same when one was typed decomposed (QC-R6A-2)", () => {
    renderArea({
      ...retype(emptyQuestion(), "essay"),
      prompt: "Viết",
      rubric: [
        { id: null, label: "Ý tưởng".normalize("NFC") },
        { id: null, label: "Ý tưởng".normalize("NFD") },
      ],
    });
    expect(screen.getByRole("alert")).toHaveTextContent("Hai tiêu chí bị trùng tên.");
    expect(screen.getByRole("textbox", { name: "Tiêu chí 2" })).toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });
});

describe("matching", () => {
  function matching(over: Partial<QuestionValues> = {}): QuestionValues {
    return {
      ...emptyQuestion(),
      type: "matching",
      prompt: "Nối",
      options: [option("cat"), option("dog"), option("bird")],
      matchAllowReuse: false,
      matchItems: [
        { id: null, text: "meow", correctOption: 0 },
        { id: null, text: "woof", correctOption: 1 },
      ],
      ...over,
    };
  }

  it("letters the items, numbers the options in roman, tags an unused option Extra and says how it is marked", () => {
    renderArea(matching());
    expect(screen.getByRole("textbox", { name: "Mục A" })).toHaveValue("meow");
    expect(screen.getByRole("textbox", { name: "Mục B" })).toHaveValue("woof");
    expect(screen.getByRole("textbox", { name: "Lựa chọn iii" })).toHaveValue("bird");
    expect(screen.getAllByText("Thừa")).toHaveLength(1);
    expect(screen.getByText("Thừa")).toHaveAttribute(
      "title",
      "Không nối với mục nào, nên dùng làm phương án gây nhiễu",
    );
    expect(
      screen.getByText(/Chấm tự động khi học viên nộp bài. Mỗi mục được phần điểm/),
    ).toBeVisible();
  });

  it("removing an option unsets the items that used it and renumbers the rest", async () => {
    const { user, last } = renderArea(
      matching({
        matchItems: [
          { id: null, text: "meow", correctOption: 0 },
          { id: null, text: "tweet", correctOption: 2 },
        ],
      }),
    );
    await user.click(screen.getByRole("button", { name: "Xoá lựa chọn i" }));
    expect(last().options.map((entry) => entry.text)).toEqual(["dog", "bird"]);
    expect(last().matchItems!.map((item) => item.correctOption)).toEqual([-1, 1]);
    expect(
      screen.getByRole("combobox", { name: "Lựa chọn đúng cho mục A" }),
    ).toHaveAttribute("aria-invalid", "true");
    expect(blockingIssue(last(), "x")).toBe("questionEditor.errors.matchItemUnset");
  });

  it("picks an item's option from the select and flags one option on two items unless reuse is on", async () => {
    const { user, last } = renderArea(matching());
    await user.click(screen.getByRole("combobox", { name: "Lựa chọn đúng cho mục B" }));
    await user.click(
      within(await screen.findByRole("listbox")).getByRole("option", {
        name: /^i\s*cat$/,
      }),
    );
    expect(last().matchItems![1]!.correctOption).toBe(0);
    expect(screen.getByText(/Mỗi lựa chọn chỉ nối với một mục/)).toBeInTheDocument();
    expect(blockingIssue(last(), "x")).toBe("questionEditor.errors.matchOptionTaken");

    await user.click(
      screen.getByRole("switch", { name: "Mỗi lựa chọn có thể dùng nhiều lần" }),
    );
    expect(last().matchAllowReuse).toBe(true);
    expect(blockingIssue(last(), "x")).toBeNull();
  });

  it("toasts the bounds of 20 items and 12 options", async () => {
    const options = Array.from({ length: 12 }, (_, i) => option(`o${i}`));
    const { user, last } = renderArea(
      matching({
        options,
        matchAllowReuse: true,
        matchItems: Array.from({ length: 20 }, (_, i) => ({
          id: null,
          text: `i${i}`,
          correctOption: i % 12,
        })),
      }),
    );
    await user.click(screen.getByRole("button", { name: "Thêm mục" }));
    expect(await screen.findByText("Tối đa 20 mục")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Thêm lựa chọn" }));
    expect(await screen.findByText("Tối đa 12 lựa chọn")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Lựa chọn xii" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Mục T" })).toBeInTheDocument();
    expect(blockingIssue(last() ?? matching(), "x")).toBeNull();
  });
});
