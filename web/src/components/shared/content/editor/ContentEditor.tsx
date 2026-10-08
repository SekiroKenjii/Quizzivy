import { useState, type CSSProperties, type ReactNode } from "react";
import { EditorContent, useEditor, type Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import type { SemanticContent } from "../model";
import { ContentView } from "../ContentView";
import { validateContent } from "../validation";
import { fromEditorDoc, toEditorJSON } from "./adapter";
import { contentExtensions, type EditorNotice } from "./extensions";
import { validEditorProfile, type EditorProfile } from "./profile";
import { ContentToolbar } from "./ContentToolbar";
import { useContentPaste } from "./useContentPaste";
import { PastePreview } from "./PastePreview";
import { NoticeBand } from "./NoticeBand";
import { EditorFooter, EditorPlaceholder } from "./EditorFooter";
import { editorShortcut } from "./shortcuts";
import "../content.css";

type Frame = { minHeight?: number; fontSize?: number; footer: boolean };

const FRAMES: Record<EditorProfile, Frame> = {
  option: { footer: false },
  prompt: { minHeight: 96, fontSize: 15, footer: true },
  question: { minHeight: 84, fontSize: 14, footer: true },
  document: { minHeight: 256, footer: true },
};

/** ContentEditorProps are the editor's field: its document, its profile and the frame the deck draws around it. */
export type ContentEditorProps = {
  initialContent: SemanticContent;
  onChange: (content: SemanticContent) => void;
  label: string;
  gapLabel?: (() => string) | undefined;
  id?: string;
  profile?: EditorProfile;
  tools?: ((editor: Editor) => ReactNode) | undefined;
  minHeight?: number | undefined;
  fontSize?: number | undefined;
  footer?: boolean | undefined;
  placeholder?: string | undefined;
  fileNotice?: string | undefined;
  readOnly?: boolean | undefined;
};

function frameStyle(minHeight?: number, fontSize?: number): CSSProperties {
  return {
    ...(minHeight ? { "--content-editor-min-height": `${minHeight}px` } : {}),
    ...(fontSize ? { "--content-editor-font-size": `${fontSize}px` } : {}),
  } as CSSProperties;
}

const BOX =
  "content-editor bg-card shadow-card relative min-w-0 rounded-[10px] border transition-[border-color,box-shadow] duration-150 motion-reduce:transition-none";

function ActiveEditor({
  initialContent,
  onChange,
  label,
  id,
  profile,
  gapLabel,
  tools,
  footer,
  placeholder,
  fileNotice,
  style,
}: Readonly<
  Omit<ContentEditorProps, "profile" | "footer"> & {
    profile: EditorProfile;
    footer: boolean;
    style: CSSProperties;
  }
>) {
  const { t } = useTranslation();
  const [notice, setNotice] = useState<EditorNotice>();
  const { paste, previewPaste, applyPaste, closePaste } = useContentPaste(
    profile,
    setNotice,
  );
  const [extensions] = useState(() =>
    contentExtensions(setNotice, profile, previewPaste),
  );
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
            : "semantic-content content-editor-body outline-none",
        ...(id ? { id } : {}),
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": label,
      },
    },
    onUpdate: ({ editor }) => {
      const parsed = fromEditorDoc(editor.state.doc);
      if (parsed.ok && parsed.value.format === "semantic_v1") {
        setNotice(undefined);
        onChange(parsed.value);
      }
    },
  });
  if (!editor) return <p role="status">{t("contentEditor.loading")}</p>;
  const message =
    notice === "file"
      ? (fileNotice ?? t("contentEditor.fileNotice"))
      : notice &&
        t(`contentEditor.${notice}`, { shortcut: editorShortcut("V", true).label });
  return (
    <div className={BOX} style={style}>
      <ContentToolbar
        editor={editor}
        label={label}
        profile={profile}
        gapLabel={gapLabel}
      />
      {tools?.(editor)}
      <div className="relative min-w-0 overflow-x-auto">
        {placeholder && <EditorPlaceholder editor={editor} text={placeholder} />}
        <EditorContent editor={editor} />
      </div>
      {message && (
        <NoticeBand message={message} onDismiss={() => setNotice(undefined)} />
      )}
      {footer && <EditorFooter editor={editor} />}
      {paste && (
        <PastePreview
          content={paste.result}
          imagesLeftOut={paste.imagesLeftOut}
          onClose={closePaste}
          onRestoreFocus={() => {
            if (!editor.isDestroyed)
              editor.commands.focus(undefined, { scrollIntoView: false });
          }}
          onApply={applyPaste}
        />
      )}
    </div>
  );
}

/**
 * ContentEditor edits a validated candidate in the deck's frame: the toolbar,
 * the box, the notice band and the word-count footer. Callers must key each
 * document to isolate its undo history. The frame's minimum height, text size
 * and footer default by profile; `fileNotice` is what the notice band says
 * about a pasted or dropped file; `readOnly` renders the document with
 * `ContentView` and no toolbar or footer.
 */
export function ContentEditor(props: Readonly<ContentEditorProps>) {
  const { t } = useTranslation();
  const profile = props.profile ?? "document";
  const frame = FRAMES[profile];
  const [initial] = useState(() => validateContent(props.initialContent));
  const style = frameStyle(
    props.minHeight ?? frame.minHeight,
    props.fontSize ?? frame.fontSize,
  );
  if (props.readOnly)
    return (
      <div className={BOX} style={style}>
        <ContentView document={props.initialContent} className="content-editor-body" />
      </div>
    );
  if (
    !initial.ok ||
    initial.value.format !== "semantic_v1" ||
    !validEditorProfile(initial.value, profile)
  )
    return (
      <div className={BOX}>
        <NoticeBand message={t("contentEditor.invalidContent")} />
      </div>
    );
  return (
    <ActiveEditor
      {...props}
      profile={profile}
      footer={props.footer ?? frame.footer}
      initialContent={initial.value}
      style={style}
    />
  );
}
