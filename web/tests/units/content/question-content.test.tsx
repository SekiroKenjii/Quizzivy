import { render, screen } from "@testing-library/react";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { isQuestionContent } from "@/components/shared/content/questionContent";
import {
  markdownToQuestionContent,
  plainTextMarkdown,
} from "@/components/shared/content/editor/markdown";
import { contentPlainText } from "@/components/shared/content/plainText";
import { plainOptionContent } from "@/components/shared/content/optionContent";
import { QuestionBody } from "@/features/take-test/components/QuestionBody";
import { emptyQuestion, questionSchema } from "@/features/question-bank/questionSchema";
import "@/lib/i18n";

test("converts supported Markdown only, retaining Vietnamese, emphasis and safe links", () => {
  const content = markdownToQuestionContent(
    "## Đọc kỹ\n\n**Tiếng Việt** và *ngữ pháp*.\n\n1. [Đọc thêm](https://example.com/reading)\n2. Dòng hai",
  );
  expect(content).not.toBeNull();
  expect(contentPlainText(content!)).toBe(
    "Đọc kỹ\n\nTiếng Việt và ngữ pháp.\n\nĐọc thêm\nDòng hai",
  );
  const { container } = render(
    <QuestionProse text={contentPlainText(content!)} content={content} />,
  );
  expect(container.querySelector("strong")).toHaveTextContent("Tiếng Việt");
  expect(screen.getByRole("link", { name: "Đọc thêm" })).toHaveAttribute(
    "rel",
    "noopener noreferrer",
  );
  expect(container.querySelector("ol")).toBeInTheDocument();
});

test.each([
  "`code`",
  "> quotation",
  "![image](https://example.com/a.png)",
  "[link](http://example.com)",
  "<script>hidden</script>",
  "#### Too deep",
  "[link][ref]\n\n[ref]: https://example.com",
  '[link](https://example.com "title")',
])(
  "refuses unsupported conversion without returning a partial document: %s",
  (text) => {
    expect(markdownToQuestionContent(text)).toBeNull();
  },
);

test("keeps legacy Markdown reading and escapes literal text when formatting is explicitly removed", () => {
  const literal = "<img> **literal** & value &lt;tag&gt;\n2. text";
  const { container, rerender } = render(<QuestionProse text="**Legacy**" />);
  expect(container.querySelector("strong")).toHaveTextContent("Legacy");
  rerender(<QuestionProse text={plainTextMarkdown(literal)} />);
  expect(container).toHaveTextContent("<img> **literal** & value &lt;tag&gt;");
  expect(container.querySelector("img,strong,ol")).toBeNull();
});

test("rejects unbound nodes and mismatched companion text on forms and readers", () => {
  const content = plainOptionContent("Câu hỏi");
  expect(
    isQuestionContent({
      ...content,
      blocks: [
        { type: "paragraph", content: [{ type: "gap", id: "gap1", label: "1" }] },
      ],
    }),
  ).toBe(false);
  expect(isQuestionContent({ ...content, isCorrect: true })).toBe(false);
  const value = {
    ...emptyQuestion(),
    type: "short_answer",
    options: [],
    prompt: "wrong",
    promptContent: content,
  };
  expect(questionSchema.safeParse(value).success).toBe(false);
  expect(questionSchema.safeParse({ ...value, prompt: "Câu hỏi" }).success).toBe(true);
  expect(
    questionSchema.safeParse({ ...value, type: "fill_blank", prompt: "Câu hỏi" })
      .success,
  ).toBe(false);
  render(<QuestionProse text="wrong" content={content} />);
  expect(screen.getByRole("alert")).toBeVisible();
});

test("the live question renderer preserves semantic prompts while the answer remains editable", () => {
  const content = plainOptionContent("Think");
  content.blocks[0]!.content = [{ type: "text", text: "Think", marks: ["underline"] }];
  const { container } = render(
    <QuestionBody
      question={{
        id: "q",
        sectionId: "s",
        type: "short_answer",
        prompt: "Think",
        promptContent: content,
        points: 1,
      }}
      answer={undefined}
      onAnswer={() => undefined}
    />,
  );
  expect(container.querySelector("u")).toHaveTextContent("Think");
  expect(screen.getByRole("textbox")).toBeEnabled();
});

test.each(["promptContent", "explanationContent"])(
  "malformed %s yields a form validation result without throwing",
  (field) => {
    const result = questionSchema.safeParse({
      ...emptyQuestion(),
      prompt: "Câu hỏi",
      explanation: "Lời giải",
      [field]: { format: "semantic_v1", blocks: null },
    });
    expect(result.success).toBe(false);
  },
);
