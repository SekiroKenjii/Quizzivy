import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { contentPlainText } from "@/components/shared/content/plainText";
import {
  isQuestionPromptContent,
  type QuestionPromptContent,
} from "@/components/shared/content/questionContent";
import { nextBlankOrdinal, reconcileGapBlanks } from "../blankContent";
import type { QuestionValues } from "../questionSchema";
import { SwitchToMarkdown } from "./ProseMode";

const EMPTY: QuestionPromptContent = {
  format: "semantic_v1",
  blocks: [{ type: "paragraph", content: [] }],
};

/**
 * RichBlankEditor is a fill-in-the-blank prompt's rich body, always live. It
 * keeps each blank's answers bound to its gap through formatting, moves, undo
 * and removal. While `leaving` it asks "Switch to Markdown" under its toolbar.
 */
export function RichBlankEditor({
  value,
  leaving,
  focusOnMount = false,
  onCancelLeave,
  onConfirmLeave,
  onChange,
}: Readonly<{
  value: QuestionValues;
  leaving: boolean;
  focusOnMount?: boolean;
  onCancelLeave: () => void;
  onConfirmLeave: () => void;
  onChange: (value: QuestionValues) => void;
}>) {
  const { t } = useTranslation();
  const [initial] = useState(() => value.promptContent ?? EMPTY);
  useEffect(() => {
    if (focusOnMount) document.getElementById("question-prompt")?.focus();
  }, [focusOnMount]);
  return (
    <ContentEditor
      initialContent={initial}
      id="question-prompt"
      label={t("questionEditor.prompt")}
      profile="prompt"
      gapLabel={() => String(nextBlankOrdinal(value.blanks))}
      tools={() =>
        leaving && (
          <SwitchToMarkdown onCancel={onCancelLeave} onConfirm={onConfirmLeave} />
        )
      }
      onChange={(document) => {
        if (isQuestionPromptContent(document))
          onChange({
            ...value,
            prompt: contentPlainText(document),
            promptContent: document,
            blanks: reconcileGapBlanks(document, value.blanks),
          });
      }}
    />
  );
}
