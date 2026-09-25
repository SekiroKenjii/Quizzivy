import { render, screen } from "@testing-library/react";
import { OptionText } from "@/components/shared/content/OptionText";
import {
  isOptionContent,
  plainOptionContent,
} from "@/components/shared/content/optionContent";
import { contentPlainText } from "@/components/shared/content/plainText";
import { emptyQuestion, questionSchema } from "@/features/question-bank/questionSchema";
import { QuestionBody } from "@/features/take-test/components/QuestionBody";
import "@/lib/i18n";

test("preserves literal legacy text, Unicode and line breaks during conversion", () => {
  const text = "<img onerror=alert(1)> **nghề**\nH₂O";
  const content = plainOptionContent(text);
  expect(isOptionContent(content)).toBe(true);
  expect(contentPlainText(content)).toBe(text);
  const { container } = render(<OptionText text={text} />);
  expect(container.textContent).toBe(text);
  expect(container.querySelector("img,strong")).toBeNull();
});

test("rejects keys, unsupported nodes and mismatched projections", () => {
  const content = plainOptionContent("think");
  expect(isOptionContent({ ...content, isCorrect: true })).toBe(false);
  expect(
    isOptionContent({
      ...content,
      blocks: [{ type: "paragraph", content: [{ type: "gap", id: "1", label: "1" }] }],
    }),
  ).toBe(false);
  const question = {
    ...emptyQuestion(),
    prompt: "Select",
    options: [
      { id: null, text: "different", content, isCorrect: true },
      { id: null, text: "other", isCorrect: false },
    ],
  };
  expect(questionSchema.safeParse(question).success).toBe(false);
  render(<OptionText text="different" content={content} />);
  expect(screen.getByRole("alert")).toBeVisible();
});

test("renders marked sounds inside an accessible learner choice", () => {
  const content = plainOptionContent("think");
  content.blocks[0]!.content = [
    { type: "text", text: "th", marks: ["underline"] },
    { type: "text", text: "ink", marks: [] },
  ];
  const { container } = render(
    <QuestionBody
      question={{
        id: "question",
        sectionId: "section",
        type: "single_choice",
        prompt: "Choose",
        points: 1,
        options: [{ id: "option", text: "think", content }],
      }}
      answer={undefined}
      onAnswer={() => undefined}
    />,
  );
  expect(container.querySelector("u")).toHaveTextContent("th");
  expect(screen.getByRole("radio", { name: "think" })).toBeEnabled();
});
