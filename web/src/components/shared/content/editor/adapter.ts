import { withinContentBudget } from "../budget";
import type { JSONContent } from "@tiptap/core";
import { z } from "zod";
import type {
  ContentBlock,
  ContentCell,
  ContentInline,
  ContentMark,
  SemanticContent,
} from "../model";
import { CONTENT_LIMITS } from "../model";
import { validateContent, type ContentValidation } from "../validation";

function inlineJSON(node: ContentInline): JSONContent[] {
  switch (node.type) {
    case "text":
      return [
        { type: "text", text: node.text, marks: node.marks.map((type) => ({ type })) },
      ];
    case "break":
      return [{ type: "hardBreak" }];
    case "gap":
      return [{ type: "gap", attrs: { id: node.id, label: node.label } }];
    case "link":
      return node.content.map((text) => ({
        type: "text",
        text: text.text,
        marks: [
          ...text.marks.map((type) => ({ type })),
          { type: "link", attrs: { href: node.href } },
        ],
      }));
  }
}

function blockJSON(node: ContentBlock): JSONContent {
  switch (node.type) {
    case "paragraph":
      return { type: "paragraph", content: node.content.flatMap(inlineJSON) };
    case "heading":
      return {
        type: "heading",
        attrs: { level: node.level },
        content: node.content.flatMap(inlineJSON),
      };
    case "list":
      return {
        type: node.ordered ? "orderedList" : "bulletList",
        ...(node.ordered ? { attrs: { start: node.start } } : {}),
        content: node.items.map((item) => ({
          type: "listItem",
          content: item.map(blockJSON),
        })),
      };
    case "table":
      return {
        type: "table",
        content: node.rows.map((row) => ({
          type: "tableRow",
          content: row.map((cell) => ({
            type: cell.header ? "tableHeader" : "tableCell",
            attrs: { rowspan: cell.rowSpan, colspan: cell.colSpan },
            content: cell.content.map(blockJSON),
          })),
        })),
      };
    case "image":
      return {
        type: "contentImage",
        attrs: { assetId: node.assetId, label: node.alt },
      };
    case "audio":
      return {
        type: "contentAudio",
        attrs: { assetId: node.assetId, label: node.label },
      };
  }
}

/** toEditorJSON adapts validated semantic content without storing the editor's private schema. */
export function toEditorJSON(document: SemanticContent): JSONContent {
  return { type: "doc", content: document.blocks.map(blockJSON) };
}

const editorMark = z.strictObject({
  type: z.string(),
  attrs: z.record(z.string(), z.unknown()).optional(),
});
const editorNode = z.strictObject({
  type: z.string(),
  attrs: z.record(z.string(), z.unknown()).optional(),
  marks: z.array(editorMark).optional(),
  text: z.string().optional(),
  content: z.array(z.unknown()).optional(),
});
type EditorNode = z.infer<typeof editorNode>;
const empty = z.strictObject({});
const headingAttrs = z.strictObject({
  level: z.union([z.literal(1), z.literal(2), z.literal(3)]),
});
const orderedAttrs = z.strictObject({
  start: z.number().int(),
  type: z.null().optional(),
});
const cellAttrs = z.strictObject({
  rowspan: z.number().int(),
  colspan: z.number().int(),
  colwidth: z.null().optional(),
  align: z.null().optional(),
});
const gapAttrs = z.strictObject({ id: z.string(), label: z.string() });
const assetAttrs = z.strictObject({ assetId: z.string(), label: z.string() });
const linkAttrs = z.strictObject({
  href: z.string(),
  title: z.null().optional(),
  target: z.literal("_blank").optional(),
  rel: z.literal("noopener noreferrer nofollow").optional(),
  class: z.null().optional(),
});
const markType = z.enum([
  "bold",
  "italic",
  "underline",
  "strike",
  "superscript",
  "subscript",
]);

function readNode(
  input: unknown,
  depth: number,
  budget: { nodes: number },
): EditorNode {
  if (depth > CONTENT_LIMITS.depth || ++budget.nodes > CONTENT_LIMITS.nodes * 3)
    throw new Error("limit");
  const node = editorNode.parse(input);
  if ((node.content?.length ?? 0) > CONTENT_LIMITS.nodes) throw new Error("limit");
  if (node.type !== "text" && (node.text !== undefined || node.marks?.length))
    throw new Error("schema");
  return node;
}

function readInline(input: unknown, budget: { nodes: number }): ContentInline {
  const node = readNode(input, 0, budget);
  if (node.content !== undefined) throw new Error("schema");
  if (node.type === "gap") return { type: "gap", ...gapAttrs.parse(node.attrs) };
  empty.parse(node.attrs ?? {});
  if (node.type === "hardBreak") return { type: "break" };
  if (node.type !== "text" || !node.text) throw new Error("schema");
  const marks: ContentMark[] = [];
  let href: string | undefined;
  for (const mark of node.marks ?? []) {
    if (mark.type === "link") {
      if (href) throw new Error("schema");
      href = linkAttrs.parse(mark.attrs).href;
    } else {
      empty.parse(mark.attrs ?? {});
      marks.push(markType.parse(mark.type));
    }
  }
  const text = { type: "text" as const, text: node.text, marks };
  return href ? { type: "link", href, content: [text] } : text;
}

function readCell(
  input: unknown,
  depth: number,
  budget: { nodes: number },
): ContentCell {
  const node = readNode(input, depth, budget);
  if (node.type !== "tableCell" && node.type !== "tableHeader")
    throw new Error("schema");
  const attrs = cellAttrs.parse(node.attrs);
  return {
    header: node.type === "tableHeader",
    rowSpan: attrs.rowspan,
    colSpan: attrs.colspan,
    content: (node.content ?? []).map((child) => readBlock(child, depth + 1, budget)),
  };
}

function readChildren(
  input: unknown,
  expected: string,
  depth: number,
  budget: { nodes: number },
): unknown[] {
  const node = readNode(input, depth, budget);
  if (node.type !== expected) throw new Error("schema");
  empty.parse(node.attrs ?? {});
  return node.content ?? [];
}

function readBlock(
  input: unknown,
  depth: number,
  budget: { nodes: number },
): ContentBlock {
  const node = readNode(input, depth, budget);
  const children = node.content ?? [];
  switch (node.type) {
    case "paragraph":
      empty.parse(node.attrs ?? {});
      return {
        type: "paragraph",
        content: children.map((child) => readInline(child, budget)),
      };
    case "heading":
      return {
        type: "heading",
        ...headingAttrs.parse(node.attrs),
        content: children.map((child) => readInline(child, budget)),
      };
    case "bulletList":
    case "orderedList": {
      const ordered = node.type === "orderedList";
      const start = ordered
        ? orderedAttrs.parse(node.attrs).start
        : (empty.parse(node.attrs ?? {}), 1);
      return {
        type: "list",
        ordered,
        start,
        items: children.map((item) =>
          readChildren(item, "listItem", depth + 1, budget).map((child) =>
            readBlock(child, depth + 2, budget),
          ),
        ),
      };
    }
    case "table":
      empty.parse(node.attrs ?? {});
      return {
        type: "table",
        rows: children.map((row) =>
          readChildren(row, "tableRow", depth + 1, budget).map((cell) =>
            readCell(cell, depth + 2, budget),
          ),
        ),
      };
    case "contentImage":
    case "contentAudio": {
      if (children.length) throw new Error("schema");
      const attrs = assetAttrs.parse(node.attrs);
      return node.type === "contentImage"
        ? { type: "image", assetId: attrs.assetId, alt: attrs.label }
        : { type: "audio", assetId: attrs.assetId, label: attrs.label };
    }
    default:
      throw new Error("schema");
  }
}

/** fromEditorJSON rejects unsupported editor structure before it can become learner content. */
export function fromEditorJSON(input: unknown): ContentValidation {
  if (!withinContentBudget(input)) return { ok: false, issue: "limit" };
  try {
    const budget = { nodes: 0 };
    const children = readChildren(input, "doc", 0, budget);
    return validateContent({
      format: "semantic_v1",
      blocks: children.map((child) => readBlock(child, 1, budget)),
    });
  } catch {
    return { ok: false, issue: "schema" };
  }
}
