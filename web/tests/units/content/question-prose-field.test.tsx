import { BlankPromptField } from "@/features/question-bank/components/BlankPromptField";
import { emptyQuestion } from "@/features/question-bank/questionSchema";
import { render, screen } from "@testing-library/react";
import { QuestionProseField } from "@/features/question-bank/components/QuestionProseField";
import { plainOptionContent } from "@/components/shared/content/optionContent";
import "@/lib/i18n";

afterEach(() => vi.unstubAllEnvs());

test("the question pilot flag gates conversion but retains access to stored rich content", () => {
  vi.stubEnv("VITE_RICH_QUESTION_EDITOR", "false");
  const onChange = vi.fn();
  const view = render(
    <QuestionProseField
      text="text"
      id="prompt"
      label="Nội dung câu hỏi"
      prompt
      onChange={onChange}
    />,
  );
  expect(
    screen.queryByRole("button", { name: "Định dạng: Nội dung câu hỏi" }),
  ).not.toBeInTheDocument();
  view.rerender(
    <QuestionProseField
      text="text"
      content={plainOptionContent("text")}
      id="prompt"
      label="Nội dung câu hỏi"
      prompt
      onChange={onChange}
    />,
  );
  expect(
    screen.getByRole("button", { name: "Chỉnh sửa: Nội dung câu hỏi" }),
  ).toBeEnabled();
  expect(onChange).not.toHaveBeenCalled();
});

test("rich blank conversion follows the pilot flag while legacy text remains editable", () => {
  vi.stubEnv("VITE_RICH_QUESTION_EDITOR", "false");
  const value = {
    ...emptyQuestion(),
    type: "fill_blank" as const,
    options: [],
    prompt: "Điền {{1}}",
  };
  const { rerender } = render(<BlankPromptField value={value} onChange={vi.fn()} />);
  expect(
    screen.queryByRole("button", { name: "Định dạng: Nội dung câu hỏi" }),
  ).toBeNull();
  vi.stubEnv("VITE_RICH_QUESTION_EDITOR", "true");
  rerender(<BlankPromptField value={value} onChange={vi.fn()} />);
  expect(
    screen.getByRole("button", { name: "Định dạng: Nội dung câu hỏi" }),
  ).toBeEnabled();
});
