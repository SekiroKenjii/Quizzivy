import { useState, type ReactElement } from "react";
import { MarkdownProseEditor } from "@/features/question-bank/components/MarkdownProseEditor";
import { QuestionProseField } from "@/features/question-bank/components/QuestionProseField";
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

export const cases: Record<string, () => ReactElement> = {
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
