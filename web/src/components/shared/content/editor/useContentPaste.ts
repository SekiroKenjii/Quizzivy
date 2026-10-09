import { useCallback, useRef, useState } from "react";
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
  imagesLeftOut: number;
};

type Conversion =
  | { outcome: "preview"; paste: PendingPaste }
  | { outcome: "refused"; notice: EditorNotice };

async function convertPaste(
  pending: PendingPaste,
  html: string,
  profile: EditorProfile,
): Promise<Conversion> {
  const refused = { outcome: "refused", notice: "pasteBlocked" } as const;
  try {
    const { clipboardHTML } = await import("./clipboardHTML");
    if (pending.view.isDestroyed) return refused;
    const converted = clipboardHTML(html);
    if (!converted) return refused;
    if (!converted.content) return { outcome: "refused", notice: "file" };
    const transaction = pasteTransaction(pending.state, converted.content, profile);
    const parsed = transaction && fromEditorDoc(transaction.doc);
    if (!parsed?.ok || parsed.value.format !== "semantic_v1") return refused;
    return {
      outcome: "preview",
      paste: {
        ...pending,
        content: converted.content,
        result: parsed.value,
        imagesLeftOut: converted.imagesLeftOut,
      },
    };
  } catch {
    return refused;
  }
}

/** useContentPaste isolates a local conversion preview and rejects application after the document changes; a refused paste closes the preview and reports through notify. */
export function useContentPaste(
  profile: EditorProfile,
  notify: (notice: EditorNotice | undefined) => void,
) {
  const [paste, setPaste] = useState<PendingPaste | null>(null);
  const awaited = useRef<PendingPaste | null>(null);
  const show = useCallback((next: PendingPaste | null) => {
    awaited.current = next;
    setPaste(next);
  }, []);
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
        imagesLeftOut: 0,
      };
      notify(undefined);
      show(pending);
      void convertPaste(pending, html, profile).then((conversion) => {
        if (view.isDestroyed || awaited.current !== pending) return;
        if (conversion.outcome === "preview") {
          show(conversion.paste);
          return;
        }
        show(null);
        notify(conversion.notice);
      });
    },
    [notify, profile, show],
  );
  function applyPaste() {
    if (!paste?.content) return;
    if (paste.view.isDestroyed || !paste.view.state.doc.eq(paste.state.doc)) {
      notify("pasteStale");
    } else {
      const transaction = pasteTransaction(paste.state, paste.content, profile);
      if (transaction) paste.view.dispatch(transaction);
      else notify("pasteBlocked");
    }
    show(null);
  }
  return { paste, previewPaste, applyPaste, closePaste: () => show(null) };
}
