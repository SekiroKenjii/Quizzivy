import { useTranslation } from "react-i18next";
import { ContentInlineView } from "./ContentInlineView";
import { isOptionContent } from "./optionContent";
import { contentPlainText } from "./plainText";
import "./content.css";

/** OptionText renders legacy options literally and validates versioned inline content before rendering. */
export function OptionText({
  text,
  content,
}: Readonly<{ text: string; content?: unknown }>) {
  const { t } = useTranslation();
  if (content == null)
    return <span className="break-words whitespace-pre-wrap">{text}</span>;
  if (!isOptionContent(content) || contentPlainText(content) !== text)
    return <span role="alert">{t("contentEditor.invalidContent")}</span>;
  return (
    <span className="semantic-content whitespace-pre-wrap">
      {content.blocks[0]!.content.map((node, index) => (
        <ContentInlineView key={index} node={node} />
      ))}
    </span>
  );
}
