import { z } from "zod";
import type { components } from "@/lib/api/schema";
import { withinContentBudget } from "./budget";
import {
  CONTENT_LIMITS,
  QUESTION_MEDIA_LIMITS,
  type ContentBlock,
  type ProseBlock,
  type ProseDocument,
} from "./model";
import { contentPlainText } from "./plainText";
import { contentStringLength } from "./unicode";
import { validateContent, type ContentIssue } from "./validation";

export type QuestionContent = components["schemas"]["QuestionContent"];

/** A prompt: prose with gaps, and media among its own blocks (DG-109). */
export type QuestionPromptContent = components["schemas"]["QuestionPromptContent"];

/** An explanation: prose without gaps, and media among its own blocks (DG-109). */
export type QuestionExplanationContent =
  components["schemas"]["QuestionExplanationContent"];

/** A prompt or explanation as the server sends it; render it only through the guards below. */
export type StoredQuestionProse = QuestionPromptContent | QuestionExplanationContent;

/** ProseField names the rules a question's prose follows: a prompt's or an explanation's. */
export type ProseField = "prompt" | "explanation";

/** QuestionMediaBlock is an image or audio block of a question's prose. */
export type QuestionMediaBlock =
  | components["schemas"]["QuestionContentImage"]
  | components["schemas"]["QuestionPromptAudio"]
  | components["schemas"]["QuestionExplanationAudio"];

/** QuestionMediaRef is an asset a question's prose names, its id in lower case. */
export type QuestionMediaRef = { id: string; kind: "image" | "audio" };

const ASSET_ID =
  /^([0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}|00000000-0000-0000-0000-000000000000|ffffffff-ffff-ffff-ffff-ffffffffffff)$/i;
const ONLY_SPACE =
  /^[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+$/u;

const assetId = z.string().regex(ASSET_ID);

/** altText accepts an image's alt as the server does: 0 to 1000 scalars, and empty, never white space only, for a decorative image. */
export function validAlt(alt: string): boolean {
  return contentStringLength(alt) <= QUESTION_MEDIA_LIMITS.alt && !ONLY_SPACE.test(alt);
}

/** altDraft is what an alt field keeps of what was typed: white space alone is no description, so it is kept as empty, the decorative mark. */
export function altDraft(value: string): string {
  return ONLY_SPACE.test(value) ? "" : value;
}

const image = z.strictObject({
  type: z.literal("image"),
  assetId,
  alt: z.string().refine(validAlt),
});
const media = {
  prompt: z.discriminatedUnion("type", [
    image,
    z.strictObject({ type: z.literal("audio") }),
  ]),
  explanation: z.discriminatedUnion("type", [
    image,
    z.strictObject({ type: z.literal("audio"), assetId }),
  ]),
};

function isMediaBlock(block: unknown): boolean {
  if (typeof block !== "object" || block === null) return false;
  const type = (block as { type?: unknown }).type;
  return type === "image" || type === "audio";
}

function boundProse(block: ContentBlock, gaps: boolean): boolean {
  switch (block.type) {
    case "image":
    case "audio":
      return false;
    case "paragraph":
    case "heading":
      return gaps || block.content.every((node) => node.type !== "gap");
    case "list":
      return block.items.every((item) => item.every((node) => boundProse(node, gaps)));
    case "table":
      return block.rows.every((row) =>
        row.every((cell) => cell.content.every((node) => boundProse(node, gaps))),
      );
  }
}

/** QuestionProseValidation is a question prose's verdict and, when valid, its document. */
export type QuestionProseValidation =
  { ok: true; value: ProseDocument } | { ok: false; issue: ContentIssue | "media" };

function mediaIssue(blocks: QuestionMediaBlock[]): boolean {
  if (blocks.length > QUESTION_MEDIA_LIMITS.blocks) return true;
  const kinds = new Map<string, string>();
  let markers = 0;
  for (const block of blocks) {
    if (!("assetId" in block)) {
      markers++;
      continue;
    }
    const id = block.assetId.toLowerCase();
    if ((kinds.get(id) ?? block.type) !== block.type) return true;
    kinds.set(id, block.type);
  }
  return markers > 1;
}

/**
 * validateQuestionProse checks a prompt or an explanation by DG-109's rules,
 * as `content.ParsePromptWithMedia` and `ParseExplanationWithMedia` do: prose
 * as `validateContent` takes it (gaps in a prompt only), and media only among
 * the document's own blocks, at most eight. An image names its asset and an
 * alt of 0 to 1000 characters that is not white space only; a prompt's audio
 * is the bare marker, once; an explanation's names its asset. One id is one
 * kind. A legacy Markdown document is no question prose.
 */
export function validateQuestionProse(
  input: unknown,
  field: ProseField,
): QuestionProseValidation {
  if (!withinContentBudget(input)) return { ok: false, issue: "limit" };
  const shape = z
    .strictObject({ format: z.literal("semantic_v1"), blocks: z.array(z.unknown()) })
    .safeParse(input);
  if (!shape.success || shape.data.blocks.length === 0)
    return { ok: false, issue: "schema" };
  const all = shape.data.blocks;
  const mediaBlocks: QuestionMediaBlock[] = [];
  for (const block of all.filter(isMediaBlock)) {
    const parsed = media[field].safeParse(block);
    if (!parsed.success) return { ok: false, issue: "media" };
    mediaBlocks.push(parsed.data);
  }
  if (mediaIssue(mediaBlocks)) return { ok: false, issue: "media" };
  const prose = all.filter((block) => !isMediaBlock(block));
  let proseBlocks: ContentBlock[] = [];
  if (prose.length > 0) {
    const parsed = validateContent({ format: "semantic_v1", blocks: prose });
    if (!parsed.ok) return parsed;
    if (parsed.value.format !== "semantic_v1") return { ok: false, issue: "schema" };
    proseBlocks = parsed.value.blocks;
    if (!proseBlocks.every((block) => boundProse(block, field === "prompt")))
      return { ok: false, issue: "schema" };
  }
  let nextMedia = 0;
  let nextProse = 0;
  const blocks: ProseBlock[] = all.map((block) =>
    isMediaBlock(block) ? mediaBlocks[nextMedia++]! : proseBlocks[nextProse++]!,
  );
  const value: ProseDocument = { format: "semantic_v1", blocks };
  if (contentStringLength(contentPlainText(value)) > CONTENT_LIMITS.text)
    return { ok: false, issue: "limit" };
  return { ok: true, value };
}

/** questionMedia lists the assets a question's prose names, once each in document order, ids in lower case. */
export function questionMedia(document: ProseDocument): QuestionMediaRef[] {
  const seen = new Set<string>();
  const refs: QuestionMediaRef[] = [];
  for (const block of document.blocks) {
    if ((block.type !== "image" && block.type !== "audio") || !("assetId" in block))
      continue;
    const id = block.assetId.toLowerCase();
    if (seen.has(id)) continue;
    seen.add(id);
    refs.push({ id, kind: block.type });
  }
  return refs;
}

/** holdsMedia reports whether a question's prose holds an image or audio, which Markdown cannot. */
export function holdsMedia(
  document: Readonly<{ blocks: readonly ProseBlock[] }>,
): boolean {
  return document.blocks.some(
    (block) => block.type === "image" || block.type === "audio",
  );
}

/** placesAudio reports whether a prompt holds the marker that places the question's own audio. */
export function placesAudio(document: ProseDocument): boolean {
  return document.blocks.some(
    (block) => block.type === "audio" && !("assetId" in block),
  );
}

/** withoutAudioMarker removes a prompt's audio marker, for a question that no longer has audio of its own. */
export function withoutAudioMarker<T extends ProseDocument>(document: T): T {
  if (!placesAudio(document)) return document;
  const blocks = document.blocks.filter(
    (block) => block.type !== "audio" || "assetId" in block,
  );
  return {
    ...document,
    blocks: blocks.length > 0 ? blocks : [{ type: "paragraph", content: [] }],
  };
}

/** isGapProse validates prose with gaps and no media: an imported prompt's, or a group material's member before DG-109. */
export function isGapProse(value: unknown): boolean {
  const parsed = validateContent(value);
  return (
    parsed.ok &&
    parsed.value.format === "semantic_v1" &&
    parsed.value.blocks.every((node) => boundProse(node, true))
  );
}

/** isQuestionContent validates a group's instructions: prose only, no gaps and no media. */
export function isQuestionContent(value: unknown): value is QuestionContent {
  const parsed = validateContent(value);
  return (
    parsed.ok &&
    parsed.value.format === "semantic_v1" &&
    parsed.value.blocks.every((node) => boundProse(node, false))
  );
}

export const questionContentSchema = z.custom<QuestionContent>(
  isQuestionContent,
  "questionEditor.errors.questionContent",
);

/** isQuestionPromptContent validates a prompt: prose with gaps, and media by DG-109's rules. */
export function isQuestionPromptContent(
  value: unknown,
): value is QuestionPromptContent {
  return validateQuestionProse(value, "prompt").ok;
}

/** isQuestionExplanationContent validates an explanation: prose without gaps, and media by DG-109's rules. */
export function isQuestionExplanationContent(
  value: unknown,
): value is QuestionExplanationContent {
  return validateQuestionProse(value, "explanation").ok;
}

export const questionPromptContentSchema = z.custom<QuestionPromptContent>(
  isQuestionPromptContent,
  "questionEditor.errors.questionContent",
);

export const questionExplanationContentSchema = z.custom<QuestionExplanationContent>(
  isQuestionExplanationContent,
  "questionEditor.errors.questionContent",
);
