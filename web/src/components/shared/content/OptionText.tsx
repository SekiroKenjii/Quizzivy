import { useTranslation } from "react-i18next";
import { nfc } from "@/lib/nfc";
import { ContentInlineView } from "./ContentInlineView";
import { isOptionContent } from "./optionContent";
import { contentPlainText } from "./plainText";
import { trueFalseLabelKey } from "./trueFalse";
import "./content.css";

/**
 * OptionText renders legacy options literally and validates versioned inline
 * content before rendering. Given the question's `type`, a true/false option
 * stored as exactly "True" or "False" reads in the reader's language.
 */
export function OptionText({
  text,
  content,
  type,
}: Readonly<{ text: string; content?: unknown; type?: string }>) {
  const { t } = useTranslation();
  const canonical = trueFalseLabelKey(type, text, content);
  if (canonical !== null)
    return <span className="break-words whitespace-pre-wrap">{t(canonical)}</span>;
  if (content == null)
    return <span className="break-words whitespace-pre-wrap">{nfc(text)}</span>;
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
