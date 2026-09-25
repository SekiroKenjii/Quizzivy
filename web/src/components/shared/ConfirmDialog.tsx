import { useRef, type ReactNode, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

/**
 * One dialog shape for every action that asks first; destructive confirms are
 * the app's only red (§12). Without `onConfirm` it is a notice: one button
 * that closes it, for a refusal that has nothing left to ask (A-06a, A-07).
 * Closing returns focus to `returnFocus`, or else to the element that had it
 * when the dialog opened, unless the confirmed action already moved focus
 * somewhere outside the dialog.
 */
export function ConfirmDialog({
  open,
  className,
  onOpenChange,
  title,
  description,
  children,
  confirmLabel,
  cancelLabel,
  destructive = false,
  disabled = false,
  pending = false,
  error = null,
  onConfirm,
  returnFocus,
}: Readonly<{
  open: boolean;
  className?: string;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  children?: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  destructive?: boolean;
  disabled?: boolean;
  pending?: boolean;
  error?: string | null;
  onConfirm?: () => void;
  returnFocus?: RefObject<HTMLElement | null>;
}>) {
  const { t } = useTranslation();
  const opener = useRef<HTMLElement | null>(null);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn("gap-4 p-5 sm:max-w-md", className)}
        onOpenAutoFocus={() => {
          const active = document.activeElement;
          opener.current =
            active instanceof HTMLElement && active !== document.body ? active : null;
        }}
        onCloseAutoFocus={(event) => {
          const target = returnFocus?.current ?? opener.current;
          opener.current = null;
          if (focusMovedAway(event.currentTarget) || !target?.isConnected) return;
          event.preventDefault();
          target.focus();
        }}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description === undefined ? null : (
            <DialogDescription>{description}</DialogDescription>
          )}
        </DialogHeader>
        {children}
        {error === null ? null : (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        )}
        <DialogFooter>
          {onConfirm === undefined ? (
            <Button className="w-full" onClick={() => onOpenChange(false)}>
              {confirmLabel}
            </Button>
          ) : (
            <>
              <Button
                variant="outline"
                className="flex-1"
                onClick={() => onOpenChange(false)}
              >
                {cancelLabel ?? t("common.cancel")}
              </Button>
              <Button
                variant={destructive ? "destructive" : "default"}
                className="flex-1"
                disabled={disabled || pending}
                onClick={onConfirm}
              >
                {confirmLabel}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function focusMovedAway(dialog: EventTarget | null): boolean {
  const active = document.activeElement;
  return (
    active instanceof HTMLElement &&
    active !== document.body &&
    !(dialog instanceof Node && dialog.contains(active))
  );
}
