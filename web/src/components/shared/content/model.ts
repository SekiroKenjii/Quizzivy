export type ContentMark =
  "bold" | "italic" | "underline" | "strike" | "superscript" | "subscript";

export type ContentInline =
  | { type: "text"; text: string; marks: ContentMark[] }
  | { type: "break" }
  | { type: "gap"; id: string; label: string }
  | {
      type: "link";
      href: string;
      content: { type: "text"; text: string; marks: ContentMark[] }[];
    };

export type ContentCell = {
  header: boolean;
  rowSpan: number;
  colSpan: number;
  content: ContentBlock[];
};

export type ContentBlock =
  | { type: "paragraph"; content: ContentInline[] }
  | { type: "heading"; level: 1 | 2 | 3; content: ContentInline[] }
  | { type: "list"; ordered: boolean; start: number; items: ContentBlock[][] }
  | { type: "table"; rows: ContentCell[][] }
  | { type: "image"; assetId: string; alt: string }
  | { type: "audio"; assetId: string; label: string };

export type SemanticContent = { format: "semantic_v1"; blocks: ContentBlock[] };
export type ContentDocument =
  SemanticContent | { format: "legacy_markdown_v1"; markdown: string };

export const CONTENT_LIMITS = {
  nodes: 2048,
  depth: 16,
  text: 100_000,
  url: 2000,
  rows: 50,
  columns: 12,
} as const;
