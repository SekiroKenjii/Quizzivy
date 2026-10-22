import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { nfc } from "@/lib/nfc";
import { ContentView, type AssetNode } from "./ContentView";
import { contentPlainText } from "./plainText";
import { useProseMediaRenderer } from "./proseMediaRenderer";
import {
  validateQuestionProse,
  type ProseField,
  type StoredQuestionProse,
} from "./questionContent";

/**
 * QuestionProse preserves historical Markdown and validates rich content
 * without falling back to a different interpretation. `field` reads the
 * content as a prompt or an explanation, whose media draw through the
 * nearest ProseMediaRendererContext; without one, an image shows its alt text
 * and audio nothing.
 */
export function QuestionProse({
  text,
  content,
  field = "prompt",
  className,
}: Readonly<{
  text: string;
  content?: StoredQuestionProse | null | undefined;
  field?: ProseField;
  className?: string;
}>) {
  const { t } = useTranslation();
  const render = useProseMediaRenderer();
  if (content == null)
    return <Markdown {...(className ? { className } : {})}>{text}</Markdown>;
  const parsed = validateQuestionProse(content, field);
  if (!parsed.ok || contentPlainText(parsed.value) !== text)
    return <p role="alert">{t("contentEditor.invalidContent")}</p>;
  return (
    <ContentView
      document={content}
      field={field}
      renderMedia={(node) => (render ? render(node, field) : <MediaAlt node={node} />)}
      {...(className ? { className } : {})}
    />
  );
}

/** MediaAlt stands for a media block a reader was signed no file for: an image's alt text, or nothing. */
export function MediaAlt({ node }: Readonly<{ node: AssetNode }>) {
  if (node.type !== "image" || node.alt === "") return null;
  return (
    <p role="note" className="text-muted-fg text-sm italic">
      {nfc(node.alt)}
    </p>
  );
}
