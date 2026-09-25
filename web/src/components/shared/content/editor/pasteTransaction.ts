import { Slice } from "@tiptap/pm/model";
import { closeHistory } from "@tiptap/pm/history";
import type { EditorState, Transaction } from "@tiptap/pm/state";
import type { SemanticContent } from "../model";
import { fromEditorDoc, toEditorJSON } from "./adapter";
import { validEditorProfile, type EditorProfile } from "./profile";

/** pasteTransaction validates the complete replacement at the captured selection before returning one undoable edit. */
export function pasteTransaction(
  state: EditorState,
  document: SemanticContent,
  profile: EditorProfile,
): Transaction | null {
  if (!validEditorProfile(document, profile)) return null;
  try {
    const node = state.schema.nodeFromJSON(toEditorJSON(document));
    const transaction = closeHistory(state.tr).replaceSelection(
      Slice.maxOpen(node.content),
    );
    const parsed = fromEditorDoc(transaction.doc);
    if (!parsed.ok || !validEditorProfile(parsed.value, profile)) return null;
    return transaction.setMeta("structuredPaste", true).scrollIntoView();
  } catch {
    return null;
  }
}
