import { Extension, Node } from "@tiptap/core";
import { Plugin } from "@tiptap/pm/state";
import StarterKit from "@tiptap/starter-kit";
import { TableKit } from "@tiptap/extension-table";
import Subscript from "@tiptap/extension-subscript";
import Superscript from "@tiptap/extension-superscript";
import { fromEditorJSON } from "./adapter";
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
            if (!transaction.docChanged || fromEditorJSON(transaction.doc.toJSON()).ok)
              return true;
            queueMicrotask(() => notify("editBlocked"));
            return false;
          },
          props: {
            handlePaste(_view, event) {
              if (
                !event.clipboardData?.files.length &&
                !event.clipboardData?.getData("text/html")
              )
                return false;
              notify("pasteBlocked");
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
