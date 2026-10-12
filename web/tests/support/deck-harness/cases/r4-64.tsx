import { useState, type ReactElement } from "react";
import { MarkdownProseEditor } from "@/features/question-bank/components/MarkdownProseEditor";
import { QuestionProseField } from "@/features/question-bank/components/QuestionProseField";
import { BlankPromptField } from "@/features/question-bank/components/BlankPromptField";
import {
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import type { QuestionContent } from "@/components/shared/content/questionContent";

const PROMPT = "What the writer says about parks in paragraph B?";
const EXPLANATION = "";
const RICH: QuestionContent = {
  format: "semantic_v1",
  blocks: [{ type: "paragraph", content: [{ type: "text", text: PROMPT, marks: [] }] }],
};

function useProse(text: string, content: QuestionContent | null, explanation = false) {
  const [value, setValue] = useState({ text, content });
  return (
    <QuestionProseField
      id={explanation ? "question-explanation" : "question-prompt"}
      label={explanation ? "Explanation" : "Prompt"}
      prompt={!explanation}
      text={value.text}
      content={value.content}
      onChange={(nextText, nextContent) =>
        setValue({ text: nextText, content: nextContent })
      }
    />
  );
}

const BLANK: QuestionValues = {
  ...emptyQuestion(),
  type: "fill_blank",
  options: [],
  prompt: "Cities that plan parks early see [1] in health.",
  promptContent: {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: [
          { type: "text", text: "Cities that plan parks early see ", marks: [] },
          { type: "gap", id: "gap-1", label: "1" },
          { type: "text", text: " in health.", marks: [] },
        ],
      },
    ],
  },
  blanks: [
    {
      id: null,
      gapId: "gap-1",
      ordinal: 1,
      acceptedAnswers: ["gains"],
      caseSensitive: false,
    },
  ],
};

export const cases: Record<string, () => ReactElement> = {
  "blank-gaps": function BlankGaps() {
    const [value, setValue] = useState(BLANK);
    return <BlankPromptField value={value} onChange={setValue} />;
  },
  "blank-markdown": function BlankMarkdown() {
    const [value, setValue] = useState<QuestionValues>({
      ...BLANK,
      prompt: "Cities that plan parks early see {{1}} in health.",
      promptContent: null,
      blanks: [{ ...BLANK.blanks[0]!, gapId: null }],
    });
    return <BlankPromptField value={value} onChange={setValue} />;
  },
  "prose-rich": function ProseRich() {
    return useProse(PROMPT, RICH);
  },
  "prose-markdown": function ProseMarkdown() {
    return useProse(PROMPT, null);
  },
  "explanation-rich": function ExplanationRich() {
    return useProse(PROMPT, RICH, true);
  },
  "markdown-prompt": function MarkdownPrompt() {
    const [value, setValue] = useState(PROMPT);
    return (
      <MarkdownProseEditor
        id="question-prompt"
        label="Prompt"
        value={value}
        onChange={setValue}
        minHeight={96}
        fontSize={15}
      />
    );
  },
  "markdown-explanation": function MarkdownExplanation() {
    const [value, setValue] = useState(EXPLANATION);
    return (
      <MarkdownProseEditor
        id="question-explanation"
        label="Explanation"
        value={value}
        onChange={setValue}
        minHeight={84}
        fontSize={14}
      />
    );
  },
};
