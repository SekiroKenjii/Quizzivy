import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { cn } from "@/lib/utils";
import type { ContentBlock } from "./model";
import { validateContent } from "./validation";
import "./content.css";
import { ContentInlineView, type GapRenderer } from "./ContentInlineView";

function Block({
  node,
  renderGap,
}: Readonly<{ node: ContentBlock; renderGap?: GapRenderer | undefined }>) {
  const { t } = useTranslation();
  switch (node.type) {
    case "paragraph":
      return (
        <p>
          {node.content.map((inline, i) => (
            <ContentInlineView key={i} node={inline} renderGap={renderGap} />
          ))}
        </p>
      );
    case "heading": {
      const Heading = `h${node.level + 1}` as "h2" | "h3" | "h4";
      return (
        <Heading>
          {node.content.map((inline, i) => (
            <ContentInlineView key={i} node={inline} renderGap={renderGap} />
          ))}
        </Heading>
      );
    }
    case "list": {
      const List = node.ordered ? "ol" : "ul";
      return (
        <List {...(node.ordered ? { start: node.start } : {})}>
          {node.items.map((item, i) => (
            <li key={i}>
              {item.map((block, j) => (
                <Block key={j} node={block} renderGap={renderGap} />
              ))}
            </li>
          ))}
        </List>
      );
    }
    case "table":
      return (
        <div
          className="content-table-scroll"
          role="region"
          aria-label={t("contentEditor.table")}
          // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- Scroll regions need keyboard access independently of cell content.
          tabIndex={0}
        >
          <table>
            <tbody>
              {node.rows.map((row, i) => (
                <tr key={i}>
                  {row.map((cell, j) => {
                    const Cell = cell.header ? "th" : "td";
                    return (
                      <Cell key={j} colSpan={cell.colSpan} rowSpan={cell.rowSpan}>
                        {cell.content.map((block, k) => (
                          <Block key={k} node={block} renderGap={renderGap} />
                        ))}
                      </Cell>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    case "image":
    case "audio":
      return (
        <div className="content-asset" role="note">
          <span>{node.type === "image" ? node.alt : node.label}</span>
          <span className="text-muted-foreground text-xs">
            {t("contentEditor.assetUnbound")}
          </span>
        </div>
      );
  }
}

/** ContentView renders validated candidate content without editor code or imported HTML. */
export function ContentView({
  document,
  className,
  renderGap,
}: Readonly<{
  document: unknown;
  className?: string;
  renderGap?: GapRenderer | undefined;
}>) {
  const { t } = useTranslation();
  const parsed = useMemo(() => validateContent(document), [document]);
  if (!parsed.ok)
    return (
      <p role="alert" className="text-sm">
        {t("contentEditor.invalidContent")}
      </p>
    );
  if (parsed.value.format === "legacy_markdown_v1")
    return (
      <Markdown {...(className ? { className } : {})}>{parsed.value.markdown}</Markdown>
    );
  return (
    <div className={cn("semantic-content", className)}>
      {parsed.value.blocks.map((node, i) => (
        <Block key={i} node={node} renderGap={renderGap} />
      ))}
    </div>
  );
}
