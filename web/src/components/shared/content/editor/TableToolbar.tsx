import { useEditorState, type Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { useRovingToolbar } from "./roving";

function keepSelection(event: { preventDefault: () => void }) {
  event.preventDefault();
}

/** TableToolbar is the row of table commands shown while the caret is in a table; merge and split are disabled with a hint when they cannot run. */
export function TableToolbar({ editor }: Readonly<{ editor: Editor }>) {
  const { t } = useTranslation();
  const state = useEditorState({
    editor,
    selector: ({ editor: e }) => ({
      merge: e.can().mergeCells(),
      split: e.can().splitCell(),
    }),
  });
  const tools = [
    { key: "addRow", run: () => editor.chain().focus().addRowAfter().run() },
    { key: "addColumn", run: () => editor.chain().focus().addColumnAfter().run() },
    { key: "deleteRow", run: () => editor.chain().focus().deleteRow().run() },
    { key: "deleteColumn", run: () => editor.chain().focus().deleteColumn().run() },
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
      danger: true,
      run: () => editor.chain().focus().deleteTable().run(),
    },
  ];
  const roving = useRovingToolbar(
    tools.filter((tool) => !tool.disabled).map((tool) => tool.key),
  );
  const caption = t("contentEditor.tableCaption");
  return (
    <div
      role="toolbar"
      aria-label={caption}
      onKeyDown={roving.onKeyDown}
      className="bg-sidebar flex flex-wrap items-center gap-0.5 border-b px-1.5 py-1"
    >
      <span
        aria-hidden="true"
        className="text-muted-fg pr-1.5 pl-1 text-[11.5px] font-medium"
      >
        {caption}
      </span>
      {tools.map((tool) => (
        <button
          key={tool.key}
          type="button"
          data-roving={tool.key}
          tabIndex={roving.tabIndexFor(tool.key)}
          onFocus={() => roving.remember(tool.key)}
          onPointerDown={keepSelection}
          onMouseDown={keepSelection}
          onClick={tool.run}
          disabled={tool.disabled}
          title={tool.disabled ? t("contentEditor.mergeSplitHint") : undefined}
          className={cn(
            "hover:bg-hover h-7 rounded-[6px] px-[9px] text-xs font-medium whitespace-nowrap disabled:opacity-45 disabled:hover:bg-transparent",
            tool.danger ? "text-danger-ink" : "text-fg",
          )}
        >
          {t(`contentEditor.${tool.key}`)}
        </button>
      ))}
    </div>
  );
}
