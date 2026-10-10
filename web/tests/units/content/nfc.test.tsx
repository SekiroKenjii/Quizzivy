import { render } from "@testing-library/react";
import { Markdown } from "@/components/shared/Markdown";
import { ContentView } from "@/components/shared/content/ContentView";
import { OptionText } from "@/components/shared/content/OptionText";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { fromEditorJSON } from "@/components/shared/content/editor/adapter";
import { markdownToQuestionContent } from "@/components/shared/content/editor/markdown";
import { plainOptionContent } from "@/components/shared/content/optionContent";
import { contentPlainText } from "@/components/shared/content/plainText";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";
import { nfc } from "@/lib/nfc";
import "@/lib/i18n";

const COMPOSED = "Người phụ nữ đề nghị làm gì? ấ ầ ẩ ẫ ậ ắ ằ ẳ ẵ ặ ơ ư ờ ữ";
const DECOMPOSED = COMPOSED.normalize("NFD");
const UNDRAWN_MARKS = /[̛̂̆]/u;

function semantic(text: string): QuestionPromptContent {
  return {
    format: "semantic_v1",
    blocks: [
      { type: "heading", level: 1, content: [{ type: "text", text, marks: [] }] },
      {
        type: "paragraph",
        content: [{ type: "text", text, marks: ["italic", "bold"] }],
      },
    ],
  } as QuestionPromptContent;
}

test("the fixture is decomposed into marks Be Vietnam Pro's subsets do not draw", () => {
  expect(DECOMPOSED).not.toBe(COMPOSED);
  expect(DECOMPOSED).toMatch(UNDRAWN_MARKS);
  expect(nfc(DECOMPOSED)).toBe(COMPOSED);
});

test("ContentView renders a decomposed row composed", () => {
  const { container } = render(<ContentView document={semantic(DECOMPOSED)} />);
  expect(container.textContent).toBe(COMPOSED + COMPOSED);
  expect(container.textContent).not.toMatch(UNDRAWN_MARKS);
});

test("QuestionProse accepts a decomposed row whose text and content agree, and composes both paths", () => {
  const content = semantic(DECOMPOSED);
  const rich = render(
    <QuestionProse text={contentPlainText(content)} content={content} />,
  );
  expect(rich.container.querySelector("[role=alert]")).toBeNull();
  expect(rich.container.textContent).toBe(COMPOSED + COMPOSED);
  const legacy = render(<QuestionProse text={`**${DECOMPOSED}**`} />);
  expect(legacy.container.textContent).toBe(COMPOSED);
});

test("Markdown renders decomposed source composed", () => {
  const { container } = render(<Markdown>{`*${DECOMPOSED}*`}</Markdown>);
  expect(container.querySelector("em")?.textContent).toBe(COMPOSED);
});

test("OptionText composes a legacy option and a versioned one", () => {
  const legacy = render(<OptionText text={DECOMPOSED} />);
  expect(legacy.container.textContent).toBe(COMPOSED);
  const versioned = render(
    <OptionText text={DECOMPOSED} content={plainOptionContent(DECOMPOSED)} />,
  );
  expect(versioned.container.querySelector("[role=alert]")).toBeNull();
  expect(versioned.container.textContent).toBe(COMPOSED);
});

test("the content editor's output and the Markdown conversion are composed before save", () => {
  const parsed = fromEditorJSON({
    type: "doc",
    content: [{ type: "paragraph", content: [{ type: "text", text: DECOMPOSED }] }],
  });
  expect(parsed.ok && contentPlainText(parsed.value)).toBe(COMPOSED);
  const converted = markdownToQuestionContent(DECOMPOSED);
  expect(converted && contentPlainText(converted)).toBe(COMPOSED);
});
