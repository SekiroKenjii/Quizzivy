import type {
  ContentDocument,
  ContentInline,
  ProseBlock,
  ProseDocument,
} from "./model";

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

function blockText(node: ProseBlock): string {
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
      return "label" in node ? node.label : "";
  }
}

function isMedia(node: ProseBlock): boolean {
  return node.type === "image" || node.type === "audio";
}

/**
 * contentPlainText projects validated content without adding asset URLs or
 * answer metadata. An image projects to its alt text and a question's audio
 * to nothing, and a media block that projects to nothing adds no separator
 * (`question-media-v1.json`); an empty paragraph still keeps its own.
 */
export function contentPlainText(document: ContentDocument | ProseDocument): string {
  if (document.format === "legacy_markdown_v1") return document.markdown;
  return document.blocks
    .flatMap((node) => {
      const text = blockText(node);
      return text === "" && isMedia(node) ? [] : [text];
    })
    .join("\n\n");
}
