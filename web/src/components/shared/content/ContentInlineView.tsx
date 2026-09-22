import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { ContentInline, ContentMark } from "./model";

function markedText(value: string, marks: ContentMark[]): ReactNode {
  return marks.reduce<ReactNode>((child, mark) => {
    switch (mark) {
      case "bold":
        return <strong>{child}</strong>;
      case "italic":
        return <em>{child}</em>;
      case "underline":
        return <u>{child}</u>;
      case "strike":
        return <s>{child}</s>;
      case "superscript":
        return <sup>{child}</sup>;
      case "subscript":
        return <sub>{child}</sub>;
    }
  }, value);
}

/** ContentInlineView renders an already validated inline node without HTML injection or asset requests. */
export function ContentInlineView({ node }: Readonly<{ node: ContentInline }>) {
  const { t } = useTranslation();
  switch (node.type) {
    case "text":
      return <>{markedText(node.text, node.marks)}</>;
    case "break":
      return <br />;
    case "gap":
      return (
        <span
          className="content-gap"
          role="img"
          aria-label={t("contentEditor.gapLabel", { label: node.label })}
        >
          {node.label}
        </span>
      );
    case "link":
      return (
        <a href={node.href} target="_blank" rel="noopener noreferrer">
          {node.content.map((text, i) => (
            <ContentInlineView key={i} node={text} />
          ))}
        </a>
      );
  }
}
