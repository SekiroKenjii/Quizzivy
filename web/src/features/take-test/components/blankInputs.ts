/**
 * Turns each `{{n}}` in a rendered prompt into an empty slot the student's
 * input is mounted into.
 *
 * A rehype plugin rather than splitting the prompt string, and for a reason
 * that shows up the moment a teacher writes `**She {{1}} here**`: splitting the
 * source would cut the emphasis in half and the two pieces would render as
 * literal asterisks. Operating on the tree leaves the formatting around a blank
 * exactly where the teacher put it.
 *
 * It runs AFTER rehype-sanitize, so the only markup it introduces is the markup
 * written here (§2). The slot carries the ordinal and nothing else; what gets
 * rendered into it is decided in React, where the store lives.
 *
 * Sibling of question-bank/blankSlots, which does the same for the teacher's
 * read-only preview. Kept separate because the two differ in the only way that
 * matters -- one is typed into and one is looked at -- and merging them would
 * mean a flag that decides whether a student can answer.
 */
const PLACEHOLDER = /\{\{(\d+)\}\}/g;

interface TextNode {
  type: "text";
  value: string;
}

interface ElementNode {
  type: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: HastNode[];
}

type HastNode = TextNode | ElementNode;

export function blankInputs() {
  return (tree: ElementNode) => splice(tree);
}

function splice(node: ElementNode): void {
  if (!node.children) return;

  const next: HastNode[] = [];
  for (const child of node.children) {
    if (isText(child)) {
      next.push(...tokenize(child.value));
      continue;
    }
    splice(child);
    next.push(child);
  }
  node.children = next;
}

function tokenize(value: string): HastNode[] {
  const nodes: HastNode[] = [];
  let cursor = 0;

  for (const match of value.matchAll(PLACEHOLDER)) {
    const at = match.index;
    if (at > cursor) nodes.push({ type: "text", value: value.slice(cursor, at) });
    nodes.push(slot(match[1] ?? ""));
    cursor = at + match[0].length;
  }

  if (cursor === 0) return [{ type: "text", value }];
  if (cursor < value.length) nodes.push({ type: "text", value: value.slice(cursor) });
  return nodes;
}

function slot(ordinal: string): ElementNode {
  // A span, because sanitising already ran and a span is what survives it
  // everywhere. The data attribute is what React matches on.
  return {
    type: "element",
    tagName: "span",
    properties: { "data-blank": ordinal },
    children: [],
  };
}

function isText(node: HastNode): node is TextNode {
  return node.type === "text";
}
