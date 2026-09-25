import { useRef, type ReactNode } from "react";
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
 * Closing returns focus to the element that had it when the dialog opened,
 * when that element is still on the page.
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
}>) {
  const { t } = useTranslation();
  const opener = useRef<HTMLElement | null>(null);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn("gap-4 p-5 sm:max-w-md", className)}
        onOpenAutoFocus={() => {
          opener.current =
            document.activeElement instanceof HTMLElement
              ? document.activeElement
              : null;
        }}
        onCloseAutoFocus={(event) => {
          const target = opener.current;
          opener.current = null;
          if (target === null || !target.isConnected) return;
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
