import { useDeferredValue, useMemo } from "react";
import { useEditorState, type Editor } from "@tiptap/react";
import type { Node as ProseNode } from "@tiptap/pm/model";
import { useTranslation } from "react-i18next";
import { editorShortcut } from "./shortcuts";

const FILLED = new Set(["table", "gap", "contentImage", "contentAudio"]);

function documentText(doc: ProseNode): string {
  return doc.textBetween(0, doc.content.size, " ", " ");
}

function isBlank(doc: ProseNode): boolean {
  let blank = true;
  doc.descendants((node) => {
    if (!blank) return false;
    if (FILLED.has(node.type.name) || (node.isText && node.text?.trim())) blank = false;
    return blank;
  });
  return blank;
}

function countWords(text: string): number {
  const trimmed = text.trim();
  return trimmed ? trimmed.split(/\s+/u).length : 0;
}

/** EditorFooter is the box's last row: the paste hint, and the words count, which counts a deferred copy of the text outside the keystroke path. */
export function EditorFooter({ editor }: Readonly<{ editor: Editor }>) {
  const { t } = useTranslation();
  const text = useEditorState({
    editor,
    selector: ({ editor: current }) => documentText(current.state.doc),
  });
  const deferred = useDeferredValue(text);
  const words = useMemo(() => countWords(deferred), [deferred]);
  return (
    <div className="text-muted-fg flex flex-wrap justify-between gap-x-3 gap-y-1 border-t px-3 py-1.5 text-[11.5px] leading-normal">
      <span className="min-w-0">
        {t("contentEditor.footerHint", { shortcut: editorShortcut("V", true).label })}
      </span>
      <span className="whitespace-nowrap tabular-nums">
        {t("contentEditor.words", { count: words })}
      </span>
    </div>
  );
}

/** EditorPlaceholder shows the field's placeholder while the document holds no text, table, gap or asset. */
export function EditorPlaceholder({
  editor,
  text,
}: Readonly<{ editor: Editor; text: string }>) {
  const blank = useEditorState({
    editor,
    selector: ({ editor: current }) => isBlank(current.state.doc),
  });
  if (!blank) return null;
  return (
    <span
      aria-hidden="true"
      className="text-muted-fg pointer-events-none absolute top-3 right-4 left-4 [font-size:var(--content-editor-font-size,inherit)] leading-[1.6]"
    >
      {text}
    </span>
  );
}
