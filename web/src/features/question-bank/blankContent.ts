import { contentPlainText } from "@/components/shared/content/plainText";
import type { ContentBlock, ContentInline } from "@/components/shared/content/model";
import { questionGaps } from "@/components/shared/content/gaps";
import {
  isQuestionPromptContent,
  type QuestionPromptContent,
} from "@/components/shared/content/questionContent";
import type { QuestionValues } from "./questionSchema";

type Blank = QuestionValues["blanks"][number];

function mapInlines(
  document: QuestionPromptContent,
  transform: (node: ContentInline) => ContentInline[],
): QuestionPromptContent {
  const visit = (block: ContentBlock): ContentBlock => {
    switch (block.type) {
      case "paragraph":
      case "heading":
        return { ...block, content: block.content.flatMap(transform) };
      case "list":
        return { ...block, items: block.items.map((item) => item.map(visit)) };
      case "table":
        return {
          ...block,
          rows: block.rows.map((row) =>
            row.map((cell) => ({ ...cell, content: cell.content.map(visit) })),
          ),
        };
      default:
        return block;
    }
  };
  const result = { ...document, blocks: document.blocks.map(visit) };
  if (!isQuestionPromptContent(result))
    throw new Error("Invalid prompt transformation");
  return result;
}

/** bindLegacyBlanks converts matching unique Markdown markers and preserves each blank's answers. */
export function bindLegacyBlanks(
  document: QuestionPromptContent,
  blanks: Blank[],
  nextID: () => string = () => crypto.randomUUID(),
): { content: QuestionPromptContent; blanks: Blank[] } | null {
  const ids = new Map<number, string>();
  const source = new Map(blanks.map((blank) => [blank.ordinal, blank]));
  if (source.size !== blanks.length) return null;
  try {
    const content = mapInlines(document, (node) => {
      if (node.type !== "text") return [node];
      const parts: ContentInline[] = [];
      let at = 0;
      for (const match of node.text.matchAll(/\{\{(\d+)\}\}/g)) {
        const ordinal = Number(match[1]);
        if (!source.has(ordinal) || ids.has(ordinal))
          throw new Error("Ambiguous blank marker");
        if (match.index > at)
          parts.push({ ...node, text: node.text.slice(at, match.index) });
        const id = nextID();
        ids.set(ordinal, id);
        parts.push({ type: "gap", id, label: String(ordinal) });
        at = match.index + match[0].length;
      }
      if (at < node.text.length) parts.push({ ...node, text: node.text.slice(at) });
      return parts;
    });
    if (ids.size !== source.size || /\{\{\d+\}\}/.test(contentPlainText(content)))
      return null;
    return {
      content,
      blanks: blanks.map((blank) => ({ ...blank, gapId: ids.get(blank.ordinal)! })),
    };
  } catch {
    return null;
  }
}

/** reconcileGapBlanks keeps orphaned answers available for undo or explicit removal and adds empty keys for new gaps. */
export function reconcileGapBlanks(
  document: QuestionPromptContent,
  blanks: Blank[],
): Blank[] {
  const byGap = new Set(blanks.map((blank) => blank.gapId));
  const ordinals = new Set(blanks.map((blank) => blank.ordinal));
  let ordinal = 1;
  const added = questionGaps(document)
    .filter((gap) => !byGap.has(gap.id))
    .map((gap) => {
      while (ordinals.has(ordinal)) ordinal++;
      ordinals.add(ordinal);
      return {
        id: null,
        gapId: gap.id,
        ordinal,
        acceptedAnswers: [],
        caseSensitive: false,
      };
    });
  return [...blanks, ...added];
}

/** blankMarkdownProjection restores ordinal markers before explicitly removing rich formatting. */
export function blankMarkdownProjection(
  document: QuestionPromptContent,
  blanks: Blank[],
): QuestionPromptContent {
  const ordinals = new Map(blanks.map((blank) => [blank.gapId, blank.ordinal]));
  return mapInlines(document, (node) =>
    node.type === "gap"
      ? [{ type: "text", text: `{{${ordinals.get(node.id)}}}`, marks: [] }]
      : [node],
  );
}

/** nextBlankOrdinal finds an unused answer label without changing existing identities. */
export function nextBlankOrdinal(blanks: Blank[]): number {
  const used = new Set(blanks.map((blank) => blank.ordinal));
  let ordinal = 1;
  while (used.has(ordinal)) ordinal++;
  return ordinal;
}
