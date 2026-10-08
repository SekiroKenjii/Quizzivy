import { useId, useRef, useState, type RefObject, type SubmitEvent } from "react";
import type { Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover";
import { safeContentURL } from "../validation";

function currentLink(editor: Editor): string | null {
  const href: unknown = editor.getAttributes("link").href;
  return editor.isActive("link") && typeof href === "string" ? href : null;
}

function applyLink(editor: Editor, href: string, inLink: boolean) {
  const chain = editor.chain().focus();
  if (inLink) chain.extendMarkRange("link").setLink({ href }).run();
  else if (editor.state.selection.empty)
    chain
      .insertContent({
        type: "text",
        text: href.replace(/^https:\/\//i, ""),
        marks: [{ type: "link", attrs: { href } }],
      })
      .run();
  else chain.setLink({ href }).run();
}

/**
 * LinkPanel is the open link popover, anchored to the toolbar's "Link"
 * button. Enter applies the address, an address `safeContentURL` refuses is
 * reported in an alert, and with no selection the address without its scheme
 * becomes the text. Esc returns focus to the button; applying or removing
 * returns it to the editor.
 */
export function LinkPanel({
  editor,
  anchor,
  onClose,
}: Readonly<{
  editor: Editor;
  anchor: RefObject<HTMLButtonElement | null>;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const id = useId();
  const [existing] = useState(() => currentLink(editor));
  const [draft, setDraft] = useState(existing ?? "https://");
  const [invalid, setInvalid] = useState(false);
  const done = useRef(false);

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const href = draft.trim();
    if (!safeContentURL(href)) {
      setInvalid(true);
      return;
    }
    done.current = true;
    onClose();
    applyLink(editor, href, existing !== null);
  }
  function remove() {
    done.current = true;
    onClose();
    editor.chain().focus().extendMarkRange("link").unsetLink().run();
  }

  return (
    <Popover
      modal
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <PopoverAnchor virtualRef={anchor} />
      <PopoverContent
        align="start"
        collisionPadding={8}
        aria-label={t("contentEditor.link")}
        className="w-68 max-w-[calc(100vw-24px)] rounded-[10px] p-1.5 data-[scale=deck]:rounded-[10px] data-[scale=deck]:p-1.5"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          if (done.current) editor.commands.focus(undefined, { scrollIntoView: false });
          else anchor.current?.focus();
        }}
      >
        <form noValidate onSubmit={submit} className="flex flex-col gap-2 p-1.5">
          <label className="flex flex-col gap-1.5 text-[12.5px] font-medium">
            {t("contentEditor.linkAddress")}
            <input
              type="text"
              inputMode="url"
              autoComplete="off"
              spellCheck={false}
              value={draft}
              placeholder={t("contentEditor.linkPlaceholder")}
              aria-invalid={invalid}
              aria-describedby={invalid ? `${id}-error` : undefined}
              onChange={(event) => {
                setDraft(event.target.value);
                setInvalid(false);
              }}
              className="bg-bg aria-invalid:border-danger h-[34px] rounded-[7px] border px-2.5 text-[13px] font-normal"
            />
          </label>
          {invalid && (
            <span
              id={`${id}-error`}
              role="alert"
              className="text-danger-ink -mt-1 text-xs"
            >
              {t("contentEditor.linkInvalid")}
            </span>
          )}
          <div className="flex justify-end gap-1.5">
            {existing !== null && (
              <button
                type="button"
                onClick={remove}
                className="text-danger-ink hover:bg-hover h-[30px] rounded-[7px] px-2.5 text-[12.5px] font-medium"
              >
                {t("contentEditor.linkRemove")}
              </button>
            )}
            <button
              type="submit"
              className="bg-primary text-primary-fg h-[30px] rounded-[7px] px-3 text-[12.5px] font-semibold"
            >
              {t("contentEditor.linkApply")}
            </button>
          </div>
        </form>
      </PopoverContent>
    </Popover>
  );
}
