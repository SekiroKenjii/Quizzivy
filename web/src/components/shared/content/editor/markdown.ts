import { unified } from "unified";
import remarkParse from "remark-parse";
import {
  CONTENT_LIMITS,
  type ContentBlock,
  type ContentInline,
  type ContentMark,
} from "../model";
import { isQuestionContent, type QuestionContent } from "../questionContent";

const parser = unified().use(remarkParse);
type Root = ReturnType<typeof parser.parse>;
type BlockNode = Root["children"][number];
type InlineNode = Extract<BlockNode, { type: "paragraph" }>["children"][number];

function unsupported(): never {
  throw new Error("unsupported Markdown structure");
}

function inlines(
  nodes: InlineNode[],
  marks: ContentMark[] = [],
  depth = 0,
): ContentInline[] {
  if (depth > CONTENT_LIMITS.depth) return unsupported();
  return nodes.flatMap((node): ContentInline[] => {
    switch (node.type) {
      case "text":
        return node.value === "" ? [] : [{ type: "text", text: node.value, marks }];
      case "break":
        return [{ type: "break" }];
      case "strong":
      case "emphasis":
        return inlines(
          node.children,
          [
            ...new Set<ContentMark>([
              ...marks,
              node.type === "strong" ? "bold" : "italic",
            ]),
          ],
          depth + 1,
        );
      case "link": {
        if (node.title) return unsupported();
        const content = inlines(node.children, marks, depth + 1);
        if (!content.every((child) => child.type === "text")) return unsupported();
        return [{ type: "link", href: node.url, content }];
      }
      default:
        return unsupported();
    }
  });
}

function blocks(nodes: BlockNode[], depth = 0): ContentBlock[] {
  if (depth > CONTENT_LIMITS.depth) return unsupported();
  return nodes.map((node): ContentBlock => {
    switch (node.type) {
      case "paragraph":
        return { type: "paragraph", content: inlines(node.children) };
      case "heading":
        if (node.depth > 3) return unsupported();
        return {
          type: "heading",
          level: node.depth as 1 | 2 | 3,
          content: inlines(node.children),
        };
      case "list":
        return {
          type: "list",
          ordered: node.ordered ?? false,
          start: node.start ?? 1,
          items: node.children.map((item) => blocks(item.children, depth + 1)),
        };
      default:
        return unsupported();
    }
  });
}

/** markdownToQuestionContent converts only the supported semantic subset and never returns a partial document. */
export function markdownToQuestionContent(markdown: string): QuestionContent | null {
  if (
    markdown.length > CONTENT_LIMITS.text ||
    new TextEncoder().encode(markdown).length > CONTENT_LIMITS.bytes
  )
    return null;
  try {
    const root = parser.parse(markdown);
    const content = {
      format: "semantic_v1",
      blocks: root.children.length
        ? blocks(root.children)
        : [{ type: "paragraph", content: [] }],
    };
    return isQuestionContent(content) ? content : null;
  } catch {
    return null;
  }
}

/** plainTextMarkdown escapes semantic projections before the user explicitly discards rich structure. */
export function plainTextMarkdown(text: string): string {
  return text
    .split("\n")
    .map((line) =>
      line
        .replace(/[\\`*_[\]()<>#+.!|~-]/g, "\\$&")
        .replaceAll("&", "&amp;")
        .replace(/^\s+/u, (space) =>
          Array.from(space, (character) => `&#${character.codePointAt(0)};`).join(""),
        ),
    )
    .join("  \n");
}
