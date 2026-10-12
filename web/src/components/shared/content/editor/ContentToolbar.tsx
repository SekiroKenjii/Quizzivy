import { Fragment } from "react";
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
import type { EditorProfile } from "./profile";
import { editorShortcut, type Shortcut } from "./shortcuts";
import { useRovingToolbar } from "./roving";
import { ToolButton } from "./ToolButton";
import { LinkPopover } from "./LinkPopover";
import { TableToolbar } from "./TableToolbar";
import { insertGap } from "./gapCommands";

type ButtonTool = {
  key: string;
  icon: LucideIcon;
  label?: string | undefined;
  shortcut?: Shortcut;
  active?: boolean;
  disabled?: boolean;
  run: () => void;
};

type Tool = ButtonTool | { key: "link"; link: true };

type ToolGroup = { key: string; tools: Tool[] };

const OPTION_GROUPS = new Set(["marks", "history"]);

/**
 * ContentToolbar is the editor's formatting toolbar, in the deck's order, and
 * the table row while the caret is in a table. It subscribes only to the
 * selection state its controls display. "Insert gap" shows only in a profile
 * that holds gaps, and the option profile keeps the marks and the history.
 */
export function ContentToolbar({
  editor,
  label,
  profile = "document",
  gapLabel,
}: Readonly<{
  gapLabel?: (() => string) | undefined;
  editor: Editor;
  label: string;
  profile?: EditorProfile;
}>) {
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
      link: e.isActive("link"),
      table: e.isActive("table"),
      undo: e.can().undo(),
      redo: e.can().redo(),
    }),
  });
  const gaps = profile === "prompt" || profile === "document";
  const allGroups: ToolGroup[] = [
    {
      key: "marks",
      tools: [
        {
          key: "bold",
          icon: Bold,
          shortcut: editorShortcut("B"),
          active: state.bold,
          run: () => editor.chain().focus().toggleBold().run(),
        },
        {
          key: "italic",
          icon: Italic,
          shortcut: editorShortcut("I"),
          active: state.italic,
          run: () => editor.chain().focus().toggleItalic().run(),
        },
        {
          key: "underline",
          icon: Underline,
          shortcut: editorShortcut("U"),
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
      ],
    },
    {
      key: "blocks",
      tools: [
        {
          key: "heading",
          icon: Heading2,
          active: state.heading,
          run: () =>
            state.heading
              ? editor.chain().focus().setParagraph().run()
              : editor.chain().focus().setHeading({ level: 3 }).run(),
        },
        {
          key: "paragraph",
          icon: Pilcrow,
          run: () => editor.chain().focus().setParagraph().run(),
        },
      ],
    },
    {
      key: "lists",
      tools: [
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
      ],
    },
    {
      key: "insert",
      tools: [
        { key: "link", link: true },
        {
          key: "insertTable",
          icon: Table2,
          label: state.table ? t("contentEditor.inTable") : undefined,
          disabled: state.table,
          run: () =>
            editor
              .chain()
              .focus()
              .insertTable({ rows: 2, cols: 2, withHeaderRow: true })
              .run(),
        },
        ...(gaps
          ? [
              {
                key: "insertGap",
                icon: TextCursorInput,
                run: () => insertGap(editor, gapLabel),
              },
            ]
          : []),
      ],
    },
    {
      key: "history",
      tools: [
        {
          key: "undo",
          icon: Undo2,
          shortcut: editorShortcut("Z"),
          disabled: !state.undo,
          run: () => editor.chain().focus().undo().run(),
        },
        {
          key: "redo",
          icon: Redo2,
          shortcut: editorShortcut("Z", true),
          disabled: !state.redo,
          run: () => editor.chain().focus().redo().run(),
        },
      ],
    },
  ];
  const groups = allGroups.filter(
    (group) => profile !== "option" || OPTION_GROUPS.has(group.key),
  );
  const roving = useRovingToolbar(
    groups.flatMap((group) =>
      group.tools
        .filter((tool) => "link" in tool || !tool.disabled)
        .map((tool) => tool.key),
    ),
  );
  return (
    <>
      <div
        role="toolbar"
        aria-label={t("contentEditor.toolbar", { label })}
        onKeyDown={roving.onKeyDown}
        className="bg-sidebar flex [scrollbar-width:thin] flex-nowrap items-center gap-0.5 overflow-x-auto rounded-t-[10px] border-b px-1.5 py-[5px]"
      >
        {groups.map((group, index) => (
          <Fragment key={group.key}>
            {index > 0 && (
              <span
                aria-hidden="true"
                className="bg-border mx-1 h-[18px] w-px flex-none"
              />
            )}
            {group.tools.map((tool) => {
              const common = {
                "data-roving": tool.key,
                tabIndex: roving.tabIndexFor(tool.key),
                onFocus: () => roving.remember(tool.key),
              };
              if ("link" in tool)
                return (
                  <LinkPopover
                    key={tool.key}
                    editor={editor}
                    active={state.link}
                    {...common}
                  />
                );
              const name = tool.label ?? t(`contentEditor.${tool.key}`);
              return (
                <ToolButton
                  key={tool.key}
                  icon={tool.icon}
                  label={name}
                  title={
                    tool.shortcut
                      ? t("contentEditor.withShortcut", {
                          label: name,
                          shortcut: tool.shortcut.label,
                        })
                      : name
                  }
                  aria-keyshortcuts={tool.shortcut?.aria}
                  aria-pressed={tool.active}
                  disabled={tool.disabled}
                  onClick={tool.run}
                  {...common}
                />
              );
            })}
          </Fragment>
        ))}
      </div>
      {state.table && profile !== "option" && <TableToolbar editor={editor} />}
    </>
  );
}
