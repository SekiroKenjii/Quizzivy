import { useCallback, useState } from "react";
import type { EditorState } from "@tiptap/pm/state";
import type { EditorView } from "@tiptap/pm/view";
import type { SemanticContent } from "../model";
import { fromEditorDoc } from "./adapter";
import { pasteTransaction } from "./pasteTransaction";
import type { EditorProfile } from "./profile";
import type { EditorNotice } from "./extensions";
import { CLIPBOARD_HTML_LIMIT } from "./clipboardLimits";

type PendingPaste = {
  state: EditorState;
  view: EditorView;
  content: SemanticContent | null;
  result: SemanticContent | null;
  failed: boolean;
};

async function convertPaste(
  pending: PendingPaste,
  html: string,
  profile: EditorProfile,
): Promise<PendingPaste> {
  try {
    const { clipboardHTML } = await import("./clipboardHTML");
    if (pending.view.isDestroyed) return { ...pending, failed: true };
    const content = clipboardHTML(html);
    const transaction = content && pasteTransaction(pending.state, content, profile);
    const parsed = transaction && fromEditorDoc(transaction.doc);
    const result =
      parsed?.ok && parsed.value.format === "semantic_v1" ? parsed.value : null;
    return { ...pending, content, result, failed: !result };
  } catch {
    return { ...pending, failed: true };
  }
}

/** useContentPaste isolates a local conversion preview and rejects application after the document changes. */
export function useContentPaste(
  profile: EditorProfile,
  notify: (notice: EditorNotice | undefined) => void,
) {
  const [paste, setPaste] = useState<PendingPaste | null>(null);
  const previewPaste = useCallback(
    (view: EditorView, html: string) => {
      if (html.length > CLIPBOARD_HTML_LIMIT) {
        notify("pasteBlocked");
        return;
      }
      const pending: PendingPaste = {
        state: view.state,
        view,
        content: null,
        result: null,
        failed: false,
      };
      notify(undefined);
      setPaste(pending);
      void convertPaste(pending, html, profile).then((result) => {
        if (!view.isDestroyed)
          setPaste((current) => (current === pending ? result : current));
      });
    },
    [notify, profile],
  );
  function applyPaste() {
    if (!paste?.content || paste.failed) return;
    if (paste.view.isDestroyed || !paste.view.state.doc.eq(paste.state.doc)) {
      notify("pasteStale");
    } else {
      const transaction = pasteTransaction(paste.state, paste.content, profile);
      if (transaction) paste.view.dispatch(transaction);
      else notify("pasteBlocked");
    }
    setPaste(null);
  }
  return { paste, previewPaste, applyPaste, closePaste: () => setPaste(null) };
}
