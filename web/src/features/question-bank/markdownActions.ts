/** MarkdownEdit is a Markdown field's text after an action, and the selection to restore in it. */
export type MarkdownEdit = { value: string; start: number; end: number };

/**
 * wrapSelection puts `before` and `after` around the selected text, or around
 * `placeholder` when nothing is selected, and selects what it wrapped.
 */
export function wrapSelection(
  value: string,
  start: number,
  end: number,
  before: string,
  after: string,
  placeholder: string,
): MarkdownEdit {
  const selected = value.slice(start, end) || placeholder;
  const from = start + before.length;
  return {
    value: value.slice(0, start) + before + selected + after + value.slice(end),
    start: from,
    end: from + selected.length,
  };
}

/** prefixLine puts `prefix` at the start of the caret's line and keeps the caret on its text. */
export function prefixLine(value: string, caret: number, prefix: string): MarkdownEdit {
  const lineStart = value.lastIndexOf("\n", caret - 1) + 1;
  const at = caret + prefix.length;
  return {
    value: value.slice(0, lineStart) + prefix + value.slice(lineStart),
    start: at,
    end: at,
  };
}

function separatorBefore(text: string): string {
  if (!text || text.endsWith("\n\n")) return "";
  return text.endsWith("\n") ? "\n" : "\n\n";
}

/**
 * insertBlock inserts `block` at the caret on lines of its own, with a blank
 * line between it and any text around it, and leaves the caret after it.
 */
export function insertBlock(value: string, caret: number, block: string): MarkdownEdit {
  const before = value.slice(0, caret);
  const after = value.slice(caret);
  const head = before + separatorBefore(before) + block;
  const at = head.length;
  return {
    value: head + (after === "" || after.startsWith("\n") ? "" : "\n\n") + after,
    start: at,
    end: at,
  };
}

/** markdownWords counts the words of a Markdown text, reading its syntax characters as spaces. */
export function markdownWords(value: string): number {
  const words = value.replaceAll(/[#*_~>|[\]()!-]/g, " ").trim();
  return words ? words.split(/\s+/u).length : 0;
}
