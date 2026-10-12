import type { Editor } from "@tiptap/react";

/**
 * insertGap puts a new gap at the editor's caret and focuses the editor. Its
 * label is `gapLabel()` when the host numbers gaps itself, else the smallest
 * number no gap in the document carries. The toolbar's "Insert gap" and the
 * fill-in-the-blank answer area's empty card both run it, so they insert
 * alike.
 */
export function insertGap(editor: Editor, gapLabel?: (() => string) | undefined) {
  const labels = new Set<string>();
  editor.state.doc.descendants((node) => {
    if (node.type.name === "gap") labels.add(String(node.attrs.label));
  });
  let label = 1;
  while (labels.has(String(label))) label++;
  editor
    .chain()
    .focus()
    .insertContent({
      type: "gap",
      attrs: { id: crypto.randomUUID(), label: gapLabel?.() ?? String(label) },
    })
    .run();
}
