import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ContentView } from "./ContentView";
import { contentPlainText } from "./plainText";
import { gapBindingsMatch } from "./gaps";
import { isQuestionPromptContent, type QuestionPromptContent } from "./questionContent";

/** RichBlankPrompt resolves trusted answer controls only after validating the complete gap binding set. */
export function RichBlankPrompt<T extends { gapId?: string | null | undefined }>({
  text,
  content,
  blanks,
  renderBlank,
  className,
}: Readonly<{
  text: string;
  content: QuestionPromptContent;
  blanks: readonly T[];
  renderBlank: (blank: T) => ReactNode;
  className?: string;
}>) {
  const { t } = useTranslation();
  if (
    !isQuestionPromptContent(content) ||
    contentPlainText(content) !== text ||
    !gapBindingsMatch(content, blanks)
  )
    return <p role="alert">{t("contentEditor.invalidContent")}</p>;
  const bindings = new Map(blanks.map((blank) => [blank.gapId, blank]));
  return (
    <ContentView
      document={content}
      {...(className ? { className } : {})}
      renderGap={(gap) => {
        const blank = bindings.get(gap.id);
        return blank ? renderBlank(blank) : null;
      }}
    />
  );
}
