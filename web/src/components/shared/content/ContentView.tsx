import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";
import type { ContentBlock, ProseBlock } from "./model";
import { validateContent } from "./validation";
import { validateQuestionProse, type ProseField } from "./questionContent";
import "./content.css";
import { ContentInlineView, type GapRenderer } from "./ContentInlineView";

/** AssetNode is an image or audio block: a material's, or a question prose's (DG-109). */
export type AssetNode = Extract<ProseBlock, { type: "image" | "audio" }>;

/** AssetRenderer resolves a material's validated asset nodes only from the caller's authorized bindings. */
export type AssetRenderer = (
  node: Extract<ContentBlock, { type: "image" | "audio" }>,
) => ReactNode;

/** ProseAssetRenderer resolves a question prose's media blocks (DG-109) from the caller's signed assets. */
export type ProseAssetRenderer = (node: AssetNode) => ReactNode;

function UnboundAsset({ node }: Readonly<{ node: AssetNode }>) {
  const { t } = useTranslation();
  if (node.type === "audio" && !("label" in node)) return null;
  const text = node.type === "image" ? node.alt : node.label;
  if (text === "") return null;
  return (
    <div className="content-asset" role="note">
      <span>{nfc(text)}</span>
      <span className="text-muted-foreground text-xs">
        {t("contentEditor.assetUnbound")}
      </span>
    </div>
  );
}

function Block({
  node,
  renderGap,
  renderAsset,
}: Readonly<{
  node: ProseBlock;
  renderGap?: GapRenderer | undefined;
  renderAsset?: ProseAssetRenderer | undefined;
}>) {
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
                <Block
                  key={j}
                  node={block}
                  renderGap={renderGap}
                  renderAsset={renderAsset}
                />
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
                          <Block
                            key={k}
                            node={block}
                            renderGap={renderGap}
                            renderAsset={renderAsset}
                          />
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
      if (renderAsset) return <>{renderAsset(node)}</>;
      return <UnboundAsset node={node} />;
  }
}

/**
 * ContentView renders validated candidate content without editor code or
 * imported HTML. Given `field`, it reads the document as a question's prompt
 * or explanation, whose media follow DG-109's rules and draw through
 * `renderMedia`; a group material's draw through `renderAsset`. A media block
 * with no renderer shows its alt text, and a question's audio nothing.
 */
export function ContentView({
  document,
  field,
  className,
  renderGap,
  renderAsset,
  renderMedia,
}: Readonly<{
  document: unknown;
  field?: ProseField | undefined;
  className?: string;
  renderGap?: GapRenderer | undefined;
  renderAsset?: AssetRenderer | undefined;
  renderMedia?: ProseAssetRenderer | undefined;
}>) {
  const { t } = useTranslation();
  const parsed = useMemo(
    () =>
      field === undefined
        ? validateContent(document)
        : validateQuestionProse(document, field),
    [document, field],
  );
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
        <Block
          key={i}
          node={node}
          renderGap={renderGap}
          renderAsset={
            field === undefined ? (renderAsset as ProseAssetRenderer) : renderMedia
          }
        />
      ))}
    </div>
  );
}
