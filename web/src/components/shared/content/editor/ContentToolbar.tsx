import { useEditorState, type Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import {
  Bold,
  Italic,
  Underline,
  Strikethrough,
  Superscript,
  Subscript,
  List,
  ListOrdered,
  Undo2,
  Redo2,
  Table2,
  TextCursorInput,
  Heading2,
  Pilcrow,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";

type Tool = {
  key: string;
  icon: LucideIcon;
  active?: boolean;
  disabled?: boolean;
  run: () => void;
};

function ToolButton({ tool }: Readonly<{ tool: Tool }>) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      aria-label={t(`contentEditor.${tool.key}`)}
      title={t(`contentEditor.${tool.key}`)}
      aria-pressed={tool.active}
      disabled={tool.disabled}
      onClick={tool.run}
      className={cn(
        "hover:bg-muted focus-visible:outline-ring inline-flex size-9 shrink-0 items-center justify-center rounded-md border border-transparent transition-colors focus-visible:outline-2 disabled:opacity-40 motion-reduce:transition-none",
        tool.active && "border-border bg-muted",
      )}
    >
      <tool.icon size={16} aria-hidden="true" />
    </button>
  );
}

/** ContentToolbar subscribes only to the selection state displayed by its controls. */
export function ContentToolbar({
  editor,
  profile = "document",
}: Readonly<{ editor: Editor; profile?: "document" | "option" | "question" }>) {
  const { t } = useTranslation();
  const state = useEditorState({
    editor,
    selector: ({ editor: e }) => ({
      bold: e.isActive("bold"),
      italic: e.isActive("italic"),
      underline: e.isActive("underline"),
      strike: e.isActive("strike"),
      subscript: e.isActive("subscript"),
      superscript: e.isActive("superscript"),
      bullet: e.isActive("bulletList"),
      ordered: e.isActive("orderedList"),
      heading: e.isActive("heading"),
      table: e.isActive("table"),
      undo: e.can().undo(),
      redo: e.can().redo(),
      split: e.can().splitCell(),
      merge: e.can().mergeCells(),
    }),
  });
  const tools: Tool[] = [
    {
      key: "bold",
      icon: Bold,
      active: state.bold,
      run: () => editor.chain().focus().toggleBold().run(),
    },
    {
      key: "italic",
      icon: Italic,
      active: state.italic,
      run: () => editor.chain().focus().toggleItalic().run(),
    },
    {
      key: "underline",
      icon: Underline,
      active: state.underline,
      run: () => editor.chain().focus().toggleUnderline().run(),
    },
    {
      key: "strike",
      icon: Strikethrough,
      active: state.strike,
      run: () => editor.chain().focus().toggleStrike().run(),
    },
    {
      key: "superscript",
      icon: Superscript,
      active: state.superscript,
      run: () => editor.chain().focus().unsetSubscript().toggleSuperscript().run(),
    },
    {
      key: "subscript",
      icon: Subscript,
      active: state.subscript,
      run: () => editor.chain().focus().unsetSuperscript().toggleSubscript().run(),
    },
    {
      key: "heading",
      icon: Heading2,
      active: state.heading,
      run: () => editor.chain().focus().toggleHeading({ level: 2 }).run(),
    },
    {
      key: "paragraph",
      icon: Pilcrow,
      run: () => editor.chain().focus().setParagraph().run(),
    },
    {
      key: "bulletList",
      icon: List,
      active: state.bullet,
      run: () => editor.chain().focus().toggleBulletList().run(),
    },
    {
      key: "orderedList",
      icon: ListOrdered,
      active: state.ordered,
      run: () => editor.chain().focus().toggleOrderedList().run(),
    },
    {
      key: "insertTable",
      icon: Table2,
      disabled: state.table,
      run: () =>
        editor
          .chain()
          .focus()
          .insertTable({ rows: 2, cols: 2, withHeaderRow: true })
          .run(),
    },
    {
      key: "insertGap",
      icon: TextCursorInput,
      run: () =>
        editor
          .chain()
          .focus()
          .insertContent({
            type: "gap",
            attrs: { id: crypto.randomUUID(), label: t("contentEditor.newGap") },
          })
          .run(),
    },
    {
      key: "undo",
      icon: Undo2,
      disabled: !state.undo,
      run: () => editor.chain().focus().undo().run(),
    },
    {
      key: "redo",
      icon: Redo2,
      disabled: !state.redo,
      run: () => editor.chain().focus().redo().run(),
    },
  ];
  return (
    <div className="bg-muted/20 border-b p-2">
      <div
        role="group"
        aria-label={t("contentEditor.formatting")}
        className="flex flex-wrap gap-0.5"
      >
        {tools
          .filter(
            (tool) =>
              profile === "document" ||
              (profile === "question" && tool.key !== "insertGap") ||
              [
                "bold",
                "italic",
                "underline",
                "strike",
                "superscript",
                "subscript",
                "undo",
                "redo",
              ].includes(tool.key),
          )
          .map((tool) => (
            <ToolButton key={tool.key} tool={tool} />
          ))}
      </div>
      {state.table && (
        <div
          role="group"
          aria-label={t("contentEditor.table")}
          className="mt-2 flex flex-wrap gap-1 border-t pt-2"
        >
          {[
            { key: "addRow", run: () => editor.chain().focus().addRowAfter().run() },
            {
              key: "addColumn",
              run: () => editor.chain().focus().addColumnAfter().run(),
            },
            { key: "deleteRow", run: () => editor.chain().focus().deleteRow().run() },
            {
              key: "deleteColumn",
              run: () => editor.chain().focus().deleteColumn().run(),
            },
            {
              key: "mergeCells",
              disabled: !state.merge,
              run: () => editor.chain().focus().mergeCells().run(),
            },
            {
              key: "splitCell",
              disabled: !state.split,
              run: () => editor.chain().focus().splitCell().run(),
            },
            {
              key: "deleteTable",
              run: () => editor.chain().focus().deleteTable().run(),
            },
          ].map((tool) => (
            <button
              key={tool.key}
              type="button"
              onClick={tool.run}
              disabled={tool.disabled}
              className="hover:bg-muted focus-visible:outline-ring rounded-md px-2 py-1.5 text-xs focus-visible:outline-2 disabled:opacity-40"
            >
              {t(`contentEditor.${tool.key}`)}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
