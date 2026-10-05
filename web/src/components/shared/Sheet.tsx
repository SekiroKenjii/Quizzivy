import { useRef, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Dialog as Primitive } from "radix-ui";

import { useDeckScale } from "@/components/ui/deck-scale";
import { preventLocalEscapeDismissal } from "@/lib/localEscape";
import { cn } from "@/lib/utils";

type SheetWidth = 320 | 380 | 420;

const FRAMES: Record<
  SheetWidth,
  { panel: string; header: string; body: string; footer: string }
> = {
  320: {
    panel: "w-[min(320px,100%)]",
    header: "px-4.5 pt-4",
    body: "gap-4.5 px-4.5 pt-4.5 pb-4",
    footer: "px-4.5 py-3.5",
  },
  380: {
    panel: "w-[min(380px,100%)]",
    header: "border-b px-4 py-3.5",
    body: "p-3.5",
    footer: "px-4 py-3.5",
  },
  420: {
    panel: "w-[min(420px,100%)]",
    header: "border-b px-4.5 py-4",
    body: "gap-4.5 p-4.5",
    footer: "px-4.5 py-3.5",
  },
};

const MOTION =
  "duration-200 data-[state=closed]:animate-out data-[state=open]:animate-in motion-reduce:animate-none!";

/**
 * Sheet is the panel the Teacher deck pins to the right edge: a modal dialog
 * of the deck's 320, 380 or 420px, or the whole width when the window is
 * narrower. `title` names it and is drawn in the header with `subtitle` under
 * it, `leading` before it and the close button after it; the body is the only
 * part that scrolls, and `footer` stays under it. Esc, the backdrop and the
 * close button ask to close through `onOpenChange`. It is controlled, so it
 * remembers what opened it: on close, focus goes back to that control, or to
 * the `<main>` it was in when the control has gone, unless the page has
 * already put focus somewhere else. It sits at `--z-sheet`, under dialogs and
 * menus, and tells content inside it that it is on a deck surface when it was
 * opened from one.
 */
export function Sheet({
  open,
  onOpenChange,
  title,
  subtitle,
  leading,
  width = 420,
  footer,
  children,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  subtitle?: string | undefined;
  leading?: ReactNode;
  width?: SheetWidth | undefined;
  footer?: ReactNode;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  const deck = useDeckScale();
  const opener = useRef<HTMLElement | null>(null);
  const page = useRef<HTMLElement | null>(null);
  const frame = FRAMES[width];
  return (
    <Primitive.Root open={open} onOpenChange={onOpenChange}>
      <Primitive.Portal>
        <Primitive.Overlay
          data-slot="sheet-overlay"
          className={cn(
            "bg-overlay data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-(--z-sheet)",
            MOTION,
          )}
        />
        <Primitive.Content
          data-slot="sheet"
          data-scale={deck ? "deck" : undefined}
          className={cn(
            "bg-card text-fg shadow-float data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right fixed inset-y-0 right-0 z-(--z-sheet) flex flex-col border-l outline-none!",
            MOTION,
            frame.panel,
          )}
          onEscapeKeyDown={preventLocalEscapeDismissal}
          onOpenAutoFocus={() => {
            const active = document.activeElement;
            opener.current =
              active instanceof HTMLElement && active !== document.body ? active : null;
            page.current = opener.current?.closest("main") ?? null;
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            const target = opener.current?.isConnected ? opener.current : page.current;
            opener.current = null;
            page.current = null;
            const active = document.activeElement;
            if (active === null || active === document.body) target?.focus();
          }}
        >
          <div
            data-slot="sheet-header"
            className={cn("flex flex-none items-center gap-3", frame.header)}
          >
            {leading}
            <div className="min-w-0 flex-1">
              <Primitive.Title className="text-md font-semibold">
                {title}
              </Primitive.Title>
              {subtitle !== undefined && (
                <Primitive.Description className="text-muted-fg text-meta leading-normal">
                  {subtitle}
                </Primitive.Description>
              )}
            </div>
            <Primitive.Close
              aria-label={t("common.close")}
              className="hover:bg-hover rounded-seg grid size-8 flex-none place-items-center"
            >
              <X aria-hidden="true" className="size-4.25" />
            </Primitive.Close>
          </div>
          <div
            data-slot="sheet-body"
            className={cn("flex min-h-0 flex-1 flex-col overflow-y-auto", frame.body)}
          >
            {children}
          </div>
          {footer !== undefined && footer !== null && footer !== false && (
            <div
              data-slot="sheet-footer"
              className={cn("flex flex-none gap-2 border-t", frame.footer)}
            >
              {footer}
            </div>
          )}
        </Primitive.Content>
      </Primitive.Portal>
    </Primitive.Root>
  );
}
