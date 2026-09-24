import { Extension, Node, isMacOS, isiOS } from "@tiptap/core";
import { AllSelection, Plugin, Selection, TextSelection } from "@tiptap/pm/state";
import { closeHistory } from "@tiptap/pm/history";
import { Slice } from "@tiptap/pm/model";
import type { EditorView } from "@tiptap/pm/view";
import StarterKit from "@tiptap/starter-kit";
import { TableKit } from "@tiptap/extension-table";
import Subscript from "@tiptap/extension-subscript";
import Superscript from "@tiptap/extension-superscript";
import { fromEditorJSON, toEditorJSON } from "./adapter";
import { isOptionContent, plainOptionContent } from "../optionContent";
import { validEditorProfile } from "./profile";
import { safeContentURL } from "../validation";

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

export type EditorNotice = "pasteBlocked" | "editBlocked" | "pasteStale";

/** contentExtensions limits the editor to the W03 candidate content vocabulary. */
export function contentExtensions(
  notify: (notice: EditorNotice) => void = () => undefined,
  profile: "document" | "option" | "question" | "prompt" = "document",
  previewPaste: (view: EditorView, html: string) => void = () => notify("pasteBlocked"),
) {
  let plainPaste = false;
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
            const parsed = fromEditorJSON(transaction.doc.toJSON());
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
                if (event.clipboardData?.files.length) {
                  event.preventDefault();
                  notify("pasteBlocked");
                  return true;
                }
                const html = !preferPlain && event.clipboardData?.getData("text/html");
                if (html) {
                  event.preventDefault();
                  previewPaste(view, html);
                  return true;
                }
                if (profile !== "option") return false;
                event.preventDefault();
                const text = event.clipboardData?.getData("text/plain") ?? "";
                const document = plainOptionContent(text);
                if (!isOptionContent(document)) {
                  notify("pasteBlocked");
                  return true;
                }
                const paragraph = view.state.schema.nodeFromJSON(
                  toEditorJSON(document),
                ).firstChild;
                if (paragraph)
                  view.dispatch(
                    view.state.tr
                      .replaceSelection(new Slice(paragraph.content, 0, 0))
                      .scrollIntoView(),
                  );
                return true;
              },
              drop(view, event) {
                if (view.dragging) return false;
                event.preventDefault();
                notify("pasteBlocked");
                return true;
              },
            },
            handleDrop(_view, event, _slice, moved) {
              if (moved) return false;
              event.preventDefault();
              notify("pasteBlocked");
              return true;
            },
          },
        }),
      ],
    }),
  ];
}
