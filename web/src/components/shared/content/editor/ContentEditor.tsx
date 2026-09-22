import { useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import type { SemanticContent } from "../model";
import { isOptionContent } from "../optionContent";
import { validateContent } from "../validation";
import { fromEditorJSON, toEditorJSON } from "./adapter";
import { contentExtensions, type EditorNotice } from "./extensions";
import { ContentToolbar } from "./ContentToolbar";
import "../content.css";

function ActiveEditor({
  initialContent,
  onChange,
  label,
  profile = "document",
}: Readonly<{
  initialContent: SemanticContent;
  onChange: (content: SemanticContent) => void;
  label: string;
  profile?: "document" | "option";
}>) {
  const { t } = useTranslation();
  const [notice, setNotice] = useState<EditorNotice>();
  const [extensions] = useState(() => contentExtensions(setNotice, profile));
  const [content] = useState(() => toEditorJSON(initialContent));
  const editor = useEditor({
    extensions,
    content,
    enableContentCheck: true,
    shouldRerenderOnTransaction: false,
    editorProps: {
      attributes: {
        class:
          profile === "option"
            ? "semantic-content min-h-12 px-3 py-2 outline-none"
            : "semantic-content min-h-64 p-5 outline-none",
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": label,
      },
    },
    onUpdate: ({ editor }) => {
      const parsed = fromEditorJSON(editor.getJSON());
      if (parsed.ok && parsed.value.format === "semantic_v1") {
        setNotice(undefined);
        onChange(parsed.value);
      }
    },
  });
  if (!editor) return <p role="status">{t("contentEditor.loading")}</p>;
  return (
    <div className="content-editor bg-card focus-within:ring-ring/30 overflow-hidden rounded-lg border shadow-sm focus-within:ring-2">
      <ContentToolbar editor={editor} profile={profile} />
      <EditorContent editor={editor} />
      {notice && (
        <p role="alert" className="border-t px-4 py-3 text-sm">
          {t(`contentEditor.${notice}`)}
        </p>
      )}
    </div>
  );
}

/** ContentEditor edits a validated candidate; callers must key each document to isolate its undo history. */
export function ContentEditor(
  props: Readonly<{
    initialContent: SemanticContent;
    onChange: (content: SemanticContent) => void;
    label: string;
    profile?: "document" | "option";
  }>,
) {
  const { t } = useTranslation();
  const [initial] = useState(() => validateContent(props.initialContent));
  return initial.ok &&
    initial.value.format === "semantic_v1" &&
    (props.profile !== "option" || isOptionContent(initial.value)) ? (
    <ActiveEditor {...props} initialContent={initial.value} />
  ) : (
    <p role="alert">{t("contentEditor.invalidContent")}</p>
  );
}
