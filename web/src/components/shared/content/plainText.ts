import type { ContentBlock, ContentDocument, ContentInline } from "./model";

function inlineText(node: ContentInline): string {
  switch (node.type) {
    case "text":
      return node.text;
    case "break":
      return "\n";
    case "gap":
      return `[${node.label}]`;
    case "link":
      return node.content.map((text) => text.text).join("");
  }
}

function blockText(node: ContentBlock): string {
  switch (node.type) {
    case "paragraph":
    case "heading":
      return node.content.map(inlineText).join("");
    case "list":
      return node.items.map((item) => item.map(blockText).join("\n")).join("\n");
    case "table":
      return node.rows
        .map((row) =>
          row.map((cell) => cell.content.map(blockText).join("\n")).join("\t"),
        )
        .join("\n");
    case "image":
      return node.alt;
    case "audio":
      return node.label;
  }
}

/** contentPlainText projects validated content without adding asset URLs or answer metadata. */
export function contentPlainText(document: ContentDocument): string {
  return document.format === "legacy_markdown_v1"
    ? document.markdown
    : document.blocks.map(blockText).join("\n\n");
}
