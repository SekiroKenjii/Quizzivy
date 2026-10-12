import { Fragment } from "react";
import { useTranslation } from "react-i18next";
import {
  Bold,
  Heading2,
  Italic,
  Link,
  List,
  ListOrdered,
  Strikethrough,
  Table2,
  type LucideIcon,
} from "lucide-react";
import { ToolButton } from "@/components/shared/content/editor/ToolButton";
import { useRovingToolbar } from "@/components/shared/content/editor/roving";
import {
  insertBlock,
  prefixLine,
  wrapSelection,
  type MarkdownEdit,
} from "../markdownActions";

/** MarkdownAction turns a Markdown field's text and selection into the text after a toolbar action. */
export type MarkdownAction = (
  value: string,
  start: number,
  end: number,
) => MarkdownEdit;

type Tool = {
  key: string;
  icon: LucideIcon;
  label: string;
  title: string;
  run: MarkdownAction;
};

function useTools(): Tool[][] {
  const { t } = useTranslation();
  const tool = (
    key: string,
    icon: LucideIcon,
    label: string,
    run: MarkdownAction,
  ): Tool => ({ key, icon, label, title: t(`markdownEditor.${key}`), run });
  const table = [
    `| ${t("markdownEditor.column", { n: 1 })} | ${t("markdownEditor.column", { n: 2 })} |`,
    "| --- | --- |",
    "|  |  |",
  ].join("\n");
  return [
    [
      tool("bold", Bold, t("contentEditor.bold"), (value, start, end) =>
        wrapSelection(value, start, end, "**", "**", t("markdownEditor.boldText")),
      ),
      tool("italic", Italic, t("contentEditor.italic"), (value, start, end) =>
        wrapSelection(value, start, end, "*", "*", t("markdownEditor.italicText")),
      ),
      tool("strike", Strikethrough, t("contentEditor.strike"), (value, start, end) =>
        wrapSelection(value, start, end, "~~", "~~", t("markdownEditor.strikeText")),
      ),
    ],
    [
      tool("heading", Heading2, t("contentEditor.heading"), (value, start) =>
        prefixLine(value, start, "### "),
      ),
    ],
    [
      tool("bulletList", List, t("contentEditor.bulletList"), (value, start) =>
        prefixLine(value, start, "- "),
      ),
      tool("orderedList", ListOrdered, t("contentEditor.orderedList"), (value, start) =>
        prefixLine(value, start, "1. "),
      ),
    ],
    [
      tool("link", Link, t("contentEditor.link"), (value, start, end) =>
        wrapSelection(
          value,
          start,
          end,
          "[",
          "](https://)",
          t("markdownEditor.linkText"),
        ),
      ),
      tool("insertTable", Table2, t("contentEditor.insertTable"), (value, start) =>
        insertBlock(value, start, table),
      ),
    ],
  ];
}

/**
 * MarkdownToolbar is the Markdown mode's toolbar: each action wraps the
 * selection, prefixes the caret's line or inserts a block, through `onEdit`.
 * It is one tab stop, with arrow keys between its buttons.
 */
export function MarkdownToolbar({
  label,
  disabled = false,
  onEdit,
}: Readonly<{
  label: string;
  disabled?: boolean;
  onEdit: (action: MarkdownAction) => void;
}>) {
  const { t } = useTranslation();
  const groups = useTools();
  const roving = useRovingToolbar(
    disabled ? [] : groups.flatMap((group) => group.map((tool) => tool.key)),
  );
  return (
    <div
      role="toolbar"
      aria-label={t("contentEditor.toolbar", { label })}
      onKeyDown={roving.onKeyDown}
      className="flex min-w-0 flex-1 [scrollbar-width:thin] flex-nowrap items-center gap-0.5 overflow-x-auto"
    >
      {groups.map((group, index) => (
        <Fragment key={group[0]!.key}>
          {index > 0 && (
            <span
              aria-hidden="true"
              className="bg-border mx-1 h-[18px] w-px flex-none"
            />
          )}
          {group.map((tool) => (
            <ToolButton
              key={tool.key}
              data-roving={tool.key}
              tabIndex={roving.tabIndexFor(tool.key)}
              onFocus={() => roving.remember(tool.key)}
              icon={tool.icon}
              label={tool.label}
              title={tool.title}
              disabled={disabled}
              onClick={() => onEdit(tool.run)}
            />
          ))}
        </Fragment>
      ))}
    </div>
  );
}
