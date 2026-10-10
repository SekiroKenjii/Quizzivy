import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { AnswerReview } from "@/features/attempts/components/AnswerReview";
import type { AdminQuestion } from "@/features/attempts/api";
import "@/lib/i18n";

const QUESTION = {
  type: "true_false",
  prompt: "The museum opens at nine.",
  options: [
    { id: "t", ordinal: 0, text: "True", isCorrect: false },
    { id: "f", ordinal: 1, text: "False", isCorrect: true },
  ],
  blanks: [],
} as unknown as AdminQuestion;

function row(label: string) {
  return screen.getByText(label).closest("div")!;
}

describe("the teacher's review of a true/false answer stored as a boolean (QA-47-2)", () => {
  it("marks False as the student's choice when they answered false", () => {
    render(
      <AnswerReview
        question={QUESTION}
        answer={{ answer: { type: "true_false", value: false } }}
      />,
    );
    expect(within(row("Sai")).getByText("học viên chọn · đúng")).toBeInTheDocument();
    expect(within(row("Đúng")).queryByText(/học viên chọn/)).toBeNull();
  });

  it("marks True as the student's wrong choice when they answered true", () => {
    render(
      <AnswerReview
        question={QUESTION}
        answer={{ answer: { type: "true_false", value: true } }}
      />,
    );
    expect(within(row("Đúng")).getByText("học viên chọn")).toBeInTheDocument();
    expect(within(row("Sai")).getByText("đáp án đúng")).toBeInTheDocument();
  });
});
