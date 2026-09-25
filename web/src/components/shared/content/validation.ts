import { contentStringLength } from "./unicode";
import { withinContentBudget } from "./budget";
import { z } from "zod";
import {
  CONTENT_LIMITS,
  type ContentBlock,
  type ContentCell,
  type ContentDocument,
  type ContentInline,
} from "./model";

const mark = z.enum([
  "bold",
  "italic",
  "underline",
  "strike",
  "superscript",
  "subscript",
]);
const marks = z
  .array(mark)
  .max(6)
  .refine(
    (values) =>
      new Set(values).size === values.length &&
      !(values.includes("superscript") && values.includes("subscript")),
  );
function boundedString(min: number, max: number) {
  return z.string().refine((value) => {
    const length = contentStringLength(value);
    return length >= min && length <= max;
  });
}

const text = z.strictObject({
  type: z.literal("text"),
  text: boundedString(1, CONTENT_LIMITS.text),
  marks,
});
const id = z.string().regex(/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/);
const asset = z.uuid();

/** safeContentURL permits HTTPS navigation with a portable ASCII authority and no credentials or unsafe characters. */
export function safeContentURL(value: string): boolean {
  if (!/^https:\/\//i.test(value) || contentStringLength(value) > CONTENT_LIMITS.url)
    return false;
  if (
    Array.from(value).some((character) => {
      const code = character.codePointAt(0)!;
      return (
        character === "\\" ||
        /\s/u.test(character) ||
        code < 32 ||
        (code >= 127 && code <= 159)
      );
    }) ||
    /%(?![0-9a-f]{2})/i.test(value)
  )
    return false;
  const authority = value.slice(8).split(/[/?#]/u, 1)[0]!;
  const [host, port, extra] = authority.split(":");
  if (extra !== undefined || !host || host.length > 253) return false;
  if (
    !host
      .split(".")
      .every((label) => /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/i.test(label))
  )
    return false;
  return (
    port === undefined ||
    (/^\d{1,5}$/.test(port) && Number(port) >= 1 && Number(port) <= 65535)
  );
}

const inline: z.ZodType<ContentInline> = z.discriminatedUnion("type", [
  text,
  z.strictObject({ type: z.literal("break") }),
  z.strictObject({ type: z.literal("gap"), id, label: boundedString(1, 32) }),
  z.strictObject({
    type: z.literal("link"),
    href: z.string().refine(safeContentURL),
    content: z.array(text).min(1).max(CONTENT_LIMITS.nodes),
  }),
]);

const blocks: z.ZodType<ContentBlock[]> = z.lazy(() =>
  z.array(block).min(1).max(CONTENT_LIMITS.nodes),
);
const cell: z.ZodType<ContentCell> = z.strictObject({
  header: z.boolean(),
  rowSpan: z.number().int().min(1).max(CONTENT_LIMITS.rows),
  colSpan: z.number().int().min(1).max(CONTENT_LIMITS.columns),
  content: blocks,
});
const block: z.ZodType<ContentBlock> = z.lazy(() =>
  z.discriminatedUnion("type", [
    z.strictObject({
      type: z.literal("paragraph"),
      content: z.array(inline).max(CONTENT_LIMITS.nodes),
    }),
    z.strictObject({
      type: z.literal("heading"),
      level: z.union([z.literal(1), z.literal(2), z.literal(3)]),
      content: z.array(inline).max(CONTENT_LIMITS.nodes),
    }),
    z.strictObject({
      type: z.literal("list"),
      ordered: z.boolean(),
      start: z.number().int().min(1).max(9999),
      items: z.array(blocks).min(1).max(CONTENT_LIMITS.nodes),
    }),
    z.strictObject({
      type: z.literal("table"),
      rows: z
        .array(z.array(cell).max(CONTENT_LIMITS.columns))
        .min(1)
        .max(CONTENT_LIMITS.rows),
    }),
    z.strictObject({
      type: z.literal("image"),
      assetId: asset,
      alt: boundedString(1, CONTENT_LIMITS.imageAlt),
    }),
    z.strictObject({
      type: z.literal("audio"),
      assetId: asset,
      label: boundedString(1, CONTENT_LIMITS.audioLabel),
    }),
  ]),
);

const document: z.ZodType<ContentDocument> = z.discriminatedUnion("format", [
  z.strictObject({
    format: z.literal("legacy_markdown_v1"),
    markdown: boundedString(0, CONTENT_LIMITS.text),
  }),
  z.strictObject({ format: z.literal("semantic_v1"), blocks }),
]);

export type ContentIssue =
  "limit" | "schema" | "duplicate_gap" | "table_grid" | "nested_table" | "asset_kind";
export type ContentValidation =
  { ok: true; value: ContentDocument } | { ok: false; issue: ContentIssue };

function occupyCell(
  occupied: Set<number>[],
  row: number,
  column: number,
  cell: ContentCell,
): boolean {
  if (
    row + cell.rowSpan > occupied.length ||
    column + cell.colSpan > CONTENT_LIMITS.columns
  )
    return false;
  for (let y = row; y < row + cell.rowSpan; y++) {
    for (let x = column; x < column + cell.colSpan; x++) {
      if (occupied[y]!.has(x)) return false;
      occupied[y]!.add(x);
    }
  }
  return true;
}

function validGrid(rows: ContentCell[][]): boolean {
  const occupied = Array.from({ length: rows.length }, () => new Set<number>());
  let width = 0;
  for (const [row, cells] of rows.entries()) {
    let column = 0;
    for (const cell of cells) {
      while (occupied[row]!.has(column)) column++;
      if (!occupyCell(occupied, row, column, cell)) return false;
      column += cell.colSpan;
    }
    width = Math.max(width, occupied[row]!.size);
  }
  return width > 0 && occupied.every((row) => row.size === width);
}

type GraphState = {
  nodes: number;
  textLength: number;
  gaps: Set<string>;
  assets: Map<string, "image" | "audio">;
};

function inlineIssue(
  content: ContentInline[],
  state: GraphState,
): ContentIssue | undefined {
  for (const inline of content) {
    state.nodes++;
    if (inline.type === "text") state.textLength += contentStringLength(inline.text);
    if (inline.type === "link") {
      state.nodes += inline.content.length;
      state.textLength += inline.content.reduce(
        (sum, text) => sum + contentStringLength(text.text),
        0,
      );
    }
    if (inline.type === "gap") {
      if (state.gaps.has(inline.id)) return "duplicate_gap";
      state.gaps.add(inline.id);
      state.textLength += contentStringLength(inline.label);
    }
  }
  return undefined;
}

type PendingBlock = { block: ContentBlock; inTable: boolean };

function blockIssue(
  { block, inTable }: PendingBlock,
  queue: PendingBlock[],
  state: GraphState,
): ContentIssue | undefined {
  switch (block.type) {
    case "table":
      if (inTable) return "nested_table";
      if (!validGrid(block.rows)) return "table_grid";
      state.nodes += block.rows.reduce((sum, row) => sum + row.length, 0);
      queue.push(
        ...block.rows.flatMap((row) =>
          row.flatMap((cell) =>
            cell.content.map((child) => ({ block: child, inTable: true })),
          ),
        ),
      );
      break;
    case "list":
      if (
        (!block.ordered && block.start !== 1) ||
        block.items.some((item) => item[0]?.type !== "paragraph")
      )
        return "schema";
      queue.push(
        ...block.items.flatMap((item) =>
          item.map((child) => ({ block: child, inTable })),
        ),
      );
      break;
    case "paragraph":
    case "heading":
      return inlineIssue(block.content, state);
    case "image":
    case "audio": {
      const id = block.assetId.toLowerCase();
      const kind = state.assets.get(id);
      if (kind && kind !== block.type) return "asset_kind";
      state.assets.set(id, block.type);
      state.textLength += contentStringLength(
        block.type === "image" ? block.alt : block.label,
      );
      break;
    }
  }
  return undefined;
}

function graphIssue(content: ContentBlock[]): ContentIssue | undefined {
  const queue = content.map((block) => ({ block, inTable: false }));
  const state: GraphState = {
    nodes: 0,
    textLength: 0,
    gaps: new Set(),
    assets: new Map(),
  };
  while (queue.length) {
    if (++state.nodes > CONTENT_LIMITS.nodes) return "limit";
    const issue = blockIssue(queue.pop()!, queue, state);
    if (issue) return issue;
  }
  return state.textLength > CONTENT_LIMITS.text || state.nodes > CONTENT_LIMITS.nodes
    ? "limit"
    : undefined;
}

/** validateContent rejects unsupported structures and answer/provenance fields instead of stripping them. */
export function validateContent(input: unknown): ContentValidation {
  if (!withinContentBudget(input)) return { ok: false, issue: "limit" };
  const parsed = document.safeParse(input);
  if (!parsed.success) return { ok: false, issue: "schema" };
  const issue =
    parsed.data.format === "semantic_v1" ? graphIssue(parsed.data.blocks) : undefined;
  return issue ? { ok: false, issue } : { ok: true, value: parsed.data };
}
