import { useRef, type ComponentProps, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

const FRAME =
  "bg-card shadow-float flex max-h-[calc(100%-2rem)] w-[min(27.5rem,calc(100%-1.5rem))] max-w-none flex-col gap-3.5 overflow-y-auto rounded-2xl p-5.5 sm:max-w-none";

const TOP =
  "top-[12%] translate-y-0 min-[768px]:top-[50%] min-[768px]:translate-y-[-50%]";

/**
 * DeckDialog is the dialog the Student deck draws: 440px or the width less
 * 24px, a title with one sentence under it, no close button, and whatever the
 * caller puts beneath. It is centred; `placement="top"` sits it 12% from the
 * top below 768, for a dialog with a field, which the phone keyboard would
 * otherwise cover. It is controlled, so it remembers what opened it: on close,
 * focus goes back to that control, or to the `<main>` it was in when the
 * control has gone. Esc and the backdrop close it through `onOpenChange`.
 */
export function DeckDialog({
  open,
  onOpenChange,
  title,
  description,
  placement = "center",
  children,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: ReactNode;
  placement?: "center" | "top";
  children: ReactNode;
}>) {
  const opener = useRef<HTMLElement | null>(null);
  const page = useRef<HTMLElement | null>(null);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className={cn(FRAME, placement === "top" && TOP)}
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
          target?.focus();
        }}
      >
        <div>
          <DialogTitle className="text-lg leading-normal font-semibold">
            {title}
          </DialogTitle>
          <DialogDescription className="text-muted-fg mt-1 text-base leading-[1.55] text-pretty">
            {description}
          </DialogDescription>
        </div>
        {children}
      </DialogContent>
    </Dialog>
  );
}

/** DeckDialogActions is the dialog's row of buttons, at its right edge. */
export function DeckDialogActions({ children }: Readonly<{ children: ReactNode }>) {
  return <div className="flex flex-wrap justify-end gap-2">{children}</div>;
}

/**
 * DeckDialogCancel is the dialog's secondary button: 42px, flat, with no
 * shadow under it, as the deck draws it. It is a plain button, so inside a
 * form it does not submit.
 */
export function DeckDialogCancel({
  className,
  ...props
}: Readonly<ComponentProps<typeof Button>>) {
  return (
    <Button
      type="button"
      variant="outline"
      size="lg"
      className={cn("shadow-none in-data-[scale=deck]:font-medium", className)}
      {...props}
    />
  );
}
