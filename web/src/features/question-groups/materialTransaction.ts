import { closeHistory } from "@tiptap/pm/history";
import {
  NodeSelection,
  type EditorState,
  type SelectionBookmark,
  type Transaction,
} from "@tiptap/pm/state";
import { fromEditorDoc } from "@/components/shared/content/editor/adapter";
import type { MediaAsset } from "@/features/media/api";

/**
 * materialTransaction inserts one authorized asset at the captured selection
 * only when the entire resulting document remains valid. A selected block,
 * such as the image or audio inserted last, is kept: the asset goes after it.
 */
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
    const created = node.create({ assetId: asset.id, label });
    const at = selection.resolve(state.doc);
    const transaction = closeHistory(state.tr).setSelection(at);
    if (at instanceof NodeSelection) {
      transaction
        .insert(at.to, created)
        .setSelection(NodeSelection.create(transaction.doc, at.to));
    } else {
      transaction.replaceSelectionWith(created);
    }
    transaction.scrollIntoView();
    return fromEditorDoc(transaction.doc).ok
      ? transaction.setMeta("structuredPaste", true)
      : null;
  } catch {
    return null;
  }
}
