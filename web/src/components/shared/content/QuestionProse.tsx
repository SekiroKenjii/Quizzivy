import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { ContentView } from "./ContentView";
import { contentPlainText } from "./plainText";
import { isQuestionContent, type QuestionContent } from "./questionContent";

/** QuestionProse preserves historical Markdown and validates rich content without falling back to a different interpretation. */
export function QuestionProse({
  text,
  content,
  className,
}: Readonly<{
  text: string;
  content?: QuestionContent | null | undefined;
  className?: string;
}>) {
  const { t } = useTranslation();
  if (content == null)
    return <Markdown {...(className ? { className } : {})}>{text}</Markdown>;
  if (!isQuestionContent(content) || contentPlainText(content) !== text)
    return <p role="alert">{t("contentEditor.invalidContent")}</p>;
  return <ContentView document={content} {...(className ? { className } : {})} />;
}
