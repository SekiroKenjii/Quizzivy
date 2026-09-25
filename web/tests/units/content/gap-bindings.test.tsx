import { render, screen, fireEvent } from "@testing-library/react";
import { vi } from "vitest";
import { QuestionBody } from "@/features/take-test/components/QuestionBody";
import { questionSchema, emptyQuestion } from "@/features/question-bank/questionSchema";
import {
  bindLegacyBlanks,
  reconcileGapBlanks,
  blankMarkdownProjection,
} from "@/features/question-bank/blankContent";
import { markdownToQuestionContent } from "@/components/shared/content/editor/markdown";
import { contentPlainText } from "@/components/shared/content/plainText";
import { questionGaps } from "@/components/shared/content/gaps";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";
import { BlanksEditor } from "@/features/question-bank/components/BlanksEditor";
import { publishProblem } from "@/features/tests/publishProblem";
import { t } from "i18next";
import "@/lib/i18n";

const content: QuestionPromptContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "table",
      rows: [
        [
          {
            header: false,
            rowSpan: 1,
            colSpan: 1,
            content: [
              { type: "paragraph", content: [{ type: "gap", id: "b", label: "2" }] },
            ],
          },
          {
            header: false,
            rowSpan: 1,
            colSpan: 1,
            content: [
              { type: "paragraph", content: [{ type: "gap", id: "a", label: "1" }] },
            ],
          },
        ],
      ],
    },
  ],
};
const blanks = [
  { id: null, gapId: "a", ordinal: 1, acceptedAnswers: ["one"], caseSensitive: false },
  { id: null, gapId: "b", ordinal: 2, acceptedAnswers: ["two"], caseSensitive: false },
];

test("rich gaps bind student inputs by identity inside tables, including restored answers and disabled submission", () => {
  const onAnswer = vi.fn();
  const question = {
    id: "q",
    sectionId: "s",
    type: "fill_blank" as const,
    prompt: "[2]\t[1]",
    promptContent: content,
    points: 2,
    blanks: blanks.map((blank) => ({
      id: `frozen-${blank.gapId}`,
      gapId: blank.gapId,
      ordinal: blank.ordinal,
      caseSensitive: false,
    })),
  };
  const answer = {
    type: "fill_blank" as const,
    values: { "frozen-a": "one", "frozen-b": "two" },
  };
  const { rerender } = render(
    <QuestionBody question={question} answer={answer} onAnswer={onAnswer} />,
  );
  const inputs = screen.getAllByRole("textbox");
  expect(inputs[0]).toHaveValue("two");
  expect(inputs[1]).toHaveValue("one");
  fireEvent.change(inputs[0]!, { target: { value: "changed" } });
  expect(onAnswer).toHaveBeenCalledWith({
    type: "fill_blank",
    values: { "frozen-a": "one", "frozen-b": "changed" },
  });
  rerender(
    <QuestionBody question={question} answer={answer} onAnswer={onAnswer} disabled />,
  );
  expect(
    screen.getAllByRole("textbox").every((input) => input.hasAttribute("disabled")),
  ).toBe(true);
  rerender(
    <QuestionBody
      question={{ ...question, blanks: question.blanks.slice(1) }}
      answer={answer}
      onAnswer={onAnswer}
    />,
  );
  expect(screen.getByRole("alert")).toBeVisible();
  expect(screen.queryByRole("textbox")).toBeNull();
});

test("form validation requires a complete unique binding set and refuses gaps in other interactions", () => {
  const value = {
    ...emptyQuestion(),
    type: "fill_blank",
    options: [],
    prompt: "[2]\t[1]",
    promptContent: content,
    blanks,
  };
  expect(questionSchema.safeParse(value).success).toBe(true);
  for (const invalid of [
    { ...value, blanks: blanks.slice(1) },
    { ...value, blanks: [blanks[0], { ...blanks[1], gapId: "a" }] },
    { ...value, blanks: [blanks[0], { ...blanks[1], gapId: null }] },
    { ...value, type: "short_answer", blanks: [] },
    { ...value, explanationContent: content, explanation: value.prompt },
  ])
    expect(questionSchema.safeParse(invalid).success).toBe(false);
});

test("the publish outline validates rich bindings without requiring Markdown markers", () => {
  const question = {
    id: "q",
    type: "fill_blank" as const,
    prompt: "[2]\t[1]",
    promptContent: content,
    points: 2,
    tags: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    blanks: blanks.map((blank) => ({ ...blank, id: `row-${blank.gapId}` })),
  };
  expect(publishProblem(question, t)).toBeNull();
  expect(publishProblem({ ...question, blanks: question.blanks.slice(1) }, t)).toBe(
    "Chỗ trống chưa khớp với đề bài.",
  );
});

test("legacy conversion retains answer associations and blocks repeated or missing markers without partial conversion", () => {
  let sequence = 0;
  const converted = bindLegacyBlanks(
    markdownToQuestionContent("**They** {{2}}, she {{1}}.")!,
    blanks,
    () => `g${++sequence}`,
  )!;
  expect(converted.blanks[0]?.gapId).toBe("g2");
  expect(converted.blanks[1]?.gapId).toBe("g1");
  expect(converted.blanks[0]?.acceptedAnswers).toEqual(["one"]);
  expect(
    contentPlainText(blankMarkdownProjection(converted.content, converted.blanks)),
  ).toBe("They {{2}}, she {{1}}.");
  for (const text of [
    "{{1}} {{1}} {{2}}",
    "[{{1}}](https://example.com) {{1}} {{2}}",
    "{{1}}",
    "{{1}} {{3}}",
    "[{{1}}](https://example.com) {{2}}",
  ])
    expect(bindLegacyBlanks(markdownToQuestionContent(text)!, blanks)).toBeNull();
});

test("gap deletion preserves orphaned answers for undo and requires explicit confirmation to discard them", () => {
  const after: QuestionPromptContent = {
    format: "semantic_v1",
    blocks: [{ type: "paragraph", content: [{ type: "gap", id: "b", label: "2" }] }],
  };
  const reconciled = reconcileGapBlanks(after, blanks);
  expect(reconciled).toEqual(blanks);
  expect(reconcileGapBlanks(content, reconciled)).toEqual(blanks);
  const onChange = vi.fn();
  render(
    <BlanksEditor
      prompt="[2]"
      content={after}
      blanks={reconciled}
      onChange={onChange}
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent("Ô này đã được bỏ khỏi nội dung");
  expect(screen.getByRole("button", { name: "Xoá chỗ trống 2" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Xoá chỗ trống 1" }));
  expect(onChange).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Xoá đáp án" }));
  expect(onChange).toHaveBeenCalledWith([blanks[1]]);
});

test("adding a gap keeps old answer keys and appends an empty key with a new ordinal", () => {
  const added: QuestionPromptContent = {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: [{ type: "gap", id: "new", label: "3" }, ...questionGaps(content)],
      },
    ],
  };
  const next = reconcileGapBlanks(added, blanks);
  expect(next.slice(0, 2)).toEqual(blanks);
  expect(next[2]).toMatchObject({ gapId: "new", ordinal: 3, acceptedAnswers: [] });
});
