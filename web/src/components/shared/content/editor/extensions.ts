import { Extension, Node } from "@tiptap/core";
import { Plugin } from "@tiptap/pm/state";
import { Slice } from "@tiptap/pm/model";
import StarterKit from "@tiptap/starter-kit";
import { TableKit } from "@tiptap/extension-table";
import Subscript from "@tiptap/extension-subscript";
import Superscript from "@tiptap/extension-superscript";
import { fromEditorJSON, toEditorJSON } from "./adapter";
import { isOptionContent, plainOptionContent } from "../optionContent";
import { isQuestionContent } from "../questionContent";
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

export type EditorNotice = "pasteBlocked" | "editBlocked";

/** contentExtensions limits the editor to the W03 candidate content vocabulary. */
export function contentExtensions(
  notify: (notice: EditorNotice) => void = () => undefined,
  profile: "document" | "option" | "question" = "document",
) {
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
          filterTransaction(transaction) {
            if (!transaction.docChanged) return true;
            const parsed = fromEditorJSON(transaction.doc.toJSON());
            if (
              parsed.ok &&
              (profile === "document" ||
                (profile === "option"
                  ? isOptionContent(parsed.value)
                  : isQuestionContent(parsed.value)))
            )
              return true;
            queueMicrotask(() => notify("editBlocked"));
            return false;
          },
          props: {
            handleKeyDown(view, event) {
              if (profile !== "option" || event.key !== "Enter" || event.isComposing)
                return false;
              const hardBreak = view.state.schema.nodes.hardBreak;
              if (!hardBreak) return false;
              view.dispatch(
                view.state.tr.replaceSelectionWith(hardBreak.create()).scrollIntoView(),
              );
              return true;
            },
            handlePaste(view, event) {
              if (
                event.clipboardData?.files.length ||
                event.clipboardData?.getData("text/html")
              ) {
                notify("pasteBlocked");
                return true;
              }
              if (profile !== "option") return false;
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
