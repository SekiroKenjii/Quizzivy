import { useState, type ReactElement } from "react";
import { MarkdownProseEditor } from "@/features/question-bank/components/MarkdownProseEditor";

const PROMPT = "What the writer says about parks in paragraph B?";
const EXPLANATION = "";

export const cases: Record<string, () => ReactElement> = {
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
