import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { EditorContent, useEditor, type Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import type { ProseDocument, SemanticContent } from "../model";
import { ContentView } from "../ContentView";
import { fromEditorDoc, toEditorJSON } from "./adapter";
import { contentExtensions, PROSE_MEDIA_FILES, type EditorNotice } from "./extensions";
import {
  mediaField,
  validateForProfile,
  validEditorProfile,
  type EditorProfile,
} from "./profile";
import { useProseMediaHost } from "./proseMedia";
import { ContentToolbar } from "./ContentToolbar";
import { useContentPaste } from "./useContentPaste";
import { PastePreview } from "./PastePreview";
import { NoticeBand } from "./NoticeBand";
import { EditorFooter, EditorPlaceholder } from "./EditorFooter";
import { editorShortcut } from "./shortcuts";
import { EDITOR_BOX as BOX, frameStyle } from "./frame";
import "../content.css";

type Frame = { minHeight?: number; fontSize?: number; footer: boolean };

const PROMPT: Frame = { minHeight: 96, fontSize: 15, footer: true };
const QUESTION: Frame = { minHeight: 84, fontSize: 14, footer: true };

const FRAMES: Record<EditorProfile, Frame> = {
  option: { footer: false },
  prompt: PROMPT,
  question: QUESTION,
  questionPrompt: PROMPT,
  blankPrompt: PROMPT,
  explanation: QUESTION,
  document: { minHeight: 256, footer: true },
};

/**
 * ContentEditorProps are the editor's field: its document, its profile and
 * the frame the deck draws around it. A field of a media profile reports a
 * ProseDocument, which may hold a question's media; every other field a
 * SemanticContent.
 */
export type ContentEditorProps<
  D extends SemanticContent | ProseDocument = SemanticContent,
> = {
  initialContent: D;
  onChange: (content: NoInfer<D>) => void;
  label: string;
  describedBy?: string | undefined;
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
  onEditor?: ((editor: Editor | null) => void) | undefined;
};

function ActiveEditor({
  initialContent,
  onChange,
  label,
  describedBy,
  id,
  profile,
  gapLabel,
  tools,
  footer,
  placeholder,
  fileNotice,
  onEditor,
  style,
}: Readonly<
  Omit<ContentEditorProps<SemanticContent | ProseDocument>, "profile" | "footer"> & {
    profile: EditorProfile;
    footer: boolean;
    style: CSSProperties;
  }
>) {
  const { t } = useTranslation();
  const [notice, setNotice] = useState<EditorNotice>();
  const host = useProseMediaHost();
  const field = mediaField(profile);
  const { paste, previewPaste, applyPaste, closePaste } = useContentPaste(
    profile,
    setNotice,
  );
  const [extensions] = useState(() =>
    contentExtensions(
      setNotice,
      profile,
      previewPaste,
      field === null ? undefined : { views: host?.nodeViews },
    ),
  );
  const [content] = useState(() => toEditorJSON(initialContent, profile));
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
        ...(describedBy ? { "aria-describedby": describedBy } : {}),
      },
    },
    onUpdate: ({ editor }) => {
      const parsed = fromEditorDoc(editor.state.doc, profile);
      if (parsed.ok && parsed.value.format === "semantic_v1") {
        setNotice(undefined);
        onChange(parsed.value);
      }
    },
  });
  useEffect(() => {
    if (!editor || !onEditor) return;
    onEditor(editor);
    return () => onEditor(null);
  }, [editor, onEditor]);
  useEffect(() => {
    if (!editor || !host || field === null) return;
    host.register(field, editor);
    return () => host.register(field, null);
  }, [editor, host, field]);
  if (!editor) return <p role="status">{t("contentEditor.loading")}</p>;
  const message =
    notice === "file"
      ? (fileNotice ?? t("contentEditor.fileNotice"))
      : notice &&
        t(`contentEditor.${notice}`, { shortcut: editorShortcut("V", true).label });
  const takeFiles = (node: HTMLDivElement) => {
    if (host === null || field === null) return;
    const onFiles = (event: Event) => {
      event.preventDefault();
      host.files((event as CustomEvent<File[]>).detail, field, editor);
    };
    node.addEventListener(PROSE_MEDIA_FILES, onFiles);
    return () => node.removeEventListener(PROSE_MEDIA_FILES, onFiles);
  };
  return (
    <div className={BOX} style={style}>
      <ContentToolbar
        editor={editor}
        label={label}
        profile={profile}
        gapLabel={gapLabel}
        media={host !== null && field !== null ? { host, field } : undefined}
      />
      {tools?.(editor)}
      <div ref={takeFiles} className="relative min-w-0 overflow-x-auto">
        {placeholder && <EditorPlaceholder editor={editor} text={placeholder} />}
        <EditorContent editor={editor} />
      </div>
      {message && (
        <NoticeBand message={message} onDismiss={() => setNotice(undefined)} />
      )}
      {host !== null && field !== null && host.status(field)}
      {footer && <EditorFooter editor={editor} />}
      {paste && (
        <PastePreview
          content={paste.result}
          field={field ?? undefined}
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
 * `ContentView` and no toolbar or footer. `onEditor` receives the live editor
 * once it exists and `null` when it goes, for a host command that acts at its
 * caret; it must be a stable callback.
 */
export function ContentEditor<
  D extends SemanticContent | ProseDocument = SemanticContent,
>(props: Readonly<ContentEditorProps<D>>) {
  const { t } = useTranslation();
  const profile = props.profile ?? "document";
  const frame = FRAMES[profile];
  const [initial] = useState(() => validateForProfile(props.initialContent, profile));
  const style = frameStyle(
    props.minHeight ?? frame.minHeight,
    props.fontSize ?? frame.fontSize,
  );
  if (props.readOnly)
    return (
      <div className={BOX} style={style}>
        <ContentView
          document={props.initialContent}
          field={mediaField(profile) ?? undefined}
          className="content-editor-body"
        />
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
      onChange={props.onChange as (content: SemanticContent | ProseDocument) => void}
      style={style}
    />
  );
}
