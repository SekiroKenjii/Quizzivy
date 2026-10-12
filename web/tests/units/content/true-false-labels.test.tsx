import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { trueFalseLabelKey } from "@/components/shared/content/trueFalse";
import {
  isPlainOptionContent,
  plainOptionContent,
} from "@/components/shared/content/optionContent";
import { OptionText } from "@/components/shared/content/OptionText";
import { QuestionBody } from "@/features/take-test/components/QuestionBody";
import { AnswerReview } from "@/features/attempts/components/AnswerReview";
import type { StudentQuestion } from "@/features/take-test/api";
import type { AdminQuestion } from "@/features/attempts/api";
import { ReviewItem } from "@/features/results/components/ReviewItem";
import type { ResultQuestion } from "@/features/results/api";
import "@/lib/i18n";

const SHUFFLED: StudentQuestion = {
  id: "q1",
  sectionId: "s1",
  type: "true_false",
  prompt: "The museum opens at nine.",
  points: 1,
  options: [
    { id: "f", text: "False" },
    { id: "t", text: "True" },
  ],
};

describe("trueFalseLabelKey", () => {
  it("translates only the canonical texts of a true/false option", () => {
    expect(trueFalseLabelKey("true_false", "True", null)).toBe("trueFalse.true");
    expect(trueFalseLabelKey("true_false", "False", undefined)).toBe("trueFalse.false");
    expect(trueFalseLabelKey("true_false", "true", null)).toBeNull();
    expect(trueFalseLabelKey("true_false", "Đúng", null)).toBeNull();
    expect(trueFalseLabelKey("single_choice", "True", null)).toBeNull();
    expect(trueFalseLabelKey(undefined, "True", null)).toBeNull();
    expect(
      trueFalseLabelKey("true_false", "True", {
        format: "semantic_v1",
        blocks: [
          { type: "paragraph", content: [{ type: "text", text: "True", marks: [] }] },
        ],
      }),
    ).toBe("trueFalse.true");
    expect(
      trueFalseLabelKey("true_false", "True", {
        format: "semantic_v1",
        blocks: [
          {
            type: "paragraph",
            content: [{ type: "text", text: "True", marks: ["bold"] }],
          },
        ],
      }),
    ).toBeNull();
    expect(
      trueFalseLabelKey("true_false", "False", plainOptionContent("Falsely")),
    ).toBeNull();
  });

  it("isPlainOptionContent accepts the shape plainOptionContent writes, and nothing marked", () => {
    expect(isPlainOptionContent(plainOptionContent("True"), "True")).toBe(true);
    expect(isPlainOptionContent(plainOptionContent("a\nb"), "a\nb")).toBe(true);
    expect(isPlainOptionContent(plainOptionContent("True"), "False")).toBe(false);
    expect(isPlainOptionContent(null, "True")).toBe(false);
    expect(
      isPlainOptionContent(
        {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: [{ type: "text", text: "True", marks: ["italic"] }],
            },
          ],
        },
        "True",
      ),
    ).toBe(false);
  });

  it("leaves a choice option's text alone when it happens to read True", () => {
    render(<OptionText text="True" type="single_choice" />);
    expect(screen.getByText("True")).toBeInTheDocument();
  });
});

describe("the engine's true/false options", () => {
  it("labels each option by its stored text when the deal puts False first", async () => {
    const onAnswer = vi.fn();
    render(<QuestionBody question={SHUFFLED} answer={undefined} onAnswer={onAnswer} />);

    const radios = screen.getAllByRole("radio");
    expect(radios.map((radio) => radio.closest("label")?.textContent)).toEqual([
      "ASai",
      "BĐúng",
    ]);
    await userEvent.click(screen.getByRole("radio", { name: "Đúng" }));
    expect(onAnswer).toHaveBeenCalledWith({ type: "choice", optionIds: ["t"] });
  });

  it("labels Word-imported options that carry plain content, also when shuffled", async () => {
    const onAnswer = vi.fn();
    render(
      <QuestionBody
        question={{
          ...SHUFFLED,
          options: [
            { id: "f", text: "False", content: plainOptionContent("False") },
            { id: "t", text: "True", content: plainOptionContent("True") },
          ],
        }}
        answer={undefined}
        onAnswer={onAnswer}
      />,
    );
    expect(
      screen.getAllByRole("radio").map((radio) => radio.closest("label")?.textContent),
    ).toEqual(["ASai", "BĐúng"]);
    await userEvent.click(screen.getByRole("radio", { name: "Đúng" }));
    expect(onAnswer).toHaveBeenCalledWith({ type: "choice", optionIds: ["t"] });
  });

  it("shows a renamed legacy text as stored", () => {
    render(
      <QuestionBody
        question={{
          ...SHUFFLED,
          options: [
            { id: "t", text: "Đúng rồi" },
            { id: "f", text: "Sai bét" },
          ],
        }}
        answer={undefined}
        onAnswer={vi.fn()}
      />,
    );
    expect(screen.getByRole("radio", { name: "Đúng rồi" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Sai bét" })).toBeInTheDocument();
  });
});

describe("the teacher's review of a true/false answer", () => {
  it("names the options in the reader's language, in the version's order", () => {
    const question = {
      id: "q1",
      type: "true_false",
      prompt: "The museum opens at nine.",
      points: 1,
      tags: [],
      level: null,
      skill: null,
      options: [
        { id: "f", text: "False", isCorrect: true },
        { id: "t", text: "True", isCorrect: false },
      ],
      blanks: [],
      createdAt: "2026-08-20T00:00:00Z",
      updatedAt: "2026-08-20T00:00:00Z",
    } as unknown as AdminQuestion;
    render(
      <AnswerReview
        question={question}
        answer={{ answer: { type: "choice", optionIds: ["t"] } }}
      />,
    );
    const rows = screen
      .getAllByText(/^(Đúng|Sai)$/)
      .map((label) => label.closest("div")!);
    expect(
      rows.map((row) => within(row).getByText(/^(Đúng|Sai)$/).textContent),
    ).toEqual(["Sai", "Đúng"]);
    expect(screen.queryByText("True")).toBeNull();
    expect(screen.queryByText("False")).toBeNull();
  });
});

describe("the student's result for a true/false answer", () => {
  it("names the given and the correct option in the reader's language", () => {
    const question: ResultQuestion = {
      id: "q1",
      sectionId: "s1",
      type: "true_false",
      prompt: "The museum opens at nine.",
      points: 1,
      options: [
        { id: "f", text: "False" },
        { id: "t", text: "True" },
      ],
      answer: { type: "choice", optionIds: ["t"] },
      earned: 0,
      correctOptionIds: ["f"],
    };
    render(
      <ReviewItem
        question={question}
        number={1}
        review={{
          showScore: true,
          showCorrectAnswers: true,
          showExplanations: false,
          release: "on_submit",
          showClassAverage: false,
        }}
        onRetry={vi.fn()}
      />,
    );
    expect(screen.getByText("Đúng")).toBeInTheDocument();
    expect(screen.getAllByText("Sai")).toHaveLength(2);
    expect(screen.queryByText("True")).toBeNull();
    expect(screen.queryByText("False")).toBeNull();
  });
});
