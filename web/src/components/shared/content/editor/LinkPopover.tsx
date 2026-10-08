import { lazy, Suspense, useRef, useState, type ComponentProps } from "react";
import type { Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import { Link } from "lucide-react";
import { ToolButton } from "./ToolButton";

const LinkPanel = lazy(() =>
  import("./LinkPanel").then((module) => ({ default: module.LinkPanel })),
);

/** LinkPopover is the toolbar's "Link" button; its popover loads on first use, so the editor's chunk does not carry it. */
export function LinkPopover({
  editor,
  active,
  ...button
}: Readonly<
  Omit<ComponentProps<"button">, "children" | "title" | "onClick"> & {
    editor: Editor;
    active: boolean;
  }
>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const anchor = useRef<HTMLButtonElement>(null);
  const label = t("contentEditor.link");
  return (
    <>
      <ToolButton
        ref={anchor}
        icon={Link}
        label={label}
        title={label}
        aria-pressed={active}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
        {...button}
      />
      {open && (
        <Suspense fallback={null}>
          <LinkPanel editor={editor} anchor={anchor} onClose={() => setOpen(false)} />
        </Suspense>
      )}
    </>
  );
}
