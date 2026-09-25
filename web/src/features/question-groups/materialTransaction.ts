import { closeHistory } from "@tiptap/pm/history";
import type { EditorState, SelectionBookmark, Transaction } from "@tiptap/pm/state";
import { fromEditorDoc } from "@/components/shared/content/editor/adapter";
import type { MediaAsset } from "@/features/media/api";

/** materialTransaction inserts one authorized asset at the captured selection only when the entire resulting document remains valid. */
export function materialTransaction(
  state: EditorState,
  selection: SelectionBookmark,
  asset: MediaAsset,
  label: string,
): Transaction | null {
  const kind = asset.kind === "image" ? "contentImage" : "contentAudio";
  const node = state.schema.nodes[kind];
  if (!node) return null;
  try {
    const transaction = closeHistory(state.tr)
      .setSelection(selection.resolve(state.doc))
      .replaceSelectionWith(node.create({ assetId: asset.id, label }))
      .scrollIntoView();
    return fromEditorDoc(transaction.doc).ok
      ? transaction.setMeta("structuredPaste", true)
      : null;
  } catch {
    return null;
  }
}
