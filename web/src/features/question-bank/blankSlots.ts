/**
 * Turns each `{{n}}` in a rendered prompt into the slot a student will type
 * into, so the teacher sees the question the way it will be answered.
 *
 * A rehype plugin rather than a string replacement because the markers live
 * inside Markdown, and splicing HTML into the source would defeat the
 * sanitiser. It runs AFTER rehype-sanitize, so the only markup it introduces is
 * the markup written here.
 */
const PLACEHOLDER = /\{\{(\d+)\}\}/g;

const SLOT_CLASS =
  "border-input text-muted-foreground mx-1 inline-block h-9 w-28 rounded-md border px-3 text-center align-middle text-sm leading-8";

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

export function blankSlots() {
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
  return {
    type: "element",
    tagName: "span",
    properties: { className: SLOT_CLASS, "data-blank": ordinal },
    children: [{ type: "text", value: ordinal }],
  };
}

function isText(node: HastNode): node is TextNode {
  return node.type === "text";
}
