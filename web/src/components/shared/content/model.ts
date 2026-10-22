import type { components } from "@/lib/api/schema";

export type ContentMark = components["schemas"]["ContentMark"];
export type ContentInline = components["schemas"]["ContentInline"];
export type ContentCell = components["schemas"]["ContentCell"];
export type ContentBlock = components["schemas"]["ContentBlock"];
export type SemanticContent = components["schemas"]["SemanticContent"];
export type ContentDocument = components["schemas"]["ContentDocument"];

/**
 * QuestionAudio is the audio block only a question's prose holds (DG-109): a
 * prompt's marker, which names no asset, or an explanation's recording, which
 * names its asset. Neither has a label.
 */
export type QuestionAudio =
  | components["schemas"]["QuestionPromptAudio"]
  | components["schemas"]["QuestionExplanationAudio"];

/** ProseBlock is any block a content document holds: a material's, or a question prompt's or explanation's. */
export type ProseBlock = ContentBlock | QuestionAudio;

/** ProseDocument is a semantic document whose blocks may include a question's media (DG-109). */
export type ProseDocument = { format: "semantic_v1"; blocks: ProseBlock[] };

export const CONTENT_LIMITS = {
  bytes: 1_048_576,
  nodes: 2048,
  depth: 16,
  values: 24_576,
  strings: 200_000,
  text: 100_000,
  url: 2000,
  rows: 50,
  columns: 12,
} as const;

/** QUESTION_MEDIA_LIMITS bound a question prose's media (DG-109): blocks per document (`x-media-blocks`) and an image's alt, in Unicode scalars. */
export const QUESTION_MEDIA_LIMITS = { blocks: 8, alt: 1000 } as const;

/** ASSET_TEXT_LIMITS caps an image's alt text and an audio block's label, in Unicode scalars, as ContentImage and ContentAudio do. */
export const ASSET_TEXT_LIMITS = { image: 1000, audio: 200 } as const;
