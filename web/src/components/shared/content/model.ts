import type { components } from "@/lib/api/schema";

export type ContentMark = components["schemas"]["ContentMark"];
export type ContentInline = components["schemas"]["ContentInline"];
export type ContentCell = components["schemas"]["ContentCell"];
export type ContentBlock = components["schemas"]["ContentBlock"];
export type SemanticContent = components["schemas"]["SemanticContent"];
export type ContentDocument = components["schemas"]["ContentDocument"];

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

/** ASSET_TEXT_LIMITS caps an image's alt text and an audio block's label, in Unicode scalars, as ContentImage and ContentAudio do. */
export const ASSET_TEXT_LIMITS = { image: 1000, audio: 200 } as const;
