/**
 * Turns each `{{n}}` in a rendered prompt into an empty slot the student's
 * input is mounted into.
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
  // A span, because sanitising already ran and a span is what survives it everywhere.
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
