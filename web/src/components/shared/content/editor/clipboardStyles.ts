import type { ContentMark } from "../model";

const semanticProperties = new Set([
  "font-weight",
  "font-style",
  "text-decoration",
  "text-decoration-line",
  "vertical-align",
  "white-space",
]);
const decorativeProperties = new Set([
  "font-family",
  "font-size",
  "color",
  "background-color",
  "background",
  "line-height",
  "text-align",
  "text-indent",
  "table-layout",
  "mso-fareast-font-family",
  "mso-bidi-font-family",
  "mso-ansi-language",
  "mso-fareast-language",
  "mso-bidi-language",
  "mso-padding-alt",
]);

function decorative(property: string): boolean {
  return (
    decorativeProperties.has(property) ||
    /^(min-|max-)?(width|height)$/.test(property) ||
    /^(margin|padding|border)(-[a-z-]+)?$/.test(property)
  );
}

/** clipboardStyles preserves supported semantic marks and refuses ambiguous presentation rules. */
export function clipboardStyles(
  style: string,
  inherited: ContentMark[],
  intrinsic: ContentMark[] = [],
): ContentMark[] {
  const marks = new Set([...inherited, ...intrinsic]);
  for (const [property, value] of readStyles(style)) applyStyle(marks, property, value);
  for (const mark of inherited)
    if (mark === "underline" || mark === "strike") marks.add(mark);
  return [...marks];
}

function readStyles(style: string): Map<string, string> {
  const declarations = new Map<string, string>();
  for (const declaration of style.split(";")) {
    if (!declaration.trim()) continue;
    const colon = declaration.indexOf(":");
    if (colon < 1) throw new Error("style");
    const property = declaration.slice(0, colon).trim().toLowerCase();
    const raw = declaration
      .slice(colon + 1)
      .trim()
      .toLowerCase();
    const value = raw.endsWith("!important") ? raw.slice(0, -10).trimEnd() : raw;
    if (/[\\@]|url\s*\(|expression\s*\(/i.test(value)) throw new Error("style");
    if (semanticProperties.has(property)) {
      declarations.delete(property);
      declarations.set(property, value);
    } else if (!decorative(property)) throw new Error("style");
  }
  return declarations;
}

function applyStyle(marks: Set<ContentMark>, property: string, value: string) {
  if (value === "inherit") return;
  switch (property) {
    case "font-weight":
      if (!/^(?:normal|bold|[1-9]00)$/.test(value)) throw new Error("weight");
      if (value === "bold" || Number(value) >= 600) marks.add("bold");
      else marks.delete("bold");
      break;
    case "font-style":
      if (!["normal", "italic", "oblique"].includes(value)) throw new Error("italic");
      if (value === "normal") marks.delete("italic");
      else marks.add("italic");
      break;
    case "text-decoration":
    case "text-decoration-line":
      applyDecoration(marks, value);
      break;
    case "vertical-align":
      applyVertical(marks, value);
      break;
    case "white-space":
      if (!["normal", "nowrap"].includes(value)) throw new Error("whitespace");
      break;
  }
}

function applyDecoration(marks: Set<ContentMark>, value: string) {
  const parts = value.split(/\s+/);
  if (!parts.every((part) => ["none", "underline", "line-through"].includes(part)))
    throw new Error("decoration");
  if (parts.includes("none") && parts.length !== 1) throw new Error("decoration");
  marks.delete("underline");
  marks.delete("strike");
  if (parts.includes("underline")) marks.add("underline");
  if (parts.includes("line-through")) marks.add("strike");
}

function applyVertical(marks: Set<ContentMark>, value: string) {
  if (!["baseline", "super", "sub", "top", "middle", "bottom"].includes(value))
    throw new Error("vertical");
  marks.delete("superscript");
  marks.delete("subscript");
  if (value === "super") marks.add("superscript");
  if (value === "sub") marks.add("subscript");
}
