import { Extension, Node, isMacOS, isiOS, type NodeViewRenderer } from "@tiptap/core";
import { AllSelection, Plugin, Selection, TextSelection } from "@tiptap/pm/state";
import { closeHistory } from "@tiptap/pm/history";
import { Slice } from "@tiptap/pm/model";
import type { EditorView } from "@tiptap/pm/view";
import StarterKit from "@tiptap/starter-kit";
import { TableKit } from "@tiptap/extension-table";
import Subscript from "@tiptap/extension-subscript";
import Superscript from "@tiptap/extension-superscript";
import { fromEditorDoc, toEditorJSON } from "./adapter";
import { isOptionContent, plainOptionContent } from "../optionContent";
import { validEditorProfile, type EditorProfile } from "./profile";
import { clipboardHasText } from "./clipboardText";
import { safeContentURL } from "../validation";
import { mediaField } from "./profile";

const Gap = Node.create({
  name: "gap",
  group: "inline",
  inline: true,
  atom: true,
  marks: "",
  addAttributes: () => ({ id: { default: null }, label: { default: null } }),
  renderHTML: ({ node }) => [
    "span",
    { class: "content-gap", "data-gap-id": node.attrs.id, contenteditable: "false" },
    node.attrs.label as string,
  ],
});

function assetNode(name: string) {
  return Node.create({
    name,
    group: "block",
    atom: true,
    addAttributes: () => ({ assetId: { default: null }, label: { default: null } }),
    renderHTML: ({ node }) => [
      "div",
      { class: "content-asset", contenteditable: "false" },
      node.attrs.label as string,
    ],
  });
}

function proseMediaNode(
  name: "proseImage" | "proseAudio",
  attrs: Record<string, { default: unknown }>,
  view: NodeViewRenderer | undefined,
) {
  return Node.create({
    name,
    group: "block",
    atom: true,
    selectable: true,
    draggable: false,
    addAttributes: () => attrs,
    renderHTML: () => ["figure", { "data-media": name, contenteditable: "false" }],
    ...(view ? { addNodeView: () => view } : {}),
  });
}

/** EditorNotice names a message of the notice band; "file" is the host's notice for a pasted or dropped file. */
export type EditorNotice = "pasteBlocked" | "editBlocked" | "pasteStale" | "file";

/**
 * PROSE_MEDIA_FILES is the event a field that holds question media sends from
 * its editor's DOM with the files pasted or dropped into it, the caret already
 * at the place they landed; a listener that takes them cancels it, and files
 * nobody takes go to the notice band.
 */
export const PROSE_MEDIA_FILES = "prose-media-files";

/** MediaFiles turns on a media field's file events; `views` draws its media nodes, as the page's host brings them. */
export type MediaFiles = Readonly<{
  views?: Readonly<Record<"proseImage" | "proseAudio", NodeViewRenderer>> | undefined;
}>;

function pastePlainText(
  view: EditorView,
  event: ClipboardEvent,
  profile: EditorProfile,
  notify: (notice: EditorNotice) => void,
) {
  const text = event.clipboardData?.getData("text/plain") ?? "";
  if (profile !== "option") {
    view.pasteText(text, event);
    return;
  }
  const document = plainOptionContent(text);
  if (!isOptionContent(document)) {
    notify("pasteBlocked");
    return;
  }
  const paragraph = view.state.schema.nodeFromJSON(toEditorJSON(document)).firstChild;
  if (paragraph)
    view.dispatch(
      view.state.tr
        .replaceSelection(new Slice(paragraph.content, 0, 0))
        .scrollIntoView(),
    );
}

function droppedNotice(event: DragEvent): EditorNotice {
  return event.dataTransfer?.files.length ? "file" : "pasteBlocked";
}

/** contentExtensions limits the editor to the W03 candidate content vocabulary. */
export function contentExtensions(
  notify: (notice: EditorNotice) => void = () => undefined,
  profile: EditorProfile = "document",
  previewPaste: (view: EditorView, html: string) => void = () => notify("pasteBlocked"),
  media?: MediaFiles,
) {
  let plainPaste = false;
  const takesFiles = media !== undefined && mediaField(profile) !== null;
  const take = (
    files: FileList | undefined,
    view: EditorView,
    at: Readonly<{ clientX: number; clientY: number }> | null,
  ) => {
    if (!takesFiles || files === undefined || files.length === 0) return false;
    const landed = at && view.posAtCoords({ left: at.clientX, top: at.clientY });
    if (landed)
      view.dispatch(
        view.state.tr.setSelection(
          TextSelection.near(view.state.doc.resolve(landed.pos)),
        ),
      );
    return !view.dom.dispatchEvent(
      new CustomEvent(PROSE_MEDIA_FILES, {
        detail: [...files],
        bubbles: true,
        cancelable: true,
      }),
    );
  };
  return [
    StarterKit.configure({
      blockquote: false,
      code: false,
      codeBlock: false,
      horizontalRule: false,
      trailingNode: false,
      heading: { levels: [1, 2, 3] },
      link: {
        openOnClick: false,
        autolink: false,
        linkOnPaste: false,
        isAllowedUri: safeContentURL,
      },
    }),
    TableKit.configure({ table: { resizable: false } }),
    Subscript,
    Superscript,
    Gap,
    assetNode("contentImage"),
    assetNode("contentAudio"),
    proseMediaNode(
      "proseImage",
      { assetId: { default: null }, alt: { default: "" } },
      media?.views?.proseImage,
    ),
    proseMediaNode(
      "proseAudio",
      { assetId: { default: null } },
      media?.views?.proseAudio,
    ),
    Extension.create({
      name: "contentBoundary",
      addProseMirrorPlugins: () => [
        new Plugin({
          appendTransaction(transactions, _previous, current) {
            return transactions.some(
              (transaction) =>
                transaction.getMeta("blur") || transaction.getMeta("structuredPaste"),
            )
              ? closeHistory(current.tr)
              : null;
          },
          filterTransaction(transaction) {
            if (!transaction.docChanged) return true;
            const parsed = fromEditorDoc(transaction.doc, profile);
            if (parsed.ok && validEditorProfile(parsed.value, profile)) return true;
            queueMicrotask(() => notify("editBlocked"));
            return false;
          },
          props: {
            handleKeyDown(view, event) {
              const mod = isMacOS() || isiOS() ? event.metaKey : event.ctrlKey;
              if (
                mod &&
                !event.altKey &&
                !event.shiftKey &&
                !event.isComposing &&
                event.key.toLowerCase() === "a"
              ) {
                view.dispatch(
                  view.state.tr.setSelection(new AllSelection(view.state.doc)),
                );
                view.focus();
                return true;
              }
              if (
                event.ctrlKey &&
                !event.altKey &&
                !event.metaKey &&
                !event.isComposing &&
                (event.key === "Home" || event.key === "End")
              ) {
                const edge =
                  event.key === "Home"
                    ? Selection.atStart(view.state.doc)
                    : Selection.atEnd(view.state.doc);
                view.dispatch(
                  view.state.tr
                    .setSelection(
                      event.shiftKey
                        ? TextSelection.create(
                            view.state.doc,
                            view.state.selection.anchor,
                            edge.head,
                          )
                        : edge,
                    )
                    .scrollIntoView(),
                );
                view.focus();
                return true;
              }
              if (profile !== "option" || event.key !== "Enter" || event.isComposing)
                return false;
              const hardBreak = view.state.schema.nodes.hardBreak;
              if (!hardBreak) return false;
              view.dispatch(
                view.state.tr.replaceSelectionWith(hardBreak.create()).scrollIntoView(),
              );
              return true;
            },
            handleDOMEvents: {
              keydown(_view, event) {
                plainPaste =
                  event.shiftKey &&
                  (event.metaKey || event.ctrlKey) &&
                  event.key.toLowerCase() === "v";
                return false;
              },
              keyup() {
                plainPaste = false;
                return false;
              },
              paste(view, event) {
                const preferPlain = plainPaste;
                plainPaste = false;
                const clipboard = event.clipboardData;
                if (!clipboard) return false;
                const html = preferPlain ? "" : clipboard.getData("text/html");
                if (html && clipboardHasText(html)) {
                  event.preventDefault();
                  previewPaste(view, html);
                  return true;
                }
                const text = clipboard.getData("text/plain");
                if (!text && (html || clipboard.files.length)) {
                  event.preventDefault();
                  if (!take(clipboard.files, view, null)) notify("file");
                  return true;
                }
                if (profile !== "option" && !html && !clipboard.files.length)
                  return false;
                event.preventDefault();
                pastePlainText(view, event, profile, notify);
                return true;
              },
              drop(view, event) {
                if (view.dragging) return false;
                event.preventDefault();
                if (!take(event.dataTransfer?.files, view, event))
                  notify(droppedNotice(event));
                return true;
              },
            },
            handleDrop(view, event, _slice, moved) {
              if (moved) return false;
              event.preventDefault();
              if (!take(event.dataTransfer?.files, view, event))
                notify(droppedNotice(event));
              return true;
            },
          },
        }),
      ],
    }),
  ];
}
