import { useEffect, useState } from "react";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { contentPlainText } from "@/components/shared/content/plainText";
import {
  isQuestionContent,
  type QuestionContent,
  type QuestionPromptContent,
} from "@/components/shared/content/questionContent";
import { SwitchToMarkdown } from "./ProseMode";

const EMPTY: QuestionContent = {
  format: "semantic_v1",
  blocks: [{ type: "paragraph", content: [] }],
};

/**
 * RichProseEditor is a prompt's or an explanation's rich body, always live.
 * It starts from the stored content, or from an empty paragraph when the
 * field has none, and reports each valid document with its plain text.
 * While `leaving` it asks "Switch to Markdown" under its toolbar. A document
 * whose text is exactly `starter` opens empty, showing `placeholder`, and the
 * starter stays stored until the first edit.
 */
export function RichProseEditor({
  content,
  id,
  label,
  describedBy,
  minHeight,
  fontSize,
  leaving,
  focusOnMount = false,
  placeholder,
  starter,
  onCancelLeave,
  onConfirmLeave,
  onChange,
}: Readonly<{
  content: QuestionPromptContent | null;
  id: string;
  label: string;
  describedBy?: string | undefined;
  minHeight: number;
  fontSize: number;
  leaving: boolean;
  focusOnMount?: boolean;
  placeholder?: string | undefined;
  starter?: string | undefined;
  onCancelLeave: () => void;
  onConfirmLeave: () => void;
  onChange: (text: string, content: QuestionContent) => void;
}>) {
  const [initial] = useState(() =>
    content === null || (starter !== undefined && contentPlainText(content) === starter)
      ? EMPTY
      : content,
  );
  useEffect(() => {
    if (focusOnMount) document.getElementById(id)?.focus();
  }, [focusOnMount, id]);
  return (
    <ContentEditor
      initialContent={initial}
      id={id}
      label={label}
      describedBy={describedBy}
      profile="question"
      minHeight={minHeight}
      fontSize={fontSize}
      placeholder={placeholder}
      tools={() =>
        leaving && (
          <SwitchToMarkdown onCancel={onCancelLeave} onConfirm={onConfirmLeave} />
        )
      }
      onChange={(document) => {
        if (isQuestionContent(document)) onChange(contentPlainText(document), document);
      }}
    />
  );
}
