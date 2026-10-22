import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ContentView } from "./ContentView";
import { contentPlainText } from "./plainText";
import { gapBindingsMatch } from "./gaps";
import { isQuestionPromptContent, type StoredQuestionProse } from "./questionContent";
import { useProseMediaRenderer } from "./proseMediaRenderer";
import { MediaAlt } from "./QuestionProse";

/** RichBlankPrompt resolves trusted answer controls only after validating the complete gap binding set. */
export function RichBlankPrompt<T extends { gapId?: string | null | undefined }>({
  text,
  content,
  blanks,
  renderBlank,
  className,
}: Readonly<{
  text: string;
  content: StoredQuestionProse;
  blanks: readonly T[];
  renderBlank: (blank: T) => ReactNode;
  className?: string;
}>) {
  const { t } = useTranslation();
  const render = useProseMediaRenderer();
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
      field="prompt"
      renderMedia={(node) =>
        render ? render(node, "prompt") : <MediaAlt node={node} />
      }
      {...(className ? { className } : {})}
      renderGap={(gap) => {
        const blank = bindings.get(gap.id);
        return blank ? renderBlank(blank) : null;
      }}
    />
  );
}
