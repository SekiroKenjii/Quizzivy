import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/lib/i18n";
import { Toaster } from "@/components/ui/sonner";
import { AnswerArea } from "@/features/question-bank/components/AnswerArea";
import {
  choiceProblems,
  emptyQuestion,
  gapProblems,
  questionSchema,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import {
  readTrueFalse,
  retype,
  typeLocked,
} from "@/features/question-bank/questionType";
import { plainOptionContent } from "@/components/shared/content/optionContent";
import { publishProblem } from "@/features/tests/publishProblem";
import type { AdminQuestion } from "@/features/question-bank/api";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";

type Option = QuestionValues["options"][number];

function option(text: string, isCorrect = false): Option {
  return { id: null, text, isCorrect };
}

function renderArea(start: Partial<QuestionValues>) {
  const seen: QuestionValues[] = [];
  function Harness() {
    const [value, setValue] = useState<QuestionValues>({
      ...emptyQuestion(),
      prompt: "Pick one",
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
  return { user: userEvent.setup(), seen };
}

const GAP_PROMPT: QuestionPromptContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "She ", marks: [] },
        { type: "gap", id: "g1", label: "1" },
        { type: "text", text: " and ", marks: [] },
        { type: "gap", id: "g2", label: "2" },
      ],
    },
  ],
};

describe("the single-choice options", () => {
  it("letters the rows, names each control by its letter and marks the correct one", async () => {
    const { user, seen } = renderArea({
      type: "single_choice",
      options: [option("went", true), option("goes")],
    });

    expect(
      screen.getByRole("group", {
        name: "Lựa chọn · bấm vào ô tròn để đánh dấu đáp án đúng",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Lựa chọn A" })).toHaveValue("went");
    expect(screen.getByRole("textbox", { name: "Lựa chọn B" })).toHaveValue("goes");
    expect(screen.getAllByText("Đáp án đúng")).toHaveLength(1);
    expect(screen.getByText("Chấm tự động khi học viên nộp bài.")).toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: "Đánh dấu B là đáp án đúng" }));
    expect(seen.at(-1)!.options.map((entry) => entry.isCorrect)).toEqual([false, true]);
  });

  it("keeps two options at least and toasts at eight instead of adding a ninth", async () => {
    const { user, seen } = renderArea({
      type: "single_choice",
      options: [option("a", true), option("b")],
    });
    expect(screen.getByRole("button", { name: "Xoá lựa chọn A" })).toBeDisabled();

    for (let added = 0; added < 6; added += 1)
      await user.click(screen.getByRole("button", { name: "Thêm lựa chọn" }));
    expect(screen.getByRole("textbox", { name: "Lựa chọn H" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Xoá lựa chọn A" })).toBeEnabled();
    const before = seen.length;

    await user.click(screen.getByRole("button", { name: "Thêm lựa chọn" }));
    expect(await screen.findByText("Tối đa 8 lựa chọn")).toBeInTheDocument();
    expect(seen).toHaveLength(before);
    expect(screen.queryByRole("textbox", { name: "Lựa chọn I" })).toBeNull();
  });
});

describe("the multiple-choice options", () => {
  it("asks for two ticks with none, and warns, without blocking, with one", async () => {
    const { user } = renderArea({
      type: "multiple_choice",
      options: [option("a"), option("b"), option("c")],
    });
    const group = screen.getByRole("group", {
      name: "Lựa chọn · đánh dấu mọi đáp án đúng",
    });
    expect(group).toHaveAccessibleDescription("Hãy đánh dấu ít nhất hai đáp án đúng.");
    expect(
      screen.getByText(
        "Chấm tự động khi học viên nộp bài. Học viên phải chọn đúng các lựa chọn đã đánh dấu.",
      ),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("checkbox", { name: "Đánh dấu A là đáp án đúng" }),
    );
    expect(group).toHaveAccessibleDescription(
      "Mới có một đáp án được đánh dấu. Hãy đánh dấu thêm, hoặc chuyển sang Một đáp án.",
    );

    await user.click(
      screen.getByRole("checkbox", { name: "Đánh dấu C là đáp án đúng" }),
    );
    expect(group).not.toHaveAttribute("aria-describedby");
  });

  it("saves and publishes a one-tick multiple choice clean while the warning shows", () => {
    const value: QuestionValues = {
      ...emptyQuestion(),
      type: "multiple_choice",
      prompt: "Pick",
      options: [option("a", true), option("b")],
    };
    renderArea(value);
    expect(
      screen.getByText(
        "Mới có một đáp án được đánh dấu. Hãy đánh dấu thêm, hoặc chuyển sang Một đáp án.",
      ),
    ).toBeInTheDocument();

    expect(choiceProblems(value).oneCorrectOfMany).toBe(true);
    expect(questionSchema.safeParse(value).success).toBe(true);
    const stored = {
      id: "018f0000-0000-7000-8000-0000000000a1",
      type: "multiple_choice",
      prompt: "Pick",
      points: 1,
      tags: [],
      level: null,
      skill: null,
      options: [
        { id: "018f0000-0000-7000-8000-0000000000b1", text: "a", isCorrect: true },
        { id: "018f0000-0000-7000-8000-0000000000b2", text: "b", isCorrect: false },
      ],
      createdAt: "2026-10-10T00:00:00Z",
      updatedAt: "2026-10-10T00:00:00Z",
    } as unknown as AdminQuestion;
    expect(publishProblem(stored, i18n.t)).toBeNull();
  });
});

const TRUE_ID = "018f0000-0000-7000-8000-0000000000c1";
const FALSE_ID = "018f0000-0000-7000-8000-0000000000c2";

function stored(
  text: string,
  isCorrect: boolean,
  id: string,
  content?: boolean,
): Option {
  return {
    id,
    text,
    isCorrect,
    ...(content ? { content: plainOptionContent(text) } : {}),
  };
}

describe("the true/false field", () => {
  it("reads a canonical pair in order and keeps both ids through a toggle", async () => {
    const { user, seen } = renderArea({
      type: "true_false",
      options: [stored("True", true, TRUE_ID), stored("False", false, FALSE_ID)],
    });
    expect(screen.getByRole("radio", { name: "Đúng" })).toBeChecked();

    await user.click(screen.getByRole("radio", { name: "Sai" }));
    expect(seen.at(-1)!.options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: false },
      { id: FALSE_ID, text: "False", isCorrect: true },
    ]);
    await user.click(screen.getByRole("radio", { name: "Đúng" }));
    expect(seen.at(-1)!.options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: true },
      { id: FALSE_ID, text: "False", isCorrect: false },
    ]);
  });

  it("reads an imported pair stored False first by its text, not its position", async () => {
    const { user, seen } = renderArea({
      type: "true_false",
      options: [
        stored("False", true, FALSE_ID, true),
        stored("True", false, TRUE_ID, true),
      ],
    });
    expect(screen.getByRole("radio", { name: "Sai" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Đúng" })).not.toBeChecked();

    await user.click(screen.getByRole("radio", { name: "Đúng" }));
    expect(seen.at(-1)!.options).toEqual([
      {
        id: TRUE_ID,
        text: "True",
        isCorrect: true,
        content: plainOptionContent("True"),
      },
      {
        id: FALSE_ID,
        text: "False",
        isCorrect: false,
        content: plainOptionContent("False"),
      },
    ]);
  });

  it("takes the first of a renamed legacy pair as true and normalises the texts", async () => {
    const { user, seen } = renderArea({
      type: "true_false",
      options: [stored("Đúng rồi", true, TRUE_ID), stored("Sai bét", false, FALSE_ID)],
    });
    expect(screen.getByRole("radio", { name: "Đúng" })).toBeChecked();

    await user.click(screen.getByRole("radio", { name: "Sai" }));
    expect(seen.at(-1)!.options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: false },
      { id: FALSE_ID, text: "False", isCorrect: true },
    ]);
    expect(screen.getByRole("radio", { name: "Sai" })).toBeChecked();
  });

  it("reads a reversed legacy pair by its truth words, before falling back to position", async () => {
    const { user, seen } = renderArea({
      type: "true_false",
      options: [stored("Sai", true, FALSE_ID), stored("Đúng", false, TRUE_ID)],
    });
    expect(screen.getByRole("radio", { name: "Sai" })).toBeChecked();

    await user.click(screen.getByRole("radio", { name: "Đúng" }));
    expect(seen.at(-1)!.options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: true },
      { id: FALSE_ID, text: "False", isCorrect: false },
    ]);
  });

  it.each([
    [["No", "Yes"], 1],
    [["không", "Có."], 1],
    [["F", "T"], 1],
    [["sai", "Lựa chọn khác"], 1],
    [["Một", "Hai"], 0],
  ])("finds the true one of %j at index %i", (texts, index) => {
    const options = texts.map((text, at) =>
      stored(text, at === index, at === 0 ? FALSE_ID : TRUE_ID),
    );
    const answer = readTrueFalse(options);
    expect(answer.trueOption).toBe(options[index]);
    expect(answer.trueIsCorrect).toBe(true);
  });

  it("takes the option beside a lone canonical False as true", () => {
    const answer = readTrueFalse([
      stored("False", false, FALSE_ID),
      stored("Đúng", true, TRUE_ID),
    ]);
    expect(answer.trueIsCorrect).toBe(true);
    expect(answer.trueOption?.id).toBe(TRUE_ID);
    expect(answer.falseOption?.id).toBe(FALSE_ID);
  });
});

describe("retype", () => {
  const base: QuestionValues = {
    ...emptyQuestion(),
    prompt: "Keep me",
    points: 3,
    tags: ["a"],
    explanation: "Because",
    options: [option("x"), option("y", true), option("z", true)],
    type: "multiple_choice",
  };

  it("carries options to single choice and keeps only the first tick", () => {
    const next = retype(base, "single_choice");
    expect(next.options.map((entry) => entry.isCorrect)).toEqual([false, true, false]);
    expect(next).toMatchObject({
      prompt: "Keep me",
      points: 3,
      tags: ["a"],
      explanation: "Because",
    });
  });

  it("rebuilds true/false as the two canonical options and drops them for other types", () => {
    expect(retype(base, "true_false").options).toEqual([
      { id: null, text: "True", isCorrect: true },
      { id: null, text: "False", isCorrect: false },
    ]);
    expect(retype(base, "short_answer").options).toEqual([]);
    expect(
      retype({ ...base, sampleAnswer: "m" }, "fill_blank").sampleAnswer,
    ).toBeNull();
    expect(retype(retype(base, "true_false"), "single_choice").options).toEqual([
      { id: null, text: "", isCorrect: true },
      { id: null, text: "", isCorrect: false },
    ]);
  });

  it("keeps a reversed true/false answer and its ids when the type is set again", () => {
    const reversed: QuestionValues = {
      ...base,
      type: "true_false",
      options: [stored("False", true, FALSE_ID), stored("True", false, TRUE_ID)],
    };
    expect(retype(reversed, "true_false").options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: false },
      { id: FALSE_ID, text: "False", isCorrect: true },
    ]);
  });

  it("locks the type while the rich prompt holds a gap", () => {
    expect(
      typeLocked({ type: "fill_blank", promptContent: GAP_PROMPT, blanks: [] }),
    ).toBe(true);
    expect(typeLocked({ type: "fill_blank", promptContent: null, blanks: [] })).toBe(
      false,
    );
  });
});

describe("gapProblems", () => {
  it("names the gaps whose answers say nothing, in prompt order", () => {
    expect(
      gapProblems(GAP_PROMPT, [
        { gapId: "g2", acceptedAnswers: [" "] },
        { gapId: "g1", acceptedAnswers: ["lives"] },
      ]).emptyGapLabels,
    ).toEqual(["2"]);
    expect(gapProblems(GAP_PROMPT, []).emptyGapLabels).toEqual(["1", "2"]);
  });
});

describe("the grading note", () => {
  it("adds the any-accepted rule for fill in the blank", () => {
    renderArea({ type: "fill_blank", prompt: "She {{1}} here.", options: [] });
    const note = screen.getByText(
      "Chấm tự động khi học viên nộp bài. Câu trả lời đúng khi khớp với một đáp án được chấp nhận.",
    );
    expect(within(note.parentElement!).queryByRole("img")).toBeNull();
  });
});
